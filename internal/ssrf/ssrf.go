package ssrf

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"syscall"
	"time"
)

// NewSecureHTTPClient returns an http.Client that prevents SSRF by blocking
// connections to private, loopback, and other restricted IPs.
func NewSecureHTTPClient() *http.Client {
	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
		Control: func(network, address string, c syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}

			// Strip IPv6 Zone Identifiers if present (e.g. fe80::1%eth0)
			host = strings.Split(host, "%")[0]
			ip := net.ParseIP(host)
			if ip == nil {
				return fmt.Errorf("invalid IP: %s", host)
			}

			// Block internal / local / loopback routing
			if ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() {
				return fmt.Errorf("access to local or private IP %s is not allowed", ip.String())
			}

			return nil
		},
	}

	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	return &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
	}
}
