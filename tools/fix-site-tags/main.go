// fix-site-tags §59.531: 站上种子标签修复工具（独立运维工具·不进主程序）。
//
// 场景：已发布到站点的种子因标签投递错误（如财神双 11 案：重复值被站方
// 整栏作废）被 Bot 拒——种子已入库不能重发（重发=duplicate），只能上站
// 编辑。修与发的区别（用户定案）：只需要上站按数据库重新勾选正确的标签，
// 不存在发布时的站点交互逻辑（不碰种子文件/不推种/不走 dedup）。
//
// 正确标签的唯一真相源：主程序发布链的组装单点 AssembleTagArrayForSite
// （§59.531 从 Execute ⑤段提取——工具与发布同源消费·防双链分叉）。
//
// 用法（29 上运行）：
//   go run ./tools/fix-site-tags -hash <info_hash> -site 财神 [-tid 257204] [-dry]
//   tid 省略=按 info_hash+站名查发布记录最新 torrent_id
//   -dry 只打印「站上现值 vs 应投值」差异·不提交
//
// 交互：GET edit.php（取表单全域现值）→ 只替换 tags[4][] 域 → POST
// takeedit.php（其余域原样回传——最小干预）。幂等：重复执行=提交同样的
// 正确值。
//
// 限制：NexusPHP 站（edit.php/takeedit.php 标准形态）；tp.HDR 走初态
// BuildTechProfile（不经 FinalizeTitle 仲裁——修复场景 tags 列推理产物
// 已终局·HDR 标题补位差异仅影响 MI 缺 HDR 行的边缘形态）。
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"mime/multipart"
	"net/url"
	"os"
	"regexp"
	"strings"

	"github.com/ranfish/pt-forward/internal/httpclient"
	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/publish"
	sitepkg "github.com/ranfish/pt-forward/internal/site"
	"github.com/ranfish/pt-forward/internal/titleparser"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var (
	hash   = flag.String("hash", "", "info_hash（必填）")
	site   = flag.String("site", "", "目标站名（必填·如 财神）")
	tidArg = flag.String("tid", "", "站上种子 ID（空=查发布记录）")
	dry    = flag.Bool("dry", false, "只打印差异不提交")
	dbPath = flag.String("db", "data/pt-forward.db", "SQLite DB 路径")
)

