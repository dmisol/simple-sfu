package main

import (
	"crypto/tls"
	"log"
	"net"

	"github.com/dmisol/simple-sfu/pkg/defs"
	rtc "github.com/dmisol/simple-sfu/pkg/rtc"
	"github.com/valyala/fasthttp"
	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"
)

func main() {
	c, err := defs.ReadConf("conf.yaml")
	if err != nil {
		log.Println("conf", err)
		return
	}

	room := rtc.NewRoom(c)
	sh := fasthttp.FSHandler("static", 0)

	srv := fasthttp.Server{
		Handler: func(r *fasthttp.RequestCtx) {
			switch string(r.Path()) {
			case "/ws":
				room.Handler(r)
			default:
				sh(r)
			}
		},
	}

	if len(c.Hosts) == 0 {
		panic(srv.ListenAndServe(net.JoinHostPort("", c.Port)))
	}

	m := &autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist(c.Hosts...),
		Cache:      autocert.DirCache("/tmp/certs"),
	}

	cfg := &tls.Config{
		GetCertificate: m.GetCertificate,
		NextProtos: []string{
			"http/1.1", acme.ALPNProto,
		},
	}

	// Let's Encrypt tls-alpn-01 only works on port 443.
	ln, err := net.Listen("tcp4", "0.0.0.0:443") /* #nosec G102 */
	if err != nil {
		panic(err)
	}

	lnTls := tls.NewListener(ln, cfg)
	panic(srv.Serve(lnTls))
}
