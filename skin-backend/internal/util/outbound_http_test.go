package util

import (
	"context"
	"errors"
	"net"
	"net/http"
	"reflect"
	"testing"
	"time"
)

func TestGuardedDialContextPinsOnlyPublicResolvedAddressesExactly(t *testing.T) {
	var lookupHosts []string
	var dialAddresses []string
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()
	lookup := func(_ context.Context, host string) ([]net.IPAddr, error) {
		lookupHosts = append(lookupHosts, host)
		return []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}}, nil
	}
	dial := func(_ context.Context, network, address string) (net.Conn, error) {
		if network != "tcp" {
			t.Fatalf("network=%q, want tcp", network)
		}
		dialAddresses = append(dialAddresses, address)
		return client, nil
	}

	connection, err := guardedDialContext(lookup, dial)(context.Background(), "tcp", "example.com:443")
	if err != nil || connection != client {
		t.Fatalf("guarded dial=(%#v, %v), want injected connection", connection, err)
	}
	if !reflect.DeepEqual(lookupHosts, []string{"example.com"}) || !reflect.DeepEqual(dialAddresses, []string{"93.184.216.34:443"}) {
		t.Fatalf("lookup=%#v dial=%#v, want resolved address pinned", lookupHosts, dialAddresses)
	}
}

func TestGuardedDialContextRejectsPrivateOrMixedResolutionBeforeDialExactly(t *testing.T) {
	for name, addresses := range map[string][]net.IPAddr{
		"private": {{IP: net.ParseIP("127.0.0.1")}},
		"mixed":   {{IP: net.ParseIP("93.184.216.34")}, {IP: net.ParseIP("10.0.0.1")}},
		"empty":   {},
	} {
		t.Run(name, func(t *testing.T) {
			dialCalls := 0
			dial := guardedDialContext(
				func(context.Context, string) ([]net.IPAddr, error) { return addresses, nil },
				func(context.Context, string, string) (net.Conn, error) {
					dialCalls++
					return nil, errors.New("must not dial")
				},
			)
			connection, err := dial(context.Background(), "tcp", "example.com:80")
			if connection != nil || !errors.Is(err, ErrUnsafeURL) || dialCalls != 0 {
				t.Fatalf("guarded dial=(%#v, %v), calls=%d; want nil, ErrUnsafeURL, zero calls", connection, err, dialCalls)
			}
		})
	}
}

func TestGuardedDialContextValidatesLiteralAddressesWithoutDNSExactly(t *testing.T) {
	lookupCalls := 0
	dialCalls := 0
	dial := guardedDialContext(
		func(context.Context, string) ([]net.IPAddr, error) {
			lookupCalls++
			return nil, nil
		},
		func(context.Context, string, string) (net.Conn, error) {
			dialCalls++
			return nil, errors.New("dial failed")
		},
	)
	if _, err := dial(context.Background(), "tcp", "127.0.0.1:80"); !errors.Is(err, ErrUnsafeURL) {
		t.Fatalf("private literal error=%v, want ErrUnsafeURL", err)
	}
	wantDialErr := errors.New("public dial failed")
	dial = guardedDialContext(
		func(context.Context, string) ([]net.IPAddr, error) {
			lookupCalls++
			return nil, nil
		},
		func(_ context.Context, _ string, address string) (net.Conn, error) {
			dialCalls++
			if address != "93.184.216.34:443" {
				t.Fatalf("literal dial address=%q, want pinned literal", address)
			}
			return nil, wantDialErr
		},
	)
	if _, err := dial(context.Background(), "tcp", "93.184.216.34:443"); !errors.Is(err, wantDialErr) {
		t.Fatalf("public literal error=%v, want exact dial error", err)
	}
	if lookupCalls != 0 || dialCalls != 1 {
		t.Fatalf("literal lookup calls=%d dial calls=%d, want 0 and 1", lookupCalls, dialCalls)
	}
}

func TestSecureOutboundHTTPClientDisablesProxyAndRevalidatesRedirectsExactly(t *testing.T) {
	client := NewSecureOutboundHTTPClient(3 * time.Second)
	transport, ok := client.Transport.(*http.Transport)
	if !ok || transport.Proxy != nil || transport.DialContext == nil || transport.DialTLSContext != nil || client.Timeout != 3*time.Second {
		t.Fatalf("secure client configuration mismatch: client=%#v transport=%#v", client, transport)
	}
	req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1/private", nil)
	if err != nil {
		t.Fatal(err)
	}
	err = client.CheckRedirect(req, nil)
	if !errors.Is(err, ErrUnsafeURL) {
		t.Fatalf("private redirect error=%v, want ErrUnsafeURL", err)
	}
}
