package media

import (
	"log"
	"sync"

	"github.com/dmisol/simple-sfu/pkg/defs"
	"github.com/pion/webrtc/v3"
)

func NewTrackTeplicator() (tr *TrackReplicator) {
	tr = &TrackReplicator{
		tracks: make(map[int64]*webrtc.TrackLocalStaticRTP),
	}
	return
}

type TrackReplicator struct {
	mu     sync.Mutex
	tracks map[int64]*webrtc.TrackLocalStaticRTP // [id] - neighbour users, to whom
}

func (tr *TrackReplicator) Run(r defs.TrackRTPReader, stop func()) {
	defer stop()

	kind := r.Kind().String()

	for {
		p, _, err := r.ReadRTP()
		if err != nil {
			log.Println("track", kind, err)
			return
		}
		func() {
			tr.mu.Lock()
			defer tr.mu.Unlock()

			toDel := make([]int64, 0)
			for id, dest := range tr.tracks {
				if err := dest.WriteRTP(p); err != nil {
					log.Println("writeRtp() failed", id, kind)
					toDel = append(toDel, id)
				}
			}

			for _, v := range toDel {
				delete(tr.tracks, v)
			}
		}()
	}
}

func (tr *TrackReplicator) Add(id int64, t *webrtc.TrackLocalStaticRTP) {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	tr.tracks[id] = t
}
