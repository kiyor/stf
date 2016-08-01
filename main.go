/* -.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.

* File Name : main.go

* Purpose :

* Creation Date : 12-14-2015

* Last Modified : Thu 28 Jul 2016 06:08:09 PM PDT

* Created By : Kiyor

_._._._._._._._._._._._._._._._._._._._._.*/

package main

import (
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/armon/go-socks5"
	"github.com/wsxiaoys/terminal/color"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	fdir     *string = flag.String("d", ".", "Mount Dir")
	fport    *string = flag.String("p", ":30000", "Listening Port")
	upstream *string = flag.String("upstream", "scheme://ip:port or ip:port", "setup proxy")

	sock *bool = flag.Bool("socks5", false, "socks5 mode")

	bridge               *string = flag.String("bridge", "host/ip/host:ip", "quick setup http/+https proxy with upstream 80/+443")
	crt                  *string = flag.String("crt", "", "crt location if using brdige mode")
	key                  *string = flag.String("key", "", "key location if using brdige mode")
	bridgeIp, bridgeHost string

	version *bool = flag.Bool("version", false, "output version and exit")

	rt = flag.Int("return", -1, "debug test return code")

	tcp      bool
	isbridge bool

	timeout *time.Duration = flag.Duration("timeout", 5*time.Minute, "timeout")

	proxyClient = http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	proxyMethod = false

	ch        = make(chan bool)
	wg        = new(sync.WaitGroup)
	stop      bool
	buildtime string
	VER       = "1.0"
)

func init() {
	flag.Parse()
	if *version {
		fmt.Printf("%v.%v", VER, buildtime)
		os.Exit(0)
	}
	if *upstream != "scheme://ip:port or ip:port" {
		proxyMethod = true
		u := *upstream
		if u[:4] != "http" {
			tcp = true
		}
	}
	if *bridge != "host/ip/host:ip" {
		isbridge = true
		proxyMethod = true
		p := strings.Split(*bridge, ":")
		if len(p) > 1 {
			bridgeHost = p[0]
			bridgeIp = p[1]
		} else {
			if ip := net.ParseIP(*bridge); ip == nil {
				bridgeHost = *bridge
			}
			bridgeIp = *bridge
		}
		*upstream = *bridge
	}
	p := *fport
	if p[:1] != ":" {
		p = ":" + p
		fport = &p
	}

}

func getips() string {
	ips, err := net.InterfaceAddrs()
	if err != nil {
		panic(err)
	}
	var s string
	for _, v := range ips {
		ip := strings.Split(v.String(), "/")[0]
		if ip != "127.0.0.1" {
			s += strings.Split(v.String(), "/")[0] + *fport + " "
		}
	}
	return s
}

func main() {

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		if stop {
			return
		}
		wg.Add(1)
		t1 := time.Now()
		defer wg.Done()
		defer func() {
			var res string
			if proxyMethod {
				res = fmt.Sprintf("%v %v %v %v %v", req.RemoteAddr, req.Method, req.URL.String(), NanoToSecond(time.Since(t1)), w.Header().Get("X-Upstream-Response-Time"))
			} else {
				res = fmt.Sprintf("%v %v %v %v %v", req.RemoteAddr, req.Method, req.URL.String(), NanoToSecond(time.Since(t1)), "-")
			}
			if *colors {
				log.Println(color.Sprintf("@{g}%s", res))
			} else {
				log.Println(res)
			}
			if *veryverbose {
				dumpRequest(req, true, true)
			}
		}()
		ch <- true
		if proxyMethod {
			if isbridge {
				scheme := "https"
				if req.TLS == nil {
					scheme = "http"
				}
				proxyHandler(w, req, fmt.Sprintf("%s://%s", scheme, bridgeIp))
			} else {
				proxyHandler(w, req, *upstream)
			}
			return
		}
		if *rt != -1 {
			w.WriteHeader(*rt)
			w.Write(dumpRequest(req, true, false))
			return
		}
		w.Header().Add("Cache-Control", "no-cache")
		if req.Method == "GET" {
			f := &fileHandler{http.Dir(*fdir)}
			f.ServeHTTP(w, req)
		} else if req.Method == "POST" || req.Method == "PUT" {
			uploadHandler(w, req)
		}
	})

	log.Println("Listening on", getips())
	if proxyMethod {
		log.Println("Upstream", *upstream, "tcp", tcp)
	}

	done := make(chan bool)

	if *sock {
		go func() {
			conf := &socks5.Config{}
			server, err := socks5.New(conf)
			if err != nil {
				panic(err)
			}

			if err := server.ListenAndServe("tcp", *fport); err != nil {
				panic(err)
			}
		}()
	} else if tcp {
		go tcpProxy()
	} else {
		if !isbridge {
			go func() {
				if err := http.ListenAndServe(*fport, mux); err != nil {
					panic(err)
				}
			}()
		} else {
			go func() {
				if err := http.ListenAndServe(":80", mux); err != nil {
					panic(err)
				}
			}()
			if len(*crt) > 0 && len(*key) > 0 {
				go func() {
					if err := http.ListenAndServeTLS(":443", *crt, *key, mux); err != nil {
						panic(err)
					}
				}()
			}
		}
	}

	t := time.Tick(*timeout)
	go func() {
		for {
			select {
			case <-t:
				log.Println(os.Args[0], "auto stop, no more request accessable")
				stop = true
				wg.Wait()
				done <- true
			case <-ch:
				t = time.Tick(*timeout)
			}
		}
	}()

	if <-done {
		log.Println("stop")
		os.Exit(0)
	}
}

