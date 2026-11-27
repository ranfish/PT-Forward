package publish

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// §59.589 Scandal 案锚（2026-11-27 243 实测轨表 E08）——
// 自动截图选轨选错韩文根因：FindSubtitleStreamID 原按「过滤后候选
// 列表位置」数 mpv sid，而被 disposition 滤掉的轨（hearing_impaired
// 等）在 mpv 轨表依然占号——韩剧 NF 源语言轨组（kor SDH/kor HI）
// 排在 chi 轨前，kor HI 被滤致 chi 从真实 sid=3 偏移成 sid=2=
// 韩文 HI 轨。用户 forced sid=3 手动出中文互证。
//
// 修复：SubIndex 字段（字幕流绝对序 1-based=mpv sid 语义）——Detect
// 对全部 ffprobe 字幕流行计数（含被滤轨）。
//
// 阴性对照：Detect 的 subIdx 计数回退到 append 处（被滤轨不计数）
// → 本锚 FAIL（E08 场景 sid 退回 2）。
func TestFindSubtitleStreamID_FilteredTrackOffset(t *testing.T) {
	// E08 实测轨表：kor SDH / kor HI(hearing_impaired=1·被滤) / chi Simplified / chi Traditional
	e08 := `{"streams":[
{"index":2,"codec_name":"subrip","disposition":{"comment":0,"hearing_impaired":0,"visual_impaired":0},"tags":{"language":"kor","title":"SDH"}},
{"index":3,"codec_name":"subrip","disposition":{"comment":0,"hearing_impaired":1,"visual_impaired":0},"tags":{"language":"kor"}},
{"index":4,"codec_name":"subrip","disposition":{"comment":0,"hearing_impaired":0,"visual_impaired":0},"tags":{"language":"chi","title":"Simplified"}},
{"index":5,"codec_name":"subrip","disposition":{"comment":0,"hearing_impaired":0,"visual_impaired":0},"tags":{"language":"chi","title":"Traditional"}}
]}`
	// E01 形态加 forced 轨：kor Forced / kor SDH / kor HI(被滤) / chi Simplified / chi Traditional
	e01 := `{"streams":[
{"index":2,"codec_name":"subrip","disposition":{"comment":0,"hearing_impaired":0,"visual_impaired":0,"forced":1},"tags":{"language":"kor","title":"Forced"}},
{"index":3,"codec_name":"subrip","disposition":{"comment":0,"hearing_impaired":0,"visual_impaired":0},"tags":{"language":"kor","title":"SDH"}},
{"index":4,"codec_name":"subrip","disposition":{"comment":0,"hearing_impaired":1,"visual_impaired":0},"tags":{"language":"kor"}},
{"index":5,"codec_name":"subrip","disposition":{"comment":0,"hearing_impaired":0,"visual_impaired":0},"tags":{"language":"chi","title":"Simplified"}},
{"index":6,"codec_name":"subrip","disposition":{"comment":0,"hearing_impaired":0,"visual_impaired":0},"tags":{"language":"chi","title":"Traditional"}}
]}`
	// 回归：无被滤轨（UNABOMBER 简化形态）——列表序=绝对序无错位
	noFiltered := `{"streams":[
{"index":3,"codec_name":"webvtt","disposition":{"comment":0,"hearing_impaired":0,"visual_impaired":0},"tags":{"language":"eng","title":"English [Original] (CC)"}},
{"index":4,"codec_name":"webvtt","disposition":{"comment":0,"hearing_impaired":0,"visual_impaired":0},"tags":{"language":"chi","title":"中文（简体）"}},
{"index":5,"codec_name":"webvtt","disposition":{"comment":0,"hearing_impaired":0,"visual_impaired":0},"tags":{"language":"chi","title":"中文（繁體）"}}
]}`

	cases := []struct {
		name    string
		json    string
		wantSid int
	}{
		{"E08_韩剧被滤轨偏移案", e08, 3},
		{"E01_forced+被滤轨双前置", e01, 4},
		{"无被滤轨回归", noFiltered, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := NewSubtitleDetector(nil)
			d.ffprobePath = fakeFFprobe(t, tc.json)
			sid, err := d.FindSubtitleStreamID(context.Background(), "fake.mkv")
			if err != nil {
				t.Fatalf("FindSubtitleStreamID: %v", err)
			}
			if sid != tc.wantSid {
				t.Errorf("sid=%d want %d（SubIndex 绝对序——旧列表位错位形态应已根修）", sid, tc.wantSid)
			}
		})
	}
}

// fakeFFprobe 生成输出预置 JSON 的 ffprobe 替身脚本（§59.338 离线驱动
// 同思路——链上代码全真实，仅 ffprobe 二进制替身）。
func fakeFFprobe(t *testing.T, jsonOut string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "ffprobe-fake")
	script := "#!/bin/sh\ncat <<'EOF'\n" + jsonOut + "\nEOF\n"
	if err := os.WriteFile(p, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}
