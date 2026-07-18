package util

import (
	"errors"
	"net"
	"net/netip"
	"net/url"
	"strings"
)

var ErrUnsafeURL = errors.New("unsafe outbound URL")

var nonPublicOutboundPrefixes = []netip.Prefix{
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("2001:db8::/32"),
}

func ValidateOutboundURL(raw string) error {
	return validateOutboundURL(raw, net.LookupIP)
}

func validateOutboundURL(raw string, lookupIP func(string) ([]net.IP, error)) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ErrUnsafeURL
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return ErrUnsafeURL
	}
	host := u.Hostname()
	if strings.EqualFold(host, "localhost") {
		return ErrUnsafeURL
	}
	if ip := net.ParseIP(host); ip != nil {
		if !isPublicIP(ip) {
			return ErrUnsafeURL
		}
		return nil
	}
	addrs, err := lookupIP(host)
	if err != nil {
		return err
	}
	if len(addrs) == 0 {
		return ErrUnsafeURL
	}
	for _, ip := range addrs {
		if !isPublicIP(ip) {
			return ErrUnsafeURL
		}
	}
	return nil
}

func isPublicIP(ip net.IP) bool {
	if !ip.IsGlobalUnicast() || ip.IsPrivate() {
		return false
	}
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return false
	}
	addr = addr.Unmap()
	for _, prefix := range nonPublicOutboundPrefixes {
		if prefix.Contains(addr) {
			return false
		}
	}
	return true
}
