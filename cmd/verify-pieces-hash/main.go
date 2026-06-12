package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/ranfish/pt-forward/internal/crypto"
	"github.com/ranfish/pt-forward/internal/model"
)

type siteRow struct {
	ID                    int
	Name                  string
	Domain                string
	APIDomain             string
	Framework             string
	Passkey               string
	Cookie                string
	SupportsPiecesHashAPI bool
}

type verifyResult struct {
	ID       uint
	Name     string
	Domain   string
	APIBase  string
	Path     string
	HTTPCode int
	Ret      interface{}
	Msg      string
	Status   string
}

func main() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	db, err := gorm.Open(sqlite.Open("/home/incast/PT-Forward/data/pt-forward.db"), &gorm.Config{})
	if err != nil {
		panic(err)
	}

	key := os.Getenv("ENCRYPTION_KEY")
	if key == "" {
		var kv struct{ Value string }
		db.Raw("SELECT value FROM system_settings WHERE key = 'encryption_key'").Scan(&kv)
		key = kv.Value
	}
	if key == "" {
		panic("no encryption key")
	}

	enc, err := crypto.NewCredentialEncryptor(key)
	if err != nil {
		panic(err)
	}
	if err := crypto.RegisterCallbacks(db, enc, logger); err != nil {
		panic(err)
	}

	var sites []model.Site
	db.Where("framework = ? AND supports_pieces_hash_api = ?", "nexusphp", true).Find(&sites)

	logger.Info("loaded sites", zap.Int("count", len(sites)))

	testHash := "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2"
	client := &http.Client{Timeout: 15 * time.Second, Transport: &http.Transport{
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
		Proxy:               nil,
		MaxIdleConnsPerHost: 2,
	}}

	type job struct {
		site  model.Site
		index int
	}
	jobs := make(chan job, len(sites))
	results := make(chan verifyResult, len(sites))

	workers := 10
	for w := 0; w < workers; w++ {
		go func() {
			for j := range jobs {
				results <- verifySite(j.site, testHash, client)
			}
		}()
	}

	for i, s := range sites {
		jobs <- job{site: s, index: i}
	}
	close(jobs)

	var allResults []verifyResult
	for i := 0; i < len(sites); i++ {
		r := <-results
		allResults = append(allResults, r)
		fmt.Fprintf(os.Stderr, "\r%d/%d", i+1, len(sites))
	}
	fmt.Fprintln(os.Stderr)

	sort.Slice(allResults, func(i, j int) bool {
		return allResults[i].ID < allResults[j].ID
	})

	fmt.Println("site_id\tname\tdomain\tapi_base\tpath\thttp_code\tret\tmsg\tstatus")
	for _, r := range allResults {
		retStr := fmt.Sprintf("%v", r.Ret)
		if len(retStr) > 20 {
			retStr = retStr[:20]
		}
		msgStr := r.Msg
		if len(msgStr) > 50 {
			msgStr = msgStr[:50]
		}
		fmt.Printf("%d\t%s\t%s\t%s\t%s\t%d\t%s\t%s\t%s\n",
			r.ID, r.Name, r.Domain, r.APIBase, r.Path, r.HTTPCode, retStr, msgStr, r.Status)
	}

	fmt.Fprintln(os.Stderr)
	ok, authFail, notFound, connFail, other := 0, 0, 0, 0, 0
	for _, r := range allResults {
		switch r.Status {
		case "OK":
			ok++
		case "AUTH_FAIL":
			authFail++
		case "API_NOT_FOUND":
			notFound++
		case "CONN_FAIL":
			connFail++
		default:
			other++
		}
	}
	fmt.Fprintf(os.Stderr, "OK: %d  AUTH_FAIL: %d  API_NOT_FOUND: %d  CONN_FAIL: %d  OTHER: %d  Total: %d\n",
		ok, authFail, notFound, connFail, other, len(allResults))
}

func verifySite(s model.Site, testHash string, client *http.Client) verifyResult {
	apiBase := "https://" + s.Domain
	if s.APIDomain != "" {
		apiBase = "https://" + s.APIDomain
	}

	hasPasskey := s.Passkey != "" && len(s.Passkey) == 32
	hasCookie := s.Cookie != ""

	bodyMap := map[string]interface{}{
		"pieces_hash": []string{testHash},
	}
	if hasPasskey {
		bodyMap["passkey"] = s.Passkey
	}
	bodyBytes, _ := json.Marshal(bodyMap)

	paths := []string{"/api/pieces-hash", "/api/torrents/pieces-hash"}

	for _, path := range paths {
		url := apiBase + path
		req, err := http.NewRequestWithContext(context.Background(), "POST", url, bytes.NewReader(bodyBytes))
		if err != nil {
			return verifyResult{ID: s.ID, Name: s.Name, Domain: s.Domain, APIBase: apiBase, Status: "REQ_ERR", Msg: err.Error()}
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36")
		if hasCookie {
			req.Header.Set("Cookie", s.Cookie)
		}

		resp, err := client.Do(req)
		if err != nil {
			return verifyResult{ID: s.ID, Name: s.Name, Domain: s.Domain, APIBase: apiBase, Path: path, Status: "CONN_FAIL"}
		}
		defer resp.Body.Close()

		if resp.StatusCode == 404 {
			continue
		}

		if resp.StatusCode != 200 {
			status := "HTTP_ERR"
			if resp.StatusCode == 401 || resp.StatusCode == 403 {
				status = "AUTH_FAIL"
			}
			return verifyResult{ID: s.ID, Name: s.Name, Domain: s.Domain, APIBase: apiBase, Path: path, HTTPCode: resp.StatusCode, Status: status}
		}

		var apiResp struct {
			Ret  int            `json:"ret"`
			Msg  string         `json:"msg"`
			Data map[string]int `json:"data"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
			return verifyResult{ID: s.ID, Name: s.Name, Domain: s.Domain, APIBase: apiBase, Path: path, HTTPCode: 200, Status: "PARSE_ERR", Msg: err.Error()}
		}

		if apiResp.Ret == 0 || (apiResp.Ret == 0 && apiResp.Data != nil) {
			return verifyResult{ID: s.ID, Name: s.Name, Domain: s.Domain, APIBase: apiBase, Path: path, HTTPCode: 200, Ret: apiResp.Ret, Msg: apiResp.Msg, Status: "OK"}
		}

		if apiResp.Ret != 0 && (strings.Contains(apiResp.Msg, "not found") || strings.Contains(apiResp.Msg, "could not be found")) {
			return verifyResult{ID: s.ID, Name: s.Name, Domain: s.Domain, APIBase: apiBase, Path: path, HTTPCode: 200, Ret: apiResp.Ret, Msg: apiResp.Msg, Status: "API_ERROR"}
		}

		return verifyResult{ID: s.ID, Name: s.Name, Domain: s.Domain, APIBase: apiBase, Path: path, HTTPCode: 200, Ret: apiResp.Ret, Msg: apiResp.Msg, Status: "API_ERROR"}
	}

	return verifyResult{ID: s.ID, Name: s.Name, Domain: s.Domain, APIBase: apiBase, HTTPCode: 404, Status: "API_NOT_FOUND"}
}
