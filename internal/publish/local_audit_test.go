package publish

import (
	"strings"
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/titleparser"
)

// model2TP 构造最小 TechProfile（音频/分辨率带值断言用）
func model2TP(audio, resolution string) titleparser.TechProfile {
	return titleparser.TechProfile{AudioCodec: audio, Resolution: resolution}
}

// §59.504 LocalAudit 终局 6 条规则（12→6·可达性审查后——文案=用户定版
// 纯事实+带值；阈值 IMAGES 对齐审核门 3；解析单源 model.ParseScreenshotColumn）
func TestRunLocalAuditRules(t *testing.T) {
	cfg := &model.PublishFormConfig{FormFields: map[string]string{
		model.FieldDomainMedium:     "medium_sel[4]",
		model.FieldDomainStandard:   "standard_sel[4]",
		model.FieldDomainAudiocodec: "audiocodec_sel[4]",
	}}
	meta := &model.TorrentMetadata{Title: "Arco 2025 UHD BluRay DTS-HD MA 7.1 mUHD-FRDS",
		Subtitle: "副标题", MediaInfo: "General", IMDbURL: "https://imdb.com/tt1",
		Screenshots: `["a.jpg","b.jpg","c.jpg"]`}

	// ① 全域齐备（截图 3 张）→ 零提示
	formFull := map[string]string{"medium_sel[4]": "7", "standard_sel[4]": "6", "audiocodec_sel[4]": "12"}
	f := RunLocalAudit(localAuditInput{Form: formFull, Cfg: cfg, Meta: meta})
	if len(f) != 0 {
		t.Errorf("全域齐备应零提示, got %v", f)
	}

	// ② 音频未映射+标题有音频痕迹 → 带值文案
	formNoAudio := map[string]string{"medium_sel[4]": "7", "standard_sel[4]": "6"}
	f2 := RunLocalAudit(localAuditInput{Form: formNoAudio, Cfg: cfg, Meta: meta,
		TP: model2TP("DTS-HD MA", "")})
	if !hasCode(f2, "LOCAL_AUDIO_UNMAPPED") {
		t.Errorf("应提示音频未映射, got %v", codes(f2))
	}
	for _, x := range f2 {
		if x.Code == "LOCAL_AUDIO_UNMAPPED" && x.Message != "音频域未选择（DTS-HD MA）" {
			t.Errorf("音频文案应带值, got %q", x.Message)
		}
	}

	// ③ 分辨率未映射 → 带值
	formNoStd := map[string]string{"medium_sel[4]": "7", "audiocodec_sel[4]": "12"}
	f3 := RunLocalAudit(localAuditInput{Form: formNoStd, Cfg: cfg, Meta: meta,
		TP: model2TP("", "2160p")})
	for _, x := range f3 {
		if x.Code == "LOCAL_STANDARD_UNMAPPED" && x.Message != "分辨率域未选择（2160p）" {
			t.Errorf("分辨率文案应带值, got %q", x.Message)
		}
	}

	// ④ 人工粤语无据
	f4 := RunLocalAudit(localAuditInput{Form: formFull, Cfg: cfg, Meta: meta,
		TagOverrides: []string{"cantonese_audio", "lucky:cantonese_audio"}})
	if !hasCode(f4, "LOCAL_TAG_OVERRIDE_NO_EVIDENCE") {
		t.Errorf("人工粤语无据应提示, got %v", codes(f4))
	}

	// ⑤ 截图 2 张 < 3 → 截图不足（N 张）带张数
	metaShort := *meta
	metaShort.Screenshots = `["a.jpg","b.jpg"]`
	f5 := RunLocalAudit(localAuditInput{Form: formFull, Cfg: cfg, Meta: &metaShort})
	found := false
	for _, x := range f5 {
		if x.Code == "LOCAL_IMAGES_INSUFFICIENT" {
			found = true
			if x.Message != "截图不足（2 张）" {
				t.Errorf("截图文案应带张数, got %q", x.Message)
			}
		}
	}
	if !found {
		t.Errorf("截图 2 张应提示不足, got %v", codes(f5))
	}

	// ⑥ 禁发组带组名
	meta6 := *meta
	meta6.Title = "Movie 2026 1080p BluRay x265-FGT"
	f6 := RunLocalAudit(localAuditInput{Form: formFull, Cfg: cfg, Meta: &meta6})
	for _, x := range f6 {
		if x.Code == "LOCAL_BANNED_GROUP" && x.Message != "标题含多站禁发组（FGT）" {
			t.Errorf("禁发组文案应带组名, got %q", x.Message)
		}
	}

	// ⑦ §59.504 已删规则零残留（6 条删减回归锚）
	for _, f := range [][]LocalAuditFinding{f, f2, f3, f4, f5, f6} {
		for _, x := range f {
			for _, dead := range []string{"LOCAL_TITLE_REASSEMBLED", "LOCAL_TITLE_NON_ASCII",
				"LOCAL_SUBTITLE_EMPTY", "LOCAL_MEDIAINFO_EMPTY", "LOCAL_IMDB_EMPTY", "LOCAL_TITLE_WORD_FORM"} {
				if x.Code == dead {
					t.Errorf("已删规则 %s 不应触发", dead)
				}
			}
		}
	}
}

// §59.504 FormatLocalAudit 去内部 CODE 前缀（英文代号对用户无意义·
// 溯源走 logs 列 JSON）
func TestFormatLocalAuditNoCode(t *testing.T) {
	out := FormatLocalAudit([]LocalAuditFinding{
		{Code: "LOCAL_MEDIUM_UNMAPPED", Message: "媒介域未选择"},
		{Code: "LOCAL_AUDIO_UNMAPPED", Message: "音频域未选择（DTS）"},
	})
	if strings.Contains(out, "LOCAL_") {
		t.Errorf("输出不应含内部 CODE, got %q", out)
	}
	want := "内部规范提示[媒介域未选择；音频域未选择（DTS）]"
	if out != want {
		t.Errorf("got %q, want %q", out, want)
	}
}

func hasCode(f []LocalAuditFinding, code string) bool {
	for _, x := range f {
		if x.Code == code {
			return true
		}
	}
	return false
}

func codes(f []LocalAuditFinding) []string {
	out := make([]string, 0, len(f))
	for _, x := range f {
		out = append(out, x.Code)
	}
	return out
}

// §59.166 回归审核：禁发组词边界负例（Altitude 含 alt 不命中——青蛙/藏宝阁同款边界语义）
func TestBannedGroupWordBoundary(t *testing.T) {
	if g := containsBannedGroup("Movie.2026.Altitude.1080p-GROUP"); g != "" {
		t.Errorf("Altitude 不应误命中 alt, got %s", g)
	}
	if g := containsBannedGroup("Movie 2026 1080p BluRay x265-XiaoMi"); g == "" {
		t.Error("XiaoMi（大小写不敏感）应命中")
	}
}
