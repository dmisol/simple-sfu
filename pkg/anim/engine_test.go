package anim

import (
	"context"
	"path"
	"testing"
	"time"

	"github.com/dmisol/simple-sfu/pkg/defs"
	"github.com/pion/interceptor"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v3"
)

var (
	ap *AudioProc
	ae *AnimEngine
)

func TestAsClient(t *testing.T) {

	ctx, _ := context.WithTimeout(context.Background(), 300*time.Second)

	cf := path.Join("testdata", "client.yaml")
	conf, err := defs.ReadConf(cf)
	if err != nil {
		t.Fatal(err)
	}
	ij := &defs.InitialJson{
		Dir:  path.Join(defs.RamDisk, "testAsClient"),
		Ftar: "todo...",
		W:    conf.W,
		H:    conf.H,
		FPS:  conf.FPS,
	}
	ae, err = newAnimEngine(ctx, defs.Addr, onVideo, ij)
	if err != nil {
		t.Fatal(err)
	}

	ars := &audioRtpSender{}

	ap = NewAudioProc(ars, ae)
	<-ctx.Done()
}

func onVideo() {
	// todo: read both tracks and write as a file
}

type audioRtpSender struct {
}

func (a *audioRtpSender) ReadRTP() (p *rtp.Packet, ic interceptor.Attributes, err error) {
	// todo: read ogg file and feed it as rtp packets
	return
}

func (a *audioRtpSender) Kind() webrtc.RTPCodecType {
	return webrtc.RTPCodecTypeAudio
}
