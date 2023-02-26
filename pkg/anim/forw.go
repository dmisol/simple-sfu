package anim

import (
	"log"
	"sync"

	"github.com/dmisol/simple-sfu/pkg/defs"
	"github.com/dmisol/simple-sfu/pkg/media"
	"github.com/pion/webrtc/v3"
)

func NewAnimator(uc *defs.UserCtx, welcome func(), stop func(), id int64, ij *defs.InitialJson) (anim *MediaAnimator) {
	anim = &MediaAnimator{
		welcome: welcome,
		stop:    stop,
	}
	anim.Println("starting")
	defer anim.Println("running")

	v, err := newAnimEngine(uc, defs.Addr, anim.onEncodedVideo, stop, ij)
	if err != nil {
		anim.Println("anim engine", err)

		anim.stop()
		anim = nil
		return
	}
	anim.ae = v
	return
}

type MediaAnimator struct {
	mu   sync.Mutex
	a, v *media.TrackReplicator //[kind]

	ap *AudioProc
	ae *AnimEngine

	welcome func()
	stop    func()
}

func (anim *MediaAnimator) Replicate(t *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
	if t.Kind() == webrtc.RTPCodecTypeAudio {
		anim.mu.Lock()
		defer anim.mu.Unlock()

		// this will:
		// 1. store opus packets for re-tx-ing
		// 2. feed encoded audio to a server
		anim.ap = NewAudioProc(t, nil /*anim.ae*/, &anim.ae.CanProcess)
		anim.Println("audio processing")
	}
}

func (anim *MediaAnimator) Add(id int64, t *webrtc.TrackLocalStaticRTP) {
	anim.Println("adding track", id, t.Kind().String())
	anim.mu.Lock()
	defer anim.mu.Unlock()

	if t.Kind() == webrtc.RTPCodecTypeAudio {
		if anim.a != nil {
			anim.a.Add(id, t)
			return
		}
	} else {
		if anim.v != nil {
			anim.v.Add(id, t)
			return
		}
	}
	anim.Println("can't add track of given kind", t, t.Kind().String())
}

func (anim *MediaAnimator) onEncodedVideo() {
	anim.Println("encoded video appeared")

	anim.mu.Lock()
	defer anim.mu.Unlock()

	tr := media.NewTrackReplicator()
	go tr.Run(anim.ap, anim.stop)
	anim.a = tr

	tr = media.NewTrackReplicator()
	go tr.Run(anim.ae, anim.stop)
	anim.v = tr

	anim.ap.Play()
	anim.Println("replicators started")

	anim.welcome()
}

func (anim *MediaAnimator) Println(i ...interface{}) {
	log.Println("MA", i)
}
