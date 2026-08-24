package server

import (
	"net/http"
	"time"
)

// ProxyHTTPClient returns a client whose transport honors HTTP_PROXY, HTTPS_PROXY,
// and NO_PROXY (Go net/http default). Cluster egress is expected to set HTTP_PROXY.
func ProxyHTTPClient(timeout time.Duration) *http.Client {
	transport := http.DefaultTransport
	if base, ok := http.DefaultTransport.(*http.Transport); ok {
		transport = base.Clone()
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}
}
