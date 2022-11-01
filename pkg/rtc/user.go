package rtc

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/fasthttp/websocket"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v3"
)

const (
	timeout = 2 * time.Hour
)

func NewUser(ctx context.Context, ws *websocket.Conn) (u *User) {
	u = &User{ws: ws}
	u.ctx, u.cancel = context.WithTimeout(ctx, timeout)
	go u.run()

	return
}

type User struct {
	Id     string
	ctx    context.Context
	cancel func()
	a, v   subs // neighbour users, who should receive rtp-s from remoteTracks

	localTracks map[string]*webrtc.TrackLocalStaticRTP // [id] - who publishes
	localPC     *webrtc.PeerConnection                 // sdp with unified plan

	remoteTracks map[string]*webrtc.TrackRemote // [a/v]
	remotePC     *webrtc.PeerConnection

	ws *websocket.Conn
}

func (u *User) AddSender(id string, t *webrtc.TrackLocalStaticRTP) {
	if t.Kind() == webrtc.RTPCodecTypeAudio {
		u.a.mu.Lock()
		defer u.a.mu.Unlock()

		u.a.tracks[id] = t
		return
	}
	u.v.mu.Lock()
	defer u.v.mu.Unlock()

	u.v.tracks[id] = t
}

func (u *User) RemoveSender(id string) {
	u.a.remove(id)
	u.v.remove(id)
}

func (u *User) run() {
	defer u.ws.Close()
	defer u.cancel()
	for {
		select {
		case <-u.ctx.Done():
			return
		default:
			mt, msg, err := u.ws.ReadMessage()
			if err != nil {
				u.Println("ws conn", err)
				return
			}
			if err = u.process(mt, msg); err != nil {
				u.Println("ws data", err)
				return
			}
		}
	}

}

func (u *User) process(mt int, p []byte) (err error) {
	// todo

	return
}

func (u *User) Println(i ...interface{}) {
	log.Println("USER", i)
}

// ---------------------

type subs struct {
	mu     sync.Mutex
	tracks map[string]*webrtc.TrackLocalStaticRTP // [id] - neighbour users, to whom
}

func (s *subs) remove(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.tracks, id)
}

func (s *subs) SendRTP(p *rtp.Packet) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, dest := range s.tracks {
		if err := dest.WriteRTP(p); err != nil {
			// todo: remove user by [id]
			log.Println("need to remove", id)
		}
	}
}

func (s *subs) OnVideoTrack() {

}
