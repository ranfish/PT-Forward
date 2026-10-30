package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/ranfish/pt-forward/internal/httpclient"
	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/site"

	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// §59.370 流水线 Phase 1：五二（52pt.site）上传页实时抓取——与 docs/32 比对
func min(a, b int) int { if a < b { return a }; return b }

func main() {
	gdb, err := gorm.Open(sqlite.Open("data/pt-forward.db?_journal_mode=WAL&_busy_timeout=5000"), &gorm.Config{Logger: gormlogger.Discard})
	if err != nil {
		panic(err)
	}
	log, _ := zap.NewProduction()
	provider := site.NewProvider(gdb, nil, log)
	cfg, err := provider.GetSiteConfig(context.Background(), "五二")
	if err != nil {
		panic(err)
	}
	var siteRow model.Site
	proxy := ""
	if err := gdb.Where("domain = ?", "52pt.site").First(&siteRow).Error; err == nil {
		proxy = site.ResolveSiteProxy(gdb, context.Background(), &siteRow)
	}
	client := httpclient.NewSiteHTTPClient(httpclient.SiteHTTPConfig{ProxyURL: proxy})
	var html string
	for i := 0; i < 3; i++ {
		req, _ := http.NewRequest("GET", "https://52pt.site/upload.php", nil)
		httpclient.SetBrowserHeaders(req.Header, cfg.Cookie)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		req = req.WithContext(ctx)
		resp, err := client.Do(req)
		if err != nil {
			cancel()
			fmt.Printf("attempt %d: %v\n", i+1, err)
			time.Sleep(2 * time.Second)
			continue
		}
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		cancel()
		html = string(b)
		fmt.Printf("attempt %d: status=%d len=%d\n", i+1, resp.StatusCode, len(html))
		if resp.StatusCode == 200 && strings.Contains(html, "<select") {
			break
		}
		fmt.Printf("  body head: %.120q\n", html[:min(120, len(html))])
		if strings.Contains(html, "takelogin") {
			fmt.Println("登录墙——凭证问题")
			return
		}
		time.Sleep(2 * time.Second)
	}
	if html == "" {
		panic("failed")
	}
	fmt.Printf("PAGE: %.600q\n\n", html[:min(600, len(html))])
	for _, sel := range []string{"name=\"type\"", "name=\"medium_sel\"", "name=\"codec_sel\"", "name=\"standard_sel\"", "name=\"audiocodec_sel\"", "name=\"team_sel\"", "name=\"tags[]\"", "name=\"small_descr\"", "name=\"descr\"", "name=\"url\"", "name=\"uplver\""} {
		i := strings.Index(html, sel)
		if i < 0 {
			fmt.Printf("\n=== %s: 不存在 ===\n", sel)
			continue
		}
		end := i + 3000
		if end > len(html) {
			end = len(html)
		}
		fmt.Printf("\n=== %s ===\n", sel)
		n := 0
		for _, line := range strings.Split(html[i:end], "\n") {
			t := strings.TrimSpace(line)
			if strings.Contains(t, "<option") || strings.Contains(t, "<input") && strings.Contains(t, "checkbox") {
				fmt.Println(t)
				n++
				if n > 45 {
					break
				}
			}
			if strings.Contains(t, "</select>") {
				break
			}
		}
	}
}
