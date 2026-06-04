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

		torrentID, err := resolveTorrentID(config, s.Framework)
		if err != nil {
			fmt.Printf("SKIP  %-12s 获取种子ID: %v\n", s.Name, err)
			skipCount++
			time.Sleep(500 * time.Millisecond)
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		data, dlErr := a.DownloadTorrent(ctx, config, torrentID)
		cancel()

		if dlErr != nil {
			fmt.Printf("FAIL  %-12s id=%-8s %v\n", s.Name, torrentID, dlErr)
			failCount++
			failList = append(failList, s.Name)
		} else if len(data) < 4 || !isBencode(data) {
			preview := string(data)
			if len(preview) > 60 {
				preview = preview[:60]
			}
			fmt.Printf("FAIL  %-12s id=%-8s 非种子(%dB): %s\n", s.Name, torrentID, len(data), preview)
			failCount++
			failList = append(failList, s.Name)
		} else {
			fmt.Printf("PASS  %-12s id=%-8s OK (%dB)\n", s.Name, torrentID, len(data))
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

func resolveTorrentID(config *model.SiteConfig, framework string) (string, error) {
	if framework == "yemapt" {
		return resolveYemaptTorrentID(config)
	}
	if framework == "tnode" {
		return resolveTnodeTorrentID(config)
	}
	browseURL := buildBrowseURL(config, framework)
	if browseURL == "" {
		return "", fmt.Errorf("no browse URL for %s", config.Domain)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", browseURL, nil)
	if err != nil {
		return "", err
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
		return "", err
	}
	defer resp.Body.Close()

	var buf bytes.Buffer
	io.Copy(&buf, resp.Body)
	html := buf.String()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d (%dB)", resp.StatusCode, len(html))
	}

	id := extractID(html, framework, config.Domain)
	if id == "" {
		return "", fmt.Errorf("no torrent ID in page (%dB)", len(html))
	}
	return id, nil
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
			regexp.MustCompile(`/torrent/(\d+)`),
		}
	default:
		patterns = []*regexp.Regexp{
			regexp.MustCompile(`download\.php\?id=(\d+)`),
			regexp.MustCompile(`download\?id=(\d+)`),
			regexp.MustCompile(`details\.php\?id=(\d+)`),
		}
	}

	for _, re := range patterns {
		m := re.FindStringSubmatch(html)
		if len(m) >= 2 {
			return m[1]
		}
	}
	return ""
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

func isBencode(data []byte) bool {
	return len(data) >= 3 && data[0] == 'd' && data[1] >= '0' && data[1] <= '9' && data[2] == ':'
}

// debugSite is not called, see main() above
