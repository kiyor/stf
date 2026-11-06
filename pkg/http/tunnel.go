/* -.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.

* File Name : tunnel.go

* Purpose :

* Creation Date : 10-05-2017

* Last Modified : Mon 16 Apr 2018 02:20:07 AM UTC

* Created By : Kiyor

_._._._._._._._._._._._._._._._._._._._._.*/

package http

import (
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"errors"
	"golang.org/x/net/proxy"
)

type Pxy struct {
	Next      string
	dial      func(network, address string) (net.Conn, error)
	transport *http.Transport
}

func NewProxy(next string) *Pxy {
	dial := toDial(next)
	return &Pxy{
		Next: next,
		dial: dial,
		// One shared Transport: a Transport per request would keep its idle
		// keep-alive connections open forever and leak file descriptors.
		transport: &http.Transport{
			Dial:                dial,
			MaxIdleConnsPerHost: 8,
			IdleConnTimeout:     90 * time.Second,
		},
	}
}

// hopHeaders are connection-specific and must not be forwarded (RFC 7230 6.1).
var hopHeaders = []string{
	"Connection", "Proxy-Connection", "Keep-Alive", "Proxy-Authenticate",
	"Proxy-Authorization", "Te", "Trailer", "Transfer-Encoding", "Upgrade",
}

func stripHopHeaders(h http.Header) {
	for _, f := range h["Connection"] {
		for _, k := range strings.Split(f, ",") {
			h.Del(strings.TrimSpace(k))
		}
	}
	for _, k := range hopHeaders {
		h.Del(k)
	}
}

// ServeHTTP is the main handler for all requests.
func (p *Pxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	t1 := time.Now()
	dial := p.dial
	if dial == nil {
		dial = toDial(p.Next)
	}
	var req, rec int64
	if r.Method == "CONNECT" {
		host := r.URL.Host
		hij, ok := w.(http.Hijacker)
		if !ok {
			http.Error(w, "hijacking not supported", http.StatusInternalServerError)
			return
		}

		server, err := dial("tcp", host)
		if err != nil {
			log.Println(err.Error())
			http.Error(w, "Bad Gateway", http.StatusBadGateway)
			return
		}
		defer server.Close()

		client, _, err := hij.Hijack()
		if err != nil {
			log.Println(err.Error())
			return
		}
		defer client.Close()

		if _, err := client.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n")); err != nil {
			return
		}

		errCh := make(chan error, 2)
		sizeCh := make(chan int64, 2)

		go Copy(server, client, errCh, sizeCh)
		go Copy(client, server, errCh, sizeCh)
		// When either direction ends, close both sides so the other copy unblocks.
		e := <-errCh
		req = <-sizeCh
		client.Close()
		server.Close()
		<-errCh
		rec = <-sizeCh
		if e != nil && !errors.Is(e, net.ErrClosed) {
			log.Println(e.Error())
		}
	} else {
		transport := p.transport
		if transport == nil {
			transport = &http.Transport{Dial: dial}
			defer transport.CloseIdleConnections()
		}
		outReq := r.Clone(r.Context())
		outReq.RequestURI = ""
		stripHopHeaders(outReq.Header)

		if clientIP, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
			if prior, ok := outReq.Header["X-Forwarded-For"]; ok {
				clientIP = strings.Join(prior, ", ") + ", " + clientIP
			}
			outReq.Header.Set("X-Forwarded-For", clientIP)
		}

		res, err := transport.RoundTrip(outReq)
		if err != nil {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		defer res.Body.Close()
		stripHopHeaders(res.Header)

		for key, value := range res.Header {
			for _, v := range value {
				w.Header().Add(key, v)
			}
		}

		w.WriteHeader(res.StatusCode)
		// 		rec, err = io.Copy(w, res.Body)
		// 		if err != nil {
		// 			log.Println(err.Error())
		// 		}
		var n int64
		for {
			n, err = io.CopyN(w, res.Body, 16*1024)
			rec += n
			if err != nil {
				break
			}
		}
		if err == io.EOF {
			err = nil
		}
		if err != nil {
			log.Println(err.Error())
		}
	}

	log.Println(r.RemoteAddr, req, rec, time.Since(t1))
}

func Copy(dst io.Writer, src io.Reader, errCh chan error, sizeCh chan int64) {
	var err error
	var size, n int64
	for {
		n, err = io.CopyN(dst, src, 16*1024)
		size += n
		n = 0
		if err != nil {
			break
		}
	}
	if err == io.EOF {
		err = nil
	}
	if tcpConn, ok := dst.(*net.TCPConn); ok {
		tcpConn.CloseWrite()
	}

	errCh <- err
	sizeCh <- size
}

type closeWriter interface {
	CloseWrite() error
}

// toDial creates a dial function with optional SOCKS5 proxy support
func toDial(s string) func(network, address string) (net.Conn, error) {
	if s == "[ip:port](:user:pass)" {
		return (&net.Dialer{
			Timeout: 5 * time.Second,
		}).Dial
	}
	var a *proxy.Auth
	p := strings.Split(s, ":")
	if len(p) > 2 {
		a = new(proxy.Auth)
		a.User = p[2]
		if len(p) > 3 {
			a.Password = strings.Join(p[3:], ":")
		}
		s = strings.Join(p[:2], ":")
	}
	dialer, err := proxy.SOCKS5("tcp", s,
		a,
		&net.Dialer{
			KeepAlive: 5 * time.Second,
			Timeout:   5 * time.Second,
		},
	)
	if err != nil {
		log.Println(err.Error())
		return func(network, addr string) (net.Conn, error) { return nil, err }
	}
	return dialer.Dial
}
