// cc_decrypt — CookieCloud 加密数据解密调试工具
// 用法: go run tools/cc_decrypt.go <encrypted.json> <uuid> <password> [--domains-only]
package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
)

func md5String(inputs ...string) string {
	h := md5.New()
	for _, s := range inputs {
		io.WriteString(h, s)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func bytesToKey(salt, data []byte, keyLen, bl int) (key, iv []byte) {
	var concat, lastHash []byte
	totalLen := keyLen + bl
	h := md5.New()
	for len(concat) < totalLen {
		h.Reset()
		h.Write(append(lastHash, append(data, salt...)...))
		lastHash = h.Sum(nil)
		concat = append(concat, lastHash...)
	}
	return concat[:keyLen], concat[keyLen:totalLen]
}

func main() {
	if len(os.Args) < 4 {
		fmt.Println("用法: cc_decrypt <encrypted.json> <uuid> <password> [--domains-only]")
		os.Exit(1)
	}
	uuid, password := os.Args[2], os.Args[3]
	domainsOnly := len(os.Args) > 4 && os.Args[4] == "--domains-only"

	raw, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Println("read file:", err)
		os.Exit(1)
	}
	var resp struct {
		Encrypted string `json:"encrypted"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		fmt.Println("parse json:", err)
		os.Exit(1)
	}

	// PT-Forward client.go:113 同款密钥派生
	keyPassword := md5String(uuid, "-", password)[:16]

	enc, _ := base64.StdEncoding.DecodeString(resp.Encrypted)
	if string(enc[:8]) != "Salted__" {
		fmt.Println("not OpenSSL format")
		os.Exit(1)
	}
	salt := enc[8:16]
	body := enc[16:]
	key, iv := bytesToKey(salt, []byte(keyPassword), 32, 16)
	block, _ := aes.NewCipher(key)
	mode := cipher.NewCBCDecrypter(block, iv)
	dec := make([]byte, len(body))
	mode.CryptBlocks(dec, body)
	pad := int(dec[len(dec)-1])
	if pad > 0 && pad <= 16 {
		dec = dec[:len(dec)-pad]
	}

	// PT-Forward client.go:118-120 同款结构
	var result struct {
		CookieData map[string][]struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		} `json:"cookie_data"`
	}
	if err := json.Unmarshal(dec, &result); err != nil {
		fmt.Printf("解密成功但 JSON 解析失败: %v\n前 200 字节: %s\n", err, string(dec[:min(200, len(dec))]))
		os.Exit(1)
	}

	var domains []string
	for d := range result.CookieData {
		domains = append(domains, d)
	}
	sort.Strings(domains)
	fmt.Printf("CC 域名数: %d\n", len(domains))
	for _, d := range domains {
		if domainsOnly {
			fmt.Println(d)
			continue
		}
		fmt.Printf("  %s: %d cookies\n", d, len(result.CookieData[d]))
	}
}

func min(a, b int) int { if a < b { return a }; return b }