func main() {
	flag.Parse()
	if *hash == "" || *site == "" {
		fmt.Fprintln(os.Stderr, "-hash 与 -site 必填（用法见文件头）")
		os.Exit(1)
	}

	gdb, siteRow, meta := loadAll()
	ctx := context.Background()

	// tid：参数或发布记录
	tid := *tidArg
	if tid == "" {
		var rec struct{ TorrentID string }
		if err := gdb.Raw(
			`SELECT torrent_id FROM publish_result_records
			 WHERE source_info_hash = ? AND target_site = ? AND torrent_id != '' AND torrent_id != '0'
			 ORDER BY id DESC LIMIT 1`, *hash, *site).Scan(&rec).Error; err != nil || rec.TorrentID == "" {
			fatal(fmt.Sprintf("发布记录无 tid（%s@%s）——用 -tid 显式指定", (*hash)[:12], *site))
		}
		tid = rec.TorrentID
	}

	// 组装（与发布链同源）
	cfg := model.ParseFormConfig(siteRow.PublishFormConfig)
	if cfg == nil || !cfg.Enabled {
		fatal("站点发布配置缺失")
	}
	domM, domR, domV, domA := titleparser.DOMFieldsFromDetailSource(meta.DetailSourceJSON)
	tp := titleparser.BuildTechProfile(meta.Title, publishTechText(meta), domM, domR, domV, domA)
	ex := &publish.PublishExecutor{}
	_, tagArray, applied := ex.AssembleTagArrayForSite(meta, cfg, *site, tp.HDR, nil)
	if len(tagArray) == 0 {
		fatal("组装结果零标签（meta tags 列或值集键缺失）")
	}

	client := httpclient.NewSiteHTTPClient(httpclient.SiteHTTPConfig{
		Domain: siteRow.Domain, ProxyURL: sitepkg.ResolveSiteProxy(gdb, ctx, siteRow), Timeout: 0,
	})
	base := "https://" + siteRow.Domain

	// ① GET 编辑页
	editURL := base + "/edit.php?id=" + tid
	html, code := fetch(client, editURL, siteRow.Cookie)
	if code != 200 {
		fatal(fmt.Sprintf("GET edit.php HTTP %d", code))
	}

	// ② 解析表单全域现值
	fields, currentTags := parseForm(html)
	if _, ok := fields["id"]; !ok {
		fatal("编辑页无 id 域（页面形态异常或权限不足）")
	}

	// ③ diff
	fmt.Printf("═══ 标签修复预览 ═══\n  种子: %s\n  tid : %s（%s）\n  站上现值: %v\n  应投值  : %v\n",
		meta.Title, tid, *site, currentTags, applied)
	if *dry {
		fmt.Println("  [dry] 未提交")
		return
	}
	if fmt.Sprint(currentTags) == fmt.Sprint(applied) {
		fmt.Println("  站上已是正确值·无需修复")
		return
	}

	// ④ POST：全域原样 + tags 替换
	form := url.Values{}
	for k, v := range fields {
		form.Set(k, v)
	}
	form.Del("tags[4][]")
	for _, kv := range tagArray {
		form.Add(kv.Key, kv.Value)
	}
	resp, err := postForm(client, base+"/takeedit.php", siteRow.Cookie, form)
	if err != nil {
		fatal("POST takeedit: " + err.Error())
	}
	body, _ := io.ReadAll(resp.Body)
	_ = body
	// 成功判定：POST 后重新 GET edit 复查 checked（响应页形态不可靠——NP 成功
	// 返回 200 页面·Obsession 实测非 302；DB 层真值以 edit 页 checked 为准）
	html2, code2 := fetch(client, editURL, siteRow.Cookie)
	if code2 == 200 {
		_, cur2 := parseForm(html2)
		if sameSet(cur2, applied) {
			fmt.Printf("  ✓ 修复完成（DB 层复查 checked=%v）——详情: %s/details.php?id=%s\n", cur2, base, tid)
			return
		}
		fmt.Printf("  ⚠ 提交后复查不符：现 %v vs 应 %v\n", cur2, applied)
		return
	}
	fmt.Printf("  ⚠ 响应 HTTP %d·复查 GET %d\n", resp.StatusCode, code2)
}

func publishTechText(meta *model.TorrentMetadata) string {
	raw := meta.BDInfo
	if raw == "" {
		raw = meta.MediaInfo
	}
	return strings.NewReplacer("[code]", "", "[/code]", "").Replace(raw)
}

func loadAll() (*gorm.DB, *model.Site, *model.TorrentMetadata) {
	gdb, err := gorm.Open(sqlite.Open(*dbPath+"?mode=ro"), &gorm.Config{})
	if err != nil {
		panic(err)
	}
	var s model.Site
	if err := gdb.Where("name = ?", *site).First(&s).Error; err != nil {
		fatal("站点不存在: " + err.Error())
	}
	// §59.531: 走发布链同款 ResourceResolver 簇定位（B/C 组案实证：发布 hash
	// 常无直接 meta 行——代表行在簇内其它副本 hash 上）
	rv := publish.NewResourceResolver(gdb).ResolveResource(context.Background(), *hash)
	if rv == nil || rv.Meta == nil {
		fatal("资源无元数据（resolver 未命中——hash 不在任何簇？）")
	}
	meta := rv.Meta
	// cookie：sites 表加密列——借 siteprobe 输出注入（环境变量 SITE_COOKIE）
	// 直读不解密（工具简化：调用方先 export SITE_COOKIE=$(go run ./tools/siteprobe -domain X -cred cookie)）
	if c := os.Getenv("SITE_COOKIE"); c != "" {
		s.Cookie = c
	}
	if s.Cookie == "" {
		fatal("cookie 为空——先 export SITE_COOKIE=$(go run ./tools/siteprobe -domain <域名> -cred cookie)")
	}
	return gdb, &s, meta
}

