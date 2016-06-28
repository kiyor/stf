/* -.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.

* File Name : main.go

* Purpose :

* Creation Date : 12-14-2015

* Last Modified : Mon 27 Jun 2016 06:57:37 PM PDT

* Created By : Kiyor

_._._._._._._._._._._._._._._._._._._._._.*/

package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	fdir      *string = flag.String("d", ".", "Mount Dir")
	fport     *string = flag.String("p", ":30000", "Listening Port")
	upstream  *string = flag.String("upstream", "scheme://ip:port or ip:port", "setup proxy")
	version   *bool   = flag.Bool("v", false, "output version and exit")
	unwrapTLS         = flag.Bool("unwrap-tls", false, "remote connection with TLS exposed unencrypted locally")

	tcp bool

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
			if proxyMethod {
				log.Println(req.Method, req.URL.Path, NanoToSecond(time.Since(t1)), w.Header().Get("X-Upstream-Response-Time"))
			} else {
				log.Println(req.Method, req.URL.Path, NanoToSecond(time.Since(t1)), "-")
			}
		}()
		ch <- true
		if proxyMethod {
			proxyHandler(w, req)
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

	if tcp {
		go tcpProxy()
	} else {
		go http.ListenAndServe(*fport, mux)
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

func proxyHandler(w http.ResponseWriter, r *http.Request) {
	req, _ := http.NewRequest(r.Method, *upstream+r.URL.Path, r.Body)
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
