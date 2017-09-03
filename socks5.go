/* -.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.

* File Name : socks5.go

* Purpose :

* Creation Date : 08-28-2016

* Last Modified : Sun 03 Sep 2017 03:47:08 AM UTC

* Created By : Kiyor

_._._._._._._._._._._._._._._._._._._._._.*/

package main

import (
	"bufio"
	"context"
	"encoding/json"
	"github.com/kiyor/go-socks5"
	"github.com/kiyor/subnettool"
	"io/ioutil"
	"log"
	"net"
	"os"
	"strings"
	"sync"
)

var hostsBind = make(map[string]*net.IP)
var hostsLocker = new(sync.RWMutex)

func readHosts(file string) error {
	hostsLocker.Lock()
	defer hostsLocker.Unlock()
	hostsBind = make(map[string]*net.IP)
	lines, _ := cleanFile(file)
	for _, line := range lines {
		for strings.Contains(line, "  ") {
			line = strings.Replace(line, "  ", " ", -1)
		}
		p := strings.Split(line, " ")
		if ip := net.ParseIP(p[0]); ip != nil {
			for _, v := range p[1:] {
				hostsBind[v] = &ip
			}
		}
	}
	b, _ := json.MarshalIndent(hostsBind, "", "  ")
	log.Println(string(b))
	return nil
}

type Resolver struct {
}

func (Resolver) Resolve(ctx context.Context, name string) (context.Context, net.IP, error) {
	hostsLocker.RLock()
	if val, ok := hostsBind[name]; ok {
		log.Println("hosts found", name, *val)
		hostsLocker.RUnlock()
		return ctx, *val, nil
	}
	hostsLocker.RUnlock()
	addr, err := net.ResolveIPAddr("ip", name)
	// 	log.Println(name, addr)
	if err != nil {
		return ctx, nil, err
	}
	return ctx, addr.IP, err
}

type Rewriter struct {
}

func (Rewriter) Rewrite(ctx context.Context, request *socks5.Request) (context.Context, *socks5.AddrSpec) {
	// 	log.Println(request.RemoteAddr, ">>>", request.DestAddr)
	return ctx, request.DestAddr
}

type LogFinalizer struct {
	log *log.Logger
}

func (l *LogFinalizer) Finalize(ctx context.Context) error {
	if ctx.Value("raddr") != nil {
		l.log.Println(ctx.Value("username"), ctx.Value("raddr"), ctx.Value("daddr"), ctx.Value("request_byte"), ctx.Value("response_byte"))
	}
	return nil
}

func parseSocks5Auth(input string) socks5.StaticCredentials {
	cred := make(socks5.StaticCredentials)
	for _, v := range []string{" ", ":"} {
		if strings.Contains(input, v) {
			p := strings.Split(input, v)
			for i := 0; i < len(p); i += 2 {
				cred[p[i]] = p[i+1]
			}
			log.Println(cred)
			return cred
		}
	}
	d, err := ioutil.ReadFile(input)
	if err != nil {
		return socks5.StaticCredentials{}
	}
	err = json.Unmarshal(d, &cred)
	if err != nil {
		lines, err := cleanFile(input)
		if err != nil {
			return socks5.StaticCredentials{}
		}
		for _, line := range lines {
			p := strings.Split(line, " ")
			if len(p) > 1 {
				cred[p[0]] = p[1]
			}
		}
		return cred
	}

	return cred
}

func cleanFile(file string) ([]string, error) {
	f, err := os.Open(file)
	if err != nil {
		return []string{}, err
	}
	defer f.Close()

	var line []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		p := strings.Split(scanner.Text(), "#")
		if len(p[0]) > 0 {
			line = append(line, p[0])
		}
	}

	if err := scanner.Err(); err != nil {
		return line, err
	}
	return line, nil
}

type FireWallRuleSet struct{}

func (FireWallRuleSet) Allow(ctx context.Context, req *socks5.Request) (context.Context, bool) {
	if len(flagAllowIP) > 0 {
		for _, allow := range flagAllowIP {
			if ip := net.ParseIP(allow); ip != nil {
				if ip.Equal(req.RemoteAddr.IP) {
					return ctx, true
				}
			} else {
				if subnettool.CIDRMatch(req.RemoteAddr.IP.String(), allow) {
					return ctx, true
				}
			}
		}
		return ctx, false
	}
	if len(flagDenyIP) > 0 {
		for _, deny := range flagDenyIP {
			if ip := net.ParseIP(deny); ip != nil {
				if ip.Equal(req.RemoteAddr.IP) {
					return ctx, false
				}
			} else {
				if subnettool.CIDRMatch(req.RemoteAddr.IP.String(), deny) {
					return ctx, false
				}
			}
		}
		return ctx, true
	}
	return ctx, true
}
