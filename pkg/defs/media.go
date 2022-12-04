package defs

import (
	"github.com/pion/interceptor"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v3"
)

type Media interface {
	Replicate(*webrtc.TrackRemote, *webrtc.RTPReceiver) // track to replicate
	Add(int64, *webrtc.TrackLocalStaticRTP)             // user to receive
}

type TrackLocalRTP interface {
	Kind() webrtc.RTPCodecType
	ReadRTP() (*rtp.Packet, interceptor.Attributes, error)
}
