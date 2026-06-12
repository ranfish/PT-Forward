package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/ranfish/pt-forward/internal/crypto"
	"github.com/ranfish/pt-forward/internal/model"
)

func main() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	db, err := gorm.Open(sqlite.Open("/home/incast/PT-Forward/data/pt-forward.db"), &gorm.Config{})
	if err != nil {
		panic(err)
	}

	var kv struct{ Value string }
	db.Raw("SELECT value FROM system_settings WHERE key = 'encryption_key'").Scan(&kv)
	enc, err := crypto.NewCredentialEncryptor(kv.Value)
	if err != nil {
		panic(err)
	}
	if err := crypto.RegisterCallbacks(db, enc, logger); err != nil {
		panic(err)
	}

	targetIDs := map[uint]bool{69: true}

	var sites []model.Site
	db.Where("id IN ?", []uint{69}).Find(&sites)

	for _, s := range sites {
		var transport http.RoundTripper = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: s.SkipSSLVerify},
			Proxy:           nil,
		}
		if s.ProxyURL != "" {
			proxyURL, err := url.Parse(s.ProxyURL)
			if err == nil {
				transport = &http.Transport{
					TLSClientConfig: &tls.Config{InsecureSkipVerify: s.SkipSSLVerify},
					Proxy:           http.ProxyURL(proxyURL),
				}
			}
		}
		client := &http.Client{
			Timeout: 15 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
			Transport: transport,
		}

	for _, s := range sites {
		if !targetIDs[s.ID] {
			continue
		}

		apiBase := "https://" + s.Domain
		if s.APIDomain != "" {
			apiBase = "https://" + s.APIDomain
		}

		hasPasskey := s.Passkey != "" && len(s.Passkey) == 32
		hasCookie := s.Cookie != ""

		fmt.Printf("=== [%d] %s (%s) ===\n", s.ID, s.Name, s.Domain)
		fmt.Printf("  api_base=%s  proxy=%s  has_pk=%v(%d)  has_ck=%v(%d)\n", apiBase, s.ProxyURL, hasPasskey, len(s.Passkey), hasCookie, len(s.Cookie))

		paths := []string{"/api/pieces-hash", "/api/torrents/pieces-hash"}
		for _, path := range paths {
			bodyMap := map[string]interface{}{
				"pieces_hash": []string{"a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2"},
			}
			if hasPasskey {
				bodyMap["passkey"] = s.Passkey
			}
			bodyBytes, _ := json.Marshal(bodyMap)

			req, _ := http.NewRequestWithContext(context.Background(), "POST", apiBase+path, bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Accept", "application/json")
			req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36")
			if hasCookie {
				req.Header.Set("Cookie", s.Cookie)
			}

			resp, err := client.Do(req)
			if err != nil {
				fmt.Printf("  %s => ERROR: %v\n", path, err)
				continue
			}

			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			fmt.Printf("  %s => HTTP %d (%d bytes)\n", path, resp.StatusCode, len(body))
			preview := string(body)
			if len(preview) > 300 {
				preview = preview[:300] + "..."
			}
			fmt.Printf("  %s\n\n", preview)

			if resp.StatusCode == 200 || resp.StatusCode == 401 || resp.StatusCode == 403 {
				break
			}
		}
		fmt.Println()
	}
	}
}
