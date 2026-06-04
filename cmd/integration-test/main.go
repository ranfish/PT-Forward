package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/ranfish/pt-forward/internal/adapter"
	"github.com/ranfish/pt-forward/internal/crypto"
	"github.com/ranfish/pt-forward/internal/httpclient"
	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/site"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func main() {
	db := openDB()
	logger := zap.Must(zap.NewProduction())
	defer logger.Sync()

	provider := site.NewProvider(db, adapter.NewFactory(logger), logger)
	factory := adapter.NewFactory(logger)

	var allSites []model.Site
	if err := db.Find(&allSites).Error; err != nil {
		fmt.Fprintf(os.Stderr, "query sites: %v\n", err)
		os.Exit(1)
	}

	skipSites := map[string]string{}

	var passCount, failCount, skipCount int
	var failList []string

	for _, s := range allSites {
		if reason, ok := skipSites[s.Name]; ok {
			fmt.Printf("SKIP  %-12s %s\n", s.Name, reason)
			skipCount++
			continue
		}

		ctx0, cancel0 := context.WithTimeout(context.Background(), 20*time.Second)
		config, err := provider.GetSiteConfig(ctx0, s.Domain)
		cancel0()
		if err != nil {
			fmt.Printf("SKIP  %-12s GetSiteConfig: %v\n", s.Name, err)
			skipCount++
			time.Sleep(500 * time.Millisecond)
			continue
		}

		if config.Passkey == "" && config.Cookie == "" && config.APIKey == "" && config.BearerToken == "" {
			fmt.Printf("SKIP  %-12s 无凭据\n", s.Name)
			skipCount++
			continue
		}

		if len(os.Args) > 1 && os.Args[1] != s.Name && os.Args[1] != s.Domain {
			continue
		}
		if len(os.Args) > 1 {
			fmt.Fprintf(os.Stderr, "DEBUG %s: tpl=%q passkey=%q rsskey=%q mode=%q proxy=%q cookie_len=%d\n",
				s.Name, config.DownloadURLTemplate, config.Passkey, config.RSSKey, config.DownloadMode, config.ProxyURL, len(config.Cookie))
		}

		doer := adapter.NewHTTPDoerWithSite(config.ProxyURL, config.SkipSSLVerify)
		a := factory.Create(s.Framework, doer)

		torrentIDs, err := resolveTorrentIDs(config, s.Framework)
		if err != nil {
			fmt.Printf("SKIP  %-12s 获取种子ID: %v\n", s.Name, err)
			skipCount++
			time.Sleep(500 * time.Millisecond)
			continue
		}

		var data []byte
		var usedID string
		for _, tid := range torrentIDs {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			d, dlErr := a.DownloadTorrent(ctx, config, tid)
			cancel()
			if dlErr == nil {
				data = d
				usedID = tid
				break
			}
			fmt.Printf("  TRY  %-12s id=%-40s %v\n", s.Name, tid, dlErr)
		}

		if data == nil {
			fmt.Printf("FAIL  %-12s all %d candidates failed\n", s.Name, len(torrentIDs))
			failCount++
			failList = append(failList, s.Name)
		} else if len(data) < 4 || !isBencode(data) {
			preview := string(data)
			if len(preview) > 60 {
				preview = preview[:60]
			}
			fmt.Printf("FAIL  %-12s id=%-40s 非种子(%dB): %s\n", s.Name, usedID, len(data), preview)
			failCount++
			failList = append(failList, s.Name)
		} else {
			fmt.Printf("PASS  %-12s id=%-40s OK (%dB)\n", s.Name, usedID, len(data))
			passCount++
		}

		time.Sleep(2 * time.Second)
	}

	fmt.Println()
	fmt.Println("========== DOWNLOAD INTEGRATION TEST RESULTS ==========")
	fmt.Printf("TOTAL: %d  PASS: %d  FAIL: %d  SKIP: %d\n",
		passCount+failCount+skipCount, passCount, failCount, skipCount)
	if len(failList) > 0 {
		fmt.Printf("FAILED: %s\n", strings.Join(failList, ", "))
		os.Exit(1)
	}
	fmt.Println("ALL PASSED!")
}

