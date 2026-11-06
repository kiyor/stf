/* -.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.

* File Name : tcproxy.go

* Purpose :

* Creation Date : 06-27-2016

* Last Modified : Thu 30 Jun 2016 02:37:00 PM PDT

* Created By : Kiyor

_._._._._._._._._._._._._._._._._._._._._.*/

package tcp

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	proxy "github.com/kiyor/go-tcp-proxy"
)

var (
	connid  atomic.Uint64
	matchid atomic.Uint64

	verbose     = flag.Bool("v", false, "display server actions")
	veryverbose = flag.Bool("vv", false, "display server actions and all tcp data")
	nagles      = flag.Bool("n", false, "disable nagles algorithm")
	hex         = flag.Bool("h", false, "output hex")
	colors      = flag.Bool("c", false, "output ansi colors")
	unwrapTLS   = flag.Bool("unwrap-tls", false, "remote connection with TLS exposed unencrypted locally")
	match       = flag.String("match", "", "match regex (in the form 'regex')")
	replace     = flag.String("replace", "", "replace regex (in the form '/regex1/replacer1/regex2/replace2/' if / is delimiter)")
)

// TCPProxy starts a TCP proxy server with the given configuration
func TCPProxy(config *ProxyConfig, wg *sync.WaitGroup) {
	laddr, err := net.ResolveTCPAddr("tcp", config.LocalAddr)
	if err != nil {
		log.Println("Failed to resolve local address:", err)
		os.Exit(1)
	}

	raddr, err := net.ResolveTCPAddr("tcp", config.RemoteAddr)
	if err != nil {
		log.Println("Failed to resolve remote address:", err)
		os.Exit(1)
	}
	listener, err := net.ListenTCP("tcp", laddr)
	if err != nil {
		log.Println("Failed to open local port to listen:", err)
		os.Exit(1)
	}

	matcher := createMatcher(*match)
	replacer := createReplacer(*replace)

	if *veryverbose {
		*verbose = true
	}

	for {
		conn, err := listener.AcceptTCP()
		if err != nil {
			log.Println("Failed to accept connection:", err)
			if errors.Is(err, net.ErrClosed) {
				return
			}
			time.Sleep(50 * time.Millisecond)
			continue
		}
		if config.IsStop() {
			conn.Close()
			continue
		}
		id := connid.Add(1)

		var p *proxy.Proxy
		if *unwrapTLS {
			log.Println("Unwrapping TLS")
			p = proxy.NewTLSUnwrapped(conn, laddr, raddr, config.RemoteAddr)
		} else {
			p = proxy.New(conn, laddr, raddr)
		}

		p.Matcher = matcher
		p.Replacer = replacer

		p.Log = proxy.ColorLogger{
			Verbose:     *verbose,
			VeryVerbose: *veryverbose,
			Prefix:      fmt.Sprintf("%s #%03d ", conn.RemoteAddr().String(), id),
			Color:       *colors,
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
			log.Printf("Match #%d: %s\n", matchid.Add(1), string(m))
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
