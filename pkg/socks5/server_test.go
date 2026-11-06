package socks5

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestReadHosts(t *testing.T) {
	// Create a temporary hosts file
	tmpDir := t.TempDir()
	hostsFile := filepath.Join(tmpDir, "hosts")

	content := `127.0.0.1 localhost
192.168.1.1 router gateway
10.0.0.1 server  web-server
# This is a comment
  # Another comment
192.168.1.10   host1    host2
`

	err := os.WriteFile(hostsFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create hosts file: %v", err)
	}

	// Test reading the hosts file
	err = readHosts(hostsFile)
	if err != nil {
		t.Errorf("readHosts() failed: %v", err)
	}

	// Test that hosts are loaded correctly
	tests := []struct {
		hostname    string
		expectedIP  string
		shouldExist bool
	}{
		{"localhost", "127.0.0.1", true},
		{"router", "192.168.1.1", true},
		{"gateway", "192.168.1.1", true},
		{"server", "10.0.0.1", true},
		{"web-server", "10.0.0.1", true},
		{"host1", "192.168.1.10", true},
		{"host2", "192.168.1.10", true},
		{"nonexistent", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.hostname, func(t *testing.T) {
			hostsLocker.RLock()
			ip, exists := hostsBind[tt.hostname]
			hostsLocker.RUnlock()

			if exists != tt.shouldExist {
				t.Errorf("Host %s existence = %v, want %v", tt.hostname, exists, tt.shouldExist)
				return
			}

			if tt.shouldExist && ip.String() != tt.expectedIP {
				t.Errorf("Host %s IP = %v, want %v", tt.hostname, ip.String(), tt.expectedIP)
			}
		})
	}
}

func TestReadHostsInvalidFile(t *testing.T) {
	// Test with non-existent file
	err := readHosts("/nonexistent/hosts")
	if err != nil {
		t.Errorf("readHosts() with non-existent file should not error, got: %v", err)
	}

	// hostsBind should be reset to empty map
	hostsLocker.RLock()
	count := len(hostsBind)
	hostsLocker.RUnlock()

	if count != 0 {
		t.Errorf("hostsBind should be empty after reading invalid file, got %d entries", count)
	}
}

func TestReadHostsWithComments(t *testing.T) {
	tmpDir := t.TempDir()
	hostsFile := filepath.Join(tmpDir, "hosts")

	content := `# Comment at start
127.0.0.1 localhost # inline comment
# Another full line comment
192.168.1.1 router
`

	err := os.WriteFile(hostsFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create hosts file: %v", err)
	}

	err = readHosts(hostsFile)
	if err != nil {
		t.Errorf("readHosts() failed: %v", err)
	}

	// Check that valid entries are loaded
	hostsLocker.RLock()
	_, localhostExists := hostsBind["localhost"]
	_, routerExists := hostsBind["router"]
	hostsLocker.RUnlock()

	if !localhostExists {
		t.Error("localhost should be loaded from hosts file")
	}

	if !routerExists {
		t.Error("router should be loaded from hosts file")
	}
}

func TestCleanFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test")

	content := `line1
# comment
line2

line3
# another comment
line4`

	err := os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	lines, err := cleanFile(testFile)
	if err != nil {
		t.Errorf("cleanFile() failed: %v", err)
	}

	// Check that comments and empty lines are filtered
	expectedLines := []string{"line1", "line2", "line3", "line4"}

	if len(lines) != len(expectedLines) {
		t.Errorf("cleanFile() returned %d lines, expected %d", len(lines), len(expectedLines))
	}

	for i, expected := range expectedLines {
		if i >= len(lines) || lines[i] != expected {
			t.Errorf("cleanFile() line %d = %q, want %q", i, lines[i], expected)
		}
	}
}

func TestCleanFileNonExistent(t *testing.T) {
	lines, err := cleanFile("/nonexistent/file")

	// Should return empty slice and no error
	if err != nil {
		t.Errorf("cleanFile() with non-existent file should not error, got: %v", err)
	}

	if len(lines) != 0 {
		t.Errorf("cleanFile() should return empty slice for non-existent file, got %d lines", len(lines))
	}
}

