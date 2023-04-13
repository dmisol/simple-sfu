package rtc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dmisol/simple-sfu/pkg/anim"
	"github.com/dmisol/simple-sfu/pkg/defs"
	"github.com/dmisol/simple-sfu/pkg/media"
	"github.com/fasthttp/websocket"
	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v3"
)

const (
	timeout = 2 * time.Hour
)

func NewUser(ctx context.Context, api *webrtc.API, id int64, inviteOthers func(int64), subscribeTo func(p int64, s int64, t *webrtc.TrackLocalStaticRTP), stop func(int64), pli func(p int64, sub int64), ij *defs.InitialJson) (u *User) {
	uc := &defs.UserCtx{Id: id}
	uc.Context, uc.CancelFunc = context.WithCancel(ctx)

	u = &User{
		UserCtx:      uc,
		inviteOthers: inviteOthers, // to invite others to subscribe
		subscribeTo:  subscribeTo,
		stop:         stop,
		pli:          pli,
		wsChan:       make(chan []byte, 5), // to invite the given user to subscribe publisher[id]
		api:          api,
		initJson:     ij,
	}
	u.UserCtx.Close = u.Close

	return
}

func (u *User) Close(msg ...interface{}) {
	defer u.CancelFunc()
	u.Println("close", msg)
	u.conn.Close()
}

type User struct {
	mu sync.Mutex
	*defs.UserCtx

	conn *websocket.Conn

	inviteOthers func(int64)
	subscribeTo  func(p int64, s int64, t *webrtc.TrackLocalStaticRTP)
	stop         func(int64)
	pli          func(p int64, s int64)

	api    *webrtc.API
	wsChan chan []byte
	media  defs.Media

	publisher int32
	initJson  *defs.InitialJson

	pc []*webrtc.PeerConnection
}

func (u *User) Publisher() bool {
	return (atomic.LoadInt32(&u.publisher) > 0)
}
func (u *User) Invite(id int64) {
	pl := &defs.WsPload{
		Action: defs.ActInvite,
		Id:     id,
	}
	b, err := json.Marshal(pl)
	if err != nil {
		u.Println("(inv) can't marshal ws payload", pl)
		return
	}
	go func() { // to avoid blocking
		u.wsChan <- b
	}()
}

func (u *User) Add(id int64, t *webrtc.TrackLocalStaticRTP) {
	u.Println("connecting subscribers", id, t.Kind().String())

	u.mu.Lock()
	defer u.mu.Unlock()

	if u.media == nil {
		u.Println("can't add: replicator not started")
		return
	}
	go u.media.Add(id, t)
}

func (u *User) Del(id int64) {
	pl := &defs.WsPload{
		Action: defs.ActDelete,
		Id:     id,
	}
	b, err := json.Marshal(pl)
	if err != nil {
		u.Println("(del) can't marshal ws payload", pl)
		return
	}
	go func() { // to avoid blocking
		u.wsChan <- b
	}()
}

func (u *User) Pli(from int64) {
	u.media.Pli(from)
}

func (u *User) Handler(conn *websocket.Conn) {
	defer u.stop(u.Id)

	u.conn = conn
	defer func() {
		u.Close("Handler()")
	}()

	go u.wrHandler()

	for {
		_, msg, err := u.conn.ReadMessage()
		if err != nil {
			u.Println("ws read", err)
			return
		}
		//u.Println("mt=", mt, "msg=", string(msg))

		if err = u.process(msg); err != nil {
			u.Println("ws data", err)
			return
		}
	}
}

func (u *User) wrHandler() {
	defer func() {
		u.Close("wrHandler()")
	}()

	for {
		b := <-u.wsChan
		if err := u.conn.WriteMessage(websocket.TextMessage, b); err != nil {
			u.Println("ws write", err)
			return
		}
	}
}

func (u *User) process(p []byte) (err error) {
	var r map[string]interface{}

	if err = json.Unmarshal(p, &r); err != nil {
		log.Println("process, unmarshal", err)
		return
	}
	if r["action"] == nil {
		err = errors.New("json incomplete, action")
		return
	}
	action := r["action"].(string)

	if r["data"] == nil {
		err = errors.New("json incomplete, data " + action)
		return
	}
	data, err := json.Marshal(r["data"])
	if err != nil {
		log.Println("data marshal", err)
		return
	}
	switch action {
	case defs.ActPublish:
		go u.negotiatePublisher(data)
	case defs.ActSubscribe:
		if r["id"] == nil {
			err = errors.New("json incomplete, id " + action)
			return
		}
		id := int64(r["id"].(float64))

		u.Println(id)
		go u.negotiateSubscriber(id, data)
	default:
		err = errors.New(fmt.Sprint("unexpected ws cmd", string(p)))
	}
	return
}

