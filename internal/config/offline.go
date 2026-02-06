package config

import (
	"fmt"
	"net"
	"net/http"
	"strings"
)

// offlineTransport blocks non-localhost HTTP requests with IMDS protection
type offlineTransport struct {
	wrapped      http.RoundTripper
	blockIMDS    bool
	blockRFC1918 bool
}

// RoundTrip implements http.RoundTripper
func (t *offlineTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	host := req.URL.Hostname()

	// ALWAYS block IMDS (169.254.169.254) - critical security control
	if t.blockIMDS && isIMDS(host) {
		return nil, fmt.Errorf("security: IMDS access blocked (169.254.169.254)")
	}

	// Allow localhost and local IPs (check this before RFC1918 blocking)
	if isLocalhost(host) {
		if t.wrapped != nil {
			return t.wrapped.RoundTrip(req)
		}
		return http.DefaultTransport.RoundTrip(req)
	}

	// Block RFC1918 egress when offline
	if t.blockRFC1918 && isRFC1918External(host) {
		return nil, fmt.Errorf("offline mode: external network access blocked for %s", req.URL.Host)
	}

	return nil, fmt.Errorf("offline mode: network egress blocked for %s", req.URL.Host)
}

func isIMDS(host string) bool {
	// AWS EC2 Instance Metadata Service
	return host == "169.254.169.254"
}

func isRFC1918External(host string) bool {
	ip := net.ParseIP(host)
	if ip == nil {
		// If not an IP, consider it external
		return true
	}

	// Check if it's private/local (RFC1918, loopback, etc.)
	return !ip.IsLoopback() && !ip.IsPrivate()
}

func isLocalhost(host string) bool {
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return true
	}

	// Check for docker/compose hostnames (opa, api, ui, etc.)
	if !strings.Contains(host, ".") {
		return true
	}

	// Check if it's a local IP
	ip := net.ParseIP(host)
	if ip != nil {
		return ip.IsLoopback() || ip.IsPrivate()
	}

	return false
}

// EnableOfflineMode wraps http.DefaultTransport to block external requests
// IMDS is ALWAYS blocked regardless of offline setting
func EnableOfflineMode(alwaysBlockIMDS bool) {
	http.DefaultTransport = &offlineTransport{
		wrapped:      http.DefaultTransport,
		blockIMDS:    alwaysBlockIMDS,
		blockRFC1918: true,
	}
}

// EnableOnlineModeWithIMDSBlock allows external network but blocks IMDS
func EnableOnlineModeWithIMDSBlock() {
	http.DefaultTransport = &offlineTransport{
		wrapped:      http.DefaultTransport,
		blockIMDS:    true,
		blockRFC1918: false,
	}
}
