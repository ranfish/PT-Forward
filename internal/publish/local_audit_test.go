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
	meta := &model.TorrentMetadata{Title: "Arco 2025 UHD BluRay DTS-HD MA 7.1 mUHD-FRDS"}

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