func (u *User) Println(i ...interface{}) {
	log.Println("USER", u.Id, i)
}

func (u *User) negotiatePublisher(data []byte) {
	var offer webrtc.SessionDescription
	var err error
	if err := json.Unmarshal(data, &offer); err != nil {
		u.Println("pub offer Unmarshal()", err)
		return
	}
	//u.Println("pub offer", offer.SDP)

	pc, err := u.api.NewPeerConnection(webrtc.Configuration{SDPSemantics: webrtc.SDPSemanticsUnifiedPlanWithFallback})
	if err != nil {
		u.Close("pub peerconn", err)
		return
	}

	if _, err = pc.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio); err != nil {
		u.Close("pub add audio trx", err)
		return
	}

	if u.initJson != nil {
		u.Println("sfu with flexatar, negotiating audio only")
		u.media = anim.NewAnimator(
			u.UserCtx, u.Id,
			func() {
				u.Println("inviting for delayed audio and ftlexatar video")
				u.inviteOthers(u.Id)
			},
			func() {
				u.Close("mediaAnimator()")
			}, u.Id, u.initJson)
		if u.media == nil {
			u.Println("error: animation engine failed")
			return
		}

	} else {
		u.media = media.NewCloner(u.Id,
			func() {
				u.inviteOthers(u.Id)
			},
			func() {
				u.Close("mediaCloner()")
			})

		u.Println("regular sfu, negotiating h264 video as well")

		if _, err = pc.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo); err != nil {
			u.Close("pub add video trx", err)
			return
		}
	}

	pc.OnTrack(func(t *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		if t.Kind() == webrtc.RTPCodecTypeAudio { // we need audio anyway
			atomic.AddInt32(&u.publisher, 1)
		}

		if t.Kind() == webrtc.RTPCodecTypeVideo {
			go func() {
				ticker := time.NewTicker(time.Second)
				defer ticker.Stop()

				for {
					select {
					case <-ticker.C:
						if err := pc.WriteRTCP([]rtcp.Packet{&rtcp.PictureLossIndication{MediaSSRC: uint32(t.SSRC())}}); err != nil {
							u.Close("failed to write rtcp", t.Kind(), err)
							return
						}
						//u.Println("pli")
					case <-u.UserCtx.Done():
						pc.Close()
						return
					}
				}
			}()

		}
		u.media.Replicate(t, receiver)
	})

	pc.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		u.Println("pub ICE Connection State has changed:", connectionState.String())
		if connectionState == webrtc.ICEConnectionStateFailed ||
			connectionState == webrtc.ICEConnectionStateDisconnected {
			u.Close("publisher ICE failed")
		}
	})

	if err = pc.SetRemoteDescription(offer); err != nil {
		u.Close("pub SetRemoteDescription(offer)", err)
		return
	}

	gatherComplete := webrtc.GatheringCompletePromise(pc)

	answer, err := pc.CreateAnswer(nil)
	// see pc.generateMatchedSDP()
	// < populateSDP()
	// < addTransceiverSDP()
	// < addCandidatesToMediaDescriptions()

	if err != nil {
		u.Close("pub CreateAnswer()", err)
		return
	}
	if err = pc.SetLocalDescription(answer); err != nil {
		u.Close("pub SetLocalDescription(answer)", err)
		return
	}

	<-gatherComplete

	local := *pc.LocalDescription()
	//u.Println("local", local.SDP)
	response, err := json.Marshal(local)
	if err != nil {
		u.Close("pub Marshal(local)", err)
		return
	}

	wpl := &defs.WsPload{
		Action: defs.ActPublish,
		Data:   response,
		Id:     u.Id,
	}
	b, err := json.Marshal(wpl)
	if err != nil {
		u.Close("pub marshal resp", err)
		return
	}
	u.wsChan <- b

	u.Println("pub negotiation done")

	u.mu.Lock()
	defer u.mu.Unlock()
	//u.media = r
	u.pc = append(u.pc, pc)

	// TODO: run it!
}

