package httputil

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"syscall"
	"time"
)

// SafeClient returns an HTTP client that prevents SSRF to internal IPs
func SafeClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				d := &net.Dialer{
					Timeout:   30 * time.Second,
					KeepAlive: 30 * time.Second,
					Control: func(network, address string, c syscall.RawConn) error {
						host, _, err := net.SplitHostPort(address)
						if err != nil {
							host = address
						}
						// Strip IPv6 Zone Identifiers
						host = strings.Split(host, "%")[0]

						ip := net.ParseIP(host)
						if ip == nil {
							return fmt.Errorf("invalid IP: %s", host)
						}

						if ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() {
							return fmt.Errorf("forbidden IP: %s", ip.String())
						}
						return nil
					},
				}
				return d.DialContext(ctx, network, addr)
			},
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}
}
