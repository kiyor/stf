/* -.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.

* File Name : tcproxy.go

* Purpose :

* Creation Date : 06-27-2016

* Last Modified : Mon 27 Jun 2016 07:05:51 PM PDT

* Created By : Kiyor

_._._._._._._._._._._._._._._._._._._._._.*/

package main

import (
	"fmt"
	"github.com/kiyor/go-tcp-proxy"
	"log"
	"net"
	"os"
)

var connid = uint64(0)

func tcpProxy() {
	laddr, err := net.ResolveTCPAddr("tcp", *fport)
	if err != nil {
		log.Println("Failed to resolve local address:", err)
		os.Exit(1)
	}

	raddr, err := net.ResolveTCPAddr("tcp", *upstream)
	if err != nil {
		log.Println("Failed to resolve remote address: %s", err)
		os.Exit(1)
	}
	listener, err := net.ListenTCP("tcp", laddr)
	if err != nil {
		log.Println("Failed to open local port to listen: %s", err)
		os.Exit(1)
	}
	for {
		conn, err := listener.AcceptTCP()
		if err != nil {
			log.Println("Failed to accept connection '%s'", err)
			continue
		}
		connid++

		var p *proxy.Proxy
		if *unwrapTLS {
			log.Println("Unwrapping TLS")
			p = proxy.NewTLSUnwrapped(conn, laddr, raddr, *upstream)
		} else {
			p = proxy.New(conn, laddr, raddr)
		}

		p.Log = proxy.ColorLogger{
			Verbose:     false,
			VeryVerbose: false,
			Prefix:      fmt.Sprintf("Connection #%03d ", connid),
			Color:       false,
		}

		go p.Start()
	}
}
