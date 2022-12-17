package anim

// #cgo linux CFLAGS: -I/usr/include/opus
// #cgo linux LDFLAGS: -L/usr/lib/x86_64-linux-gnu -lopus
// #include <opus.h>
import "C"
import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"log"

	"github.com/pion/rtp"

	"github.com/zaf/resample"
)

const (
	audiochan = 1
	opusRate  = 48000
	voskRate  = 16000
)

var (
	ErrDecoding = errors.New("Error decoding opus")
)

func newConv(dest io.Writer) (c *conv) {
	c = &conv{
		dest: dest,
	}
	var err error
	if c.res, err = resample.New(c.dest, float64(opusRate), float64(voskRate), audiochan, resample.I16, resample.HighQ); err != nil {
		c.Println("resampler creating", err)
	}

	e := C.int(0)
	er := &e
	c.dec = C.opus_decoder_create(C.int(opusRate), C.int(audiochan), er)

	return
}

type conv struct {
	dest io.Writer
	dec  *C.OpusDecoder
	res  *resample.Resampler
	b    []byte
}

func (c *conv) Close() error {
	C.opus_decoder_destroy(c.dec)
	return nil
}

func (c *conv) AppendRTP(rtp *rtp.Packet) (err error) {
	return c.AppendOpusPayload(rtp.Payload)
}

func (c *conv) AppendOpusPayload(pl []byte) (err error) {
	samplesPerFrame := int(C.opus_packet_get_samples_per_frame((*C.uchar)(&pl[0]), C.int(48000)))
	pcm := make([]int16, samplesPerFrame)
	samples := C.opus_decode(c.dec, (*C.uchar)(&pl[0]), C.opus_int32(len(pl)), (*C.opus_int16)(&pcm[0]), C.int(cap(pcm)/audiochan), 0)
	if samples < 0 {
		err = ErrDecoding
		return
	}
	pcmData := make([]byte, 0)
	pcmBuffer := bytes.NewBuffer(pcmData)
	for _, v := range pcm {
		binary.Write(pcmBuffer, binary.LittleEndian, v)
	}
	err = c.appendBytes(pcmBuffer.Bytes())
	return
}

func (c *conv) appendBytes(b []byte) (err error) {
	if _, err = c.res.Write(b); err != nil {
		c.Println("resampling", err)
	}
	return
}

func (c *conv) Println(i ...interface{}) {
	log.Println("conv", i)
}