func TestCreateSocks5Config(t *testing.T) {
	tests := []struct {
		name        string
		auth        string
		hosts       string
		expectAuth  bool
		expectHosts bool
		expectError bool
	}{
		{
			name:        "no auth, no hosts",
			auth:        "",
			hosts:       "",
			expectAuth:  false,
			expectHosts: false,
			expectError: false,
		},
		{
			name:        "with auth file",
			auth:        "testauth.txt",
			hosts:       "",
			expectAuth:  true,
			expectHosts: false,
			expectError: false,
		},
		{
			name:        "with hosts file",
			auth:        "",
			hosts:       "testhosts.txt",
			expectAuth:  false,
			expectHosts: false, // Changed to false since we disabled resolver
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			var authFile, hostsFile string

			if tt.auth != "" {
				authFile = filepath.Join(tmpDir, tt.auth)
				authContent := `user1:pass1
user2:pass2`
				err := os.WriteFile(authFile, []byte(authContent), 0644)
				if err != nil {
					t.Fatalf("Failed to create auth file: %v", err)
				}
			}

			if tt.hosts != "" {
				hostsFile = filepath.Join(tmpDir, tt.hosts)
				hostsContent := `127.0.0.1 localhost
192.168.1.1 router`
				err := os.WriteFile(hostsFile, []byte(hostsContent), 0644)
				if err != nil {
					t.Fatalf("Failed to create hosts file: %v", err)
				}
			}

			config, err := createSocks5Config(authFile, hostsFile)

			if tt.expectError && err == nil {
				t.Error("createSocks5Config() should have returned error")
			}

			if !tt.expectError && err != nil {
				t.Errorf("createSocks5Config() unexpected error: %v", err)
			}

			if config == nil && !tt.expectError {
				t.Error("createSocks5Config() returned nil config")
				return
			}

			if config != nil {
				hasAuth := config.Credentials != nil
				if hasAuth != tt.expectAuth {
					t.Errorf("createSocks5Config() auth = %v, want %v", hasAuth, tt.expectAuth)
				}

				hasResolver := config.Resolver != nil
				if hasResolver != tt.expectHosts {
					t.Errorf("createSocks5Config() resolver = %v, want %v", hasResolver, tt.expectHosts)
				}
			}
		})
	}
}

// Mock test to verify the basic structure and exported functions exist
func TestSocks5ServerStructure(t *testing.T) {
	// Test that key variables exist
	if hostsBind == nil {
		t.Error("hostsBind should be initialized")
	}

	if hostsLocker == nil {
		t.Error("hostsLocker should be initialized")
	}

	// Test that functions don't panic with basic calls
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Basic function calls should not panic: %v", r)
		}
	}()

	// Test cleanFile with non-existent file
	_, err := cleanFile("")
	if err != nil {
		t.Errorf("cleanFile with empty string should not error: %v", err)
	}
}

func TestHostsThreadSafety(t *testing.T) {
	// Test that concurrent access to hostsBind doesn't cause races
	tmpDir := t.TempDir()
	hostsFile := filepath.Join(tmpDir, "hosts")

	content := `127.0.0.1 localhost
192.168.1.1 router`

	err := os.WriteFile(hostsFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create hosts file: %v", err)
	}

	// Start multiple goroutines reading and writing
	done := make(chan bool, 3)

	// Goroutine 1: Read hosts file repeatedly
	go func() {
		for i := 0; i < 10; i++ {
			readHosts(hostsFile)
			time.Sleep(time.Millisecond)
		}
		done <- true
	}()

	// Goroutine 2: Read from hostsBind
	go func() {
		for i := 0; i < 50; i++ {
			hostsLocker.RLock()
			_ = len(hostsBind)
			hostsLocker.RUnlock()
			time.Sleep(time.Millisecond / 5)
		}
		done <- true
	}()

	// Goroutine 3: Another reader
	go func() {
		for i := 0; i < 50; i++ {
			hostsLocker.RLock()
			_, exists := hostsBind["localhost"]
			_ = exists
			hostsLocker.RUnlock()
			time.Sleep(time.Millisecond / 5)
		}
		done <- true
	}()

	// Wait for all goroutines to complete
	timeout := time.After(5 * time.Second)
	for i := 0; i < 3; i++ {
		select {
		case <-done:
			// OK
		case <-timeout:
			t.Fatal("Test timed out - possible deadlock")
		}
	}
}
