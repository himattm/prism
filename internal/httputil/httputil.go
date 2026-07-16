package httputil

import (
	"errors"
	"net"
	"net/http"
	"strings"
	"syscall"
	"time"
)

// SafeClient returns an HTTP client that prevents Server-Side Request Forgery (SSRF)
// by rejecting connections to loopback, private, unspecified, and link-local unicast IP addresses.
func SafeClient(timeout time.Duration) *http.Client {
	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
		Control: func(network, address string, c syscall.RawConn) error {
			// address will be in the form "ip:port"
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}

			// Strip IPv6 Zone Identifier if present
			host = strings.Split(host, "%")[0]

			ip := net.ParseIP(host)
			if ip == nil {
				return errors.New("invalid IP address")
			}

			if ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() {
				return errors.New("connection to private/internal IP rejected")
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
		Timeout:   timeout,
	}
}
