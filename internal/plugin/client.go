package plugin

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"syscall"
	"time"
)

// secureHTTPClient returns an http.Client configured to mitigate SSRF
// by rejecting connections to private, loopback, and link-local IP addresses.
func secureHTTPClient(timeout time.Duration) *http.Client {
	dialer := &net.Dialer{
		Timeout:   timeout,
		KeepAlive: 30 * time.Second,
		Control: func(network, address string, c syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			host = strings.Split(host, "%")[0] // strip IPv6 zone identifier
			ip := net.ParseIP(host)
			if ip == nil {
				return fmt.Errorf("invalid IP: %s", host)
			}
			if ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() {
				return fmt.Errorf("SSRF protection: blocked connection to %s", ip)
			}
			return nil
		},
	}

	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			DialContext:           dialer.DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			Proxy:                 http.ProxyFromEnvironment,
		},
	}
}
