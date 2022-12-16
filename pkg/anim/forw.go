package anim

import (
	"context"
	"log"
	"sync"

	"github.com/dmisol/simple-sfu/pkg/defs"
	"github.com/dmisol/simple-sfu/pkg/media"
	"github.com/pion/webrtc/v3"
)

func NewAnimator(welcome func(), stop func(), id int64, ij *defs.InitialJson) (anim *MediaAnimator) {
	anim = &MediaAnimator{
		welcome: welcome,
		stop:    stop,
	}
	v, err := newAnimEngine(context.Background(), defs.Addr, anim.onEncodedVideo, ij)
	if v != nil {
		anim.Println("anim engine", err)
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
		anim.ap = NewAudioProc(t, anim.ae)
		anim.Println("audio processing")
	}
}

func (anim *MediaAnimator) Add(id int64, t *webrtc.TrackLocalStaticRTP) {}

func (anim *MediaAnimator) onEncodedVideo() {
	anim.Println("encoded video appeared")

	anim.mu.Lock()
	defer anim.mu.Unlock()

	tr := media.NewTrackTeplicator()
	tr.Run(anim.ap, anim.stop)
	anim.a = tr

	tr = media.NewTrackTeplicator()
	tr.Run(anim.ae, anim.stop)
	anim.v = tr

	anim.welcome()
}

func (anim *MediaAnimator) Println(i ...interface{}) {
	log.Println("MA", i)
}
