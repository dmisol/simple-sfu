package media

import (
	"log"
	"sync"

	"github.com/pion/webrtc/v3"
)

func NewCloner(srcId int64, welcome func(), stop func()) (mr *MediaCloner) {
	mr = &MediaCloner{
		id:      srcId,
		welcome: welcome,
		stop:    stop,
	}
	return
}

type MediaCloner struct {
	id   int64
	mu   sync.Mutex
	a, v *TrackReplicator //[kind]

	welcome func()
	stop    func()
}

func (r *MediaCloner) Replicate(t *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
	tr := NewTrackReplicator(r.id)

	r.mu.Lock()
	defer r.mu.Unlock()

	if t.Kind() == webrtc.RTPCodecTypeAudio {
		r.a = tr
		go tr.RunAudio(t, r.stop)
	} else {
		r.v = tr
		go tr.RunVideo(t, r.stop, r.welcome)
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
func (r *MediaCloner) Pli(id int64) {
	r.v.Pli(id)
}
