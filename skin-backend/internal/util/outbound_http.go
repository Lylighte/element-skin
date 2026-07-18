package util

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"
	"time"
)

type lookupIPAddrFunc func(context.Context, string) ([]net.IPAddr, error)
type dialContextFunc func(context.Context, string, string) (net.Conn, error)

func NewSecureOutboundHTTPClient(timeout time.Duration) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialTLSContext = nil
	transport.DialContext = guardedDialContext(net.DefaultResolver.LookupIPAddr, (&net.Dialer{Timeout: timeout}).DialContext)
	return &http.Client{
		Transport: transport,
		Timeout:   timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return errors.New("stopped after 10 redirects")
			}
			return ValidateOutboundURL(req.URL.String())
		},
	}
}

func guardedDialContext(lookup lookupIPAddrFunc, dial dialContextFunc) dialContextFunc {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil || strings.EqualFold(host, "localhost") {
			return nil, ErrUnsafeURL
		}
		if ip := net.ParseIP(host); ip != nil {
			if !isPublicIP(ip) {
				return nil, ErrUnsafeURL
			}
			return dial(ctx, network, net.JoinHostPort(ip.String(), port))
		}
		addresses, err := lookup(ctx, host)
		if err != nil {
			return nil, err
		}
		if len(addresses) == 0 {
			return nil, ErrUnsafeURL
		}
		for _, address := range addresses {
			if !isPublicIP(address.IP) {
				return nil, ErrUnsafeURL
			}
		}
		var dialErr error
		for _, resolved := range addresses {
			connection, err := dial(ctx, network, net.JoinHostPort(resolved.IP.String(), port))
			if err == nil {
				return connection, nil
			}
			dialErr = err
		}
		return nil, dialErr
	}
}
