// decrypt-cookie: 从 PT-Forward SQLite DB 解密站点 cookie。
//
// 用法：
//   ./decrypt-cookie -db /path/to/pt-forward.db -domain pt.keepfrds.com
//   ./decrypt-cookie -db /path/to/pt-forward.db -domain pt.keepfrds.com | xargs -I{} curl -b '{}' ...
//
// 环境变量：
//   DB_PATH       — 默认 DB 路径（替代 -db 参数）
package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"flag"
	"fmt"
	"os"
	"strings"

	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/pbkdf2"
)

const (
	ciphertextPrefix   = "enc:"
	ciphertextV2Prefix = "enc2:"
	pbkdf2Iterations   = 100000
)

func deriveSalt(key string) []byte {
	h := sha256.Sum256(append([]byte("pt-forward-salt-v1:"), key...))
	return h[:]
}

func newAEAD(key string, salt []byte) (cipher.AEAD, error) {
	aesKey := pbkdf2.Key([]byte(key), salt, pbkdf2Iterations, 32, sha256.New)
	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func decrypt(encKey, ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}
	if !strings.HasPrefix(ciphertext, ciphertextPrefix) && !strings.HasPrefix(ciphertext, ciphertextV2Prefix) {
		return ciphertext, nil // 明文
	}
	salt := deriveSalt(encKey)
	aead, err := newAEAD(encKey, salt)
	if err != nil {
		return "", err
	}
	encoded := ""
	if strings.HasPrefix(ciphertext, ciphertextV2Prefix) {
		encoded = ciphertext[len(ciphertextV2Prefix):]
	} else {
		// legacy salt fallback
		legacySalt := []byte("pt-forward-credential-encryption")
		aead, err = newAEAD(encKey, legacySalt)
		if err != nil {
			return "", err
		}
		encoded = ciphertext[len(ciphertextPrefix):]
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	nonce := data[:aead.NonceSize()]
	sealed := data[aead.NonceSize():]
	plaintext, err := aead.Open(nil, nonce, sealed, nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

func main() {
	dbPath := flag.String("db", "", "SQLite DB 路径（默认从 DB_PATH 环境变量读取，再默认 data/pt-forward.db）")
	domain := flag.String("domain", "", "站点域名（模糊匹配）")
	flag.Parse()

	if *dbPath == "" {
		*dbPath = os.Getenv("DB_PATH")
	}
	if *dbPath == "" {
		*dbPath = "data/pt-forward.db"
	}
	if *domain == "" {
		fmt.Fprintln(os.Stderr, "用法: decrypt-cookie -domain <domain> [-db <path>]")
		os.Exit(1)
	}

	db, err := sql.Open("sqlite3", *dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open db: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	var encKey, encCookie string
	err = db.QueryRow("SELECT value FROM system_settings WHERE key='encryption_key'").Scan(&encKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read encryption_key: %v\n", err)
		os.Exit(1)
	}
	err = db.QueryRow("SELECT cookie FROM sites WHERE domain LIKE ?", "%"+*domain+"%").Scan(&encCookie)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read cookie for %s: %v\n", *domain, err)
		os.Exit(1)
	}

	plain, err := decrypt(encKey, encCookie)
	if err != nil {
		fmt.Fprintf(os.Stderr, "decrypt: %v\n", err)
		os.Exit(1)
	}
	fmt.Print(plain)
}
