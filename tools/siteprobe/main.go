// siteprobe 摸站探针（工具化·2026-11-11 用户指令：探针应工具化入 tools）。
//
// §59.338 真实组件离线驱动法的工具化：任何站点改版/新站接入的标准摸站工具。
// 与 tools/ 兄弟工具（decrypt-cookie 等）不同，本工具【无独立 go.mod】——
// 是主模块成员（导入 internal/adapter 生产链是核心价值：离线验证=零网络
// 跑生产解析器）。
//
// 三模式（-domain 必填）：
//   凭证解密（bash 管道消费·不落盘不回显）：
//     go run ./tools/siteprobe -domain hdarea.club -cred cookie
//     go run ./tools/siteprobe -domain hdarea.club -cred passkey
//   页面抓取（curl 子进程·cookie 经 stdin 配置不进 argv/磁盘·存
//   /tmp/opencode/siteprobe/<domain>/）：
//     go run ./tools/siteprobe -domain hdarea.club -fetch "torrents.php,detail.php?id=123,upload.php"
//     （路径中字面量 PASSKEY 会被解密 passkey 替换——download/RSS 直链用）
//   离线验证（货架 HTML → 生产 adapter+Engine 全链解析·零网络）：
//     go run ./tools/siteprobe -domain hdarea.club -verify -kw Dune -tid 123
//     （需先 -fetch 存下 torrents.html/search.html/detail.html——search 页
//      抓法：torrents.php?search=KEYWORD&search_mode=0）
package main

import (
	"encoding/json"
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/ranfish/pt-forward/internal/adapter"
	"github.com/ranfish/pt-forward/internal/crypto"
	"github.com/ranfish/pt-forward/internal/metadata/extract"
	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/setting"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var (
	domain    = flag.String("domain", "", "站点域名（必填）")
	dbPath    = flag.String("db", "data/pt-forward.db", "SQLite DB 路径")
	cred      = flag.String("cred", "", "解密凭证输出：cookie | passkey | apikey")
	fetch     = flag.String("fetch", "", "逗号分隔页面路径列表（相对站根）")
	verify    = flag.Bool("verify", false, "离线验证模式（货架 HTML→生产 adapter）")
	metadata  = flag.Bool("metadata", false, "值集对账模式（好大 API·站方 site/metadata ↔ DB form_config）")
	framework = flag.String("framework", "nexusphp", "adapter 框架（verify 用）")
	kw        = flag.String("kw", "PROBE", "搜索关键词（verify·对应 search.html）")
	tid       = flag.String("tid", "", "详情页种子 ID（verify·对应 detail.html）")
	proxy     = flag.String("proxy", "", "curl 代理（空=直连）")
	baseURL   = flag.String("base", "", "站根 URL（空=https://<domain>）")
)

func main() {
	flag.Parse()
	if *domain == "" {
		fmt.Fprintln(os.Stderr, "-domain 必填（用法见文件头注释）")
		os.Exit(1)
	}
	base := *baseURL
	if base == "" {
		base = "https://" + *domain
	}

	s := loadSite()

	switch {
	case *cred != "":
		switch *cred {
		case "passkey":
			fmt.Println(s.Passkey)
		case "apikey":
			fmt.Println(s.APIKey)
		default:
			fmt.Println(s.Cookie)
		}
	case *fetch != "":
		doFetch(s, base)
	case *metadata:
		doMetadata(s, base)
	case *verify:
		doVerify(base)
	default:
		fmt.Fprintln(os.Stderr, "需指定 -cred / -fetch / -verify 之一")
		os.Exit(1)
	}
}

// loadSite 29 DB 解密加载站点行（crypto callbacks 同生产栈）。
func loadSite() *model.Site {
	gdb, err := gorm.Open(sqlite.Open(*dbPath+"?mode=ro"), &gorm.Config{})
	if err != nil {
		panic(err)
	}
	srepo := setting.NewRepository(gdb)
	key, _ := srepo.Get(context.Background(), "encryption_key")
	enc, err := crypto.NewCredentialEncryptor(key)
	if err != nil {
		panic(err)
	}
	if err := crypto.RegisterCallbacks(gdb, enc, nil); err != nil {
		panic(err)
	}
	var s model.Site
	if err := gdb.Where("domain = ?", *domain).First(&s).Error; err != nil {
		panic(fmt.Sprintf("站点 %s: %v", *domain, err))
	}
	return &s
}

