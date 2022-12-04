package media

import (
	"log"
	"sync"

	"github.com/pion/webrtc/v3"
)

func NewAnimator(welcome func(), stop func()) (mr *MediaAnimator) {
	mr = &MediaAnimator{
		welcome: welcome,
		stop:    stop,
	}
	return
}

type MediaAnimator struct {
	mu   sync.Mutex
	a, v *TrackReplicator //[kind]

	welcome func()
	stop    func()
}

func (r *MediaAnimator) Replicate(t *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
	/*
		tr := &TrackReplicator{
			tracks: make(map[int64]*webrtc.TrackLocalStaticRTP),
		}

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
	*/
}

func (r *MediaAnimator) Add(id int64, t *webrtc.TrackLocalStaticRTP) {
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
