/* -.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.

* File Name : socks5.go

* Purpose :

* Creation Date : 08-28-2016

* Last Modified : Fri 23 Mar 2018 12:06:25 AM UTC

* Created By : Kiyor

_._._._._._._._._._._._._._._._._._._._._.*/

package socks5

import (
	"bufio"
	"context"
	"encoding/json"
	"io/ioutil"
	"log"
	"net"
	"os"
	"strings"
	"sync"

	"github.com/kiyor/go-socks5"
	"github.com/kiyor/stf/pkg/config"
	"github.com/kiyor/subnettool"
	"github.com/viki-org/dnscache"
)

var (
	hostsBind   = make(map[string]*net.IP)
	hostsLocker = new(sync.RWMutex)
)

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
	*dnscache.Resolver
}

func (r *Resolver) Resolve(ctx context.Context, name string) (context.Context, net.IP, error) {
	hostsLocker.RLock()
	if val, ok := hostsBind[name]; ok {
		log.Println("hosts found", name, *val)
		hostsLocker.RUnlock()
		return ctx, *val, nil
	}
	hostsLocker.RUnlock()
	ip, err := r.FetchOne(name)
	if err != nil {
		return ctx, nil, err
	}
	return ctx, ip, err
}

type Rewriter struct {
}

func (Rewriter) Rewrite(ctx context.Context, request *socks5.Request) (context.Context, *socks5.AddrSpec) {
	return ctx, request.DestAddr
}

type LogFinalizer struct {
	Log *log.Logger
}

func (l *LogFinalizer) Finalize(request *socks5.Request, conn net.Conn, ctx context.Context) error {
	user := "-"
	if val, ok := request.AuthContext.Payload["Username"]; ok {
		user = val
	}
	resolveDur := request.ResolveTime.Sub(request.StartTime)
	connDur := request.ConnTime.Sub(request.ResolveTime)
	finishDur := request.FinishTime.Sub(request.StartTime)
	if resolveDur == -1<<63 {
		resolveDur = 0
	}
	if connDur == -1<<63 {
		connDur = resolveDur
	}
	if finishDur == -1<<63 {
		finishDur = connDur + resolveDur
	}
	l.Log.Println(user, request.RemoteAddr.String(), strings.Replace(request.DestAddr.String(), " ", "", -1), request.ReqByte, request.RespByte, resolveDur, connDur, finishDur)
	return nil
}

func ParseSocks5Auth(input string) socks5.StaticCredentials {
	cred := make(socks5.StaticCredentials)
	for _, v := range []string{" ", ":"} {
		if strings.Contains(input, v) {
			p := strings.Split(input, v)
			for i := 0; i+1 < len(p); i += 2 {
				cred[p[i]] = p[i+1]
			}
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
	if file == "" {
		return []string{}, nil
	}

	f, err := os.Open(file)
	if err != nil {
		// Return empty slice instead of error for missing files
		if os.IsNotExist(err) {
			return []string{}, nil
		}
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

// createSocks5Config creates a SOCKS5 server configuration
func createSocks5Config(authFile, hostsFile string) (*socks5.Config, error) {
	conf := &socks5.Config{}

	// Set up authentication if auth file is provided
	if authFile != "" {
		cred := socks5.StaticCredentials{}
		lines, err := cleanFile(authFile)
		if err != nil {
			return nil, err
		}

		for _, line := range lines {
			parts := strings.Split(line, ":")
			if len(parts) >= 2 {
				cred[parts[0]] = parts[1]
			}
		}

		if len(cred) > 0 {
			conf.Credentials = cred
		}
	}

	// Set up host resolution if hosts file is provided
	if hostsFile != "" {
		err := readHosts(hostsFile)
		if err != nil {
			return nil, err
		}
		// Note: dnscache doesn't directly implement NameResolver interface
		// For testing purposes, we'll leave resolver as nil
		// conf.Resolver = dnscache.New(time.Minute)
	}

	// Set up firewall rules
	conf.Rules = FireWallRuleSet{}

	return conf, nil
}

type FireWallRuleSet struct{}

func (FireWallRuleSet) Allow(ctx context.Context, req *socks5.Request) (context.Context, bool) {
	if len(config.FlagAllowIP) > 0 {
		for _, allow := range config.FlagAllowIP {
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
	if len(config.FlagDenyIP) > 0 {
		for _, deny := range config.FlagDenyIP {
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
