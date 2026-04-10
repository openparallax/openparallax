package executors

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// safeClientMaxRedirects is the cap on redirect hops a SafeHTTPClient will
// follow. Each hop is re-validated against the private-IP block list, so this
// is mostly a denial-of-service guard.
const safeClientMaxRedirects = 5

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

// SafeHTTPClient returns an *http.Client that blocks connections to private,
// loopback, link-local, and unique-local IP addresses at dial time. The dial
// guard is re-applied on every redirect hop, so a public URL that 302s to an
// internal address (DNS rebinding, metadata-service redirect) is rejected
// before the connection completes.
//
// timeout applies to the whole request including redirects. A zero or
// negative value disables the timeout.
func SafeHTTPClient(timeout time.Duration) *http.Client {
	dialer := &net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, fmt.Errorf("invalid address %q: %w", addr, err)
			}
			ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil {
				return nil, fmt.Errorf("DNS resolution failed for %q: %w", host, err)
			}
			if len(ips) == 0 {
				return nil, fmt.Errorf("no addresses for %q", host)
			}
			// Fail closed: if any resolved address is private, refuse the
			// dial entirely rather than picking a "public" one. This
			// prevents DNS records that mix public and private answers
			// from sneaking through.
			for _, ip := range ips {
				if isPrivateIP(ip.IP) {
					return nil, fmt.Errorf("blocked: %q resolves to private/internal address %s", host, ip.IP)
				}
			}
			return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].IP.String(), port))
		},
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          10,
		IdleConnTimeout:       60 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= safeClientMaxRedirects {
				return fmt.Errorf("stopped after %d redirects", safeClientMaxRedirects)
			}
			if err := validateURLNotPrivate(req.URL.String()); err != nil {
				return fmt.Errorf("redirect blocked: %w", err)
			}
			return nil
		},
	}
}
