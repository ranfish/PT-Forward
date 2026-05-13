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

const pbkdf2Iterations = 100000

var pbkdf2Salt = []byte("pt-forward-credential-encryption")

type CredentialEncryptor struct {
	aead cipher.AEAD
}

func NewCredentialEncryptor(key string) (*CredentialEncryptor, error) {
	if len(key) < 16 {
		return nil, fmt.Errorf("encryption key must be at least 16 characters, got %d", len(key))
	}
	aesKey := pbkdf2.Key([]byte(key), pbkdf2Salt, pbkdf2Iterations, 32, sha256.New)
	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return nil, fmt.Errorf("create AES cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}
	return &CredentialEncryptor{aead: aead}, nil
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
	return ciphertextPrefix + base64.StdEncoding.EncodeToString(ciphertext), nil
}

func (e *CredentialEncryptor) Decrypt(ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}
	if !strings.HasPrefix(ciphertext, ciphertextPrefix) {
		return ciphertext, nil
	}
	data, err := base64.StdEncoding.DecodeString(ciphertext[len(ciphertextPrefix):])
	if err != nil {
		return "", fmt.Errorf("base64 decode: %w", err)
	}
	if len(data) < e.aead.NonceSize() {
		return "", errors.New("ciphertext too short")
	}
	nonce := data[:e.aead.NonceSize()]
	sealed := data[e.aead.NonceSize():]
	plaintext, err := e.aead.Open(nil, nonce, sealed, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}
	return string(plaintext), nil
}

func (e *CredentialEncryptor) IsEncrypted(value string) bool {
	return strings.HasPrefix(value, ciphertextPrefix)
}
