package config

import (
	"flag"
	"testing"
)

func TestFlagSliceStringString(t *testing.T) {
	var fs FlagSliceString

	// Test empty slice
	result := fs.String()
	if result != "" {
		t.Errorf("Empty FlagSliceString.String() = %q, want empty string", result)
	}

	// Test with values - should still return empty string based on implementation
	fs = FlagSliceString{"value1", "value2"}
	result = fs.String()
	if result != "" {
		t.Errorf("FlagSliceString.String() = %q, want empty string", result)
	}
}

func TestFlagSliceStringSet(t *testing.T) {
	var fs FlagSliceString

	tests := []struct {
		value    string
		expected []string
	}{
		{"first", []string{"first"}},
		{"second", []string{"first", "second"}},
		{"third", []string{"first", "second", "third"}},
		{"", []string{"first", "second", "third", ""}}, // Empty strings should be allowed
	}

	for i, tt := range tests {
		err := fs.Set(tt.value)
		if err != nil {
			t.Errorf("Step %d: FlagSliceString.Set(%q) returned error: %v", i, tt.value, err)
		}

		if len(fs) != len(tt.expected) {
			t.Errorf("Step %d: Length = %d, want %d", i, len(fs), len(tt.expected))
		}

		for j, expected := range tt.expected {
			if j >= len(fs) || fs[j] != expected {
				t.Errorf("Step %d: fs[%d] = %q, want %q", i, j, fs[j], expected)
			}
		}
	}
}

func TestFlagSliceStringSetMultiple(t *testing.T) {
	var fs FlagSliceString

	values := []string{"192.168.1.1", "10.0.0.0/8", "172.16.0.0/12", "localhost"}

	for _, value := range values {
		err := fs.Set(value)
		if err != nil {
			t.Errorf("FlagSliceString.Set(%q) returned error: %v", value, err)
		}
	}

	if len(fs) != len(values) {
		t.Errorf("Final length = %d, want %d", len(fs), len(values))
	}

	for i, expected := range values {
		if fs[i] != expected {
			t.Errorf("fs[%d] = %q, want %q", i, fs[i], expected)
		}
	}
}

func TestFlagSliceStringWithFlag(t *testing.T) {
	// Test integration with the flag package
	// Reset the FlagAllowIP for testing
	originalAllowIP := FlagAllowIP
	originalDenyIP := FlagDenyIP
	defer func() {
		FlagAllowIP = originalAllowIP
		FlagDenyIP = originalDenyIP
	}()

	// Reset global variables
	FlagAllowIP = FlagSliceString{}
	FlagDenyIP = FlagSliceString{}

	// Create a new FlagSet for testing
	fs := flag.NewFlagSet("test", flag.ContinueOnError)

	// Register our custom flags
	fs.Var(&FlagAllowIP, "allow", "allow IP")
	fs.Var(&FlagDenyIP, "deny", "deny IP")

	// Test parsing multiple allow/deny flags
	args := []string{
		"-allow", "192.168.1.1",
		"-allow", "10.0.0.0/8",
		"-deny", "1.2.3.4",
		"-deny", "5.6.7.0/24",
		"-allow", "localhost",
	}

	err := fs.Parse(args)
	if err != nil {
		t.Errorf("Flag parsing failed: %v", err)
	}

	// Verify allow list
	expectedAllow := []string{"192.168.1.1", "10.0.0.0/8", "localhost"}
	if len(FlagAllowIP) != len(expectedAllow) {
		t.Errorf("FlagAllowIP length = %d, want %d", len(FlagAllowIP), len(expectedAllow))
	}

	for i, expected := range expectedAllow {
		if i >= len(FlagAllowIP) || FlagAllowIP[i] != expected {
			t.Errorf("FlagAllowIP[%d] = %q, want %q", i, FlagAllowIP[i], expected)
		}
	}

	// Verify deny list
	expectedDeny := []string{"1.2.3.4", "5.6.7.0/24"}
	if len(FlagDenyIP) != len(expectedDeny) {
		t.Errorf("FlagDenyIP length = %d, want %d", len(FlagDenyIP), len(expectedDeny))
	}

	for i, expected := range expectedDeny {
		if i >= len(FlagDenyIP) || FlagDenyIP[i] != expected {
			t.Errorf("FlagDenyIP[%d] = %q, want %q", i, FlagDenyIP[i], expected)
		}
	}
}

func TestFlagSliceStringReset(t *testing.T) {
	// Test that we can reset the slice
	var fs FlagSliceString

	// Add some values
	fs.Set("value1")
	fs.Set("value2")

	if len(fs) != 2 {
		t.Errorf("Initial length = %d, want 2", len(fs))
	}

	// Reset by creating new instance
	fs = FlagSliceString{}

	if len(fs) != 0 {
		t.Errorf("After reset length = %d, want 0", len(fs))
	}

	// Verify we can still add values after reset
	err := fs.Set("new_value")
	if err != nil {
		t.Errorf("Set after reset failed: %v", err)
	}

	if len(fs) != 1 || fs[0] != "new_value" {
		t.Errorf("After reset and set: fs = %v, want [new_value]", fs)
	}
}

func TestFlagSliceStringEdgeCases(t *testing.T) {
	var fs FlagSliceString

	// Test with special characters
	specialValues := []string{
		"192.168.1.1/32",
		"2001:db8::/32", // IPv6
		"example.com",
		"user@host.com",
		"value with spaces",
		"value\twith\ttabs",
		"value\nwith\nnewlines",
		"特殊字符", // Unicode characters
		"",     // Empty string
		"   ",  // Whitespace only
	}

	for i, value := range specialValues {
		err := fs.Set(value)
		if err != nil {
			t.Errorf("Set(%q) returned error: %v", value, err)
		}

		if len(fs) != i+1 {
			t.Errorf("After setting %q: length = %d, want %d", value, len(fs), i+1)
		}

		if fs[i] != value {
			t.Errorf("fs[%d] = %q, want %q", i, fs[i], value)
		}
	}
}

func TestGlobalFlagVariables(t *testing.T) {
	// Test that global variables are properly initialized
	if FlagAllowIP == nil {
		t.Error("FlagAllowIP should be initialized")
	}

	if FlagDenyIP == nil {
		t.Error("FlagDenyIP should be initialized")
	}

	// Test that they behave as expected
	originalLen := len(FlagAllowIP)

	err := FlagAllowIP.Set("test.example.com")
	if err != nil {
		t.Errorf("FlagAllowIP.Set() failed: %v", err)
	}

	if len(FlagAllowIP) != originalLen+1 {
		t.Errorf("FlagAllowIP length after set = %d, want %d", len(FlagAllowIP), originalLen+1)
	}

	// Clean up - remove the test value
	if len(FlagAllowIP) > 0 && FlagAllowIP[len(FlagAllowIP)-1] == "test.example.com" {
		FlagAllowIP = FlagAllowIP[:len(FlagAllowIP)-1]
	}
}

func TestFlagSliceStringImplementsValue(t *testing.T) {
	// Test that FlagSliceString implements flag.Value interface
	var fs FlagSliceString

	// This should compile if it implements flag.Value
	var _ flag.Value = &fs

	// Test the interface methods
	stringResult := fs.String()
	if stringResult != "" {
		t.Errorf("String() = %q, want empty string", stringResult)
	}

	err := fs.Set("test_value")
	if err != nil {
		t.Errorf("Set() = %v, want nil", err)
	}
}
