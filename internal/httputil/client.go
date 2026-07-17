package httputil

import (
	"errors"
	"net"
	"net/http"
	"strings"
	"syscall"
	"time"
)

// SafeClient creates an http.Client that prevents SSRF by blocking requests to
// local, private, unspecified, or link-local unicast IPs at the connection level.
func SafeClient(timeout time.Duration) *http.Client {
	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
		Control: func(network, address string, c syscall.RawConn) error {
			// address comes in as "IP:port" from the net dialer after resolution.
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}

			// Strip IPv6 zone identifiers if present
			host = strings.Split(host, "%")[0]

			ip := net.ParseIP(host)
			if ip == nil {
				return errors.New("invalid IP address")
			}

			if ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() {
				return errors.New("request to forbidden IP blocked")
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
