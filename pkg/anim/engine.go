package anim

import (
	"bufio"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	"log"
	"math/rand"
	"net"
	"os"
	"path"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/dmisol/simple-sfu/pkg/defs"
	"github.com/google/uuid"
	"github.com/pion/interceptor"
	"github.com/pion/mediadevices"
	"github.com/pion/mediadevices/pkg/codec/x264"
	_ "github.com/pion/mediadevices/pkg/driver/camera"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v3"
)

const (
	bridgePack          = true
	maxReadRtpAttempts  = 500 // *10ms = 5 sec
	maxFifoVideoPackets = 500 // several seconds, depending on the daya being tx'd _
	imgsInChan          = 50

	audioTest = false
)

// TODO: see pion/mediadevices/NewVideoTrack

func newAnimEngine(uc *defs.UserCtx, addr string, f func(), stop func(), ij *defs.InitialJson) (p *AnimEngine, err error) {

	// create structure
	p = &AnimEngine{
		dir: ij.Dir,
		uc:  uc,
	}
	p.Bridge = newBridge()
	p.Println("bridge ok")

	if !audioTest {
		// connect to port
		if p.conn, err = net.Dial("unix", addr); err != nil {
			p.Println("dial", err)
			return
		}
		p.Println("socket connected")

		// send initial json
		var b []byte
		if b, err = json.Marshal(ij); err != nil {
			return
		}
		p.Println("sending", string(b))
		if _, err = p.conn.Write(b); err != nil {
			return
		}
		p.Println("sent", string(b))

		// start reading images
		p.Println("start reading images")
		go func() {
			defer func() {
				p.conn.Close()
				uc.Close("stop reading images")
				stop()
			}()

			cntr := uint64(0)
			for {
				select {
				case <-uc.Done():
					p.Println("killed (ctx)")
					return
				default:
					b := make([]byte, 1024)
					i, err := p.conn.Read(b)
					if err != nil {
						p.Println("sock rd", err)
						return
					}

					log.Println("raw:", string(b))
					jsons := strings.Split(string(b[:i]), "\n")

					log.Println("jsons:", jsons)
					for _, js := range jsons {
						if len(js) < 4 {
							break
						}

						ap := &defs.AminPacket{}
						if err = json.Unmarshal([]byte(js), ap); err != nil {
							p.Println("unmarshal socket msg", err)
							return
						}

						switch ap.Type {
						case defs.TypeFile:
							name := ap.Payload
							p.Println("name:", name)
							if err = p.procImage(name); err != nil {
								p.Println("h264 encoding", name, err)
								return
							}
							if cntr == 0 {
								go f()
							}
							cntr++
						case defs.TypeMsg:
							if ap.Payload == defs.AnimPayloadReady {
								// trigger audio
								p.Println("READY msg, start processing audio")
								atomic.StoreInt32(&p.CanProcess, 1)
								continue
							} else {
								// TODO: log separately
								p.Println("ANIM ERR", ap.Payload)
								continue
							}
						default:
							p.Println("err unexpected type", js)
						}

					}
				}
			}
		}()

	}

	return
}

type AnimEngine struct {
	conn net.Conn
	dir  string
	uc   *defs.UserCtx

	index        int64
	rxseq, txseq int64
	CanProcess   int32

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
func (p *AnimEngine) Write(pcm []byte, ts int64) (i int, err error) {
	// create file
	if err = os.MkdirAll(p.dir, 0777); err != nil {
		p.Println("mkdirall", err)
		return
	}

	//ts := time.Now().UnixMilli()

	if !audioTest {
		name := fmt.Sprintf("%s/%08d.pcm", p.dir, atomic.AddInt64(&p.index, 1))
		p.Println("writing", name, len(pcm))
		if err = os.WriteFile(name, pcm, 0666); err != nil {
			p.Println("wr", err)
			return
		}
		i = len(pcm)

		// send name to socket
		w := bufio.NewWriter(p.conn)
		ap := &defs.AminPacket{
			Ts:      ts,
			Seq:     atomic.AddInt64(&p.txseq, 1),
			Type:    defs.TypeFile,
			Payload: name,
		}
		var b []byte
		if b, err = json.Marshal(ap); err != nil {
			p.Println("animpacket nmarshal", err)
			return
		}
		if _, err = w.WriteString(string(b) + "\n"); err != nil {
			return
		}
		w.Flush()
	} else {
		name := path.Join(p.dir, "audio.pcm")
		f, fe := os.OpenFile(name, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
		if fe != nil {
			err = fe
			p.Println("wr", err)
			return
		}
		defer f.Close()
		_, err = f.Write(pcm)
	}

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

	var err error
	b.x264Params, err = x264.NewParams()
	if err != nil {
		log.Println("x264Params", err)
	}
	b.x264Params.Preset = x264.PresetMedium
	b.x264Params.BitRate = 1_000_000 // 1mbps
	b.Println("x264Params")

	codecSelector := mediadevices.NewCodecSelector(
		mediadevices.WithVideoEncoders(&b.x264Params),
	)
	b.Println("codecSelector")

	b.vt = mediadevices.NewVideoTrack(b.vs, codecSelector)
	b.Println("videoTrack")

	return
}

// converts Writer to ReadCloser
// x264enc -> bridge -> relay
type Bridge struct {
	// todo: convert to RFC 6184 ?
	mu   sync.Mutex
	pkts []*rtp.Packet

	Imgs chan image.Image

	x264Params x264.Params
	vs         mediadevices.VideoSource
	vt         mediadevices.Track
	rr         mediadevices.RTPReadCloser
	started    bool
}

func (b *Bridge) ReadRTP() (p *rtp.Packet, _ interceptor.Attributes, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.pkts) > 0 {
		p, b.pkts = b.pkts[0], b.pkts[1:]
		return
	}
	if !b.started {
		b.Println("starting mediadevices.RTPReadCloser")

		b.rr, err = b.vt.NewRTPReader(b.x264Params.RTPCodec().MimeType, rand.Uint32(), 1000)
		if err != nil {
			b.Println("NewRtpReader", err)
			return
		}
		b.started = true

		b.Println("mediadevices.RTPReadCloser ok")
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

func (vs *videoSource) Close() (err error) {
	vs.Println("close")
	return
}

func (vs *videoSource) ID() (id string) {
	vs.Println("id")
	id = uuid.NewString()
	return
}

func (vs *videoSource) Read() (img image.Image, release func(), err error) {
	vs.Println("reading")
	defer vs.Println("reading done")

	//release = func() {}
	release()

	img = <-vs.imgs
	return
}

func (vs *videoSource) Println(i ...interface{}) {
	log.Println("vs", i)
}
