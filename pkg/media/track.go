package media

import (
	"errors"
	"log"
	"sync"
	"time"

	"github.com/dmisol/simple-sfu/pkg/defs"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v3"
)

func NewTrackReplicator(srcId int64) (tr *TrackReplicator) {
	tr = &TrackReplicator{
		id:     srcId,
		tracks: make(map[int64]*webrtc.TrackLocalStaticRTP),
		seqs:   make(map[int64]uint16),
	}
	return
}

type TrackReplicator struct {
	mu     sync.Mutex
	id     int64
	tracks map[int64]*webrtc.TrackLocalStaticRTP // [id] - neighbour users, to whom

	seqs map[int64]uint16 // need to keep individual to satisfy PLI
	ts   uint32           // to insert PLI properly
	bs   []*rtp.Packet
}

func (tr *TrackReplicator) RunAudio(r defs.TrackRTPReader, stop func()) {
	defer stop()

	kind := r.Kind().String()

	for {
		p, _, err := r.ReadRTP()
		if err != nil {
			tr.Println("track", kind, err)
			return
		}

		func() {
			tr.mu.Lock()
			defer tr.mu.Unlock()

			toDel := make([]int64, 0)
			for id, dest := range tr.tracks {
				if err := dest.WriteRTP(p); err != nil {
					tr.Println("writeRtp() failed", id, kind)
					toDel = append(toDel, id)
				}
			}

			for _, v := range toDel {
				delete(tr.tracks, v)
				delete(tr.seqs, v)
			}
		}()
	}
}

const safeVideoGap = 100 // 2880 between frames

func (tr *TrackReplicator) RunVideo(r defs.TrackRTPReader, stop func(), welcome func()) {
	defer stop()

	justStarted := true

	kind := r.Kind().String()
	var frame []*rtp.Packet
	keyFrame := false

	for {
		pkt, _, err := r.ReadRTP()
		if err != nil {
			tr.Println("track", kind, err)
			return
		}
		//tr.Println("wire:", pkt.Timestamp, pkt.SequenceNumber, IsH264Keyframe(pkt.Payload), len(pkt.Payload), pkt.Marker)
		frame = append(frame, pkt)
		if !keyFrame {
			keyFrame = IsH264Keyframe(pkt.Payload)
		}
		if !pkt.Marker {
			continue
		}

		//tr.printFrame("frame", frame)

		tr.mu.Lock()
		{
			if keyFrame {
				tr.bs = tr.bs[:0]
				tr.bs = append(tr.bs, frame...)

				//tr.printFrame("bootstrap", tr.bs)
				if justStarted {
					justStarted = false
					go welcome()
					tr.Println("first keyframe")
				}
			}
			tr.ts = pkt.Timestamp

			toDel := make([]int64, 0)
			for id, dest := range tr.tracks {
				seq := tr.seqs[id]
				for _, p := range frame {
					p.SequenceNumber = seq
					seq++

					//tr.Println("  ", p.Timestamp, p.SequenceNumber, IsH264Keyframe(p.Payload), len(p.Payload), p.Marker)

					if err := dest.WriteRTP(p); err != nil {
						tr.Println("writeRtp() failed", id, kind)
						toDel = append(toDel, id)
						break // to next track
					}
				}
				tr.seqs[id] = seq
			}

			for _, v := range toDel {
				delete(tr.tracks, v)
				delete(tr.seqs, v)
			}
		}
		tr.mu.Unlock()

		frame = frame[:0]
		keyFrame = false
	}
}

func (tr *TrackReplicator) Pli(id int64) {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	tr.pli(id)
}

// Pli() > LOCK > pli()
// Add() > LOCK > pli()
func (tr *TrackReplicator) pli(id int64) error {
	tr.ts += safeVideoGap

	s, ok := tr.seqs[id]
	if !ok {
		tr.Println("PLI from unknown user", id)
		return errors.New("request from unknown user")
	}
	tr.Println("ignorimg pli from", id)

	dest := tr.tracks[id]

	if len(tr.bs) > 0 {
		tr.Println("sending bootstrap to", id)

		for _, p := range tr.bs {
			p.SequenceNumber = s
			p.Timestamp = tr.ts
			s++

			// tr.Println(p.Timestamp, p.SequenceNumber, IsH264Keyframe(p.Payload), len(p.Payload), p.Marker)
			if err := dest.WriteRTP(p); err != nil {
				tr.Println("writeRtp() failed on Pli", id)
				delete(tr.seqs, id)
				delete(tr.tracks, id)
				return errors.New("destination failed to write")
			}
		}
	}

	tr.seqs[id] = s
	return nil
}

func (tr *TrackReplicator) Add(id int64, t *webrtc.TrackLocalStaticRTP) {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	s := uint16(time.Now().UnixNano())
	tr.seqs[id] = s
	tr.tracks[id] = t

	if len(tr.bs) > 0 {
		tr.pli(id)
	}

}

func (tr *TrackReplicator) printFrame(str string, f []*rtp.Packet) {
	tr.Println("  ", str)
	for _, p := range f {
		tr.Println(p.Timestamp, p.SequenceNumber, IsH264Keyframe(p.Payload), len(p.Payload), p.Marker)
	}
}

func (tr *TrackReplicator) Println(i ...interface{}) {
	log.Printf("MEDIA %d %v", tr.id, i)
}

// IsH264Keyframe detects if h264 payload is a keyframe
// this code was taken from https://github.com/jech/galene/blob/codecs/rtpconn/rtpreader.go#L45
// all credits belongs to Juliusz Chroboczek @jech and the awesome Galene SFU
func IsH264Keyframe(payload []byte) bool {
	if len(payload) < 1 {
		return false
	}
	nalu := payload[0] & 0x1F
	if nalu == 0 {
		// reserved
		return false
	} else if nalu <= 23 {
		// simple NALU
		return nalu == 5
	} else if nalu == 24 || nalu == 25 || nalu == 26 || nalu == 27 {
		// STAP-A, STAP-B, MTAP16 or MTAP24
		i := 1
		if nalu == 25 || nalu == 26 || nalu == 27 {
			// skip DON
			i += 2
		}
		for i < len(payload) {
			if i+2 > len(payload) {
				return false
			}
			length := uint16(payload[i])<<8 |
				uint16(payload[i+1])
			i += 2
			if i+int(length) > len(payload) {
				return false
			}
			offset := 0
			if nalu == 26 {
				offset = 3
			} else if nalu == 27 {
				offset = 4
			}
			if offset >= int(length) {
				return false
			}
			n := payload[i+offset] & 0x1F
			if n == 7 {
				return true
			} else if n >= 24 {
				// is this legal?
				log.Println("Non-simple NALU within a STAP")
			}
			i += int(length)
		}
		if i == len(payload) {
			return false
		}
		return false
	} else if nalu == 28 || nalu == 29 {
		// FU-A or FU-B
		if len(payload) < 2 {
			return false
		}
		if (payload[1] & 0x80) == 0 {
			// not a starting fragment
			return false
		}
		return payload[1]&0x1F == 7
	}
	return false
}
