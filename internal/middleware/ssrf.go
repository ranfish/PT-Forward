package middleware

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"
)

func contextWithTimeout(d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), d)
}

func ValidateSafeURL(rawURL string) error {
	return validateURL(rawURL, false)
}

func ValidatePublicURL(rawURL string) error {
	return validateURL(rawURL, true)
}

func ValidateProxyURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" && scheme != "socks5" && scheme != "socks5h" {
		return fmt.Errorf("URL scheme must be http, https, socks5 or socks5h, got %q", scheme)
	}
	if u.Hostname() == "" {
		return fmt.Errorf("URL host is empty")
	}
	return nil
}

func validateURL(rawURL string, blockPrivate bool) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("URL scheme must be http or https, got %q", scheme)
	}

	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("URL host is empty")
	}

	if !blockPrivate {
		return nil
	}

	if ip := net.ParseIP(host); ip != nil {
		if isPrivateIP(ip) {
			return fmt.Errorf("URL host resolves to private/reserved IP %s", ip)
		}
		return nil
	}

	lower := strings.ToLower(host)
	if lower == "localhost" || strings.HasSuffix(lower, ".local") ||
		strings.HasSuffix(lower, ".internal") || strings.HasSuffix(lower, ".localhost") {
		return fmt.Errorf("URL host %q is a private hostname", host)
	}

	resolver := &net.Resolver{}
	ctx, cancel := contextWithTimeout(5 * time.Second)
	defer cancel()
	ips, err := resolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil
	}
	for _, ipAddr := range ips {
		if isPrivateIP(ipAddr.IP) {
			return fmt.Errorf("URL host %q resolves to private/reserved IP %s", host, ipAddr.IP)
		}
	}

	return nil
}

var privateNets = func() []*net.IPNet {
	cidrs := []string{
		"127.0.0.0/8",
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"169.254.0.0/16",
		"100.64.0.0/10",
		"198.18.0.0/15",
		"::1/128",
		"fc00::/7",
		"fe80::/10",
	}
	nets := make([]*net.IPNet, 0, len(cidrs))
	for _, cidr := range cidrs {
		_, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			panic(fmt.Sprintf("invalid private CIDR: %s", cidr))
		}
		nets = append(nets, ipNet)
	}
	return nets
}()

func isPrivateIP(ip net.IP) bool {
	if ip.IsUnspecified() {
		return true
	}
	for _, n := range privateNets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}
