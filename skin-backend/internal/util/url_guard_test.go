package util

import (
	"errors"
	"net"
	"testing"
)

func TestValidateOutboundURLBlocksUnsafeAndAllowsPublicLiteral(t *testing.T) {
	blocked := []string{
		"http://127.0.0.1/x",
		"http://localhost/x",
		"http://169.254.169.254/latest/meta-data",
		"http://10.0.0.5/x",
		"http://192.168.1.1/x",
		"http://172.16.0.1/x",
		"http://[::1]/x",
		"http://0.0.0.0/x",
		"http://224.0.0.1/x",
		"http://239.255.255.250/x",
		"http://100.100.100.200/latest/meta-data",
		"http://198.18.0.1/x",
		"http://203.0.113.1/x",
		"http://[2001:db8::1]/x",
		"file:///etc/passwd",
		"ftp://internal/x",
		"",
	}
	for _, raw := range blocked {
		if err := ValidateOutboundURL(raw); err == nil {
			t.Fatalf("expected %q to be blocked", raw)
		}
	}
	if err := ValidateOutboundURL("http://1.1.1.1/x"); err != nil {
		t.Fatalf("public IP literal should be allowed: %v", err)
	}
	if err := ValidateOutboundURL("https://[2606:4700:4700::1111]/x"); err != nil {
		t.Fatalf("public IPv6 literal should be allowed: %v", err)
	}
}

func TestValidateOutboundURLChecksEveryResolvedAddressExactly(t *testing.T) {
	resolverErr := errors.New("dns unavailable")
	for _, tc := range []struct {
		name  string
		addrs []net.IP
		err   error
		want  error
	}{
		{name: "resolver error", err: resolverErr, want: resolverErr},
		{name: "empty answer", addrs: []net.IP{}, want: ErrUnsafeURL},
		{name: "private answer", addrs: []net.IP{net.ParseIP("10.0.0.1")}, want: ErrUnsafeURL},
		{name: "mixed public and private", addrs: []net.IP{net.ParseIP("1.1.1.1"), net.ParseIP("192.168.1.2")}, want: ErrUnsafeURL},
		{name: "all public", addrs: []net.IP{net.ParseIP("1.1.1.1"), net.ParseIP("2606:4700:4700::1111")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			err := validateOutboundURL("https://textures.example/path", func(host string) ([]net.IP, error) {
				calls++
				if host != "textures.example" {
					t.Fatalf("resolver host=%q; want textures.example", host)
				}
				return tc.addrs, tc.err
			})
			if !errors.Is(err, tc.want) || calls != 1 {
				t.Fatalf("validation err=%v calls=%d; want err=%v calls=1", err, calls, tc.want)
			}
		})
	}
}
