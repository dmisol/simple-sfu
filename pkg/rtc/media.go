package rtc

import (
	"log"
	"sync"

	"github.com/pion/webrtc/v3"
)

type replicator struct {
	mu   sync.Mutex
	a, v *trackReplicator //[kind]

	welcome func()
	stop    func()
}

func (r *replicator) Replicate(t *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
	tr := &trackReplicator{}

	r.mu.Lock()
	defer r.mu.Unlock()

	if t.Kind() == webrtc.RTPCodecTypeAudio {
		r.a = tr
	} else {
		r.v = tr
	}
	go tr.run(t, r.stop)
	if r.a != nil && r.v != nil {
		go r.welcome() // both src tracks ready, ivite to add localTracks
	}
}

func (r *replicator) Add(id int64, t *webrtc.TrackLocalStaticRTP) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if t.Kind() == webrtc.RTPCodecTypeAudio {
		if r.a != nil {
			r.a.Add(id, t)
			return
		}
	} else {
		if r.v != nil {
			r.v.Add(id, t)
			return
		}
	}
	log.Println("can't add track of given kind", t, t.Kind().String())
}

type trackReplicator struct {
	mu     sync.Mutex
	tracks map[int64]*webrtc.TrackLocalStaticRTP // [id] - neighbour users, to whom
}

func (tr *trackReplicator) run(r *webrtc.TrackRemote, stop func()) {
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

func (tr *trackReplicator) Add(id int64, t *webrtc.TrackLocalStaticRTP) {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	tr.tracks[id] = t
}
