package config

import (
	"net/http"
	"strings"
	"testing"
)

func TestOfflineMode(t *testing.T) {
	// Enable offline mode
	EnableOfflineMode()

	// Test that external requests are blocked
	_, err := http.Get("https://example.com")
	if err == nil {
		t.Fatal("expected error for external request in offline mode")
	}

	if !strings.Contains(err.Error(), "offline mode") {
		t.Errorf("expected 'offline mode' in error, got: %v", err)
	}

	// Test that localhost is allowed (though it may fail to connect)
	client := &http.Client{
		Transport: &offlineTransport{wrapped: http.DefaultTransport},
	}

	req, _ := http.NewRequest("GET", "http://localhost:8181", nil)
	_, err = client.Do(req)
	// Error is OK (connection refused), but should not be "offline mode" error
	if err != nil && strings.Contains(err.Error(), "offline mode") {
		t.Errorf("localhost should be allowed, got: %v", err)
	}
}

func TestIsLocalhost(t *testing.T) {
	tests := []struct {
		host     string
		expected bool
	}{
		{"localhost", true},
		{"127.0.0.1", true},
		{"::1", true},
		{"opa", true}, // Docker compose hostname
		{"api", true},
		{"192.168.1.1", true}, // Private IP
		{"10.0.0.1", true},    // Private IP
		{"example.com", false},
		{"8.8.8.8", false},
	}

	for _, tt := range tests {
		t.Run(tt.host, func(t *testing.T) {
			result := isLocalhost(tt.host)
			if result != tt.expected {
				t.Errorf("isLocalhost(%q) = %v, want %v", tt.host, result, tt.expected)
			}
		})
	}
}
