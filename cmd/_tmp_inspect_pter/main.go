// 扫描 PTer 详情页 HTML 找种子实际 tag 的 selector（区别于侧栏 sb-tag）
package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/ranfish/pt-forward/internal/crypto"
	_ "github.com/mattn/go-sqlite3"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func main() {
	dbPath := "/tmp/ptf.db"
	if len(os.Args) > 1 {
		dbPath = os.Args[1]
	}
	db, _ := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})

	var site struct{ Cookie, BaseURL string }
	db.Table("sites").Where("domain LIKE ?", "%pterclub%").Select("cookie, base_url").Scan(&site)

	var encKey string
	db.Table("system_settings").Where("key = ?", "encryption_key").Select("value").Scan(&encKey)
	enc, _ := crypto.NewCredentialEncryptor(encKey)
	cookie, _ := enc.Decrypt(site.Cookie)

	req, _ := http.NewRequestWithContext(context.Background(), "GET", site.BaseURL+"/details.php?id=859990", nil)
	req.Header.Set("Cookie", cookie)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, _ := http.DefaultClient.Do(req)
	body, _ := io.ReadAll(resp.Body)
	html := string(body)
	fmt.Println("HTML 长度:", len(html))

	// 找所有 class 含 "tag" 的元素（不限 a/span）
	fmt.Println("\n=== 所有 class 含 'tag' 的元素类型 ===")
	re := regexp.MustCompile(`<(\w+)[^>]*\sclass="([^"]*tag[^"]*)"[^>]*>`)
	seen := map[string]int{}
	for _, m := range re.FindAllStringSubmatch(html, -1) {
		// 提取 class 的第一个 token 作为类型
		cls := strings.Fields(m[2])
		key := m[1] + "." + cls[0]
		seen[key]++
	}
	for k, v := range seen {
		fmt.Printf("  %s: %d\n", k, v)
	}

	// 找种子实际的 tag 区（通常在"#outer td"内或 h1/title 附近）
	fmt.Println("\n=== '#outer' 区域含 'tag' 的元素 ===")
	if i := strings.Index(html, "id=\"outer\""); i > 0 {
		chunk := html[i:]
		for _, m := range re.FindAllStringSubmatch(chunk, -1) {
			if strings.Contains(m[2], "sb-tag") || strings.Contains(m[2], "chs_tag") {
				continue
			}
			fmt.Printf("  <%s class=%q>\n", m[1], m[2])
		}
	}

	// 找标题区附近的 tag
	fmt.Println("\n=== 标题区 <h1> 或 <h2> 附近 HTML（前 1000 字符）===")
	reTitle := regexp.MustCompile(`<(h1|h2|title)[^>]*>[^<]{0,200}</\1>`)
	for _, m := range reTitle.FindAllString(html, 3) {
		fmt.Printf("  %s\n\n", m[:min(200, len(m))])
	}

	// 检查种子是否有"种子标签"区
	fmt.Println("\n=== 'torrent_tag' / 'seed_tag' / 'tag_list' 关键字 ===")
	for _, kw := range []string{"torrent_tag", "seed_tag", "tag_list", "torrent-tags", "tags-list", "实际标签"} {
		if strings.Contains(html, kw) {
			fmt.Printf("  命中: %s\n", kw)
		}
	}

	// 找 "tag_internal"（发布表单字段）周围
	fmt.Println("\n=== 'tag_internal' 上下文 ===")
	if i := strings.Index(html, "tag_internal"); i > 0 {
		ctx := html[max(0, i-300):i+300]
		ctx = strings.ReplaceAll(ctx, "\n", " ")
		fmt.Println(ctx)
	}
}

func min(a, b int) int { if a < b { return a }; return b }
func max(a, b int) int { if a > b { return a }; return b }
