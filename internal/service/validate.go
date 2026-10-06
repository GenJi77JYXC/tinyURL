package service

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"strings"
)

var (
	ErrInvalidURL    = errors.New("invalid url: must be an absolute http(s) URL")
	ErrInvalidScheme = errors.New("invalid scheme: only http and https are allowed")
	ErrBlockedHost   = errors.New("blocked host: internal/private addresses are not allowed")
	ErrUnresolvable  = errors.New("host could not be resolved")
)

const maxURLLength = 4096

// ValidateLongURL enforces:
//  1. well-formed absolute URL;
//  2. scheme is http or https;
//  3. the host (literal or after DNS resolution) is not an internal address
//     (loopback / private / link-local / CGNAT / unspecified / multicast),
//     which prevents SSRF to cloud metadata endpoints (169.254.169.254) etc.
func ValidateLongURL(ctx context.Context, raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" || len(raw) > maxURLLength {
		return ErrInvalidURL
	}

	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return ErrInvalidURL
	}

	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return ErrInvalidScheme
	}

	host := u.Hostname()
	if host == "" {
		return ErrInvalidURL
	}

	ips, err := resolveHost(ctx, host)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrUnresolvable, err)
	}
	for _, ip := range ips {
		if isBlockedIP(ip) {
			return fmt.Errorf("%w: %s resolves to %s", ErrBlockedHost, host, ip)
		}
	}
	return nil
}

func resolveHost(ctx context.Context, host string) ([]netip.Addr, error) {
	if addr, err := netip.ParseAddr(host); err == nil {
		return []netip.Addr{addr}, nil
	}

	records, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	addrs := make([]netip.Addr, 0, len(records))
	for _, rec := range records {
		if a, ok := netip.AddrFromSlice(rec.IP); ok {
			addrs = append(addrs, a)
		}
	}
	if len(addrs) == 0 {
		return nil, errors.New("no usable addresses")
	}
	return addrs, nil
}

// isBlockedIP reports whether an IP must never be used as a redirect target.
func isBlockedIP(addr netip.Addr) bool {
	if addr.IsLoopback() || addr.IsPrivate() ||
		addr.IsLinkLocalUnicast() || addr.IsLinkLocalMulticast() ||
		addr.IsUnspecified() || addr.IsMulticast() {
		return true
	}

	// Extra guard: CGNAT 100.64.0.0/10 (not covered by IsPrivate on Go <1.24).
	// Is4 must be checked first — As4 panics on an IPv6 address.
	if addr.Is4() {
		v4 := addr.As4()
		if v4[0] == 100 && v4[1] >= 64 && v4[1] <= 127 {
			return true
		}
	}
	return false
}