func Json(i interface{}) string {
	b, err := json.MarshalIndent(i, "", "  ")
	if err != nil {
		log.Println(err.Error())
	}
	return string(b)
}

func dumpRequest(r *http.Request, b, p bool) []byte {
	dump, err := httputil.DumpRequest(r, b)
	if err != nil {
		log.Println(err.Error())
	}
	if p {
		if *colors {
			color.Printf("@{b}%s@{|}", string(dump))
		} else {
			fmt.Print(string(dump))
		}
	}
	return dump
}

func proxyHandler(w http.ResponseWriter, r *http.Request, upper string) {
	var path string
	var host string
	if strings.Contains(r.URL.String(), "http") {
		path = r.URL.String()
		host = r.URL.Host
	} else {
		path = upper + r.URL.Path
		host = r.Host
	}
	req, err := http.NewRequest(r.Method, path, r.Body)
	if err != nil {
		panic(err)
	}
	if len(bridgeHost) > 0 {
		req.Host = bridgeHost
	}
	if ip := net.ParseIP(r.Host); ip == nil {
		req.Host = host
	}
	t1 := time.Now()
	resp, err := proxyClient.Do(req)
	if err != nil {
		fmt.Fprintf(w, err.Error())
		return
	}
	defer resp.Body.Close()

	for k, v := range resp.Header {
		for _, v1 := range v {
			w.Header().Set(k, v1)
		}
	}
	w.Header().Set("X-Upstream-Response-Time", NanoToSecond(time.Since(t1)))

	io.Copy(w, resp.Body)
}

func NanoToSecond(d time.Duration) string {
	return fmt.Sprintf("%.3f", float64(d.Nanoseconds())/1000000)
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {

	p := *fdir + "/" + r.URL.Path
	d, _ := filepath.Split(p)

	f, err := os.Open(d)
	defer f.Close()

	if err != nil {
		err = os.MkdirAll(d, 0755)
		if err != nil {
			fmt.Fprintf(w, "%s\n", err.Error())
			log.Println(err.Error())
		}
		f, _ = os.Open(d)
	}
	fi, err := f.Stat()
	if err != nil {
		fmt.Fprintf(w, "%s\n", err.Error())
		log.Println(err.Error())
		return
	}
	if fi.Mode().IsRegular() {
		fmt.Fprintf(w, "%s is a file\n", d)
		log.Println(d, "is a file")
		return
	}

	out, err := os.Create(p)
	if err != nil {
		fmt.Fprintf(w, "Unable to create the file for writing. Check your write access privilege\n")
		return
	}

	defer out.Close()

	_, err = io.Copy(out, r.Body)
	if err != nil {
		fmt.Fprintln(w, err)
	}

	fmt.Fprintf(w, "File uploaded successfully : %s\n", p)
}
