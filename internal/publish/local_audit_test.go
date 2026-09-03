package publish

import (
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
)

// §59.166 LocalAudit 初始规则（advisory）
func TestRunLocalAuditRules(t *testing.T) {
	cfg := &model.PublishFormConfig{FormFields: map[string]string{
		model.FieldDomainMedium:     "medium_sel[4]",
		model.FieldDomainStandard:   "standard_sel[4]",
		model.FieldDomainAudiocodec: "audiocodec_sel[4]",
	}}
	meta := &model.TorrentMetadata{Title: "Arco 2025 UHD BluRay DTS-HD MA 7.1 mUHD-FRDS",
		Subtitle: "副标题", MediaInfo: "General", IMDbURL: "https://imdb.com/tt1",
		Screenshots: `["a.jpg","b.jpg"]`}

	// ① 全域齐备 → 零提示
	formFull := map[string]string{"medium_sel[4]": "7", "standard_sel[4]": "6", "audiocodec_sel[4]": "12"}
	f := RunLocalAudit(localAuditInput{Form: formFull, Cfg: cfg, Meta: meta, PublishTitle: meta.Title})
	if len(f) != 0 {
		t.Errorf("全域齐备应零提示, got %v", f)
	}

	// ② 音频未映射+标题有音频痕迹 → LOCAL_AUDIO_UNMAPPED
	formNoAudio := map[string]string{"medium_sel[4]": "7", "standard_sel[4]": "6"}
	f2 := RunLocalAudit(localAuditInput{Form: formNoAudio, Cfg: cfg, Meta: meta,
		PublishTitle: "Arco 2025 UHD BluRay DDP 7.1 mUHD-FRDS"})
	hasAudioHint, hasTitleHint := false, false
	for _, x := range f2 {
		if x.Code == "LOCAL_AUDIO_UNMAPPED" { hasAudioHint = true }
		if x.Code == "LOCAL_TITLE_REASSEMBLED" { hasTitleHint = true }
	}
	if !hasAudioHint { t.Errorf("应提示音频未映射, got %v", f2) }
	if !hasTitleHint { t.Errorf("应提示标题重组, got %v", f2) }

	// ③ 人工粤语无据 → LOCAL_TAG_OVERRIDE_NO_EVIDENCE
	f3 := RunLocalAudit(localAuditInput{Form: formFull, Cfg: cfg, Meta: meta,
		PublishTitle: meta.Title, TagOverrides: []string{"cantonese_audio", "lucky:cantonese_audio"}})
	hasTagHint := false
	for _, x := range f3 {
		if x.Code == "LOCAL_TAG_OVERRIDE_NO_EVIDENCE" { hasTagHint = true }
	}
	if !hasTagHint { t.Errorf("人工粤语无据应提示, got %v", f3) }
}

// §59.166 9 站共性规则（docs/38 矩阵 R6-R12）
func TestLocalAuditCommonRules(t *testing.T) {
	cfg := &model.PublishFormConfig{FormFields: map[string]string{
		model.FieldDomainMedium:     "medium_sel[4]",
		model.FieldDomainStandard:   "standard_sel[4]",
		model.FieldDomainAudiocodec: "audiocodec_sel[4]",
	}}
	formFull := map[string]string{"medium_sel[4]": "7", "standard_sel[4]": "6", "audiocodec_sel[4]": "12"}

	// 中文标题（9/9 共性禁令）
	meta := &model.TorrentMetadata{Title: "天空之城 Laputa 1986", Subtitle: "副标题",
		MediaInfo: "General", IMDbURL: "https://imdb.com/tt1", Screenshots: `["a.jpg","b.jpg"]`}
	f := RunLocalAudit(localAuditInput{Form: formFull, Cfg: cfg, Meta: meta, PublishTitle: meta.Title})
	if !hasCode(f, "LOCAL_TITLE_NON_ASCII") {
		t.Errorf("中文标题应提示, got %v", codes(f))
	}

	// 完备英文标题 → 零提示
	meta2 := &model.TorrentMetadata{Title: "Movie 2026 1080p BluRay x265 DDP5.1-GROUP", Subtitle: "副标题",
		MediaInfo: "General", IMDbURL: "https://imdb.com/tt1", Screenshots: `["a.jpg","b.jpg"]`}
	f2 := RunLocalAudit(localAuditInput{Form: formFull, Cfg: cfg, Meta: meta2, PublishTitle: meta2.Title})
	if len(f2) != 0 {
		t.Errorf("完备应零提示, got %v", codes(f2))
	}

	// 禁发组（词边界）
	meta3 := &model.TorrentMetadata{Title: "Movie 2026 1080p BluRay x265-FGT", Subtitle: "副标题",
		MediaInfo: "General", IMDbURL: "https://imdb.com/tt1", Screenshots: `["a.jpg","b.jpg"]`}
	f3 := RunLocalAudit(localAuditInput{Form: formFull, Cfg: cfg, Meta: meta3, PublishTitle: meta3.Title})
	if !hasCode(f3, "LOCAL_BANNED_GROUP") {
		t.Errorf("禁发组 FGT 应提示, got %v", codes(f3))
	}

	// 词形违规（4K/1080P/AC3/HDR10）
	meta4 := &model.TorrentMetadata{Title: "Movie 2026 4K BluRay AC3 HDR10-GROUP", Subtitle: "副标题",
		MediaInfo: "General", IMDbURL: "https://imdb.com/tt1", Screenshots: `["a.jpg","b.jpg"]`}
	f4 := RunLocalAudit(localAuditInput{Form: formFull, Cfg: cfg, Meta: meta4, PublishTitle: meta4.Title})
	if !hasCode(f4, "LOCAL_TITLE_WORD_FORM") {
		t.Errorf("词形违规应提示, got %v", codes(f4))
	}

	// 缺项（副标题/MI/IMDb/截图）
	meta5 := &model.TorrentMetadata{Title: "Movie 2026 1080p BluRay x265 DDP5.1-GROUP"}
	f5 := RunLocalAudit(localAuditInput{Form: formFull, Cfg: cfg, Meta: meta5, PublishTitle: meta5.Title})
	for _, want := range []string{"LOCAL_SUBTITLE_EMPTY", "LOCAL_MEDIAINFO_EMPTY", "LOCAL_IMDB_EMPTY", "LOCAL_IMAGES_INSUFFICIENT"} {
		if !hasCode(f5, want) {
			t.Errorf("缺项 %s 应提示, got %v", want, codes(f5))
		}
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
