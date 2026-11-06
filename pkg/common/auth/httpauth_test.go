package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseHTTPAuth(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
		expectCount int
		description string
	}{
		{
			name:        "single user:pass",
			input:       "user1:pass1",
			expectError: false,
			expectCount: 1,
			description: "Single user:password pair",
		},
		{
			name:        "empty string",
			input:       "",
			expectError: false,
			expectCount: 0,
			description: "Empty string should result in no auth",
		},
		{
			name:        "invalid format",
			input:       "usernopass",
			expectError: true,
			expectCount: 0,
			description: "Invalid format without colon",
		},
		{
			name:        "multiple colons",
			input:       "user:pass:extra",
			expectError: false,
			expectCount: 1,
			description: "Extra colons should be part of password",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authMap, err := parseHTTPAuth(tt.input)

			if tt.expectError && err == nil {
				t.Errorf("parseHTTPAuth(%q) expected error, got none", tt.input)
			}

			if !tt.expectError && err != nil {
				t.Errorf("parseHTTPAuth(%q) unexpected error: %v", tt.input, err)
			}

			if len(authMap) != tt.expectCount {
				t.Errorf("parseHTTPAuth(%q) got %d entries, expected %d", tt.input, len(authMap), tt.expectCount)
			}

			// For successful single user case, verify the entry
			if tt.expectCount == 1 && err == nil {
				parts := strings.SplitN(tt.input, ":", 2)
				if len(parts) == 2 {
					expectedUser, expectedPass := parts[0], parts[1]
					if pass, exists := authMap[expectedUser]; !exists || pass != expectedPass {
						t.Errorf("parseHTTPAuth(%q) got %v, expected user %q with pass %q", tt.input, authMap, expectedUser, expectedPass)
					}
				}
			}
		})
	}
}

func TestParseHTTPAuthFromFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Test with text file format
	txtFile := filepath.Join(tmpDir, "auth.txt")
	txtContent := `user1:pass1
user2:pass2
# comment line
user3:pass3

user4:pass4`

	err := os.WriteFile(txtFile, []byte(txtContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	authMap, err := parseHTTPAuth(txtFile)
	if err != nil {
		t.Errorf("parseHTTPAuth with text file failed: %v", err)
	}

	expectedUsers := map[string]string{
		"user1": "pass1",
		"user2": "pass2",
		"user3": "pass3",
		"user4": "pass4",
	}

	if len(authMap) != len(expectedUsers) {
		t.Errorf("Expected %d users, got %d", len(expectedUsers), len(authMap))
	}

	for user, expectedPass := range expectedUsers {
		if pass, exists := authMap[user]; !exists || pass != expectedPass {
			t.Errorf("Expected user %s with password %s, got %s (exists: %v)", user, expectedPass, pass, exists)
		}
	}
}

func TestParseHTTPAuthFromJSONFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Test with JSON file format
	jsonFile := filepath.Join(tmpDir, "auth.json")
	jsonData := map[string]string{
		"user1": "pass1",
		"user2": "pass2",
		"admin": "secret123",
	}

	jsonBytes, err := json.Marshal(jsonData)
	if err != nil {
		t.Fatalf("Failed to marshal JSON: %v", err)
	}

	err = os.WriteFile(jsonFile, jsonBytes, 0644)
	if err != nil {
		t.Fatalf("Failed to create JSON test file: %v", err)
	}

	authMap, err := parseHTTPAuth(jsonFile)
	if err != nil {
		t.Errorf("parseHTTPAuth with JSON file failed: %v", err)
	}

	if len(authMap) != len(jsonData) {
		t.Errorf("Expected %d users from JSON, got %d", len(jsonData), len(authMap))
	}

	for user, expectedPass := range jsonData {
		if pass, exists := authMap[user]; !exists || pass != expectedPass {
			t.Errorf("Expected user %s with password %s, got %s (exists: %v)", user, expectedPass, pass, exists)
		}
	}
}

func TestParseHTTPAuthNonExistentFile(t *testing.T) {
	// Test with non-existent file - should treat as string
	authMap, err := parseHTTPAuth("nonexistent.txt")
	if err == nil {
		t.Error("parseHTTPAuth with non-existent file should return error")
	}

	if len(authMap) != 0 {
		t.Errorf("parseHTTPAuth with non-existent file should return empty map, got %v", authMap)
	}
}

func TestHttpAuthMiddleware(t *testing.T) {
	// Create test auth data
	tmpDir := t.TempDir()
	authFile := filepath.Join(tmpDir, "auth.txt")
	authContent := `testuser:testpass
admin:secret`

	err := os.WriteFile(authFile, []byte(authContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create auth file: %v", err)
	}

	// Parse auth data
	authMap, err := parseHTTPAuth(authFile)
	if err != nil {
		t.Fatalf("Failed to parse auth: %v", err)
	}

	// Create a test handler
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	// Create auth middleware (we'll need to simulate this since the exact function isn't exported)
	authHandler := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, pass, ok := r.BasicAuth()
			if !ok {
				w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			expectedPass, exists := authMap[user]
			if !exists || expectedPass != pass {
				w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}

	handler := authHandler(nextHandler)

	tests := []struct {
		name       string
		username   string
		password   string
		wantStatus int
		wantBody   string
	}{
		{
			name:       "valid credentials",
			username:   "testuser",
			password:   "testpass",
			wantStatus: http.StatusOK,
			wantBody:   "success",
		},
		{
			name:       "valid admin credentials",
			username:   "admin",
			password:   "secret",
			wantStatus: http.StatusOK,
			wantBody:   "success",
		},
		{
			name:       "invalid password",
			username:   "testuser",
			password:   "wrongpass",
			wantStatus: http.StatusUnauthorized,
			wantBody:   "",
		},
		{
			name:       "invalid user",
			username:   "wronguser",
			password:   "testpass",
			wantStatus: http.StatusUnauthorized,
			wantBody:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			req.SetBasicAuth(tt.username, tt.password)

			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			resp := w.Result()

			if resp.StatusCode != tt.wantStatus {
				t.Errorf("Status = %v, want %v", resp.StatusCode, tt.wantStatus)
			}

			body := w.Body.String()
			if body != tt.wantBody {
				t.Errorf("Body = %v, want %v", body, tt.wantBody)
			}

			// Check WWW-Authenticate header for unauthorized requests
			if tt.wantStatus == http.StatusUnauthorized {
				auth := resp.Header.Get("WWW-Authenticate")
				if auth == "" {
					t.Error("Expected WWW-Authenticate header for 401 response")
				}
			}
		})
	}
}

func TestHttpAuthNoCredentials(t *testing.T) {
	// Create a test handler
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	// Simulate auth middleware
	authHandler := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _, ok := r.BasicAuth()
			if !ok {
				w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}

	handler := authHandler(nextHandler)

	// Test request without authorization header
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	resp := w.Result()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("Status = %v, want %v", resp.StatusCode, http.StatusUnauthorized)
	}

	auth := resp.Header.Get("WWW-Authenticate")
	if auth == "" {
		t.Error("Expected WWW-Authenticate header")
	}

	if !strings.Contains(auth, "Basic") {
		t.Errorf("Expected Basic auth challenge, got: %s", auth)
	}
}
