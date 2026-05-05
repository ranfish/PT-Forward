package cookiecloud

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	pkcs5SaltLen = 8
	aes256KeyLen = 32
	blockLen     = 16
)

type CookieData struct {
	Name     string  `json:"name"`
	Value    string  `json:"value"`
	Domain   string  `json:"domain"`
	Path     string  `json:"path"`
	Expires  float64 `json:"expirationDate"`
	HTTPOnly bool    `json:"httpOnly"`
	Secure   bool    `json:"secure"`
	SameSite string  `json:"sameSite"`
}

type SyncResult struct {
	DomainCookies map[string][]CookieData
	SyncedSites   int
	SkippedSites  int
}

func md5String(inputs ...string) string {
	h := md5.New()
	for _, s := range inputs {
		_, _ = io.WriteString(h, s)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func bytesToKey(salt, data []byte, h hash.Hash, keyLen, bl int) (key, iv []byte) {
	var (
		concat   []byte
		lastHash []byte
		totalLen = keyLen + bl
	)
	for ; len(concat) < totalLen; h.Reset() {
		h.Write(append(lastHash, append(data, salt...)...))
		lastHash = h.Sum(nil)
		concat = append(concat, lastHash...)
	}
	return concat[:keyLen], concat[keyLen:totalLen]
}

func pkcs7Strip(data []byte, blockSize int) ([]byte, error) {
	length := len(data)
	if length == 0 {
		return nil, errors.New("pkcs7: data is empty")
	}
	if length%blockSize != 0 {
		return nil, errors.New("pkcs7: data is not block-aligned")
	}
	padLen := int(data[length-1])
	if padLen > blockSize || padLen == 0 {
		return nil, errors.New("pkcs7: invalid padding")
	}
	ref := bytes.Repeat([]byte{byte(padLen)}, padLen)
	if !bytes.HasSuffix(data, ref) {
		return nil, errors.New("pkcs7: invalid padding")
	}
	return data[:length-padLen], nil
}

func decryptCryptoJSAES(password, ciphertext string) ([]byte, error) {
	rawEncrypted, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return nil, fmt.Errorf("base64 decode: %w", err)
	}
	if len(rawEncrypted) < 17 || len(rawEncrypted)%blockLen != 0 || string(rawEncrypted[:8]) != "Salted__" {
		return nil, errors.New("invalid ciphertext")
	}
	salt := rawEncrypted[8:16]
	encrypted := rawEncrypted[16:]
	key, iv := bytesToKey(salt, []byte(password), md5.New(), aes256KeyLen, blockLen)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes cipher: %w", err)
	}
	decrypted := make([]byte, len(encrypted))
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(decrypted, encrypted)
	decrypted, err = pkcs7Strip(decrypted, blockLen)
	if err != nil {
		return nil, fmt.Errorf("pkcs7 strip (wrong password?): %w", err)
	}
	return decrypted, nil
}

func FetchAndDecrypt(serverURL, uuid, password string) (map[string][]CookieData, error) {
	url := strings.TrimSuffix(serverURL, "/") + "/get/" + uuid

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	var body struct {
		Encrypted string `json:"encrypted"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("parse json: %w", err)
	}
	if body.Encrypted == "" {
		return nil, errors.New("no encrypted data in response")
	}

	keyPassword := md5String(uuid, "-", password)[:16]
	decrypted, err := decryptCryptoJSAES(keyPassword, body.Encrypted)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}

	var result struct {
		CookieData map[string][]CookieData `json:"cookie_data"`
	}
	if err := json.Unmarshal(decrypted, &result); err != nil {
		return nil, fmt.Errorf("parse decrypted json: %w", err)
	}

	return result.CookieData, nil
}

func normalizeDomain(d string) string {
	d = strings.TrimPrefix(d, ".")
	d = strings.TrimPrefix(d, "www.")
	return d
}

func DomainMatches(cookieDomain, target string) bool {
	cd := normalizeDomain(cookieDomain)
	td := normalizeDomain(target)
	if cd == td || cd == strings.TrimPrefix(td, "www.") {
		return true
	}
	if strings.HasSuffix(cd, "."+td) {
		return true
	}
	cdBase := strings.TrimPrefix(cd, "www.")
	tdBase := strings.TrimPrefix(td, "www.")
	return cdBase == tdBase
}

func FilterCookiesByDomain(cookies map[string][]CookieData, domain string) []CookieData {
	var result []CookieData
	seen := make(map[string]bool)
	targets := []string{domain}
	if strings.HasPrefix(domain, "www.") {
		targets = append(targets, strings.TrimPrefix(domain, "www."))
	}
	targets = append(targets, "."+domain)
	if strings.HasPrefix(domain, "www.") {
		targets = append(targets, "."+strings.TrimPrefix(domain, "www."))
	}
	for _, cookieList := range cookies {
		for _, c := range cookieList {
			for _, t := range targets {
				if DomainMatches(c.Domain, t) {
					key := c.Name + "@" + c.Domain
					if !seen[key] {
						seen[key] = true
						result = append(result, c)
					}
					break
				}
			}
		}
	}
	return result
}

func CookiesToString(cookies []CookieData) string {
	var parts []string
	for _, c := range cookies {
		parts = append(parts, c.Name+"="+c.Value)
	}
	return strings.Join(parts, "; ")
}
