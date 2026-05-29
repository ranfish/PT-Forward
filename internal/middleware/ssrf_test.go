package middleware

import (
	"net"
	"testing"
)

func TestValidateSafeURL_Public(t *testing.T) {
	tests := []string{
		"https://example.com/path",
		"http://example.com:8080/path?q=1",
		"https://api.telegram.org/bot123/sendMessage",
		"http://192.168.1.1:8080/api",
		"http://localhost:9090/api/v2/auth/login",
	}
	for _, u := range tests {
		if err := ValidateSafeURL(u); err != nil {
			t.Errorf("ValidateSafeURL(%q) should pass, got: %v", u, err)
		}
	}
}

func TestValidatePublicURL_Public(t *testing.T) {
	tests := []string{
		"https://example.com/path",
		"http://example.com:8080/path?q=1",
		"https://1.2.3.4:443/api",
	}
	for _, u := range tests {
		if err := ValidatePublicURL(u); err != nil {
			t.Errorf("ValidatePublicURL(%q) should pass, got: %v", u, err)
		}
	}
}

func TestValidatePublicURL_BlocksPrivate(t *testing.T) {
	tests := []string{
		"http://127.0.0.1:8080/api",
		"http://10.0.0.1/api",
		"http://192.168.1.1/api",
		"http://172.16.0.1/api",
		"https://localhost/api",
		"https://myhost.local/api",
		"https://host.internal/api",
	}
	for _, u := range tests {
		if err := ValidatePublicURL(u); err == nil {
			t.Errorf("ValidatePublicURL(%q) should reject private", u)
		}
	}
}

func TestValidateSafeURL_BadScheme(t *testing.T) {
	tests := []string{
		"file:///etc/passwd",
		"gopher://example.com",
		"ftp://example.com",
		"javascript:alert(1)",
	}
	for _, u := range tests {
		if err := ValidateSafeURL(u); err == nil {
			t.Errorf("ValidateSafeURL(%q) should reject bad scheme", u)
		}
	}
}

func TestValidateSafeURL_Empty(t *testing.T) {
	if err := ValidateSafeURL(""); err == nil {
		t.Error("empty URL should be rejected")
	}
}

func TestValidateSafeURL_NoHost(t *testing.T) {
	if err := ValidateSafeURL("https:///path"); err == nil {
		t.Error("URL with no host should be rejected")
	}
}

func TestIsPrivateIP(t *testing.T) {
	private := []string{
		"127.0.0.1",
		"10.0.0.1",
		"172.16.0.1",
		"192.168.1.1",
		"169.254.1.1",
		"100.64.0.1",
		"198.18.0.1",
		"::1",
		"fc00::1",
		"fe80::1",
		"0.0.0.0",
	}
	for _, s := range private {
		if !isPrivateIP(net.ParseIP(s)) {
			t.Errorf("isPrivateIP(%s) should be true", s)
		}
	}

	public := []string{
		"8.8.8.8",
		"1.1.1.1",
		"203.0.113.1",
		"2001:db8::1",
	}
	for _, s := range public {
		if isPrivateIP(net.ParseIP(s)) {
			t.Errorf("isPrivateIP(%s) should be false", s)
		}
	}
}