// doFetch curl 子进程抓页面。cookie 经 stdin 配置文件（-K -）注入——
// 不进 argv（ps 不可见）不落磁盘。PASSKEY 字面量替换（下载/RSS 直链）。
func doFetch(s *model.Site, base string) {
	dir := "/tmp/opencode/siteprobe/" + *domain
	os.MkdirAll(dir, 0o755)
	for _, p := range strings.Split(*fetch, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		url := base + "/" + strings.TrimPrefix(p, "/")
		if strings.Contains(url, "PASSKEY") && s.Passkey != "" {
			url = strings.ReplaceAll(url, "PASSKEY", s.Passkey)
		}
		name := path.Base(strings.SplitN(p, "?", 2)[0])
		if strings.HasSuffix(name, ".php") {
			name = strings.TrimSuffix(name, ".php")
		}
		// verify 货架契约命名：search 页→search·详情页→detail·首页→home
		if q := strings.SplitN(p, "?", 2); len(q) == 2 && strings.Contains(q[1], "search=") {
			name = "search"
		}
		switch name {
		case "details":
			name = "detail"
		case "index":
			name = "home"
		}
		cfg := fmt.Sprintf(`url = "%s"
header = "Cookie: %s"
header = "User-Agent: Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Safari/537.36"
silent
location
max-time = 30
write-out = "HTTP %%{http_code} %%{size_download}B -> %s.html\n"
output = "%s.html"
`, url, s.Cookie, dir+"/"+name, dir+"/"+name)
		args := []string{"-K", "-"}
		if *proxy != "" {
			args = append(args, "-x", *proxy)
		}
		cmd := exec.Command("curl", args...)
		cmd.Stdin = strings.NewReader(cfg)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "✗ %s: %v\n", p, err)
		}
	}
}

// doVerify 货架 RoundTripper 喂已抓 HTML → 生产 adapter（Engine 注入·
// Factory.Create 同栈）全链解析。
func doVerify(base string) {
	rt := &shelf{dir: "/tmp/opencode/siteprobe/" + *domain, hits: map[string]int{}}
	doer := &adapter.HTTPDoer{Client: &http.Client{Transport: rt}}
	a := adapter.NewNexusPHPAdapter(doer, zap.NewNop())
	if *framework == "nexusphp" {
		a.SetEngine(extract.NewEngine(extract.NewPublicExtractor("", ""), nil))
	}
	cfg := &model.SiteConfig{Domain: *domain, BaseURL: base, Enabled: true, Passkey: "PROBE"}
	ctx := context.Background()

	if _, err := os.Stat(rt.dir + "/torrents.html"); err == nil {
		items, err := a.SearchTorrents(ctx, cfg, *kw, nil)
		fmt.Printf("① SearchTorrents(%s): %d 条 err=%v\n", *kw, len(items), err)
		for _, it := range items {
			if it.TorrentID != "" {
				fmt.Printf("   id=%s size=%.2fGB seed=%d name=%.75s\n", it.TorrentID, float64(it.Size)/1e9, it.Seeders, it.Title)
				break
			}
		}
	}
	if *tid != "" {
		if _, err := os.Stat(rt.dir + "/detail.html"); err == nil {
			d, err := a.GetTorrentDetail(ctx, cfg, *tid)
			fmt.Printf("② GetTorrentDetail(%s): err=%v\n", *tid, err)
			if d != nil {
				fmt.Printf("   title=%.75s\n   size=%.2fGB descr=%dB MI=%dB sub=%.50s\n",
					d.Title, float64(d.Size)/1e9, len(d.Description), len(d.MediaInfo), d.Subtitle)
			}
			dc, err := a.DetectDiscount(ctx, cfg, *tid)
			fmt.Printf("③ DetectDiscount: err=%v level=%v mult=%.2f\n", err, dcLevel(dc), dcMult(dc))
		}
	}
	fmt.Printf("货架命中: %v（先 -fetch 存页再 -verify）\n", rt.hits)
}

func dcLevel(dc *model.DiscountResult) string {
	if dc == nil {
		return "NIL"
	}
	return string(dc.Level)
}

func dcMult(dc *model.DiscountResult) float64 {
	if dc == nil {
		return -1
	}
	return dc.Multiplier
}

// shelf 货架 RoundTripper：按 URL 特征回放已抓 HTML（details→detail/
// search→search/其余→torrents）。
type shelf struct {
	dir  string
	hits map[string]int
}

