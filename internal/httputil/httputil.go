package httputil

import (
	"errors"
	"net"
	"net/http"
	"strings"
	"syscall"
	"time"
)

// SafeClient returns an http.Client configured to mitigate SSRF (Server-Side Request Forgery)
// vulnerabilities by blocking loopback, private, unspecified, and link-local unicast IPs
// during the connection phase.
func SafeClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
				Control: func(network, address string, c syscall.RawConn) error {
					// Extract the host portion from the address (e.g., "192.168.1.1:80")
					host, _, err := net.SplitHostPort(address)
					if err != nil {
						// If SplitHostPort fails, try using the address directly (fallback)
						host = address
					}

					// Strip IPv6 Zone Identifiers if present (e.g., "fe80::1%eth0")
					host = strings.Split(host, "%")[0]

					ip := net.ParseIP(host)
					if ip == nil {
						// We expect the IP to be resolved at this point in the Control hook
						return errors.New("invalid IP address in dial control hook")
					}

					// Block common SSRF target IP ranges
					if ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() {
						return errors.New("access to private/local network addresses is not allowed")
					}

					return nil
				},
			}).DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}
}
