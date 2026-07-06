package httputil

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"syscall"
	"time"
)

// SafeClient creates an http.Client with SSRF protection, preventing requests
// to loopback, private, unspecified, and link-local unicast IPs.
func SafeClient(timeout time.Duration) *http.Client {
	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
		Control: func(network, address string, c syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			host = strings.Split(host, "%")[0]
			ip := net.ParseIP(host)
			if ip != nil && (ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() || ip.IsLinkLocalUnicast()) {
				return fmt.Errorf("SSRF protection: IP %s is not allowed", ip)
			}
			return nil
		},
	}

	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			DialContext:           dialer.DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}
}
