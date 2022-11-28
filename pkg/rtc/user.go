package rtc

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dmisol/simple-sfu/pkg/defs"
	"github.com/fasthttp/websocket"
	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v3"
)

const (
	timeout = 2 * time.Hour
)

func NewUser(api *webrtc.API, id int64, inviteOthers func(int64), subscribeTo func(p int64, s int64, t *webrtc.TrackLocalStaticRTP), stop func(int64)) (u *User) {
	u = &User{
		Id:           id,
		inviteOthers: inviteOthers, // to invite others to subscribe
		subscribeTo:  subscribeTo,
		stop:         stop,
		wsChan:       make(chan []byte, 5), // to invite the given user to subscribe publisher[id]
		api:          api,
	}

	return
}

type User struct {
	mu   sync.Mutex
	Id   int64
	conn *websocket.Conn // a way to stop everything

	inviteOthers func(int64)
	subscribeTo  func(p int64, s int64, t *webrtc.TrackLocalStaticRTP)
	stop         func(int64)

	api    *webrtc.API
	wsChan chan []byte
	rep    *replicator

	publisher int32

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
	u.mu.Lock()
	defer u.mu.Unlock()

	if u.rep == nil {
		u.Println("can't add: replicator not started")
		return
	}
	go u.rep.Add(id, t)
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

func (u *User) Handler(conn *websocket.Conn) {
	defer u.stop(u.Id)

	u.conn = conn
	defer u.conn.Close()

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
	defer u.conn.Close()

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

	r := &replicator{
		welcome: func() {
			u.inviteOthers(u.Id)
		},
		stop: func() {
			u.conn.Close()
		},
	}

	pc, err := u.api.NewPeerConnection(webrtc.Configuration{SDPSemantics: webrtc.SDPSemanticsUnifiedPlanWithFallback})
	if err != nil {
		u.Println("pub peerconn", err)
		return
	}

	if _, err = pc.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio); err != nil {
		u.Println("pub add audio trx", err)
		u.conn.Close()
		return
	}
	if _, err = pc.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo); err != nil {
		u.Println("pub add video trx", err)
		u.conn.Close()
		return
	}

	pc.OnTrack(func(t *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		if t.Kind() == webrtc.RTPCodecTypeAudio { // we need audio anyway
			atomic.AddInt32(&u.publisher, 1)
		}

		go func() {
			ticker := time.NewTicker(time.Second * 2)
			defer ticker.Stop()

			for {
				<-ticker.C
				if err := pc.WriteRTCP([]rtcp.Packet{&rtcp.PictureLossIndication{MediaSSRC: uint32(t.SSRC())}}); err != nil {
					u.Println("failed to write rtcp", t.Kind(), err)
					return
				}
			}
		}()
		r.Replicate(t, receiver)
	})

	pc.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		u.Println("pub ICE Connection State has changed:", connectionState.String())
		if connectionState == webrtc.ICEConnectionStateFailed ||
			connectionState == webrtc.ICEConnectionStateDisconnected {
			u.Println("publisher ICE failed")
			u.conn.Close()
		}
	})

	if err = pc.SetRemoteDescription(offer); err != nil {
		u.Println("pub SetRemoteDescription(offer)", err)
		u.conn.Close()
		return
	}

	gatherComplete := webrtc.GatheringCompletePromise(pc)

	answer, err := pc.CreateAnswer(nil)
	// see pc.generateMatchedSDP()
	// < populateSDP()
	// < addTransceiverSDP()
	// < addCandidatesToMediaDescriptions()

	if err != nil {
		u.Println("pub CreateAnswer()", err)
		u.conn.Close()
		return
	}
	if err = pc.SetLocalDescription(answer); err != nil {
		u.Println("pub SetLocalDescription(answer)", err)
		u.conn.Close()
		return
	}

	<-gatherComplete

	local := *pc.LocalDescription()
	//u.Println("local", local.SDP)
	response, err := json.Marshal(local)
	if err != nil {
		u.Println("pub Marshal(local)", err)
		u.conn.Close()
		return
	}

	wpl := &defs.WsPload{
		Action: defs.ActPublish,
		Data:   response,
		Id:     u.Id,
	}
	b, err := json.Marshal(wpl)
	if err != nil {
		u.Println("pub marshal resp", err)
		u.conn.Close()
		return
	}
	u.wsChan <- b

	u.Println("pub negotiation done")

	u.mu.Lock()
	defer u.mu.Unlock()
	u.rep = r
	u.pc = append(u.pc, pc)

	// TODO: run it!
}

func (u *User) negotiateSubscriber(srcId int64, data []byte) {
	var offer webrtc.SessionDescription
	if err := json.Unmarshal(data, &offer); err != nil {
		u.Println("sub offer Unmarshal()", err)
		u.conn.Close()
		return
	}

	pc, err := u.api.NewPeerConnection(webrtc.Configuration{SDPSemantics: webrtc.SDPSemanticsUnifiedPlanWithFallback})
	if err != nil {
		u.Println("sub peerconn", err)
		u.conn.Close()
		return
	}

	videoTrack, err := webrtc.NewTrackLocalStaticRTP(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeH264}, fmt.Sprintf("video%d", srcId), fmt.Sprint(srcId))
	if err != nil {
		u.Println("sub video track", err)
		u.conn.Close()
		return
	}
	rtpSenderV, err := pc.AddTrack(videoTrack)
	if err != nil {
		u.Println("sub video track add", err)
		u.conn.Close()
		return
	}
	go func() {
		rtcpBuf := make([]byte, 1500)
		for {
			if _, _, rtcpErr := rtpSenderV.Read(rtcpBuf); rtcpErr != nil {
				u.Println("sub rtcp video rd", err)
				return
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
		u.Println("sub audio track add", err)
		u.conn.Close()
		return
	}
	go func() {
		rtcpBuf := make([]byte, 1500)
		for {
			if _, _, rtcpErr := rtpSenderA.Read(rtcpBuf); rtcpErr != nil {
				return
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
		u.Println("pub marshal resp", err)
		u.conn.Close()
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
