package anim

import (
	"errors"
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

	fifo []*defs.RtpStorage //*rtp.Packet
	*conv
	playing    int64
	canprocess *int32

	sinceLast int64
	stop      chan bool
}

func NewAudioProc(remote defs.TrackRTPReader /**webrtc.TrackRemote*/, anim defs.TsWriter, canprocess *int32) (a *AudioProc) {
	a = &AudioProc{
		stop:       make(chan bool),
		conv:       newConv(anim),
		canprocess: canprocess,
	}
	go a.run(remote)
	return
}

// Play() is called when the first video appeared, so audio should by sync'ed
func (a *AudioProc) Play() {
	atomic.StoreInt64(&a.playing, 1)
}

func (a *AudioProc) ReadRTP() (p *rtp.Packet, xx interceptor.Attributes, err error) {
	//log.Println("ReadtRTP(delayed audio)")
	//defer log.Println("ReadtRTP(delayed audio) done")

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

		var ps *defs.RtpStorage
		ps, a.fifo = a.fifo[0], a.fifo[1:]

		// ToDo: compute delay
		p = ps.Packet
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

			if atomic.LoadInt32(a.canprocess) == 1 {
				ts := time.Now().UnixMilli()
				func(ts int64) {
					// TODO: in FIFO replace rtp with {rtp,ts}
					a.mu.Lock()
					defer a.mu.Unlock()
					ps := &defs.RtpStorage{
						Ts:     ts,
						Packet: p,
					}
					a.fifo = append(a.fifo, ps)
				}(ts)
				atomic.AddInt64(&a.sinceLast, atomic.LoadInt64(&a.playing))
				a.conv.AppendRTP(p, ts)
			}

		}
	}
}

func (a *AudioProc) Kind() (k webrtc.RTPCodecType) {
	return webrtc.RTPCodecTypeAudio
}

func (a *AudioProc) Println(i ...interface{}) {
	log.Println("audio", i)
}
