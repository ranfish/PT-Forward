package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"hash"
	"io"
	"log"
	"net/http"
	"os"
	"slices"
	"strings"
)

const (
	pkcs5SaltLen = 8
	aes256KeyLen = 32
	blockLen     = 16
)

type cookieCloudBody struct {
	UUID      string `json:"uuid,omitempty"`
	Encrypted string `json:"encrypted,omitempty"`
}

type cookieData struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	Domain   string `json:"domain"`
	Path     string `json:"path"`
	Expires  float64 `json:"expirationDate"`
	HTTPOnly bool   `json:"httpOnly"`
	Secure   bool   `json:"secure"`
	SameSite string `json:"sameSite"`
}

func md5String(inputs ...string) string {
	h := md5.New()
	for _, s := range inputs {
		io.WriteString(h, s)
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

func fetchEncrypted(apiURL, uuid string) (string, error) {
	url := strings.TrimSuffix(apiURL, "/") + "/get/" + uuid
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("server returned status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read body: %w", err)
	}
	var data cookieCloudBody
	if err := json.Unmarshal(body, &data); err != nil {
		return "", fmt.Errorf("parse json: %w", err)
	}
	if data.Encrypted == "" {
		return "", errors.New("no encrypted data in response")
	}
	return data.Encrypted, nil
}

func decryptCookies(uuid, password, encrypted string) (map[string][]cookieData, error) {
	keyPassword := md5String(uuid, "-", password)[:16]
	decrypted, err := decryptCryptoJSAES(keyPassword, encrypted)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}
	var result struct {
		CookieData   map[string][]cookieData `json:"cookie_data"`
		LocalStorage map[string]interface{}   `json:"local_storage_data"`
	}
	if err := json.Unmarshal(decrypted, &result); err != nil {
		return nil, fmt.Errorf("parse decrypted json: %w", err)
	}
	return result.CookieData, nil
}

func domainMatches(cookieDomain, target string) bool {
	cookieDomain = strings.TrimPrefix(cookieDomain, ".")
	return cookieDomain == target || strings.HasSuffix(cookieDomain, "."+target)
}

func main() {
	domain := flag.String("domain", "", "filter cookies by domain (e.g. hdhome.org)")
	outputJSON := flag.Bool("json", false, "output full cookie details as JSON")
	outputHeaders := flag.Bool("headers", false, "output as Cookie header value")
	flag.Parse()

	apiURL := os.Getenv("COOKIECLOUD_URL")
	uuid := os.Getenv("COOKIECLOUD_UUID")
	password := os.Getenv("COOKIECLOUD_PASSWORD")

	if apiURL == "" || uuid == "" || password == "" {
		fmt.Fprintln(os.Stderr, "Required environment variables:")
		fmt.Fprintln(os.Stderr, "  COOKIECLOUD_URL    - CookieCloud server URL (e.g. http://localhost:8088)")
		fmt.Fprintln(os.Stderr, "  COOKIECLOUD_UUID   - your UUID")
		fmt.Fprintln(os.Stderr, "  COOKIECLOUD_PASSWORD - your password (used as encryption key)")
		os.Exit(1)
	}

	encrypted, err := fetchEncrypted(apiURL, uuid)
	if err != nil {
		log.Fatalf("fetch: %v", err)
	}

	cookies, err := decryptCookies(uuid, password, encrypted)
	if err != nil {
		log.Fatalf("decrypt: %v", err)
	}

	if *domain != "" {
		var matched []cookieData
		for _, cs := range cookies {
			for _, c := range cs {
				if domainMatches(c.Domain, *domain) {
					matched = append(matched, c)
				}
			}
		}
		cookies = map[string][]cookieData{*domain: matched}
	}

	switch {
	case *outputJSON:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(cookies); err != nil {
			log.Fatalf("json encode: %v", err)
		}
	case *outputHeaders:
		var parts []string
		for _, cs := range cookies {
			if *domain != "" && len(cs) == 0 {
				continue
			}
			for _, c := range cs {
				parts = append(parts, c.Name+"="+c.Value)
			}
		}
		slices.Sort(parts)
		fmt.Println(strings.Join(parts, "; "))
	default:
		if *domain != "" {
			fmt.Fprintf(os.Stderr, "Cookies for %s:\n", *domain)
			cs := cookies[*domain]
			if len(cs) == 0 {
				fmt.Fprintf(os.Stderr, "  (none found)\n")
				os.Exit(1)
			}
			for _, c := range cs {
				fmt.Printf("  %s=%s\n", c.Name, c.Value)
			}
			fmt.Fprintln(os.Stderr)
			fmt.Fprintln(os.Stderr, "Cookie header:")
			var parts []string
			for _, c := range cs {
				parts = append(parts, c.Name+"="+c.Value)
			}
			fmt.Println(strings.Join(parts, "; "))
		} else {
			fmt.Fprintf(os.Stderr, "Domains found (%d total):\n", len(cookies))
			for d := range cookies {
				fmt.Fprintf(os.Stderr, "  %s (%d cookies)\n", d, len(cookies[d]))
			}
		}
	}
}