func (s *shelf) RoundTrip(req *http.Request) (*http.Response, error) {
	u := req.URL.String()
	var file string
	switch {
	case strings.Contains(u, "details.php"):
		file = s.dir + "/detail.html"
	case strings.Contains(u, "search=") || strings.Contains(u, "search_mode"):
		file = s.dir + "/search.html"
	default:
		file = s.dir + "/torrents.html"
	}
	body, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	s.hits[file]++
	return &http.Response{
		StatusCode: 200, Status: "200 OK", Proto: "HTTP/1.1", ProtoMajor: 1, ProtoMinor: 1,
		Header:     http.Header{"Content-Type": []string{"text/html; charset=utf-8"}},
		Body:       io.NopCloser(strings.NewReader(string(body))),
		Request:    req,
	}, nil
}

// ── 刀4 D1：值集对账模式（好大 API·§59.515）──
// 站方实时字典 GET /api/v1/site/metadata ↔ DB publish_form_config 六域对比。
// 退出码：0=干净 1=有差异（危险项=我方 ID 站方已删→发布会失败；
// 信息项=站方新增未映射/名称漂移）。只读零修改。

// metaDomain 站方 metadata 域 → 我方 value_mappings 标准键。
var metaDomain = map[string]string{
	"categories":   "type",
	"media":        "medium",
	"codecs":       "codec",
	"audiocodecs":  "audiocodec",
	"standards":    "standard",
	"teams":        "team",
	// sources→source_sel / processings→processing_sel：刀2 D3 定案不配置
	// （全站实测恒 0·观察项挂账）——不对账。subtitle_languages/sp_states
	// 非发布表单域。
}

type metaItem struct {
	// id 双形态（字符串为主+数字形态·§59.1324 json.Number 同族兼容）
	ID   json.Number `json:"id"`
	Name string      `json:"name"`
}

func doMetadata(s *model.Site, base string) {
	if s.APIKey == "" {
		fmt.Fprintln(os.Stderr, "❌ 站点无 API Key")
		os.Exit(2)
	}
	req, _ := http.NewRequest("GET", base+"/api/v1/site/metadata", nil)
	req.Header.Set("x-api-key", s.APIKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println("❌ 请求失败:", err)
		os.Exit(2)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode != 200 {
		fmt.Printf("❌ HTTP %d: %.200s\n", resp.StatusCode, string(body))
		os.Exit(2)
	}
	var env struct {
		Code int `json:"code"`
		Data map[string][]metaItem `json:"data"`
	}
	if err := json.Unmarshal(body, &env); err != nil || env.Code != 0 {
		fmt.Printf("❌ 信封解析失败（code=%d err=%v）\n", env.Code, err)
		os.Exit(2)
	}

	// DB 侧 value_mappings
	if s.PublishFormConfig == "" {
		fmt.Println("❌ 站点无 publish_form_config")
		os.Exit(2)
	}
	var fc struct {
		ValueMappings map[string][]struct {
			Label string `json:"label"`
			Value string `json:"value"`
		} `json:"value_mappings"`
	}
	if err := json.Unmarshal([]byte(s.PublishFormConfig), &fc); err != nil {
		fmt.Println("❌ form_config 解析失败:", err)
		os.Exit(2)
	}

	danger, info := 0, 0
	for metaDomainKey, stdKey := range metaDomain {
		siteItems := env.Data[metaDomainKey]
		ours := fc.ValueMappings[stdKey]
		siteByID := map[string]metaItem{}
		for _, it := range siteItems {
			siteByID[it.ID.String()] = it
		}
		ourIDs := map[string]bool{}
		for _, o := range ours {
			ourIDs[o.Value] = true
			si, ok := siteByID[o.Value]
			switch {
			case !ok:
				fmt.Printf("‼️ [%s] 我方 %q(id=%s) 站方已删除——发布会失败\n", stdKey, o.Label, o.Value)
				danger++
			case si.Name != o.Label:
				fmt.Printf("ℹ️ [%s] id=%s 名称漂移: %q → %q\n", stdKey, o.Value, o.Label, si.Name)
				info++
			}
		}
		for _, it := range siteItems {
			if !ourIDs[it.ID.String()] {
				fmt.Printf("ℹ️ [%s] 站方新增未映射: id=%s %q\n", stdKey, it.ID.String(), it.Name)
				info++
			}
		}
	}
	fmt.Printf("对账域=%d 危险=%d 信息=%d —— ", len(metaDomain), danger, info)
	if danger > 0 {
		fmt.Println("存在危险项（我方引用站方已删 ID）")
		os.Exit(1)
	}
	fmt.Println("干净")
}
