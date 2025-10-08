package config

import (
	"fmt"
	"net"
	"net/http"
	"strings"
)

// offlineTransport blocks non-localhost HTTP requests
type offlineTransport struct {
	wrapped http.RoundTripper
}

// RoundTrip implements http.RoundTripper
func (t *offlineTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	host := req.URL.Hostname()

	// Allow localhost and local IPs
	if isLocalhost(host) {
		if t.wrapped != nil {
			return t.wrapped.RoundTrip(req)
		}
		return http.DefaultTransport.RoundTrip(req)
	}

	return nil, fmt.Errorf("offline mode: network egress blocked for %s", req.URL.Host)
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
func EnableOfflineMode() {
	http.DefaultTransport = &offlineTransport{
		wrapped: http.DefaultTransport,
	}
}
