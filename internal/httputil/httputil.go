package httputil

import (
	"errors"
	"net"
	"net/http"
	"strings"
	"syscall"
	"time"
)

// SafeClient creates an http.Client with a custom Dialer that prevents SSRF
// (Server-Side Request Forgery) by explicitly blocking loopback, private,
// unspecified, and link-local unicast IPs at the socket level.
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
			if ip == nil {
				return errors.New("invalid IP")
			}
			if ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() {
				return errors.New("blocked SSRF IP: " + ip.String())
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
