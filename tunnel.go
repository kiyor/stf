/* -.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.

* File Name : tunnel.go

* Purpose :

* Creation Date : 10-05-2017

* Last Modified : Mon 16 Apr 2018 02:20:07 AM UTC

* Created By : Kiyor

_._._._._._._._._._._._._._._._._._._._._.*/

package main

import (
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
)

type Pxy struct{}

func NewProxy() *Pxy {
	return &Pxy{}
}

// ServeHTTP is the main handler for all requests.
func (p *Pxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	t1 := time.Now()
	dial := toDial(*next)
	var req, rec int64
	if r.Method == "CONNECT" {
		host := r.URL.Host
		hij, ok := w.(http.Hijacker)
		if !ok {
			panic("HTTP Server does not support hijacking")
		}

		client, _, err := hij.Hijack()
		if err != nil {
			log.Println(err.Error())
			return
		}

		server, err := dial("tcp", host)
		if err != nil {
			log.Println(err.Error())
			return
		}
		client.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))

		errCh := make(chan error, 2)
		sizeCh := make(chan int64, 2)

		go Copy(server, client, errCh, sizeCh)
		go Copy(client, server, errCh, sizeCh)
		req = <-sizeCh
		rec = <-sizeCh
		for i := 0; i < 2; i++ {
			e := <-errCh
			if e != nil {
				log.Println(err.Error())
			}
		}
	} else {
		transport := &http.Transport{
			Dial: dial,
		}
		outReq := new(http.Request)
		*outReq = *r

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
		res.Body.Close()
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
