package executors

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// privateRanges contains all CIDR ranges considered private or internal.
var privateRanges []*net.IPNet

func init() {
	cidrs := []string{
		"127.0.0.0/8",    // IPv4 loopback
		"10.0.0.0/8",     // RFC 1918
		"172.16.0.0/12",  // RFC 1918
		"192.168.0.0/16", // RFC 1918
		"169.254.0.0/16", // link-local / cloud metadata
		"::1/128",        // IPv6 loopback
		"fc00::/7",       // IPv6 unique local
	}
	for _, cidr := range cidrs {
		_, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			panic(fmt.Sprintf("invalid CIDR %q: %v", cidr, err))
		}
		privateRanges = append(privateRanges, ipNet)
	}
}

// isPrivateIP reports whether ip falls within a private or internal network range.
func isPrivateIP(ip net.IP) bool {
	for _, r := range privateRanges {
		if r.Contains(ip) {
			return true
		}
	}
	return false
}

// isPrivateHost resolves hostname to IP addresses and returns true if any
// resolved address is in a private network range. Returns an error if the
// hostname cannot be resolved.
func isPrivateHost(hostname string) (bool, error) {
	// Strip brackets from IPv6 literals.
	hostname = strings.TrimPrefix(hostname, "[")
	hostname = strings.TrimSuffix(hostname, "]")

	// Check if hostname is already an IP literal.
	if ip := net.ParseIP(hostname); ip != nil {
		return isPrivateIP(ip), nil
	}

	addrs, err := net.LookupHost(hostname)
	if err != nil {
		return false, fmt.Errorf("DNS resolution failed for %q: %w", hostname, err)
	}
	for _, addr := range addrs {
		ip := net.ParseIP(addr)
		if ip != nil && isPrivateIP(ip) {
			return true, nil
		}
	}
	return false, nil
}

// validateURLNotPrivate parses rawURL and checks that its host does not resolve
// to a private IP address. Returns an error describing the block reason if it does.
func validateURLNotPrivate(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	hostname := parsed.Hostname()
	if hostname == "" {
		return fmt.Errorf("URL has no hostname")
	}
	private, err := isPrivateHost(hostname)
	if err != nil {
		// Fail closed: if we can't resolve, block.
		return fmt.Errorf("blocked: request targets a private/internal network address")
	}
	if private {
		return fmt.Errorf("blocked: request targets a private/internal network address")
	}
	return nil
}
