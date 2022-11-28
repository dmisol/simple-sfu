package rtc

import (
	"log"
	"sync"
	"sync/atomic"

	"github.com/fasthttp/websocket"
	"github.com/pion/webrtc/v3"
	"github.com/valyala/fasthttp"
)

func NewRoom() (x *Room) {
	x = &Room{
		Users:    map[int64]*User{},
		upgrader: websocket.FastHTTPUpgrader{},
	}

	m := webrtc.MediaEngine{}

	if err := m.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{MimeType: "video/H264", ClockRate: 90000, Channels: 0, SDPFmtpLine: "", RTCPFeedback: nil},
		PayloadType:        96,
	}, webrtc.RTPCodecTypeVideo); err != nil {
		log.Println("reg videoo", err)
		return
	}
	if err := m.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{MimeType: "audio/opus", ClockRate: 48000, Channels: 2, SDPFmtpLine: "", RTCPFeedback: nil},
		PayloadType:        111,
	}, webrtc.RTPCodecTypeAudio); err != nil {
		log.Println("reg audio", err)
		return
	}
	/*
		settingEngine := webrtc.SettingEngine{}

		// Enable support only for TCP ICE candidates.
		settingEngine.SetNetworkTypes([]webrtc.NetworkType{
			webrtc.NetworkTypeTCP4,
			webrtc.NetworkTypeUDP4,
			//webrtc.NetworkTypeTCP6,
		})

		tcpListener, err := net.ListenTCP("tcp", &net.TCPAddr{
			IP:   net.IP{0, 0, 0, 0},
			Port: MediaPort,
		})

		if err != nil {
			log.Println("listenTCP()", err)
			return
		}

		tcpMux := webrtc.NewICETCPMux(nil, tcpListener, 8)

		udpListener, err := net.ListenUDP("udp", &net.UDPAddr{
			IP:   net.IP{0, 0, 0, 0},
			Port: MediaPort,
		})
		if err != nil {
			log.Println("listenUDP()", err)
			return
		}

		udpMux := webrtc.NewICEUDPMux(nil, udpListener)

		settingEngine.SetICETCPMux(tcpMux)
		settingEngine.SetICEUDPMux(udpMux)
	*/
	x.api = webrtc.NewAPI(
		webrtc.WithMediaEngine(&m),
		//		webrtc.WithSettingEngine(settingEngine),
	)
	return
}

type Room struct {
	mu       sync.Mutex
	Users    map[int64]*User // by [id]
	upgrader websocket.FastHTTPUpgrader
	api      *webrtc.API

	lastUid int64
}

func (x *Room) invite(src int64) {
	x.mu.Lock()
	defer x.mu.Unlock()

	for _, u := range x.Users {
		u.Invite(src)
	}
}

func (x *Room) subscribe(pub int64, sub int64, t *webrtc.TrackLocalStaticRTP) {
	x.mu.Lock()
	defer x.mu.Unlock()

	u := x.Users[pub]
	if u == nil {
		u.Println("can't subscribe to", pub)
		return
	}
	u.Add(sub, t)
}

func (x *Room) stop(uid int64) {
	x.mu.Lock()
	defer x.mu.Unlock()

	for i, u := range x.Users {
		if i != uid {
			u.Del(uid)
		}
	}
	delete(x.Users, uid)
}

func (x *Room) Handler(r *fasthttp.RequestCtx) {
	uid := atomic.AddInt64(&x.lastUid, 1)
	user := NewUser(x.api, uid, x.invite, x.subscribe, x.stop)
	err := x.upgrader.Upgrade(r, user.Handler)
	if err != nil {
		log.Print("upgrade", err)
		return
	}

	x.mu.Lock()
	defer x.mu.Unlock()
	x.Users[uid] = user
	log.Println("user added", uid)

	for i, u := range x.Users {
		if (i != uid) && u.Publisher() {
			go user.Invite(i)
		}
	}
}