func openDB() *gorm.DB {
	dbPath := "/home/incast/PT-Forward/data/pt-forward.db"
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "open db: %v\n", err)
		os.Exit(1)
	}
	enc, err := crypto.NewCredentialEncryptor("7e6ed63684b150592f2668ccb89eafe72ba189e659b1f619359b5c1a47a420f4")
	if err != nil {
		fmt.Fprintf(os.Stderr, "encryptor: %v\n", err)
		os.Exit(1)
	}
	if err := crypto.RegisterCallbacks(db, enc, zap.NewNop()); err != nil {
		fmt.Fprintf(os.Stderr, "crypto callbacks: %v\n", err)
		os.Exit(1)
	}
	return db
}

func resolveTorrentIDs(config *model.SiteConfig, framework string) ([]string, error) {
	if framework == "yemapt" {
		id, err := resolveYemaptTorrentID(config)
		return []string{id}, err
	}
	if framework == "tnode" {
		id, err := resolveTnodeTorrentID(config)
		return []string{id}, err
	}
	if framework == "unit3d" {
		id, err := resolveUnit3DTorrentID(config)
		return []string{id}, err
	}
	if framework == "rousi" {
		return resolveRousiTorrentIDs(config)
	}
	if framework == "mteam" {
		return resolveMTeamTorrentIDs(config)
	}
	if config.Domain == "star-space.net" {
		id, err := resolveStarSpaceTorrentID(config)
		return []string{id}, err
	}
	browseURL := buildBrowseURL(config, framework)
	if browseURL == "" {
		return nil, fmt.Errorf("no browse URL for %s", config.Domain)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", browseURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
	if config.Cookie != "" {
		req.Header.Set("Cookie", config.Cookie)
	}

	client := httpclient.NewSiteHTTPClient(httpclient.SiteHTTPConfig{
		ProxyURL:      config.ProxyURL,
		SkipSSLVerify: config.SkipSSLVerify,
	})
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var buf bytes.Buffer
	io.Copy(&buf, resp.Body)
	html := buf.String()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d (%dB)", resp.StatusCode, len(html))
	}

	ids := extractIDs(html, framework, config.Domain)
	if len(ids) == 0 {
		return nil, fmt.Errorf("no torrent ID in page (%dB)", len(html))
	}
	return ids, nil
}

func buildBrowseURL(config *model.SiteConfig, framework string) string {
	domain := config.Domain
	base := config.BaseURL
	if base == "" {
		base = "https://" + domain
	}
	if domain == "hdroute.org" {
		return "http://hdroute.org/browse.php"
	}
	switch framework {
	case "nexusphp":
		if domain == "totheglory.im" {
			return base + "/browse.php"
		}
		return base + "/torrents.php"
	case "generic":
		return base + "/browse.php"
	case "gazelle", "tnode":
		return base + "/torrents.php"
	case "unit3d":
		return base + "/torrents"
	case "rousi":
		return base + "/torrents.php"
	case "yemapt":
		return ""
	}
	return base + "/torrents.php"
}

func extractID(html, framework, domain string) string {
	ids := extractIDs(html, framework, domain)
	if len(ids) == 0 {
		return ""
	}
	return ids[0]
}

func extractIDs(html, framework, domain string) []string {
	var patterns []*regexp.Regexp

	switch {
	case domain == "totheglory.im":
		patterns = []*regexp.Regexp{
			regexp.MustCompile(`/dl/(\d+)/`),
			regexp.MustCompile(`details\.php\?id=(\d+)`),
		}
	case domain == "hdcity.city":
		patterns = []*regexp.Regexp{
			regexp.MustCompile(`download\?id=(\d+)`),
		}
	case framework == "gazelle":
		patterns = []*regexp.Regexp{
			regexp.MustCompile(`torrentid=(\d+)`),
		}
	case framework == "unit3d":
		patterns = []*regexp.Regexp{
			regexp.MustCompile(`/torrents/(\d+)`),
			regexp.MustCompile(`/torrent/(\d+)`),
		}
	default:
		patterns = []*regexp.Regexp{
			regexp.MustCompile(`download\.php\?id=(\d+)`),
			regexp.MustCompile(`download\?id=(\d+)`),
			regexp.MustCompile(`details\.php\?id=(\d+)`),
		}
	}

	seen := map[string]bool{}
	var ids []string
	for _, re := range patterns {
		for _, m := range re.FindAllStringSubmatch(html, -1) {
			if len(m) >= 2 && !seen[m[1]] {
				seen[m[1]] = true
				ids = append(ids, m[1])
			}
		}
		if len(ids) >= 10 {
			break
		}
	}
	return ids
}

