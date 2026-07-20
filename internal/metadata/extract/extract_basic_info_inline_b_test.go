package extract

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPublicExtractor_InlineBLabel 验证 v0.0.251 新增的 inline <b>字段名:</b>值 聚合模式。
// 11 站详情页（HDSky/HDFans/CarPT/HDArea/HDTime/HDSky/HDUpT/Oshen 等）的基本信息表
// 把所有字段聚合在一个 td.rowfollow 内，用 <b>类型:</b>值 <b>媒介:</b>值 ... 水平排列。
func TestPublicExtractor_InlineBLabel(t *testing.T) {
	tests := []struct {
		name        string
		htmlFile    string
		siteCode    string // PublicExtractor 的 siteCode（用于加载 site-specific source_key）
		wantFields  []string // 期望非空的字段
	}{
		{"CarPT_车站", "inline_b_carpt.html", "", []string{"medium", "video_codec", "audio_codec", "resolution"}},
		{"HDFans_红豆饭", "inline_b_hdfans.html", "", []string{"medium", "video_codec", "audio_codec", "resolution", "source"}},
		{"HDUpT_好多油", "inline_b_hdupt.html", "", []string{"medium", "video_codec", "audio_codec", "resolution", "source"}},
		{"Oshen_奥申", "inline_b_oshen.html", "", []string{"medium", "video_codec", "resolution"}},
		// v0.0.252 新增：星湾（中文长值不被字节长度过滤）+ 织梦（<b title=""> 模式）
		{"XingWan_星湾", "inline_b_xingwan.html", "", []string{"medium", "video_codec", "resolution"}},
		{"ZmPT_织梦", "inline_b_zmpt.html", "", []string{"medium", "resolution"}},
		// v0.0.252 新增：壹吧/百川/碟粉 用 "来源" 代替 "媒介"（site-specific source_key）
		{"1PTBar_壹吧", "inline_b_1ptba.html", "1ptba", []string{"medium", "video_codec", "audio_codec", "resolution"}},
		{"HitPT_百川", "inline_b_hitpt.html", "hitpt", []string{"medium", "video_codec", "audio_codec", "resolution"}},
		{"DiscFan_碟粉", "inline_b_discfan.html", "discfan", []string{"medium"}},
	}

	testdataDir := findAdapterTestdata()
	if testdataDir == "" {
		t.Skip("testdata not found")
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			htmlBytes, err := os.ReadFile(filepath.Join(testdataDir, tt.htmlFile))
			if err != nil {
				t.Skipf("read testdata %s: %v", tt.htmlFile, err)
			}

			p := NewPublicExtractor(tt.siteCode, "")
			seed, err := p.Extract(Input{
				PageHTML:      string(htmlBytes),
				FallbackTitle: "fallback",
			})
			if err != nil {
				t.Fatalf("Extract failed: %v", err)
			}

			for _, f := range tt.wantFields {
				var got string
				switch f {
				case "type":
					got = seed.Type
				case "medium":
					got = seed.Medium
				case "video_codec":
					got = seed.VideoCodec
				case "audio_codec":
					got = seed.AudioCodec
				case "resolution":
					got = seed.Resolution
				case "team":
					got = seed.ReleaseGroup
				case "source":
					got = seed.Source
				}
				if strings.TrimSpace(got) == "" {
					t.Errorf("field %q should not be empty", f)
				} else {
					t.Logf("  %s = %q", f, got)
				}
			}
		})
	}
}
