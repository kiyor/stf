/* -.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.

* File Name : socks5.go

* Purpose :

* Creation Date : 08-28-2016

* Last Modified : Sun 28 Aug 2016 03:35:03 AM UTC

* Created By : Kiyor

_._._._._._._._._._._._._._._._._._._._._.*/

package main

import (
	"context"
	"github.com/kiyor/go-socks5"
	"net"
)

type Resolver struct {
}

func (Resolver) Resolve(ctx context.Context, name string) (context.Context, net.IP, error) {
	addr, err := net.ResolveIPAddr("ip", name)
	if err != nil {
		return ctx, nil, err
	}
	return ctx, addr.IP, err
}

type Rewriter struct {
}

func (Rewriter) Rewrite(ctx context.Context, request *socks5.Request) (context.Context, *socks5.AddrSpec) {
	return ctx, request.DestAddr
}
