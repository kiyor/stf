/* -.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.

* File Name : socks5.go

* Purpose :

* Creation Date : 08-28-2016

* Last Modified : Sat 14 Jan 2017 12:25:33 AM UTC

* Created By : Kiyor

_._._._._._._._._._._._._._._._._._._._._.*/

package main

import (
	"bufio"
	"context"
	"encoding/json"
	"github.com/kiyor/go-socks5"
	"io/ioutil"
	"net"
	"os"
	"strings"
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

func parseSocks5Auth(input string) socks5.StaticCredentials {
	if strings.Contains(input, " ") {
		p := strings.Split(input, " ")
		return socks5.StaticCredentials{
			p[0]: p[1],
		}
	}
	cred := make(socks5.StaticCredentials)
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
