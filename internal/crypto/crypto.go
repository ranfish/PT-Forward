package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"

	"golang.org/x/crypto/pbkdf2"
)

const ciphertextPrefix = "enc:"

const ciphertextV2Prefix = "enc2:"

const pbkdf2Iterations = 100000

var legacySalt = []byte("pt-forward-credential-encryption")

type CredentialEncryptor struct {
	aead    cipher.AEAD
	legacy  cipher.AEAD
	hasLegacy bool
}

func deriveSalt(key string) []byte {
	h := sha256.Sum256(append([]byte("pt-forward-salt-v1:"), key...))
	return h[:]
}

func newAEAD(key string, salt []byte) (cipher.AEAD, error) {
	aesKey := pbkdf2.Key([]byte(key), salt, pbkdf2Iterations, 32, sha256.New)
	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return nil, fmt.Errorf("create AES cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}
	return aead, nil
}

func NewCredentialEncryptor(key string) (*CredentialEncryptor, error) {
	if len(key) < 16 {
		return nil, fmt.Errorf("encryption key must be at least 16 characters, got %d", len(key))
	}
	aead, err := newAEAD(key, deriveSalt(key))
	if err != nil {
		return nil, err
	}
	legacyAEAD, err := newAEAD(key, legacySalt)
	if err != nil {
		return nil, err
	}
	return &CredentialEncryptor{aead: aead, legacy: legacyAEAD, hasLegacy: true}, nil
}

func (e *CredentialEncryptor) Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	nonce := make([]byte, e.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}
	ciphertext := e.aead.Seal(nonce, nonce, []byte(plaintext), nil)
	return ciphertextV2Prefix + base64.StdEncoding.EncodeToString(ciphertext), nil
}

func (e *CredentialEncryptor) Decrypt(ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}
	if strings.HasPrefix(ciphertext, ciphertextV2Prefix) {
		return e.decryptWith(e.aead, ciphertext[len(ciphertextV2Prefix):])
	}
	if strings.HasPrefix(ciphertext, ciphertextPrefix) {
		return e.decryptWith(e.legacy, ciphertext[len(ciphertextPrefix):])
	}
	return ciphertext, nil
}

func (e *CredentialEncryptor) decryptWith(aead cipher.AEAD, encoded string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("base64 decode: %w", err)
	}
	if len(data) < aead.NonceSize() {
		return "", errors.New("ciphertext too short")
	}
	nonce := data[:aead.NonceSize()]
	sealed := data[aead.NonceSize():]
	plaintext, err := aead.Open(nil, nonce, sealed, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}
	return string(plaintext), nil
}

func (e *CredentialEncryptor) IsEncrypted(value string) bool {
	return strings.HasPrefix(value, ciphertextPrefix) || strings.HasPrefix(value, ciphertextV2Prefix)
}

func (e *CredentialEncryptor) IsLegacyEncrypted(value string) bool {
	return strings.HasPrefix(value, ciphertextPrefix)
}
