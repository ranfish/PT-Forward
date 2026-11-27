package adapter

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
)

// 14 刀3（v0.0.1659）: UNIT3D /api/user 形态契约+降级锚。
// 形态依据（前置验证闭环）：UNIT3D master UserResource 权威
// （uploaded/ratio/seedbonus/buffer 全 string·seeding/leeching int）
// 与 docs/34 摸站记录逐字一致。
//
// 端点分层（前置探针实证）：/api/user 是 UNIT3D v9.0.0 新增——
// 老版站（≤v8.3·monika 实测 404）恒失败→FetchUserStats 降级 HTML。
//
// 阴性对照（08 篇方法论——锚必须可触发）：
//   - 结构体退回 flexInt/float64 → 锚1 FAIL（unmarshal err 或归零）；
//   - FetchUserStats 删降级分支 → 锚2/锚3 FAIL（直接返回 err）。

// 锚1: API 契约——master 形态 JSON 解析非零无 err（V-01 根修验证）。
func TestUnit3D_UserStatsAPI_Contract(t *testing.T) {
	// UNIT3D master UserResource 权威形态（docs/34:344-347 同源）
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/user" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"username": "tester",
			"group": {"name": "Member"},
			"uploaded": "50.00 GiB",
			"downloaded": "1.00 GiB",
			"ratio": "50.00",
			"buffer": "125.00 GiB",
			"seeding": 7,
			"leeching": 0,
			"seedbonus": "100.00",
			"hit_and_runs": 0
		}`))
	}))
	defer srv.Close()

	a := NewUnit3DAdapter(&HTTPDoer{Client: srv.Client()}, zap.NewNop())
	config := &model.SiteConfig{Domain: srv.URL, APIKey: "k1"}
	stats, err := a.FetchUserStats(context.Background(), config)
	if err != nil {
		t.Fatalf("master 形态解析不应失败: %v", err)
	}
	if stats.Username != "tester" {
		t.Errorf("username=%q want tester", stats.Username)
	}
	if stats.UploadBytes != 50*1024*1024*1024 {
		t.Errorf("uploaded=%d want %d（50 GiB）——形态不匹配回归", stats.UploadBytes, 50*1024*1024*1024)
	}
	if stats.DownloadBytes != 1024*1024*1024 {
		t.Errorf("downloaded=%d want %d（1 GiB）", stats.DownloadBytes, 1024*1024*1024)
	}
	if stats.Ratio != 50.0 {
		t.Errorf("ratio=%v want 50（string \"50.00\" 契约）", stats.Ratio)
	}
	if stats.BonusPoints != 100.0 {
		t.Errorf("seedbonus=%v want 100", stats.BonusPoints)
	}
	if stats.SeedingCount != 7 {
		t.Errorf("seeding=%d want 7", stats.SeedingCount)
	}
}

// 锚2: 老版站降级——/api/user 404（v9 前无此端点·monika 实测形态）
// → 回退 HTML 路径提取。
func TestUnit3D_UserStatsAPI_FallbackOn404(t *testing.T) {
	indexHTML := `<html><body>
	<div class="ratio-bar__uploaded"><a href="/users/tester">tester</a><i class="fa"></i> 50.00 GiB</div>
	<div class="ratio-bar__downloaded"><i class="fa"></i> 2.00 GiB</div>
	<div class="top-nav__stats-ratio"><i></i> 25.00</div>
	</body></html>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/user":
			// monika 实测：Laravel 路由层 404（非 401 认证门）
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message": "The route api/user could not be found."}`))
		case "/":
			_, _ = w.Write([]byte(indexHTML))
		case "/users/tester":
			http.NotFound(w, r) // profile 页失败容错跳过
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	a := NewUnit3DAdapter(&HTTPDoer{Client: srv.Client()}, zap.NewNop())
	config := &model.SiteConfig{Domain: srv.URL, APIKey: "k1"}
	stats, err := a.FetchUserStats(context.Background(), config)
	if err != nil {
		t.Fatalf("老版站 404 应降级 HTML 成功: %v", err)
	}
	if stats.UploadBytes != 50*1024*1024*1024 {
		t.Errorf("降级 HTML uploaded=%d want 50 GiB", stats.UploadBytes)
	}
	if stats.DownloadBytes != 2*1024*1024*1024 {
		t.Errorf("降级 HTML downloaded=%d want 2 GiB", stats.DownloadBytes)
	}
	if stats.Ratio != 25.0 {
		t.Errorf("降级 HTML ratio=%v want 25", stats.Ratio)
	}
	if stats.Username != "tester" {
		t.Errorf("降级 HTML username=%q want tester", stats.Username)
	}
}

