package media

import (
	"log"
	"sync"

	"github.com/pion/webrtc/v3"
)

func NewCloner(welcome func(), stop func()) (mr *MediaCloner) {
	mr = &MediaCloner{
		welcome: welcome,
		stop:    stop,
	}
	return
}

type MediaCloner struct {
	mu   sync.Mutex
	a, v *TrackReplicator //[kind]

	welcome func()
	stop    func()
}

func (r *MediaCloner) Replicate(t *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
	tr := NewTrackTeplicator()

	r.mu.Lock()
	defer r.mu.Unlock()

	if t.Kind() == webrtc.RTPCodecTypeAudio {
		r.a = tr
	} else {
		r.v = tr
	}
	go tr.Run(t, r.stop)
	if r.a != nil && r.v != nil {
		go r.welcome() // both src tracks ready, ivite to add localTracks
	}
}

func (r *MediaCloner) Add(id int64, t *webrtc.TrackLocalStaticRTP) {
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