func resolveYemaptTorrentID(config *model.SiteConfig) (string, error) {
	if config.APIKey == "" {
		return "", fmt.Errorf("yemapt: no api key")
	}
	base := config.BaseURL
	if base == "" {
		base = "https://" + config.Domain
	}
	reqURL := base + "/openApi/torrent/fetchTorrentIdWithPiecesHash.json"
	body := `{"piecesHashList":["0000000000000000000000000000000000000000"]}`
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "POST", reqURL, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", config.APIKey)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	client := httpclient.NewSiteHTTPClient(httpclient.SiteHTTPConfig{
		ProxyURL:      config.ProxyURL,
		SkipSSLVerify: config.SkipSSLVerify,
	})
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("yemapt openApi: %v", err)
	}
	defer resp.Body.Close()
	var buf bytes.Buffer
	io.Copy(&buf, resp.Body)
	var result struct {
		Success bool              `json:"success"`
		Data    map[string]string `json:"data"`
	}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		return "", fmt.Errorf("yemapt parse: %v", err)
	}
	for _, id := range result.Data {
		return id, nil
	}
	reqURL2 := base + "/openApi/torrent/fetchTorrentIdWithPiecesHash.json"
	body2 := `{"piecesHashList":["e9f3f5dd32abcd6fc0fea4d43d32559cf0309764"]}`
	req2, _ := http.NewRequestWithContext(ctx, "POST", reqURL2, strings.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Authorization", config.APIKey)
	req2.Header.Set("User-Agent", "Mozilla/5.0")
	resp2, err := client.Do(req2)
	if err != nil {
		return "10000", nil
	}
	defer resp2.Body.Close()
	var buf2 bytes.Buffer
	io.Copy(&buf2, resp2.Body)
	var result2 struct {
		Success bool              `json:"success"`
		Data    map[string]string `json:"data"`
	}
	if err := json.Unmarshal(buf2.Bytes(), &result2); err != nil || len(result2.Data) == 0 {
		return "10000", nil
	}
	for _, id := range result2.Data {
		return id, nil
	}
	return "10000", nil
}

