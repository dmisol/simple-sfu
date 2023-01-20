package anim

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"log"
	"net"
	"os"
	"sync"
	"sync/atomic"

	"github.com/dmisol/simple-sfu/pkg/defs"
	"github.com/gen2brain/x264-go"
	"github.com/pion/interceptor"
	"github.com/pion/rtp"
	"github.com/pion/rtp/codecs"
	"github.com/pion/webrtc/v3"
)

func newAnimEngine(ctx context.Context, addr string, f func(), ij *defs.InitialJson) (p *AnimEngine, err error) {

	// create structure
	p = &AnimEngine{dir: ij.Dir}
	p.Bridge = newBridge()
	opts := &x264.Options{
		Width:     ij.W,
		Height:    ij.H,
		FrameRate: ij.FPS,
		Tune:      "zerolatency",
		Preset:    "veryfast",
		Profile:   "baseline",
		LogLevel:  x264.LogDebug,
	}
	if p.enc, err = x264.NewEncoder(p.Bridge, opts); err != nil {
		return
	}

	// connect to port
	if p.conn, err = net.Dial("tcp", addr); err != nil {
		return
	}

	// send initial json
	var b []byte
	if b, err = json.Marshal(ij); err != nil {
		return
	}
	if _, err = p.conn.Write(b); err != nil {
		return
	}

	// start reading images
	go func() {
		defer p.conn.Close()

		for {
			select {
			case <-ctx.Done():
				p.Println("killex (ctx)")
				return
			default:
				b := make([]byte, 1024)
				i, err := p.conn.Read(b)
				if err != nil {
					p.Println("sock rd", err)
					return
				}
				name := string(b[:i])
				p.Println("h264 name", name)
				if err = p.procImage(name); err != nil {
					p.Println("h264 encoding", err)
					return
				}
			}
		}
	}()
	return
}

type AnimEngine struct {
	conn net.Conn
	dir  string

	index int64

	enc *x264.Encoder
	*Bridge
}

func (p *AnimEngine) procImage(name string) (err error) {
	var r *os.File
	if r, err = os.Open(name); err != nil {
		return
	}

	var img image.Image
	if img, _, err = image.Decode(r); err != nil {
		return
	}
	// conv data to h264 and Write() to *bridge
	err = p.enc.Encode(img)
	return
}

// Write() will be called when PCM portion is ready to be sent for animation computing
func (p *AnimEngine) Write(pcm []byte) (i int, err error) {
	// create file
	if err = os.MkdirAll(p.dir, 0777); err != nil {
		p.Println("mkdirall", err)
		return
	}
	name := fmt.Sprintf("%s/%d.pcm", p.dir, atomic.AddInt64(&p.index, 1))
	p.Println("wrining", name, len(pcm))
	if err = os.WriteFile(name, pcm, 0666); err != nil {
		p.Println("wr", err)
		return
	}
	i = len(pcm)

	// send name to socket
	p.Println("sending")
	if _, err = p.conn.Write([]byte(name)); err != nil {
		p.Println("snd", err)
	}

	return
}

func (p *AnimEngine) Close() (err error) {
	p.enc.Close()
	return
}

func (p *AnimEngine) Println(i ...interface{}) {
	log.Println("anim", i)
}

func newBridge() (b *Bridge) {
	b = &Bridge{}

	payloader := &codecs.H264Payloader{}
	b.seq = rtp.NewRandomSequencer()
	b.pack = rtp.NewPacketizer(defs.MTU, defs.PtVideo, 0, payloader, b.seq, defs.ClkVideo)
	return
}

// converts Writer to ReadCloser
// x264enc -> bridge -> relay
type Bridge struct {
	// todo: convert to RFC 6184 ?
	mu   sync.Mutex
	data [][]byte

	remained []byte

	// todo..
	pack rtp.Packetizer
	seq  rtp.Sequencer
}

func (b *Bridge) Write(p []byte) (i int, err error) {
	i = len(p)

	b.mu.Lock()
	defer b.mu.Unlock()

	b.data = append(b.data, p)
	return
}

func (b *Bridge) Read(p []byte) (i int, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.remained) > 0 {
		i = copy(p, b.remained)
		b.remained = b.remained[:i]
		return
	}

	if len(b.data) == 0 {
		return
	}

	b.remained = b.data[0]
	b.data = b.data[1:]

	i = copy(p, b.remained)
	b.remained = b.remained[:i]
	return
}

func (b *Bridge) ReadRTP() (p *rtp.Packet, _ interceptor.Attributes, err error) {
	// todo!

	return
}

func (b *Bridge) Kind() (t webrtc.RTPCodecType) {
	return webrtc.RTPCodecTypeVideo
}

func (b *Bridge) Close() (err error) { return }
