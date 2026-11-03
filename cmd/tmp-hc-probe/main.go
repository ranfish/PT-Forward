package main

// §59.338 真实组件离线驱动：qb13 导出真实种子（ExportTorrent）+
// 城市站第一步接收器 POST 实测（用户授权——不提交第二步）。验证后即删。

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/ranfish/pt-forward/internal/client"
	"github.com/ranfish/pt-forward/internal/client/qbittorrent"
	"github.com/ranfish/pt-forward/internal/crypto"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// ① qb13 真实组件（crypto 解密密码）
	db, _ := gorm.Open(sqlite.Open("/home/incast/PT-Forward/data/pt-forward.db"), &gorm.Config{})
	var keyRow struct{ Value string }
	db.Raw("SELECT value FROM system_settings WHERE key = 'encryption_key'").Scan(&keyRow)
	enc, _ := crypto.NewCredentialEncryptor(keyRow.Value)
	crypto.RegisterCallbacks(db, enc, zap.NewNop())

	logger, _ := zap.NewProduction()
	mgr := client.NewManager(db, logger)
	if err := mgr.LoadClients(ctx); err != nil {
		fmt.Println("LoadClients:", err)
		os.Exit(1)
	}
	qbi, err := mgr.Get(13)
	if err != nil {
		fmt.Println("client manager:", err)
		os.Exit(1)
	}
	qb, ok := qbi.(*qbittorrent.QBClient)
	if !ok {
		fmt.Println("client 13 非 qbittorrent:", fmt.Sprintf("%T", qbi))
		os.Exit(1)
	}
	hash := "3d587ae1c5b7914613a1051e0eff9943df979604" // Helluva.Boss
	torrentData, err := qb.ExportTorrent(ctx, hash)
	if err != nil {
		fmt.Println("导出:", err)
		os.Exit(1)
	}
	os.WriteFile("/tmp/opencode/hc_real.torrent", torrentData, 0644)
	fmt.Printf("① 真实种子导出: %dB ✓\n", len(torrentData))

	// ② 抓第一步页面解析 vf/uid/action
	client := &http.Client{Timeout: 30 * time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{}},
		CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }}
	ck := strings.TrimSpace(firstLine("/tmp/opencode/hc_cookie.txt"))
	req, _ := http.NewRequestWithContext(ctx, "GET", "https://hdcity.city/upload", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36")
	req.Header.Set("Cookie", ck)
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("抓页面:", err)
		os.Exit(1)
	}
	page, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	vfM := regexp.MustCompile(`name="(vf\d+)" value="([^"]+)"`).FindSubmatch(page)
	if vfM == nil {
		fmt.Println("vf 字段未找到——页面改版？")
		os.Exit(1)
	}
	vfName, vfVal := string(vfM[1]), string(vfM[2])
	uidM := regexp.MustCompile(`name="uid" value="([^"]+)"`).FindSubmatch(page)
	actionM := regexp.MustCompile(`action="(https://hctres[^"]+)"`).FindSubmatch(page)
	fmt.Printf("② 页面解析: %s=%s… uid=%s…\n   action=%s\n", vfName, vfVal[:12], string(uidM[1])[:12], string(actionM[1]))

	// ③ POST 真实种子到接收器（不提交第二步）
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, _ := w.CreateFormFile("file", "upload.torrent")
	fw.Write(torrentData)
	w.WriteField("name", "Helluva.Boss.S02.2020.1080p.AMZN.WEB-DL.H.264.DDP5.1")
	w.WriteField("citeid", "0")
	w.WriteField(vfName, vfVal)
	w.WriteField("domainname", "hdcity.city")
	w.WriteField("uid", string(uidM[1]))
	w.Close()

	req2, _ := http.NewRequestWithContext(ctx, "POST", string(actionM[1]), &buf)
	req2.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36")
	req2.Header.Set("Content-Type", w.FormDataContentType())
	req2.Header.Set("Origin", "https://hdcity.city")
	req2.Header.Set("Referer", "https://hdcity.city/upload")
	req2.Header.Set("Cookie", ck)
	resp2, err := client.Do(req2)
	if err != nil {
		fmt.Println("POST 接收器:", err)
		os.Exit(1)
	}
	body, _ := io.ReadAll(resp2.Body)
	resp2.Body.Close()
	fmt.Printf("③ 接收器响应: HTTP %d | %dB | Content-Type: %s\n", resp2.StatusCode, len(body), resp2.Header.Get("Content-Type"))
	if loc := resp2.Header.Get("Location"); loc != "" {
		fmt.Println("  Location:", unescape(loc))
	}
	s := string(body)
	if i := strings.Index(s, "tfu"); i >= 0 {
		fmt.Println("  ★ tfu 上下文:", clamp(s, i-120, i+400))
	} else {
		fmt.Println("  body 前 600B:", clamp(s, 0, 600))
	}
	// JSON 形态探测
	var j map[string]any
	if json.Unmarshal(body, &j) == nil {
		fmt.Println("  JSON 键:", keys(j))
	}

	// ④ 立即 GET 第二步表单页（只读——不提交）
	loc := resp2.Header.Get("Location")
	if strings.HasPrefix(loc, "//") {
		loc = "https:" + loc
	}
	client2 := &http.Client{Timeout: 30 * time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{}}} // 正常跟随
	req3, _ := http.NewRequestWithContext(ctx, "GET", loc, nil)
	req3.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36")
	req3.Header.Set("Cookie", ck)
	resp3, err := client2.Do(req3)
	if err != nil {
		fmt.Println("④ GET 第二步:", err)
		return
	}
	page2, _ := io.ReadAll(resp3.Body)
	resp3.Body.Close()
	os.WriteFile("/tmp/opencode/hc_step2.html", page2, 0644)
	fmt.Printf("④ 第二步表单页: HTTP %d | %dB → hc_step2.html\n", resp3.StatusCode, len(page2))
	// name/bigname 行 label 提取
	for _, kw := range []string{"bigname", `name="name"`} {
		for _, m := range []*regexp.Regexp{
			regexp.MustCompile(`(?s)<tr[^>]*>(?:(?!</tr>).)*?` + kw + `(?:(?!</tr>).)*?</tr>`),
		} {
			if row := m.FindString(string(page2)); row != "" {
				clean := strings.Join(strings.Fields(replaceTags(row)), " ")
				fmt.Printf("  %s 行: %s\n", kw, clean[:min2(220, len(clean))])
				break
			}
		}
	}
}

func replaceTags(s string) string {
	return regexp.MustCompile(`<[^>]+>`).ReplaceAllString(s, " | ")
}
func min2(a, b int) int { if a > b { return b }; return a }

func firstLine(p string) string {
	b, _ := os.ReadFile(p)
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	return strings.TrimSpace(lines[0])
}
func unescape(s string) string {
	if u, err := url.QueryUnescape(s); err == nil {
		return u
	}
	return s
}
func clamp(s string, a, b int) string {
	if a < 0 {
		a = 0
	}
	if b > len(s) {
		b = len(s)
	}
	return s[a:b]
}
func keys(m map[string]any) []string {
	out := []string{}
	for k := range m {
		out = append(out, k)
	}
	return out
}
