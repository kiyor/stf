/* -.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.

* File Name : httpauth.go

* Purpose :

* Creation Date : 12-20-2017

* Last Modified : Thu 21 Dec 2017 12:18:09 AM UTC

* Created By : Kiyor

_._._._._._._._._._._._._._._._._._._._._.*/

package main

import (
	"net/http"
	"strings"
)

func httpCheck(user, pass string) bool {
	for _, v := range []string{" ", ":"} {
		if strings.Contains(*httpAuthFlag, v) {
			p := strings.Split(*httpAuthFlag, v)
			for i := 0; i < len(p); i += 2 {
				if user == p[i] && pass == p[i+1] {
					return true
				}
			}
		}
	}
	return false
}

func httpAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, _ := r.BasicAuth()
		if !httpCheck(user, pass) {
			w.Header().Set("WWW-Authenticate", `Basic realm="auth required"`)
			http.Error(w, "Unauthorized.", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
