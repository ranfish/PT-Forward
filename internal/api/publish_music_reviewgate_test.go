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
