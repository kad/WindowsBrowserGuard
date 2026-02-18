package telemetry

import (
	"fmt"
	"net/url"
	"strings"
)

// ParseOTLPEndpoint parses a raw OTLP endpoint URL into the host:port string,
// protocol ("grpc" or "http"), and whether TLS is disabled (insecure).
//
// Supported URL schemes:
//
//	grpc://host[:port]   — gRPC, no TLS  (default port 4317)
//	grpcs://host[:port]  — gRPC, TLS     (default port 443)
//	http://host[:port]   — HTTP, no TLS  (default port 4318)
//	https://host[:port]  — HTTP, TLS     (default port 443)
//	host:port            — gRPC, no TLS  (legacy bare address, backward-compatible)
func ParseOTLPEndpoint(rawURL string) (endpoint, protocol string, insecure bool, err error) {
	if rawURL == "" {
		return "", "", false, nil
	}

	// Legacy bare host:port with no scheme — treat as insecure gRPC
	if !strings.Contains(rawURL, "://") {
		return rawURL, "grpc", true, nil
	}

	u, parseErr := url.Parse(rawURL)
	if parseErr != nil {
		return "", "", false, fmt.Errorf("invalid OTLP endpoint %q: %w", rawURL, parseErr)
	}

	host := u.Hostname()
	if host == "" {
		return "", "", false, fmt.Errorf("invalid OTLP endpoint %q: missing host", rawURL)
	}
	port := u.Port()

	switch strings.ToLower(u.Scheme) {
	case "grpc":
		if port == "" {
			port = "4317"
		}
		return host + ":" + port, "grpc", true, nil
	case "grpcs":
		if port == "" {
			port = "443"
		}
		return host + ":" + port, "grpc", false, nil
	case "http":
		if port == "" {
			port = "4318"
		}
		return host + ":" + port, "http", true, nil
	case "https":
		if port == "" {
			port = "443"
		}
		return host + ":" + port, "http", false, nil
	default:
		return "", "", false, fmt.Errorf(
			"unsupported OTLP scheme %q in %q — use grpc://, grpcs://, http://, or https://",
			u.Scheme, rawURL,
		)
	}
}