// 锚3: 形态漂移降级——API 200 但 ratio 垃圾形态（flexFloat err 显性化）
// → 降级 HTML。
func TestUnit3D_UserStatsAPI_FallbackOnMalformed(t *testing.T) {
	indexHTML := `<html><body>
	<div class="ratio-bar__uploaded"><i></i> 10.00 GiB</div>
	</body></html>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/user":
			// 形态漂移：ratio 非数值字符串——flexFloat 契约场景返 err
			_, _ = w.Write([]byte(`{"username": "u", "uploaded": "50.00 GiB", "ratio": "garbage"}`))
		case "/":
			_, _ = w.Write([]byte(indexHTML))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	a := NewUnit3DAdapter(&HTTPDoer{Client: srv.Client()}, zap.NewNop())
	config := &model.SiteConfig{Domain: srv.URL, APIKey: "k1"}
	stats, err := a.FetchUserStats(context.Background(), config)
	if err != nil {
		t.Fatalf("形态漂移应降级 HTML 成功: %v", err)
	}
	if stats.UploadBytes != 10*1024*1024*1024 {
		t.Errorf("降级 HTML uploaded=%d want 10 GiB", stats.UploadBytes)
	}
}

// 锚4: flexFloat 单元四态——string 数值/裸数字/空串拒绝/垃圾拒绝。
func TestFlexFloatUnmarshal(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		want    float64
		wantErr bool
	}{
		{"string 数值（master 权威形态）", `"50.00"`, 50, false},
		{"裸数字（版本漂移兼容）", `50.5`, 50.5, false},
		{"空串拒绝（契约显性化）", `""`, 0, true},
		{"空白拒绝", `  `, 0, true},
		{"垃圾字符串拒绝", `"garbage"`, 0, true},
		{"带单位拒绝（ratio 不应带单位）", `"50.00 GiB"`, 0, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var f flexFloat
			err := json.Unmarshal([]byte(c.input), &f)
			if c.wantErr {
				if err == nil {
					t.Errorf("input=%q 应返回 err（契约场景显性化）", c.input)
				}
				return
			}
			if err != nil {
				t.Errorf("input=%q unexpected err: %v", c.input, err)
				return
			}
			if float64(f) != c.want {
				t.Errorf("input=%q got %v want %v", c.input, float64(f), c.want)
			}
		})
	}
}

// 锚5: 收敛后字节换算——多点判无效+PB 能力+上界守卫。
func TestParseSizeStringConverged(t *testing.T) {
	cases := []struct {
		in   string
		want int64
	}{
		{"50.00 GiB", 50 * 1024 * 1024 * 1024},
		{"1.5 GB", 1610612736},
		{"3 PiB", 3 * 1024 * 1024 * 1024 * 1024 * 1024},
		{"2 PB", 2 * 1024 * 1024 * 1024 * 1024 * 1024},
		{"1,234 MB", 1234 * 1024 * 1024}, // 千分位剥离（收敛保留能力）
		{"12345", 12345},                 // 裸数字分支
		{"1.2.3 GB", 0},                  // 多点判无效（旧手搓吸收为 1.23 的缺陷根除）
	}
	for _, c := range cases {
		if got := parseSizeString(c.in); got != c.want {
			t.Errorf("parseSizeString(%q) = %d, want %d", c.in, got, c.want)
		}
	}
	// 上界守卫（13 篇同型纪律——收敛版继承）
	if got := parseSizeString("99999999999999999999 PiB"); got != 0 {
		t.Errorf("超界输入应拒绝返 0·实得 %d", got)
	}
	if !strings.Contains("unit3d", "unit3d") {
		t.Error("unreachable")
	}
}
