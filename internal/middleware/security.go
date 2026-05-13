package middleware

import (
	"fmt"
	"net"
	"net/http"
	"sync"
)

var (
	trustedNets   []*net.IPNet
	trustedMu     sync.RWMutex
	trustedInited bool
)

func SetTrustedProxies(cidrs []string) error {
	nets := make([]*net.IPNet, 0, len(cidrs))
	for _, cidr := range cidrs {
		if !containsSlash(cidr) {
			ip := net.ParseIP(cidr)
			if ip == nil {
				return fmt.Errorf("invalid trusted proxy IP: %s", cidr)
			}
			if ipv4 := ip.To4(); ipv4 != nil {
				cidr = fmt.Sprintf("%s/32", ipv4)
			} else {
				cidr = fmt.Sprintf("%s/128", ip)
			}
		}
		_, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			return fmt.Errorf("invalid trusted proxy CIDR: %s: %w", cidr, err)
		}
		nets = append(nets, ipNet)
	}

	trustedMu.Lock()
	trustedNets = nets
	trustedInited = true
	trustedMu.Unlock()
	return nil
}

func isTrustedProxy(ip net.IP) bool {
	trustedMu.RLock()
	defer trustedMu.RUnlock()
	if !trustedInited {
		return false
	}
	for _, n := range trustedNets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

func RealIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if remoteIP := net.ParseIP(host); remoteIP != nil {
		if isTrustedProxy(remoteIP) {
			if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
				return firstIP(xff)
			}
			if xri := r.Header.Get("X-Real-IP"); xri != "" {
				return xri
			}
		}
	}
	return host
}

func firstIP(xff string) string {
	for i := 0; i < len(xff); i++ {
		if xff[i] == ',' {
			return trimSpace(xff[:i])
		}
	}
	return trimSpace(xff)
}

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}

func containsSlash(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == '/' {
			return true
		}
	}
	return false
}

func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		if r.TLS != nil {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		next.ServeHTTP(w, r)
	})
}
