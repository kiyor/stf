/* -.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.

* File Name : httpauth.go

* Purpose :

* Creation Date : 12-20-2017

* Last Modified : Thu 21 Dec 2017 12:18:09 AM UTC

* Created By : Kiyor

_._._._._._._._._._._._._._._._._._._._._.*/

package auth

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
)

// parseHTTPAuth parses HTTP authentication from various sources
func parseHTTPAuth(authSource string) (map[string]string, error) {
	authMap := make(map[string]string)

	if authSource == "" {
		return authMap, nil
	}

	// Check if it's a file path
	if _, err := os.Stat(authSource); err == nil {
		// Try to read as JSON first
		data, err := ioutil.ReadFile(authSource)
		if err != nil {
			return nil, fmt.Errorf("failed to read auth file: %v", err)
		}

		// Try JSON format first
		var jsonAuth map[string]string
		if err := json.Unmarshal(data, &jsonAuth); err == nil {
			return jsonAuth, nil
		}

		// Try text format (user:pass per line)
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}

			parts := strings.SplitN(line, ":", 2)
			if len(parts) != 2 {
				continue
			}

			authMap[parts[0]] = parts[1]
		}

		return authMap, nil
	}

	// Treat as a direct user:pass string
	parts := strings.SplitN(authSource, ":", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid auth format: %s", authSource)
	}

	authMap[parts[0]] = parts[1]
	return authMap, nil
}

func httpCheck(user, pass string, authMap map[string]string) bool {
	expectedPass, exists := authMap[user]
	// Compare in constant time; compare against a dummy when the user is unknown
	// so response timing does not reveal valid user names.
	if !exists {
		expectedPass = "\x00invalid"
	}
	ok := subtle.ConstantTimeCompare([]byte(expectedPass), []byte(pass)) == 1
	return exists && ok
}

// ValidateHTTPAuth reports whether authSource yields at least one credential.
// Call it at startup so a typo'd auth file aborts instead of serving openly.
func ValidateHTTPAuth(authSource string) error {
	m, err := parseHTTPAuth(authSource)
	if err != nil {
		return err
	}
	if len(m) == 0 {
		return fmt.Errorf("no credentials found in %q", authSource)
	}
	return nil
}

// HTTPAuth creates an HTTP Basic Authentication middleware. It fails closed:
// if authSource is set but yields no credentials, every request is rejected.
func HTTPAuth(authSource string) func(http.Handler) http.Handler {
	authMap, err := parseHTTPAuth(authSource)
	misconfigured := authSource != "" && (err != nil || len(authMap) == 0)
	if misconfigured {
		log.Printf("http auth %q unusable (%v); rejecting all requests", authSource, err)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if misconfigured {
				http.Error(w, "Server auth misconfigured.", http.StatusInternalServerError)
				return
			}
			if len(authMap) == 0 {
				// No auth configured, allow all
				next.ServeHTTP(w, r)
				return
			}

			user, pass, ok := r.BasicAuth()
			if !ok {
				w.Header().Set("WWW-Authenticate", `Basic realm="auth required"`)
				http.Error(w, "Unauthorized.", http.StatusUnauthorized)
				return
			}

			if !httpCheck(user, pass, authMap) {
				w.Header().Set("WWW-Authenticate", `Basic realm="auth required"`)
				http.Error(w, "Unauthorized.", http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
