/* -.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.

* File Name : main.go

* Purpose :

* Creation Date : 12-14-2015

* Last Modified : Sun 03 Jan 2016 12:00:48 PM PST

* Created By : Kiyor

_._._._._._._._._._._._._._._._._._._._._.*/

package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

var dir = "."
var port = ":30000"

func init() {
	if len(os.Args) > 1 {
		dir = os.Args[1]
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
			s += strings.Split(v.String(), "/")[0] + port + " "
		}
	}
	return s
}

func main() {

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Add("Cache-Control", "no-cache")
		if req.Method == "GET" {
			f := &fileHandler{http.Dir(dir)}
			f.ServeHTTP(w, req)
		} else if req.Method == "POST" {
			uploadHandler(w, req)
		}
		log.Println(req.Method, req.URL.Path)
	})

	log.Println("Listening on", getips())
	http.ListenAndServe(port, mux)
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	p := dir + "/" + r.URL.Path
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
