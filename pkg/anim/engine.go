package anim

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	"log"
	"math/rand"
	"net"
	"os"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/dmisol/simple-sfu/pkg/defs"
	"github.com/pion/interceptor"
	"github.com/pion/mediadevices"
	"github.com/pion/mediadevices/pkg/codec/x264"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v3"
)

const (
	bridgePack          = true
	maxReadRtpAttempts  = 500 // *10ms = 5 sec
	maxFifoVideoPackets = 500 // several seconds, depending on the daya being tx'd _
	imgsInChan          = 50
)

// TODO: see pion/mediadevices/NewVideoTrack

func newAnimEngine(ctx context.Context, addr string, f func(), ij *defs.InitialJson) (p *AnimEngine, err error) {

	// create structure
	p = &AnimEngine{dir: ij.Dir}
	p.Bridge = newBridge()
	p.Println("bridge ok")

	// connect to port
	if p.conn, err = net.Dial("tcp", addr); err != nil {
		p.Println("dial", err)
		return
	}
	p.Println("socket connected")

	// send initial json
	var b []byte
	if b, err = json.Marshal(ij); err != nil {
		return
	}
	if _, err = p.conn.Write(b); err != nil {
		return
	}

	// start reading images
	p.Println("start reading images")
	go func() {
		defer p.conn.Close()

		cntr := uint64(0)
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
				//log.Println("raw:", string(b[:i]))
				names := strings.Split(string(b[:i]), "\n")
				//log.Println("names:", names)
				for _, name := range names {
					if len(name) < 4 {
						break
					}
					log.Println("name:", name, "len:", len(name))
					if err = p.procImage(name); err != nil {
						p.Println("h264 encoding", name, err)
						return
					}
					if cntr == 0 {
						go f()
					}
					cntr++
				}
				/*
					name := string(b[:i])
					name = strings.TrimSuffix(name, "\n")
					if err = p.procImage(name); err != nil {
						p.Println("h264 encoding", name, err)
						return
					}
					if cntr == 0 {
						go f()
					}
					cntr++
				*/
			}
		}
	}()
	return
}

type AnimEngine struct {
	conn net.Conn
	dir  string

	index int64

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
	p.Bridge.Imgs <- img
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
	p.Println("writing", name, len(pcm))
	if err = os.WriteFile(name, pcm, 0666); err != nil {
		p.Println("wr", err)
		return
	}
	i = len(pcm)

	// send name to socket

	w := bufio.NewWriter(p.conn)
	if _, err = w.WriteString(name + "\n"); err != nil {
		return
	}
	w.Flush()

	return
}

func (p *AnimEngine) Close() (err error) {
	return
}

func (p *AnimEngine) Println(i ...interface{}) {
	log.Println("anim", i)
}

func newBridge() (b *Bridge) {
	imgs := make(chan image.Image, 50)
	b = &Bridge{
		vs:   &videoSource{imgs: imgs},
		Imgs: imgs,
	}
	b.Println("starting")
	defer b.Println("started")


	x264Params, err := x264.NewParams()
	if err != nil {
		log.Println("x264Params", err)
	}
	x264Params.Preset = x264.PresetMedium
	x264Params.BitRate = 1_000_000 // 1mbps
	b.Println("x264Params")


	codecSelector := mediadevices.NewCodecSelector(
		mediadevices.WithVideoEncoders(&x264Params),
	)
	b.Println("codecSelector")

	vt := mediadevices.NewVideoTrack(b.vs, codecSelector)
	b.Println("videoTrack")

	rr, err := vt.NewRTPReader(x264Params.RTPCodec().MimeType, rand.Uint32(), 1000)
	if err != nil {
		b.Println("NerwRtpReader", err)
		return
	}
	b.rr = rr
	return
}

// converts Writer to ReadCloser
// x264enc -> bridge -> relay
type Bridge struct {
	// todo: convert to RFC 6184 ?
	mu   sync.Mutex
	pkts []*rtp.Packet

	Imgs chan image.Image
	vs   mediadevices.VideoSource
	rr   mediadevices.RTPReadCloser
}

func (b *Bridge) ReadRTP() (p *rtp.Packet, _ interceptor.Attributes, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.pkts) > 0 {
		p, b.pkts = b.pkts[0], b.pkts[1:]
		return
	}
	for {
		pkts, _, e := b.rr.Read()
		err = e
		if err != nil {
			log.Println("mediadevices.Read()", err)
			return
		}
		if len(pkts) > 0 {
			p, b.pkts = pkts[0], pkts[1:]
			return
		}
	}

}

func (b *Bridge) Kind() (t webrtc.RTPCodecType) {
	return webrtc.RTPCodecTypeVideo
}

func (b *Bridge) Close() (err error) { return }

func (b *Bridge) Println(i ...interface{}) {
	log.Println("br", i)
}

type videoSource struct {
	imgs chan image.Image
}

func (vs *videoSource) Close() (err error) { return }

func (vs *videoSource) ID() (id string) { return }

func (vs *videoSource) Read() (img image.Image, release func(), err error) {
	release = func() {}
	img = <-vs.imgs
	return
}
