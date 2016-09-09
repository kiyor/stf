/* -.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.

* File Name : main.go

* Purpose :

* Creation Date : 12-14-2015

* Last Modified : Fri 09 Sep 2016 06:14:28 PM UTC

* Created By : Kiyor

_._._._._._._._._._._._._._._._._._._._._.*/

package main

import (
	// 	"bytes"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/dustin/go-humanize"
	"github.com/kiyor/go-socks5"
	"github.com/wsxiaoys/terminal/color"
	"io"
	"io/ioutil"
	"log"
	"mime"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	fdir     *string = flag.String("d", ".", "Mount Dir")
	fport    *string = flag.String("p", ":30000", "Listening Port")
	upstream *string = flag.String("upstream", "scheme://ip:port or ip:port", "setup proxy")

	sock       *bool = flag.Bool("socks5", false, "socks5 mode")
	uploadonly *bool = flag.Bool("uploadonly", false, "upload only POST/PUT")

	testFile *bool = flag.Bool("testfile", false, "testfile, /1(K/M/G)")

	bridge               *string = flag.String("bridge", "host/ip/host:ip", "quick setup http/+https proxy with upstream 80/+443")
	crt                  *string = flag.String("crt", "", "crt location if using brdige mode")
	key                  *string = flag.String("key", "", "key location if using brdige mode")
	bridgeIp, bridgeHost string

	version *bool = flag.Bool("version", false, "output version and exit")

	rt = flag.Int("return", -1, "debug test return code")

	tcp      bool
	isbridge bool

	timeout   *time.Duration = flag.Duration("timeout", 5*time.Minute, "timeout")
	notimeout                = flag.Bool("notimeout", false, "no timeout")

	proxyClient = http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	proxyMethod = false

	reTestFile = regexp.MustCompile(`(\d+)(b|B|k|K|m|M|g|G)(.*)`)

	ch        = make(chan bool)
	wg        = new(sync.WaitGroup)
	stop      bool
	buildtime string
	VER       = "1.0"
	bt        = make([]byte, 1024)
	serveByte uint64
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

func byteCounter() {
	ticker := time.Tick(time.Second)
	var total uint64
	var max uint64
	var avg uint64
	var emptySecond float64
	t1 := time.Now()
	defer fmt.Println()
	for {
		<-ticker
		total += serveByte
		if serveByte == 0 {
			emptySecond += 1.0
		} else if serveByte > max {
			max = serveByte
		}
		if uint64(time.Since(t1).Seconds()-emptySecond) > 0 {
			avg = total / uint64(time.Since(t1).Seconds()-emptySecond)
		}
		fmt.Printf("\rspeed: %10v/s  total: %10v  max: %10v/s  avg: %10v/s", humanize.Bytes(serveByte), humanize.Bytes(total), humanize.Bytes(max), humanize.Bytes(avg))
		serveByte = 0
	}
}

func main() {

	runtime.GOMAXPROCS(runtime.NumCPU())

	if *testFile {
		go byteCounter()
	}

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
		// 		w.Header().Add("Cache-Control", "no-cache")
		w.Header().Add("Connection", "Keep-Alive")
		if req.Method == "GET" && !*uploadonly && !*testFile {
			w.Header().Add("Cache-Control", "no-cache")
			f := &fileHandler{http.Dir(*fdir)}
			f.ServeHTTP(w, req)
		} else if *testFile {
			testFileHandler(w, req)
		} else if req.Method == "POST" || req.Method == "PUT" {
			uploadHandler(w, req)
		}
	})

	log.Println("Listening on", getips())
	if proxyMethod {
		log.Println("Upstream", *upstream, "tcp", tcp)
	}
	if *testFile {
		log.SetOutput(ioutil.Discard)
	}

	done := make(chan bool)

	if *sock {
		go func() {
			conf := &socks5.Config{}
			conf.Resolver = new(Resolver)
			conf.Rewriter = new(Rewriter)
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
			if len(*crt) > 0 && len(*key) > 0 {
				go func() {
					if err := http.ListenAndServeTLS(*fport, *crt, *key, mux); err != nil {
						panic(err)
					}
				}()
			} else {
				go func() {
					if err := http.ListenAndServe(*fport, mux); err != nil {
						panic(err)
					}
				}()
			}
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

	if *notimeout {
		*timeout = time.Duration(time.Hour * 24 * 365 * 10)
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

func testFileHandler(w http.ResponseWriter, r *http.Request) {
	if reTestFile.MatchString(r.URL.Path) {
		iStr := reTestFile.FindStringSubmatch(r.URL.Path)[1]
		l, err := strconv.Atoi(iStr)
		if err != nil {
			return
		}
		s := reTestFile.FindStringSubmatch(r.URL.Path)[2]
		if len(reTestFile.FindStringSubmatch(r.URL.Path)) > 3 {
			ext := mime.TypeByExtension(reTestFile.FindStringSubmatch(r.URL.Path)[3])
			if len(ext) > 0 {
				w.Header().Set("Content-Type", ext)
			}
		}
		switch s {
		case "b", "B":
			w.Header().Set("Content-Length", strconv.Itoa(l))
			b := make([]byte, l)
			x, _ := w.Write(b)
			serveByte += uint64(x)
		case "k", "K":
			w.Header().Set("Content-Length", strconv.Itoa(l*1024))
			for i := 0; i < l; i++ {
				x, _ := w.Write(bt)
				serveByte += uint64(x)
			}
		case "m", "M":
			w.Header().Set("Content-Length", strconv.Itoa(l*1024*1024))
			// 			var d uint64
			for i := 0; i < l; i++ {
				for j := 0; j < 1024; j++ {
					x, _ := w.Write(bt)
					// 					d += uint64(x)
					serveByte += uint64(x)
				}
				// 				fmt.Printf("\r%10v", humanize.Bytes(d))
			}
			// 			fmt.Printf("\r")
		case "g", "G":
			w.Header().Set("Content-Length", strconv.Itoa(l*1024*1024*1024))
			// 			var d uint64
			for i := 0; i < l; i++ {
				for j := 0; j < 1024; j++ {
					for k := 0; k < 1024; k++ {
						x, _ := w.Write(bt)
						// 						d += uint64(x)
						serveByte += uint64(x)
					}
					// 					fmt.Printf("\r%10v", humanize.Bytes(d))
				}
			}
			// 			fmt.Printf("\r")
		}
	}
}