func (u *User) negotiateSubscriber(srcId int64, data []byte) {
	var offer webrtc.SessionDescription
	if err := json.Unmarshal(data, &offer); err != nil {
		u.Close("sub offer Unmarshal()", err)
		return
	}

	pc, err := u.api.NewPeerConnection(webrtc.Configuration{SDPSemantics: webrtc.SDPSemanticsUnifiedPlanWithFallback})
	if err != nil {
		u.Close("sub peerconn", err)
		return
	}

	videoTrack, err := webrtc.NewTrackLocalStaticRTP(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeH264}, fmt.Sprintf("video%d", srcId), fmt.Sprint(srcId))
	if err != nil {
		u.Close("sub video track", err)
		return
	}
	rtpSenderV, err := pc.AddTrack(videoTrack)
	if err != nil {
		u.Close("sub video track add", err)
		return
	}
	go func() {
		var lastPli time.Time
		for {
			pts, _, rtcpErr := rtpSenderV.ReadRTCP()
			if rtcpErr != nil {
				return
			}
			for _, p := range pts {
				u.Println(u.Id, "sub video rtcp from", srcId, decodeRtcp(p))
				if isPli(p) {
					now := time.Now()
					if lastPli.Add(100 * time.Millisecond).After(now) {
						log.Println(u.Id, "recent pli, ignore", srcId)
						continue
					}
					u.pli(srcId, u.Id)
					lastPli = now
				}
			}
		}
	}()

	// Create a audio track
	audioTrack, err := webrtc.NewTrackLocalStaticRTP(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus}, "audio", "pion")
	if err != nil {
		u.Println("sub audio track", err)
	}

	rtpSenderA, err := pc.AddTrack(audioTrack)
	if err != nil {
		u.Close("sub audio track add", err)
		return
	}
	go func() {
		for {
			pts, _, rtcpErr := rtpSenderA.ReadRTCP()
			if rtcpErr != nil {
				return
			}
			for _, p := range pts {
				u.Println("sub audio rtcp from", srcId, decodeRtcp(p))
			}
		}
	}()
	pc.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		u.Println("Connection State has changed", connectionState.String())
	})
	pc.OnConnectionStateChange(func(s webrtc.PeerConnectionState) {
		u.Println("Peer Connection State has changed:", s.String())

		if s == webrtc.PeerConnectionStateFailed {
			u.Println("sub failed")
		}
	})

	// Set the remote SessionDescription
	if err = pc.SetRemoteDescription(offer); err != nil {
		u.Println("sub SetRemoteDescription", err)
	}

	// Create answer
	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		u.Println("sub CreateAnswer", err)
	}

	// Create channel that is blocked until ICE Gathering is complete
	gatherComplete := webrtc.GatheringCompletePromise(pc)

	// Sets the LocalDescription, and starts our UDP listeners
	if err = pc.SetLocalDescription(answer); err != nil {
		u.Println("sub SetLocalDescription", err)
	}

	// Block until ICE Gathering is complete, disabling trickle ICE
	// we do this because we only can exchange one signaling message
	// in a production application you should exchange ICE Candidates via OnICECandidate
	<-gatherComplete

	local := *pc.LocalDescription()
	response, err := json.Marshal(local)
	if err != nil {
		log.Println("Marshal(local)", err)
		return
	}

	wpl := &defs.WsPload{
		Action: defs.ActSubscribe,
		Id:     srcId,
		Data:   response,
	}
	b, err := json.Marshal(wpl)
	if err != nil {
		u.Close("pub marshal resp", err)
		return
	}
	u.wsChan <- b

	u.Println("pub negotiation done")

	u.mu.Lock()
	defer u.mu.Unlock()
	u.pc = append(u.pc, pc)

	go u.subscribeTo(srcId, u.Id, videoTrack)
	go u.subscribeTo(srcId, u.Id, audioTrack)
}

func decodeRtcp(p rtcp.Packet) string {
	switch p.(type) {
	case *rtcp.PictureLossIndication:
		return "pli"
	case *rtcp.ReceiverReport:
		return "rr"
	default:
		return "??"
	}
}

func isPli(p rtcp.Packet) bool {
	switch p.(type) {
	case *rtcp.PictureLossIndication:
		return true
	}
	return false
}
