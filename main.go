package main

import (
	"crypto/tls"
	"log"
	"net"
	"path"
	"syscall"

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

	syscall.Umask(0)

	room := rtc.NewRoom(c)
	sh := fasthttp.FSHandler("static", 0)

	srv := fasthttp.Server{
		Handler: func(r *fasthttp.RequestCtx) {
			ref := string(r.Referer())
			pth := string(r.Path())
			// log.Println(pth, "referer:", ref)

			if len(ref) == 0 {
				switch string(r.Path()) {
				case "/ws":
					room.Handler(r)
				case "/sfu": // TODO: a page permitting to select a flexatar to replace user's video
					r.SendFile(path.Join("static", "sfu.html"))
				case "/cef": // TODO: a page to subscribe all streams, without publishing
					r.SendFile(path.Join("static", "cef.html"))
				case "/":
					sh(r)
				default:
					if len(c.Redirect) == 0 {
						log.Println("redirect not set")
						r.Error("not configured", fasthttp.StatusBadRequest)
						return
					}
					qa := r.QueryArgs().String()
					dest := c.Redirect + pth
					if len(qa) > 0 {
						dest += "?" + qa
					}
					log.Println(pth, "re-routing to", dest)
					r.Redirect(dest, fasthttp.StatusPermanentRedirect)
				}
				return
			}
			sh(r)
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
