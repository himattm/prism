package httputil

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"syscall"
	"time"
)

// SafeClient returns an HTTP client configured to prevent Server-Side Request Forgery (SSRF).
// It restricts connections to loopback, private, unspecified, and link-local unicast IPs.
func SafeClient(timeout time.Duration) *http.Client {
	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
		Control: func(network, address string, c syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			// Strip IPv6 Zone Identifier if present
			host = strings.Split(host, "%")[0]
			ip := net.ParseIP(host)
			if ip != nil && (ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() || ip.IsLinkLocalUnicast()) {
				return fmt.Errorf("SSRF prevention: forbidden IP %s", ip.String())
			}
			return nil
		},
	}

	return &http.Client{
		Transport: &http.Transport{
			DialContext:           dialer.DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
		Timeout: timeout,
	}
}
