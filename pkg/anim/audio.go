package anim

import (
	"errors"
	"io"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dmisol/simple-sfu/pkg/defs"
	"github.com/pion/interceptor"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v3"
)

type AudioProc struct {
	mu sync.Mutex

	fifo []*rtp.Packet
	*conv
	enabled int64

	sinceLast int64
	stop      chan bool
}

func NewAudioProc(remote defs.TrackRTPReader /**webrtc.TrackRemote*/, anim io.Writer) (a *AudioProc) {
	a = &AudioProc{
		stop: make(chan bool),
		conv: newConv(anim),
	}
	go a.run(remote)
	return
}

// Enable() is called when the first video appeared, so audio should by sync'ed
func (a *AudioProc) Enable() {
	atomic.StoreInt64(&a.enabled, 1)
}

func (a *AudioProc) ReadRTP() (p *rtp.Packet, xx interceptor.Attributes, err error) {
	// todo: use sync.Cond
	for {
		if x := atomic.LoadInt64(&a.sinceLast); x > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	func() {
		a.mu.Lock()
		defer a.mu.Unlock()

		if len(a.fifo) == 0 {
			err = errors.New("reading empty fifo")
			return
		}

		p, a.fifo = a.fifo[0], a.fifo[1:]
	}()
	atomic.AddInt64(&a.sinceLast, -1)

	return
}

func (a *AudioProc) Close() (err error) {
	a.Println("closing")

	a.stop <- true
	a.conv.Close()

	return
}

func (a *AudioProc) run(remote defs.TrackRTPReader) { //(remote *webrtc.TrackRemote) {
	for {
		select {
		case <-a.stop:
			a.Println("killed(ctx)")
			return
		default:
			p, _, err := remote.ReadRTP()
			if err != nil {
				a.Println("rtp rd", err)
				return
			}
			func() {
				a.mu.Lock()
				a.mu.Unlock()
				a.fifo = append(a.fifo, p)
			}()
			atomic.AddInt64(&a.sinceLast, atomic.LoadInt64(&a.enabled))
			a.conv.AppendRTP(p)
		}
	}
}

func (a *AudioProc) Kind() (k webrtc.RTPCodecType) {
	return webrtc.RTPCodecTypeAudio
}

func (a *AudioProc) Println(i ...interface{}) {
	log.Println("audio", i)
}
