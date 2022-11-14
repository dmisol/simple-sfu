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
		upgrader: websocket.FastHTTPUpgrader{},
	}
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
}
