package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSetTrustedProxies_ValidCIDRs(t *testing.T) {
	err := SetTrustedProxies([]string{"10.0.0.0/8", "192.168.0.0/16"})
	if err != nil {
		t.Fatalf("valid CIDRs should not error: %v", err)
	}
}

func TestSetTrustedProxies_ValidIPs(t *testing.T) {
	err := SetTrustedProxies([]string{"10.0.0.1", "::1"})
	if err != nil {
		t.Fatalf("valid IPs should not error: %v", err)
	}
}

func TestSetTrustedProxies_InvalidIP(t *testing.T) {
	err := SetTrustedProxies([]string{"not-an-ip"})
	if err == nil {
		t.Fatal("invalid IP should return error")
	}
}

func TestSetTrustedProxies_InvalidCIDR(t *testing.T) {
	err := SetTrustedProxies([]string{"10.0.0.0/abc"})
	if err == nil {
		t.Fatal("invalid CIDR should return error")
	}
}

func TestSetTrustedProxies_Empty(t *testing.T) {
	err := SetTrustedProxies([]string{})
	if err != nil {
		t.Fatalf("empty list should not error: %v", err)
	}
}

func TestRealIP_TrustedProxy_XFF(t *testing.T) {
	SetTrustedProxies([]string{"127.0.0.1/32"})

	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "127.0.0.1:12345"
	r.Header.Set("X-Forwarded-For", "1.2.3.4, 5.6.7.8")

	ip := RealIP(r)
	if ip != "1.2.3.4" {
		t.Errorf("expected first IP from XFF, got %q", ip)
	}
}

func TestRealIP_TrustedProxy_XRealIP(t *testing.T) {
	SetTrustedProxies([]string{"127.0.0.1/32"})

	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "127.0.0.1:12345"
	r.Header.Set("X-Real-IP", "9.8.7.6")

	ip := RealIP(r)
	if ip != "9.8.7.6" {
		t.Errorf("expected X-Real-IP, got %q", ip)
	}
}

func TestRealIP_UntrustedProxy(t *testing.T) {
	SetTrustedProxies([]string{"10.0.0.0/8"})

	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "192.168.1.1:12345"
	r.Header.Set("X-Forwarded-For", "1.2.3.4")

	ip := RealIP(r)
	if ip != "192.168.1.1" {
		t.Errorf("untrusted proxy should return RemoteAddr host, got %q", ip)
	}
}

func TestRealIP_NoProxy(t *testing.T) {
	SetTrustedProxies([]string{})

	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "192.168.1.1:8080"

	ip := RealIP(r)
	if ip != "192.168.1.1" {
		t.Errorf("no proxy should return RemoteAddr host, got %q", ip)
	}
}

func TestSecurityHeaders_SetCorrectly(t *testing.T) {
	handler := SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))

	if v := rec.Header().Get("X-Content-Type-Options"); v != "nosniff" {
		t.Errorf("X-Content-Type-Options: got %q", v)
	}
	if v := rec.Header().Get("X-Frame-Options"); v != "DENY" {
		t.Errorf("X-Frame-Options: got %q", v)
	}
	if v := rec.Header().Get("Referrer-Policy"); v != "no-referrer" {
		t.Errorf("Referrer-Policy: got %q", v)
	}
}

func TestFirstIP(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"1.2.3.4, 5.6.7.8", "1.2.3.4"},
		{"1.2.3.4", "1.2.3.4"},
		{"  1.2.3.4  , 5.6.7.8", "1.2.3.4"},
	}
	for _, tt := range tests {
		got := firstIP(tt.input)
		if got != tt.want {
			t.Errorf("firstIP(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestTrimSpace(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello", "hello"},
		{"  hello  ", "hello"},
		{"\thello\t", "hello"},
		{"", ""},
	}
	for _, tt := range tests {
		got := trimSpace(tt.input)
		if got != tt.want {
			t.Errorf("trimSpace(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestContainsSlash(t *testing.T) {
	if !containsSlash("10.0.0.0/8") {
		t.Error("CIDR should contain slash")
	}
	if containsSlash("10.0.0.1") {
		t.Error("IP should not contain slash")
	}
}
