package defs

import (
	"github.com/pion/interceptor"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v3"
)

const (
	MTU      = 1200
	PtVideo  = 96
	PtAudio  = 111
	ClkVideo = 90000
	ClkAudio = 48000
)

type Media interface {
	Replicate(*webrtc.TrackRemote, *webrtc.RTPReceiver) // track to replicate
	Add(int64, *webrtc.TrackLocalStaticRTP)             // user to receive
}

type TrackRTPReader interface {
	Kind() webrtc.RTPCodecType
	ReadRTP() (*rtp.Packet, interceptor.Attributes, error)
}
