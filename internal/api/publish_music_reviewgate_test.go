package api

import (
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
)

// §59.469 附七: 音乐种审核门锚——isMusicSeed 走 title+artist+album
// 三非空（方案 B 质量门）；影视 9 字段零变化；Music 解析意外失败回退影视。
func TestCheckRequiredFieldsMusicGate(t *testing.T) {
	h := &PublishTorrentsHandler{}
	musicJSON := `{"title":"t","music":{"artist":"a","album":"b","format":"FLAC"}}`
	musicNoArtist := `{"title":"t","music":{"album":"b"}}`

	cases := []struct {
		name        string
		meta        *model.TorrentMetadata
		isMusic     bool
		wantMissing []string
	}{
		{
			name:        "音乐种全在场（music JSON 完整）",
			meta:        &model.TorrentMetadata{Title: "许景淳 - 路长情更长", DetailSourceJSON: musicJSON},
			isMusic:     true,
			wantMissing: nil,
		},
		{
			name:        "音乐种缺 artist（门内报 artist）",
			meta:        &model.TorrentMetadata{Title: "t", DetailSourceJSON: musicNoArtist},
			isMusic:     true,
			wantMissing: []string{"artist"},
		},
		{
			name:        "音乐种 title 空（门内报 title）",
			meta:        &model.TorrentMetadata{Title: "", DetailSourceJSON: musicJSON},
			isMusic:     true,
			wantMissing: []string{"title"},
		},
		{
			name:        "isMusic 但 Music 解析意外失败（回退影视·宁严勿松）",
			meta:        &model.TorrentMetadata{Title: "t", DetailSourceJSON: "not-json"},
			isMusic:     true,
			wantMissing: []string{"poster", "screenshots", "description", "mediainfo", "resolution", "video_codec", "audio_codec"},
		},
		{
			name:        "影视种零变化（缺全报·sourceDetector nil 场景）",
			meta:        &model.TorrentMetadata{Title: "Kill Bill"},
			isMusic:     false,
			wantMissing: []string{"poster", "screenshots", "description", "mediainfo", "resolution", "video_codec", "audio_codec"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := h.checkRequiredFields(c.meta, c.isMusic)
			if len(got) != len(c.wantMissing) {
				t.Fatalf("missing = %v, want %v", got, c.wantMissing)
			}
			for i := range got {
				if got[i] != c.wantMissing[i] {
					t.Fatalf("missing = %v, want %v", got, c.wantMissing)
				}
			}
		})
	}
}

// detailSourceHasMusic 精确判定通道锚。
func TestDetailSourceHasMusic(t *testing.T) {
	if !detailSourceHasMusic(`{"music":{"artist":"a"}}`) {
		t.Error("music 键在场应 true")
	}
	if detailSourceHasMusic(`{"title":"t"}`) {
		t.Error("无 music 键应 false")
	}
	if detailSourceHasMusic("") {
		t.Error("空 JSON 应 false")
	}
	if detailSourceHasMusic(`broken`) {
		t.Error("非法 JSON 应 false")
	}
}

// §59.469 附七: 音乐种跳过组映射锚——音乐标题无组名不判 no_mapping
// （音乐链已定向源站·组映射是影视获取通道语义；③ 早退曾遮蔽审核态）。
func TestClassifySeedStatusMusicSkipsGroupMapping(t *testing.T) {
	h := &PublishTorrentsHandler{}
	// 音乐种（hint true）·已获取已审核·标题无组名 → reviewed（非 no_mapping）
	meta := &model.TorrentMetadata{
		Title:            "许景淳 - 路长情更长 1994-FLAC 分轨",
		DetailSourceJSON: `{"music":{"artist":"许景淳","album":"路长情更长"}}`,
		Reviewed:         true,
	}
	if got := h.classifySeedStatusLite(nil, meta.Title, meta, true); got != "reviewed" {
		t.Errorf("音乐种已审核 status = %q, want reviewed（组映射应跳过）", got)
	}
	// 同标题影视语义（hint false）·未审核 → 组映射路径（h.sourceDetector nil
	// 场景不早退 no_mapping·走后续状态——此处锚定行为不回归即可）
	_ = h.classifySeedStatusLite(nil, meta.Title, meta, false)
}
