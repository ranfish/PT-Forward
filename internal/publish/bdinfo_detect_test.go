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

// §59.319 P3 事故回归锚：单文件种子（mkv）住在盘目录旁——DetectDiscPath
// 不得命中邻居盘（243 HDT 158 误入队案：WEB-DL mkv 的 savePath=HDT 根，
// 子目录扫描命中 Alice 的 BDMV）。规则：文件仅 .iso；目录仅 BDMV 直下。
func TestDetectDiscPath_NoNeighborHit(t *testing.T) {
	s := NewBDInfoScanner(nil)
	root := t.TempDir()
	// 邻居盘（HDT/Alice.../BDMV）
	alice := filepath.Join(root, "Alice.UHD-BDMV")
	if err := os.MkdirAll(filepath.Join(alice, "BDMV"), 0o755); err != nil {
		t.Fatal(err)
	}
	// mkv 单文件种子住在 HDT 根
	if err := os.WriteFile(filepath.Join(root, "96.2018.WEB-DL.mkv"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := s.DetectDiscPath(root, "96.2018.WEB-DL.mkv"); got != "" {
		t.Errorf("mkv 单文件应非盘, got %q（命中邻居盘=事故）", got)
	}
	// iso 单文件种子 → 命中
	if err := os.WriteFile(filepath.Join(root, "Movie.iso"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := s.DetectDiscPath(root, "Movie.iso"); got != filepath.Join(root, "Movie.iso") {
		t.Errorf("iso 单文件应命中, got %q", got)
	}
	// 盘目录种子 → BDMV 直下命中
	if got := s.DetectDiscPath(root, "Alice.UHD-BDMV"); got != alice {
		t.Errorf("盘目录应命中, got %q", got)
	}
	// mp4 单文件 → 非盘
	if err := os.WriteFile(filepath.Join(root, "show.mp4"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := s.DetectDiscPath(root, "show.mp4"); got != "" {
		t.Errorf("mp4 应非盘, got %q", got)
	}
	// 定位失败（不存在的名字）→ 不回退扫 savePath
	if got := s.DetectDiscPath(root, "ghost.torrent"); got != "" {
		t.Errorf("定位失败应空（不回退扫根目录）, got %q", got)
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
