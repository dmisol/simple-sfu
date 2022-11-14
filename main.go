package main

import (
	"crypto/tls"
	"net"

	rtc "github.com/dmisol/simple-sfu/pkg/rtc"
	"github.com/valyala/fasthttp"
	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"
)

func main() {
	room := rtc.NewRoom()
	srv := fasthttp.Server{
		Handler: room.Handler,
	}

	m := &autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist("https://sfu.flexatel.com"),
		Cache:      autocert.DirCache("/tmp/certs"),
	}

	cfg := &tls.Config{
		GetCertificate: m.GetCertificate,
		NextProtos: []string{
			"http/1.1", acme.ALPNProto,
		},
	}

	// Let's Encrypt tls-alpn-01 only works on port 443.
	ln, err := net.Listen("tcp4", "0.0.0.0:8443") /* #nosec G102 */
	if err != nil {
		panic(err)
	}

	lnTls := tls.NewListener(ln, cfg)
	panic(srv.Serve(lnTls))
}
