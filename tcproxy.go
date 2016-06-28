/* -.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.

* File Name : tcproxy.go

* Purpose :

* Creation Date : 06-27-2016

* Last Modified : Tue 28 Jun 2016 04:38:12 PM PDT

* Created By : Kiyor

_._._._._._._._._._._._._._._._._._._._._.*/

package main

import (
	"flag"
	"fmt"
	"github.com/kiyor/go-tcp-proxy"
	"log"
	"net"
	"os"
	"regexp"
	"strings"
)

var (
	connid    = uint64(0)
	matchid   = uint64(0)
	unwrapTLS = flag.Bool("unwrap-tls", false, "remote connection with TLS exposed unencrypted locally")
	match     = flag.String("match", "", "match regex (in the form 'regex')")
	replace   = flag.String("replace", "", "replace regex (in the form '/regex1/replacer1/regex2/replace2/' if / is delimiter)")
)

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

	matcher := createMatcher(*match)
	replacer := createReplacer(*replace)

	for {
		conn, err := listener.AcceptTCP()
		if err != nil {
			log.Println("Failed to accept connection '%s'", err)
		}
		if stop {
			conn.Close()
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

		p.Matcher = matcher
		p.Replacer = replacer

		p.Log = proxy.ColorLogger{
			Verbose:     false,
			VeryVerbose: false,
			Prefix:      fmt.Sprintf("Connection #%03d ", connid),
			Color:       false,
		}

		go p.Start(wg)
	}
}

func createMatcher(match string) func([]byte) {
	if match == "" {
		return nil
	}
	re, err := regexp.Compile(match)
	if err != nil {
		log.Panic("Invalid match regex: " + err.Error())
		return nil
	}

	log.Println("Matching", re.String())
	return func(input []byte) {
		ms := re.FindAll(input, -1)
		for _, m := range ms {
			matchid++
			log.Printf("Match #%d: %s\n", matchid, string(m))
		}
	}
}

func createReplacer(replace string) func([]byte) []byte {
	if replace == "" {
		return nil
	}
	delimiter := replace[:1]
	parts := strings.Split(replace, delimiter)
	parts = parts[1 : len(parts)-1]
	if len(parts)%2 != 0 {
		log.Println("Invalid replace option")
		return nil
	}

	var res []*regexp.Regexp
	var repls [][]byte
	for i := 0; i < len(parts); i += 2 {
		re, err := regexp.Compile(string(parts[i]))
		if err != nil {
			log.Println("Invalid replace regex:", err.Error())
			return nil
		}
		repl := []byte(parts[i+1])
		log.Printf("Replacing %s with %s\n", re.String(), repl)
		res = append(res, re)
		repls = append(repls, repl)
	}

	return func(input []byte) []byte {
		for k, re := range res {
			input = re.ReplaceAll(input, repls[k])
		}
		return input
	}
}
