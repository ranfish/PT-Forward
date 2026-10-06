package publish

import (
	"os"
	"path/filepath"
	"testing"
)

// §59.319 D2 识别门：RPC 文件列表判定原盘
func TestDetectOriginalDisc(t *testing.T) {
	cases := []struct {
		name  string
		files []string
		want  bool
	}{
		{"BDMV 直下", []string{"BDMV/index.bdmv", "BDMV/MOVIEOBJ.bdmv", "CERTIFICATE/id.bdmv"}, true},
		{"BDMV 嵌套", []string{"Some.Disc-BDMV/BDMV/STREAM/00001.m2ts"}, true},
		{"大小写", []string{"bdmv/STREAM/00001.m2ts"}, true},
		{"ISO", []string{"Movie.2026.UHD.BluRay.iso"}, true},
		{"mkv 单文件", []string{"Movie.2026.2160p.mkv"}, false},
		{"剧集目录", []string{"Show.S01/Show.S01E01.mkv", "Show.S01/Show.S01E02.mkv"}, false},
		{"裸 m2ts 不算", []string{"Movie/00004.m2ts"}, false},
		{"空", nil, false},
	}
	for _, c := range cases {
		if got := DetectOriginalDisc(c.files); got != c.want {
			t.Errorf("%s: DetectOriginalDisc = %v, want %v", c.name, got, c.want)
		}
	}
}

// §59.319 P0：DetectBDPath 子目录 BDMV 检测曾不可达（.iso 分支后被
// entry.IsDir() continue 跳过）——重排修复的回归锚
func TestDetectBDPath_SubDirBDMV(t *testing.T) {
	root := t.TempDir()
	// 子目录形态：savePath/盘名/BDMV
	sub := filepath.Join(root, "Some.Disc.2026-BDMV", "BDMV", "STREAM")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := DetectBDPath(root); got != filepath.Join(root, "Some.Disc.2026-BDMV") {
		t.Errorf("子目录 BDMV = %q, want %q", got, filepath.Join(root, "Some.Disc.2026-BDMV"))
	}
	// 干扰文件（mkv）不应命中
	root2 := t.TempDir()
	if err := os.WriteFile(filepath.Join(root2, "movie.mkv"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := DetectBDPath(root2); got != "" {
		t.Errorf("非盘目录应空, got %q", got)
	}
}
