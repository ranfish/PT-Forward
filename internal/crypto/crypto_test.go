package crypto

import (
	"strings"
	"testing"
)

func TestNewCredentialEncryptor_InvalidKey(t *testing.T) {
	_, err := NewCredentialEncryptor("short")
	if err == nil {
		t.Error("expected error for short key")
	}
}

func TestNewCredentialEncryptor_ValidKey(t *testing.T) {
	_, err := NewCredentialEncryptor("this-is-a-valid-32-byte-key!!!!!!!")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEncryptDecrypt_Roundtrip(t *testing.T) {
	enc, err := NewCredentialEncryptor("test-encryption-key-32bytes!!!")
	if err != nil {
		t.Fatalf("create encryptor: %v", err)
	}

	tests := []string{
		"hello world",
		"passkey_abc123",
		"cookie_value_with=equals&and?special",
		"中文内容加密测试",
		"a very long value " + strings.Repeat("x", 500),
	}

	for _, plain := range tests {
		cipher, err := enc.Encrypt(plain)
		if err != nil {
			t.Errorf("encrypt(%q): %v", plain, err)
			continue
		}
		if !strings.HasPrefix(cipher, ciphertextPrefix) {
			t.Errorf("encrypted value missing prefix: %s", cipher[:20])
			continue
		}
		decrypted, err := enc.Decrypt(cipher)
		if err != nil {
			t.Errorf("decrypt(%q): %v", plain, err)
			continue
		}
		if decrypted != plain {
			t.Errorf("roundtrip mismatch: got %q, want %q", decrypted, plain)
		}
	}
}

func TestEncryptDecrypt_EmptyString(t *testing.T) {
	enc, _ := NewCredentialEncryptor("test-encryption-key-32bytes!!!")

	encVal, err := enc.Encrypt("")
	if err != nil {
		t.Fatalf("encrypt empty: %v", err)
	}
	if encVal != "" {
		t.Errorf("encrypt('') should return '', got %q", encVal)
	}

	decVal, err := enc.Decrypt("")
	if err != nil {
		t.Fatalf("decrypt empty: %v", err)
	}
	if decVal != "" {
		t.Errorf("decrypt('') should return '', got %q", decVal)
	}
}

func TestDecrypt_PlaintextPassthrough(t *testing.T) {
	enc, _ := NewCredentialEncryptor("test-encryption-key-32bytes!!!")

	plain := "not-encrypted-value"
	dec, err := enc.Decrypt(plain)
	if err != nil {
		t.Fatalf("decrypt plaintext: %v", err)
	}
	if dec != plain {
		t.Errorf("plaintext passthrough: got %q, want %q", dec, plain)
	}
}

func TestIsEncrypted(t *testing.T) {
	enc, _ := NewCredentialEncryptor("test-encryption-key-32bytes!!!")

	if enc.IsEncrypted("") {
		t.Error("empty string should not be encrypted")
	}
	if enc.IsEncrypted("plaintext") {
		t.Error("plaintext should not be encrypted")
	}

	encVal, _ := enc.Encrypt("test")
	if !enc.IsEncrypted(encVal) {
		t.Error("encrypted value should be detected")
	}
}

func TestEncrypt_DifferentCiphertextsForSamePlaintext(t *testing.T) {
	enc, _ := NewCredentialEncryptor("test-encryption-key-32bytes!!!")

	plain := "same-plaintext"
	c1, _ := enc.Encrypt(plain)
	c2, _ := enc.Encrypt(plain)
	if c1 == c2 {
		t.Error("same plaintext should produce different ciphertexts (random nonce)")
	}
}

func TestDecrypt_InvalidCiphertext(t *testing.T) {
	enc, _ := NewCredentialEncryptor("test-encryption-key-32bytes!!!")

	_, err := enc.Decrypt(ciphertextPrefix + "not-valid-base64!!!")
	if err == nil {
		t.Error("expected error for invalid base64")
	}
}

func TestDeriveKey(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"exactly-32-bytes-key-here!!!!!!!!", 32},
		{"short", 32},
		{"this-is-a-very-long-key-that-is-more-than-32-characters", 32},
	}
	for _, tt := range tests {
		key := deriveKey(tt.input)
		if len(key) != tt.want {
			t.Errorf("deriveKey(%q) length = %d, want %d", tt.input, len(key), tt.want)
		}
	}
}
