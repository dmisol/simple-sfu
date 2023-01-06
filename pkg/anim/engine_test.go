package anim

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"path"
	"testing"
	"time"

	"github.com/dmisol/simple-sfu/pkg/defs"
	"github.com/pion/interceptor"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v3"
	"github.com/pion/webrtc/v3/pkg/media/h264writer"
	"github.com/pion/webrtc/v3/pkg/media/oggreader"
	"github.com/pion/webrtc/v3/pkg/media/oggwriter"
)

var (
	ap *AudioProc
	ae *AnimEngine
	dt = 20 * time.Millisecond
)

func TestAsClient(t *testing.T) {

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	/*
		cf := path.Join("testdata", "client.yaml")
			conf, err := defs.ReadConf(cf)
			if err != nil {
				t.Fatal(err)
			}
	*/
	f, err := ioutil.ReadFile(path.Join("testdata", "init.json"))
	if err != nil {
		t.Fatal(err)
	}
	ij := &defs.InitialJson{}
	if err = json.Unmarshal(f, ij); err != nil {
		t.Fatal(err)
	}
	ij.Dir = path.Join(defs.RamDisk, "testAsClient")

	os.MkdirAll(ij.Dir, os.ModeDir)

	ae, err = newAnimEngine(ctx, defs.Addr, onVideo, ij)
	if err != nil {
		t.Fatal(err)
	}

	ars := &audioRtpSender{tst: t}
	if err = ars.Init(path.Join("testdata", "audio.ogg")); err != nil {
		t.Fatal(err)
	}
	ap = NewAudioProc(ars, ae)

	<-ctx.Done()
}

func onVideo() {
	// todo: read both tracks and write as a file

	go func() {
		a, err := oggwriter.New("out.ogg", 48000, 2)
		if err != nil {
			log.Fatal(err)
		}
		defer a.Close()

		for {
			p, _, err := ap.ReadRTP()
			if err != nil {
				return
			}
			a.WriteRTP(p)
		}
	}()
	for {
		v, err := h264writer.New("out.h264")
		if err != nil {
			log.Fatal(err)
		}
		defer v.Close()

		p, _, err := ae.ReadRTP()
		if err != nil {
			return
		}
		v.WriteRTP(p)
	}

}

type audioRtpSender struct {
	ogg *oggreader.OggReader
	tst *testing.T
	t   time.Time
}

func (a *audioRtpSender) Init(fname string) (err error) {
	file, err := os.Open(fname)
	if err != nil {
		return
	}

	// Open on oggfile in non-checksum mode.
	a.ogg, _, err = oggreader.NewWith(file)
	if err != nil {
		return
	}
	a.t = time.Now()
	return
}

func (a *audioRtpSender) ReadRTP() (p *rtp.Packet, ic interceptor.Attributes, err error) {
	// todo: read ogg file and feed it as rtp packets

	pageData, _, err := a.ogg.ParseNextPage()
	if err != nil {
		return
	}

	req := a.t.Add(dt)
	now := time.Now()
	if now.After(req) {
		p = &rtp.Packet{Payload: pageData}
		return
	}

	time.Sleep(req.Sub(now))
	p = &rtp.Packet{Payload: pageData}
	return
}

func (a *audioRtpSender) Kind() webrtc.RTPCodecType {
	return webrtc.RTPCodecTypeAudio
}
