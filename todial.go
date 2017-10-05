/* -.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.

* File Name : todial.go

* Purpose :

* Creation Date : 10-05-2017

* Last Modified : Thu 05 Oct 2017 09:11:43 PM UTC

* Created By : Kiyor

_._._._._._._._._._._._._._._._._._._._._.*/

package main

import (
	"golang.org/x/net/proxy"
	"log"
	"net"
	"strings"
	"time"
)

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
		a.Password = p[3]
		s = strings.Join(p[:2], ":")
	}
	dialer, err := proxy.SOCKS5("tcp", s,
		a,
		&net.Dialer{
			KeepAlive: 5 * time.Second,
			Timeout:   5 * time.Second,
		},
		// 		proxy.Direct,
	)
	if err != nil {
		log.Println(err.Error())
	}
	return dialer.Dial
}
