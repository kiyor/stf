package middleware

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLogHandler(t *testing.T) {
	// Create a test handler that the LogHandler will wrap
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test response"))
	})

	// Wrap with LogHandler
	logHandler := LogHandler(testHandler)

	// Create test request
	req := httptest.NewRequest("GET", "/test/path", nil)
	req.Header.Set("User-Agent", "test-agent")
	req.Header.Set("X-Custom-Header", "custom-value")

	// Record the response
	w := httptest.NewRecorder()

	// Execute the handler
	logHandler.ServeHTTP(w, req)

	// Verify the response was passed through correctly
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body := w.Body.String()
	if body != "test response" {
		t.Errorf("Expected body 'test response', got %q", body)
	}
}

func TestLogHandlerWithDifferentMethods(t *testing.T) {
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("method: " + r.Method))
	})

	logHandler := LogHandler(testHandler)

	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			var body io.Reader
			if method == "POST" || method == "PUT" || method == "PATCH" {
				body = strings.NewReader("test body")
			}

			req := httptest.NewRequest(method, "/test", body)
			if body != nil {
				req.Header.Set("Content-Type", "text/plain")
			}

			w := httptest.NewRecorder()

			logHandler.ServeHTTP(w, req)

			resp := w.Result()
			if resp.StatusCode != http.StatusOK {
				t.Errorf("Method %s: expected status 200, got %d", method, resp.StatusCode)
			}

			expectedBody := "method: " + method
			// Note: Our test handler includes body for HEAD (test implementation detail)

			responseBody := w.Body.String()
			if responseBody != expectedBody {
				t.Errorf("Method %s: expected body %q, got %q", method, expectedBody, responseBody)
			}
		})
	}
}

func TestLogHandlerWithErrors(t *testing.T) {
	tests := []struct {
		name         string
		statusCode   int
		responseBody string
	}{
		{"Not Found", http.StatusNotFound, "404 Not Found"},
		{"Internal Server Error", http.StatusInternalServerError, "500 Internal Server Error"},
		{"Bad Request", http.StatusBadRequest, "400 Bad Request"},
		{"Unauthorized", http.StatusUnauthorized, "401 Unauthorized"},
		{"Forbidden", http.StatusForbidden, "403 Forbidden"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.responseBody))
			})

			logHandler := LogHandler(testHandler)

			req := httptest.NewRequest("GET", "/test", nil)
			w := httptest.NewRecorder()

			logHandler.ServeHTTP(w, req)

			resp := w.Result()
			if resp.StatusCode != tt.statusCode {
				t.Errorf("Expected status %d, got %d", tt.statusCode, resp.StatusCode)
			}

			body := w.Body.String()
			if body != tt.responseBody {
				t.Errorf("Expected body %q, got %q", tt.responseBody, body)
			}
		})
	}
}

func TestLogHandlerPassesHeaders(t *testing.T) {
	// Test that headers are correctly passed through
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Custom-Response", "custom-response-value")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message": "success"}`))
	})

	logHandler := LogHandler(testHandler)

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Authorization", "Bearer token123")
	req.Header.Set("Accept", "application/json")

	w := httptest.NewRecorder()

	logHandler.ServeHTTP(w, req)

	resp := w.Result()

	// Check response headers are preserved
	if resp.Header.Get("X-Custom-Response") != "custom-response-value" {
		t.Error("Custom response header not preserved")
	}

	if resp.Header.Get("Content-Type") != "application/json" {
		t.Error("Content-Type header not preserved")
	}

	// Check response body
	body := w.Body.String()
	expectedBody := `{"message": "success"}`
	if body != expectedBody {
		t.Errorf("Expected body %q, got %q", expectedBody, body)
	}
}

func TestLogHandlerWithQueryParams(t *testing.T) {
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params := r.URL.Query()
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("param1=" + params.Get("param1")))
	})

	logHandler := LogHandler(testHandler)

	req := httptest.NewRequest("GET", "/test?param1=value1&param2=value2", nil)
	w := httptest.NewRecorder()

	logHandler.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body := w.Body.String()
	if body != "param1=value1" {
		t.Errorf("Expected body 'param1=value1', got %q", body)
	}
}

func TestLogHandlerWithRequestBody(t *testing.T) {
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Read the request body
		buf := new(bytes.Buffer)
		buf.ReadFrom(r.Body)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("received: " + buf.String()))
	})

	logHandler := LogHandler(testHandler)

	requestBody := "test request data"
	req := httptest.NewRequest("POST", "/test", strings.NewReader(requestBody))
	req.Header.Set("Content-Type", "text/plain")

	w := httptest.NewRecorder()

	logHandler.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body := w.Body.String()
	expected := "received: " + requestBody
	if body != expected {
		t.Errorf("Expected body %q, got %q", expected, body)
	}
}

func TestLogHandlerPreservesContext(t *testing.T) {
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Test that request context is preserved
		if r.Context() == nil {
			t.Error("Request context is nil")
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("context preserved"))
	})

	logHandler := LogHandler(testHandler)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	logHandler.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestLogHandlerChaining(t *testing.T) {
	// Test that LogHandler can be chained with other middleware
	innerHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("final handler"))
	})

	// Chain multiple LogHandlers (simulating middleware stack)
	handler := LogHandler(LogHandler(innerHandler))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body := w.Body.String()
	if body != "final handler" {
		t.Errorf("Expected body 'final handler', got %q", body)
	}
}
