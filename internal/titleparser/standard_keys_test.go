package titleparser

import "testing"


// §59.268: 标准键折叠——数据驱动（standard_keys.json fold_to），编码器实现归标准
func TestFoldStandardKey(t *testing.T) {
	cases := []struct{ category, in, want string }{
		{"video_codec", "video.x264", "video.h264"},
		{"video_codec", "video.x265", "video.h265"},
		{"video_codec", "video.h264", "video.h264"}, // 无折叠词条原值
		{"video_codec", "video.av1", "video.av1"},
		{"medium", "medium.bluray_3d", "medium.bluray_3d"}, // 未配置域原值
		{"video_codec", "", ""},
	}
	for _, tc := range cases {
		if got := FoldStandardKey(tc.category, tc.in); got != tc.want {
			t.Errorf("[%s] %q → %q, want %q", tc.category, tc.in, got, tc.want)
		}
	}
	// 词条可扩展性：LoadStandardKeys 应能读出 fold_to 字段
	keys, err := LoadStandardKeys()
	if err != nil {
		t.Fatal(err)
	}
	folds := 0
	for _, k := range keys {
		if k.FoldTo != "" {
			folds++
		}
	}
	if folds < 2 {
		t.Errorf("fold_to 词条应 ≥2（x264/x265），实际 %d", folds)
	}
}
