package config

import (
	"net/http"
	"strings"
	"testing"
)

func TestOfflineMode(t *testing.T) {
	// Enable offline mode with IMDS blocking
	EnableOfflineMode(true)

	// Test that external requests are blocked
	_, err := http.Get("https://example.com")
	if err == nil {
		t.Fatal("expected error for external request in offline mode")
	}

	if !strings.Contains(err.Error(), "offline mode") && !strings.Contains(err.Error(), "blocked") {
		t.Errorf("expected 'offline mode' or 'blocked' in error, got: %v", err)
	}

	// Test that localhost is allowed (though it may fail to connect)
	client := &http.Client{
		Transport: &offlineTransport{
			wrapped:      http.DefaultTransport,
			blockIMDS:    true,
			blockRFC1918: true,
		},
	}

	req, _ := http.NewRequest("GET", "http://localhost:8181", nil)
	_, err = client.Do(req)
	// Error is OK (connection refused), but should not be "offline mode" error
	if err != nil && strings.Contains(err.Error(), "offline mode") {
		t.Errorf("localhost should be allowed, got: %v", err)
	}
}

func TestIMDSBlocking(t *testing.T) {
	tests := []struct {
		name      string
		blockIMDS bool
		url       string
		wantBlock bool
	}{
		{
			name:      "IMDS blocked when enabled",
			blockIMDS: true,
			url:       "http://169.254.169.254/latest/meta-data/",
			wantBlock: true,
		},
		{
			name:      "IMDS allowed when disabled (should not happen in prod)",
			blockIMDS: false,
			url:       "http://169.254.169.254/latest/meta-data/",
			wantBlock: false,
		},
		{
			name:      "Localhost not affected by IMDS block",
			blockIMDS: true,
			url:       "http://localhost:8080",
			wantBlock: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &http.Client{
				Transport: &offlineTransport{
					wrapped:      http.DefaultTransport,
					blockIMDS:    tt.blockIMDS,
					blockRFC1918: false,
				},
			}

			req, _ := http.NewRequest("GET", tt.url, nil)
			_, err := client.Do(req)

			if tt.wantBlock {
				if err == nil {
					t.Errorf("expected IMDS to be blocked but got no error")
				} else if !strings.Contains(err.Error(), "IMDS") && !strings.Contains(err.Error(), "security") {
					t.Errorf("expected IMDS/security error, got: %v", err)
				}
			} else if tt.blockIMDS == false && tt.url == "http://169.254.169.254/latest/meta-data/" {
				// This case is for testing the logic, not production behavior
				// We expect no IMDS-specific error (but connection may still fail)
				if err != nil && (strings.Contains(err.Error(), "IMDS") || strings.Contains(err.Error(), "security")) {
					t.Errorf("IMDS should not be blocked when blockIMDS=false, got: %v", err)
				}
			}
		})
	}
}

func TestOnlineModeWithIMDSBlock(t *testing.T) {
	EnableOnlineModeWithIMDSBlock()

	// Test that IMDS is still blocked in online mode
	client := &http.Client{
		Transport: &offlineTransport{
			wrapped:      http.DefaultTransport,
			blockIMDS:    true,
			blockRFC1918: false,
		},
	}

	req, _ := http.NewRequest("GET", "http://169.254.169.254/latest/meta-data/", nil)
	_, err := client.Do(req)

	if err == nil {
		t.Fatal("expected IMDS to be blocked in online mode")
	}

	if !strings.Contains(err.Error(), "IMDS") && !strings.Contains(err.Error(), "security") {
		t.Errorf("expected IMDS/security error, got: %v", err)
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
		{"169.254.169.254", false}, // IMDS should not be considered localhost
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

func TestIsIMDS(t *testing.T) {
	tests := []struct {
		host     string
		expected bool
	}{
		{"169.254.169.254", true},
		{"169.254.169.253", false},
		{"localhost", false},
		{"127.0.0.1", false},
		{"example.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.host, func(t *testing.T) {
			result := isIMDS(tt.host)
			if result != tt.expected {
				t.Errorf("isIMDS(%q) = %v, want %v", tt.host, result, tt.expected)
			}
		})
	}
}
