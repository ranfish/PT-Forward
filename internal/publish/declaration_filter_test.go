package publish

import (
	"context"
	"strings"
	"testing"

	"github.com/ranfish/pt-forward/internal/setting"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)


// §59.265: 站点固定资源声明整块剥除——包子/农场真实形态（243 QHstudIo 案）
func TestDeclarationFilter_SiteDisclaimers(t *testing.T) {
	text := "[quote][b]BaoziPT · 资源声明[/b]\n[list]本站提供的所有资源，不得下载用于商业盈利[/list][/quote]\n\n" +
		"正文描述保留\n\n" +
		"[quote][color=#000][b] 自由农场  - 资源声明 [/b][/color]\n1、本站提供的所有资源[/quote]\n\n" +
		"[quote][b][color=#0000ff]HDDolby官组作品，源站种子ID：245576，感谢原制作者发布。[/color][/b][/quote]\n\n" +
		"[quote][b][color=blue][size=5]QHstudIo官组作品，感谢原制作者发布。[/size][/color][/b][/quote]\n" +
		"[quote][b][color=red][size=5]互珍互重，禁转PTT[/size][/color][/b][/quote]"

	f := NewDeclarationFilter(nil, nil)
	got := f.Filter(text, defaultDeclarationPatterns)

	if strings.Contains(got.CleanedText, "BaoziPT") {
		t.Error("包子声明块未剥除")
	}
	if strings.Contains(got.CleanedText, "自由农场") {
		t.Error("农场声明块未剥除（双空格头形态）")
	}
	if !strings.Contains(got.CleanedText, "正文描述保留") {
		t.Error("正文误伤")
	}
	if !strings.Contains(got.CleanedText, "QHstudIo官组作品，感谢原制作者发布。") {
		t.Error("我方追加致谢块被误剥")
	}
	if !strings.Contains(got.CleanedText, "互珍互重，禁转PTT") {
		t.Error("禁转PTT块被误剥")
	}
	if len(got.RemovedDecls) != 2 {
		t.Errorf("应剥除恰好 2 块，实际 %d", len(got.RemovedDecls))
	}
}

// v0.0.1456: 音乐站试听/免责声明族+壹吧两段+禁转PTTime 防重复——真实形态锚
func TestDeclarationFilter_MusicAndPTBA(t *testing.T) {
	text := "[quote][color=Red]本站(1PTBA)提供的所有资源均是在网上搜集且由用户上传，任何涉及商业盈利目的均不得使用[/quote]\n" +
		"[quote]您的保种是(1PTBA)站长久发展的重要保证！只要资源在硬盘上不删，请自觉做到开机即做种！[/quote]\n" +
		"[quote]请于试听后24小时内删除，如果喜欢请自觉购买正版光碟！[/quote]\n" +
		"[quote]本原抓作品仅用于试听，严禁用于任何商业用途！[/quote]\n" +
		"[quote]该下载内容仅做宽带测试使用，请在下载后24小时内删除。[/quote]\n" +
		"[quote]本资源均通过网络收集，仅用于试听，严禁用于任何商业用途！[/quote]\n" +
		"[quote]文件来源与网络，试听后请购买支持正版资源。[/quote]\n" +
		"[quote][b]互珍互重，禁转PTT[/b][/quote]\n" +  // 我方声明——必须保留
		"◎片名 测试影片\n正文内容保留"

	f := NewDeclarationFilter(nil, nil)
	got := f.Filter(text, defaultDeclarationPatterns)

	for _, kw := range []string{"1PTBA", "试听", "宽带测试", "保种是"} {
		if strings.Contains(got.CleanedText, kw) {
			t.Errorf("声明块未剥净（残留 %q）", kw)
		}
	}
	if !strings.Contains(got.CleanedText, "互珍互重，禁转PTT") {
		t.Error("我方禁转PTT声明被误剥（新词 禁转PTTime 不得误伤前缀 PTT）")
	}
	if !strings.Contains(got.CleanedText, "正文内容保留") || !strings.Contains(got.CleanedText, "测试影片") {
		t.Error("正文误伤")
	}
}

// 禁转PTTime 单独锚：站方「禁转PTTime」剥·我方「禁转PTT」保
func TestDeclarationFilter_PTTimeVsPTT(t *testing.T) {
	text := "[quote]禁转PTTime 严禁转载[/quote][quote]互珍互重，禁转PTT[/quote]"
	got := NewDeclarationFilter(nil, nil).Filter(text, defaultDeclarationPatterns)
	if strings.Contains(got.CleanedText, "PTTime") {
		t.Error("站方禁转PTTime 未剥除")
	}
	if !strings.Contains(got.CleanedText, "禁转PTT") {
		t.Error("我方禁转PTT 误剥")
	}
}

// v0.0.1457 锚：随版发布+不可删除+不影响自加——合并语义四态
func TestDeclarationFilter_MergeSemantics(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.AutoMigrate(&setting.Setting{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	f := NewDeclarationFilter(setting.NewRepository(db), nil)
	ctx := context.Background()

	// ① 保存含内置词的提交 → 只存自定义（内置剔除）
	if err := f.SetPatterns(ctx, []string{defaultDeclarationPatterns[0], "我的自定义词", "我的自定义词", " "}); err != nil {
		t.Fatalf("set: %v", err)
	}
	custom := f.GetCustomPatterns(ctx)
	if len(custom) != 1 || custom[0] != "我的自定义词" {
		t.Errorf("custom=%v·内置词应剔除+去重+空串滤", custom)
	}

	// ② GetPatterns = 内置 ∪ 自定义（合并·随版）
	got := f.GetPatterns(ctx)
	has := func(list []string, s string) bool { for _, x := range list { if x == s { return true } } ; return false }
	if !has(got, defaultDeclarationPatterns[0]) || !has(got, "我的自定义词") {
		t.Errorf("合并集缺项: builtin=%v custom-in=%v", has(got, defaultDeclarationPatterns[0]), has(got, "我的自定义词"))
	}

	// ③ 意外删除防御：DB 直接清空 custom → 内置仍全在
	if err := f.SetPatterns(ctx, nil); err != nil {
		t.Fatalf("set nil: %v", err)
	}
	if len(f.GetPatterns(ctx)) != len(defaultDeclarationPatterns) {
		t.Error("内置集不完整（应恒等于默认全集）")
	}

	// ④ 旧 DB 全量快照兼容：DB 存「builtin∪旧词」形态 → 合并集包含之（行为不回退）
	legacy := append([]string{}, defaultDeclarationPatterns...)
	legacy = append(legacy, "旧版遗留词")
	if err := setting.NewRepository(db).Set(ctx, "declaration_filter_patterns",
		`[`+strings.Join(quote(legacy), `,`)+`]`); err != nil {
		t.Fatalf("seed legacy: %v", err)
	}
	if !has(f.GetPatterns(ctx), "旧版遗留词") || !has(f.GetPatterns(ctx), defaultDeclarationPatterns[0]) {
		t.Error("旧快照兼容断裂")
	}
}

func quote(ss []string) []string {
	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = `"` + s + `"`
	}
	return out
}
