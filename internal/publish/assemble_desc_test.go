package publish

import (
	"strings"
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
)


// §59.163: MI 误判防御——段名冒号全角化（探针 H1 实证；用户定案内容零丢失）。
func TestFullwidthMIColons(t *testing.T) {
	src := `感谢 FRDS 组
Source #1: GER UHD Koch Media (2019): Video, Audio, Subtitles.
Video: AUS has hardcoded subtitles.
Audio: All 7.1 TrueHD Atmos sources are very similar.
Subtitles: en - Compared AUS, GER, Amazon.
Chapters: GBR - only named source available.`
	out := FullwidthMIColons(src)
	// 段名冒号全角化
	for _, fw := range []string{"Video： AUS", "Audio： All 7.1", "Subtitles： en", "Chapters： GBR"} {
		if !strings.Contains(out, fw) {
			t.Errorf("段名冒号应全角化: %q", fw)
		}
	}
	// 半角形态清除（站方检测通过条件）
	if strings.Contains(out, "Video:") || strings.Contains(out, "Audio:") ||
		strings.Contains(out, "Subtitles:") || strings.Contains(out, "Chapters:") {
		t.Error("MI 段名半角冒号应全部替换")
	}
	// 内容零丢失（行数/其余文本不变）
	if strings.Count(out, "\n") != strings.Count(src, "\n") {
		t.Error("内容行数不应变化（零丢失）")
	}
	if !strings.Contains(out, "Source #1: GER") || !strings.Contains(out, "感谢 FRDS 组") {
		t.Error("无形态文本应原样保留")
	}
	// MI 老段名 General 也覆盖
	if got := FullwidthMIColons("General : x"); got != "General： x" {
		t.Errorf("General 空格变体应替换, got %q", got)
	}
}

// §59.166: 海报双份回归修复——发布组装不插入 Tab2 海报（简介自带）
func TestAssembleNoPosterDuplication(t *testing.T) {
	meta := &model.TorrentMetadata{
		Statement:   "感谢 FRDS 组",
		Poster:      "https://img.example.com/poster.jpg",
		Description: "[img]https://img.example.com/poster.jpg[/img]\n剧情简介正文",
	}
	out := assembleDescription(meta, &model.PublishFormConfig{FormFields: map[string]string{model.FieldDomainTechInfo: "technical_info"}})
	// Tab2 海报不注入（简介自带——双份防御）
	n := strings.Count(out, "poster.jpg")
	if n != 1 {
		t.Errorf("海报应恰好 1 次（简介自带），got %d 次", n)
	}
	if !strings.Contains(out, "剧情简介正文") {
		t.Error("简介正文应保留")
	}
}

// §59.166 修道院规范：无 techinfo 域 → MI 引用格式入简介（顺序：声明→简介→MI→截图）
func TestAssembleMIIinDescForNoTechInfoSite(t *testing.T) {
	meta := &model.TorrentMetadata{
		Statement:   "感谢 FRDS 组",
		Description: "[img]https://img.example.com/p.jpg[/img]\n剧情简介",
		MediaInfo:   "General\nComplete name : test.mkv\nVideo: AVC",
		Screenshots: `["https://img.example.com/s1.jpg"]`,
	}
	cfg := &model.PublishFormConfig{FormFields: map[string]string{}} // 修道院：无 techinfo
	out := assembleDescription(meta, cfg)
	// 顺序断言：声明 < 简介 < MI < 截图
	iDecl := strings.Index(out, "感谢 FRDS 组")
	iDesc := strings.Index(out, "剧情简介")
	iMI := strings.Index(out, "Complete name : test.mkv")
	iShot := strings.Index(out, "s1.jpg")
	if !(iDecl < iDesc && iDesc < iMI && iMI < iShot) {
		t.Errorf("顺序错误: decl=%d desc=%d mi=%d shot=%d", iDecl, iDesc, iMI, iShot)
	}
	// MI 在引用块内（半角原文）
	if !strings.Contains(out, "[quote]General") || !strings.Contains(out, "[/quote]") {
		t.Error("MI 应以引用格式插入")
	}
	if !strings.Contains(out, "Video: AVC") {
		t.Error("MI 半角原文保留（修道院无自动审核）")
	}
	// 幸运形态：有 techinfo 域 → MI 不入简介
	cfgLucky := &model.PublishFormConfig{FormFields: map[string]string{model.FieldDomainTechInfo: "technical_info"}}
	outLucky := assembleDescription(meta, cfgLucky)
	if strings.Contains(outLucky, "Complete name") {
		t.Error("有 techinfo 域的站 MI 不应入简介（幸运审核检测）")
	}
}