// 表单解析：全域现值 + tags 现勾选
var (
	reInput    = regexp.MustCompile(`(?is)<input[^>]*name="([^"]+)"[^>]*/?>`)
	reTextarea = regexp.MustCompile(`(?is)<textarea[^>]*name="([^"]+)"[^>]*>(.*?)</textarea>`)
	reSelTag   = regexp.MustCompile(`(?is)<select[^>]*name="tags[^"]*"[^>]*>(.*?)</select>`)
	reSelect   = regexp.MustCompile(`(?is)<select[^>]*name="([^"]+)"[^>]*>(.*?)</select>`)
	reSelected = regexp.MustCompile(`(?is)<option[^>]*value="([^"]*)"[^>]*selected[^>]*>`)
	reOption   = regexp.MustCompile(`(?is)<option[^>]*value="([^"]*)"`)
	reChecked  = regexp.MustCompile(`(?is)<input[^>]*name="tags\[4\]\[\]"[^>]*value="([^"]+)"[^>]*checked[^>]*/?>`)
	reChecked2 = regexp.MustCompile(`(?is)<input[^>]*checked[^>]*name="tags\[4\]\[\]"[^>]*value="([^"]+)"[^>]*/?>`)
	reVal      = regexp.MustCompile(`(?is)value="([^"]*)"`)
)

func parseForm(html string) (map[string]string, []string) {
	fields := map[string]string{}
	for _, m := range reInput.FindAllStringSubmatch(html, -1) {
		name := m[1]
		if strings.HasPrefix(name, "tags[") {
			continue
		}
		tag := m[0]
		if strings.Contains(tag, `type="checkbox"`) || strings.Contains(tag, `type="radio"`) {
			if strings.Contains(tag, "checked") {
				if v := reVal.FindStringSubmatch(tag); v != nil {
					fields[name] = v[1]
				}
			}
			continue
		}
		if v := reVal.FindStringSubmatch(tag); v != nil {
			fields[name] = v[1]
		}
	}
	for _, m := range reTextarea.FindAllStringSubmatch(html, -1) {
		if !strings.HasPrefix(m[1], "tags[") {
			fields[m[1]] = m[2]
		}
	}
	// select 域：selected option 的 value（无 selected 取首项——NP 默认形态）
	for _, m := range reSelect.FindAllStringSubmatch(html, -1) {
		name := m[1]
		if strings.HasPrefix(name, "tags[") {
			continue
		}
		body := m[2]
		if v := reSelected.FindStringSubmatch(body); v != nil {
			fields[name] = v[1]
		} else if v := reOption.FindStringSubmatch(body); v != nil {
			fields[name] = v[1]
		}
	}
	var cur []string
	seen := map[string]bool{}
	for _, re := range []*regexp.Regexp{reChecked, reChecked2} {
		for _, m := range re.FindAllStringSubmatch(html, -1) {
			if !seen[m[1]] {
				seen[m[1]] = true
				cur = append(cur, m[1])
			}
		}
	}
	return fields, cur
}

func fetch(c *http.Client, u, cookie string) (string, int) {
	req, _ := http.NewRequest("GET", u, nil)
	req.Header.Set("Cookie", cookie)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	resp, err := c.Do(req)
	if err != nil {
		fatal("GET: " + err.Error())
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return string(b), resp.StatusCode
}

func postForm(c *http.Client, u, cookie string, form url.Values) (*http.Response, error) {
	// §59.531: NP 编辑表单 enctype=multipart/form-data（Obsession 实测——
	// urlencoded POST 站方解析不出域·标签不落）
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for k, vs := range form {
		for _, v := range vs {
			_ = w.WriteField(k, v)
		}
	}
	w.Close()
	req, err := http.NewRequest("POST", u, &buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Cookie", cookie)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	return c.Do(req)
}

func fatal(msg string) {
	fmt.Fprintln(os.Stderr, "✗ "+msg)
	os.Exit(1)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// extractErrHint NP 错误提示提取（std 提示页/表单错误文案）
func extractErrHint(body string) string {
	for _, re := range []*regexp.Regexp{
		regexp.MustCompile(`(?is)<td[^>]*class="embedded"[^>]*>\s*([^<]{8,120})`),
		regexp.MustCompile(`(?is)<h1[^>]*>\s*([^<]{4,80})`),
	} {
		if m := re.FindStringSubmatch(body); m != nil {
			return strings.TrimSpace(m[1])
		}
	}
	b := strings.TrimSpace(body)
	if len(b) > 150 {
		b = b[:150]
	}
	return b
}

func sameSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	m := map[string]int{}
	for _, x := range a {
		m[x]++
	}
	for _, x := range b {
		m[x]--
		if m[x] < 0 {
			return false
		}
	}
	return true
}