func resolveTnodeTorrentID(config *model.SiteConfig) (string, error) {
	if config.Cookie == "" {
		return "", fmt.Errorf("tnode: no cookie")
	}
	base := config.BaseURL
	if base == "" {
		base = "https://" + config.Domain
	}
	client := httpclient.NewSiteHTTPClient(httpclient.SiteHTTPConfig{
		ProxyURL:      config.ProxyURL,
		SkipSSLVerify: config.SkipSSLVerify,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	detailURL := base + "/api/torrent/download/52000"
	req, _ := http.NewRequestWithContext(ctx, "GET", detailURL, nil)
	req.Header.Set("Cookie", config.Cookie)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := client.Do(req)
	if err != nil {
		return "52000", nil
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 && resp.Header.Get("Content-Type") == "application/x-bittorrent" {
		return "52000", nil
	}
	return "52000", nil
}

func resolveUnit3DTorrentID(config *model.SiteConfig) (string, error) {
	base := config.BaseURL
	if base == "" {
		base = "https://" + config.Domain
	}
	client := httpclient.NewSiteHTTPClient(httpclient.SiteHTTPConfig{
		ProxyURL:      config.ProxyURL,
		SkipSSLVerify: config.SkipSSLVerify,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	for _, path := range []string{
		"/api/torrents?perPage=5&page=1",
		"/api/torrent/filter?perPage=5&page=1",
	} {
		req, err := http.NewRequestWithContext(ctx, "GET", base+path, nil)
		if err != nil {
			continue
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "Mozilla/5.0")
		if config.Cookie != "" {
			req.Header.Set("Cookie", config.Cookie)
		}
		if config.APIKey != "" {
			req.Header.Set("Authorization", "Bearer "+config.APIKey)
		} else if config.AuthKey != "" {
			req.Header.Set("Authorization", "Bearer "+config.AuthKey)
		}

		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		var buf bytes.Buffer
		io.Copy(&buf, resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			continue
		}

		var raw map[string]json.RawMessage
		if json.Unmarshal(buf.Bytes(), &raw) != nil {
			continue
		}
		dataField, ok := raw["data"]
		if !ok {
			continue
		}
		var torrentList []struct {
			ID json.Number `json:"id"`
		}
		if json.Unmarshal(dataField, &torrentList) != nil || len(torrentList) == 0 {
			continue
		}
		for _, t := range torrentList {
			if s := t.ID.String(); s != "" && s != "0" {
				return s, nil
			}
		}
	}

	return "", fmt.Errorf("unit3d: no torrent ID found via API")
}

func resolveRousiTorrentIDs(config *model.SiteConfig) ([]string, error) {
	if config.Passkey == "" {
		return nil, fmt.Errorf("rousi: no passkey")
	}
	base := config.BaseURL
	if base == "" {
		base = "https://" + config.Domain
	}
	client := httpclient.NewSiteHTTPClient(httpclient.SiteHTTPConfig{
		ProxyURL:      config.ProxyURL,
		SkipSSLVerify: config.SkipSSLVerify,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	reqURL := base + "/api/v1/torrents?perPage=10&page=1"
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+config.Passkey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Origin", "https://rousi.pro")
	req.Header.Set("Referer", "https://rousi.pro/")
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	io.Copy(&buf, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("rousi API HTTP %d (%dB)", resp.StatusCode, buf.Len())
	}

	var result struct {
		Code int `json:"code"`
		Data struct {
			Torrents []struct {
				ID   int    `json:"id"`
				UUID string `json:"uuid"`
			} `json:"torrents"`
		} `json:"data"`
	}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		return nil, fmt.Errorf("rousi: parse error: %v", err)
	}
	var ids []string
	for _, t := range result.Data.Torrents {
		tid := t.UUID
		if tid == "" {
			tid = fmt.Sprintf("%d", t.ID)
		}
		ids = append(ids, tid)
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("rousi: no torrents in API response")
	}
	return ids, nil
}

func resolveMTeamTorrentIDs(config *model.SiteConfig) ([]string, error) {
	if config.APIKey == "" {
		return nil, fmt.Errorf("mteam: no api key")
	}
	base := config.BaseURL
	if base == "" {
		base = "https://" + config.Domain
	}
	client := httpclient.NewSiteHTTPClient(httpclient.SiteHTTPConfig{
		ProxyURL:      config.ProxyURL,
		SkipSSLVerify: config.SkipSSLVerify,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	payload, _ := json.Marshal(map[string]interface{}{
		"pageNumber": 1,
		"pageSize":   10,
		"mode":       "normal",
	})
	req, err := http.NewRequestWithContext(ctx, "POST", base+"/api/torrent/search", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", config.APIKey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	io.Copy(&buf, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("mteam search API HTTP %d (%dB)", resp.StatusCode, buf.Len())
	}

	var result struct {
		Data struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
		} `json:"data"`
	}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		return nil, fmt.Errorf("mteam: parse error: %v", err)
	}
	var ids []string
	for _, t := range result.Data.Data {
		if t.ID != "" {
			ids = append(ids, t.ID)
		}
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("mteam: no torrents in search response")
	}
	return ids, nil
}

func resolveStarSpaceTorrentID(config *model.SiteConfig) (string, error) {
	base := config.BaseURL
	if base == "" {
		base = "https://" + config.Domain
	}
	client := httpclient.NewSiteHTTPClient(httpclient.SiteHTTPConfig{
		ProxyURL:      config.ProxyURL,
		SkipSSLVerify: config.SkipSSLVerify,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	for _, path := range []string{
		"/p_torrent/video_list_g.php",
		"/browse.php",
	} {
		reqURL := base + path
		req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
		if err != nil {
			continue
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
		if config.Cookie != "" {
			req.Header.Set("Cookie", config.Cookie)
		}

		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		var buf bytes.Buffer
		io.Copy(&buf, resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			continue
		}
		html := buf.String()
		for _, re := range []*regexp.Regexp{
			regexp.MustCompile(`download\.php\?tid=(\d+)`),
			regexp.MustCompile(`download\?id=(\d+)`),
			regexp.MustCompile(`details\.php\?id=(\d+)`),
		} {
			if m := re.FindStringSubmatch(html); len(m) >= 2 {
				return m[1], nil
			}
		}
	}
	return "", fmt.Errorf("star-space: no torrent ID found")
}

func isBencode(data []byte) bool {
	return len(data) >= 3 && data[0] == 'd' && data[1] >= '0' && data[1] <= '9' && data[2] == ':'
}

// debugSite is not called, see main() above
