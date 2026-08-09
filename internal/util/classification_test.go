package util

import (
	"os"
	"path/filepath"
	"testing"
)

func TestClassifyTorrent_Title(t *testing.T) {
	tests := []struct {
		name     string
		title    string
		category string
		form     string
	}{
		// 电影
		{"movie", "The.Matrix.1999.1080p.BluRay.x264-GROUP", "movie", ""},
		{"movie chinese", "白毛女.1951.BluRay.1080p.x264-CMCT", "movie", ""},

		// 单集（标题明确 S01E03）
		{"single episode", "Series.S01E03.1080p.WEB-DL.AAC.H.264-CHDWEB", "tv_series", "single_episode"},
		{"single episode space", "The Road to Splendor 2026 S01 E03 1080p", "tv_series", "single_episode"},
		{"single episode lowercase", "the husband 2026 s01e11 1080p web h264-kbbq", "tv_series", "single_episode"},

		// 全集（Complete 标记）
		{"season complete", "Xue Ke S01 1990 Complete 1080p", "tv_series", "season_pack"},
		{"season complete 2", "Love's Ambition 2025 S01 Complete 2160p", "tv_series", "season_pack"},

		// 区间（从 E01 开始 = 全集）
		{"range from E01", "Series.S01E01-E12.1080p.WEB-DL", "tv_series", "season_pack"},
		{"range from E01 space", "The Road to Splendor 2026 S01 E01-E03 2160p", "tv_series", "season_pack"},

		// 区间（不从 E01 开始 = 部分合集）
		{"range not from E01", "Ren.Yu.S01E05-E11.2026.2160p", "tv_series", "partial_pack"},
		{"range not from E01 2", "Ren.Yu.S01E11-E12.2026.2160p", "tv_series", "partial_pack"},

		// S01 无 E 无 Complete = unknown（CHDBits/HHanClub 模式）
		{"S01 no E ambiguous", "The.Eternal.Fragrance.S01.2026.2160p.WEB-DL.H265.AAC-CHDWEB", "tv_series", "unknown"},
		{"S01 no E ambiguous 2", "Evil Hunter S01 2026 2160p WEB-DL H265 DV DDP5.1 Atmos-CHDWEB", "tv_series", "unknown"},

		// 中文标记
		{"chinese all episodes", "翡翠台 一周星星（第一季） 全26集 粤语", "tv_series", "season_pack"},
		{"chinese single ep", "攻壳机动队 第一季第5集 S01E05", "tv_series", "single_episode"},
		{"chinese range from 1", "某剧 第一季 第01-12集", "tv_series", "season_pack"},
		{"chinese range not from 1", "某剧 第一季 第05-08集", "tv_series", "partial_pack"},
		{"chinese episodes end", "某剧 16集全", "tv_series", "season_pack"},

		// 碟片
		{"bluray disc", "JoJo's Bizarre Adventure - Stone Ocean D07 2021 JPN Blu-Ray", "movie", ""},

		// 无标题
		{"empty title", "", "unknown", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ClassifyTorrent(tt.title, nil)
			if result.Category != tt.category {
				t.Errorf("category = %q, want %q (title: %s)", result.Category, tt.category, tt.title)
			}
			if result.Form != tt.form {
				t.Errorf("form = %q, want %q (title: %s)", result.Form, tt.form, tt.title)
			}
		})
	}
}

func TestClassifyTorrent_FileTreeRefinesForm(t *testing.T) {
	// S01 无 E 标题 + 文件树有 SxxExx 文件 → 文件树修正 Form

	// 单集：1 个 S01E03 文件
	t.Run("S01 title + 1 episode file → single_episode", func(t *testing.T) {
		fileTree := map[string]int64{
			"Series.S01E03.1080p.WEB.mkv": 2500000000,
		}
		result := ClassifyTorrent("Series.S01.1080p.WEB-DL-CHDWEB", fileTree)
		if result.Form != "single_episode" {
			t.Errorf("form = %q, want single_episode", result.Form)
		}
	})

	// 部分合集：2 个文件，不从 E01 开始
	t.Run("S01 title + E28 E29 files → partial_pack", func(t *testing.T) {
		fileTree := map[string]int64{
			"Series.S01E28.1080p.WEB.mkv": 2400000000,
			"Series.S01E29.1080p.WEB.mkv": 2400000000,
		}
		result := ClassifyTorrent("Series.S01.1080p.WEB-DL-CHDWEB", fileTree)
		if result.Form != "partial_pack" {
			t.Errorf("form = %q, want partial_pack", result.Form)
		}
	})

	// 全集：16 个文件，从 E01 开始
	t.Run("S01 title + E01-E16 files → season_pack", func(t *testing.T) {
		fileTree := make(map[string]int64)
		for i := 1; i <= 16; i++ {
			fileTree[filepath.Join("dir", "Series.S01E"+padEp(i)+".1080p.WEB.mkv")] = 2000000000
		}
		result := ClassifyTorrent("Series.S01.2026.2160p.WEB-DL.H265.AAC-CHDWEB", fileTree)
		if result.Form != "season_pack" {
			t.Errorf("form = %q, want season_pack", result.Form)
		}
	})

	// 电影：标题无 S\d+，文件树无 SxxExx
	t.Run("movie title + no episode files → movie", func(t *testing.T) {
		fileTree := map[string]int64{
			"The.Matrix.1999.1080p.BluRay.x264.mkv": 8000000000,
		}
		result := ClassifyTorrent("The.Matrix.1999.1080p.BluRay.x264-GROUP", fileTree)
		if result.Category != "movie" {
			t.Errorf("category = %q, want movie", result.Category)
		}
		if result.Form != "" {
			t.Errorf("form = %q, want empty", result.Form)
		}
	})

	// 音乐：仅有音频文件
	t.Run("audio files → music", func(t *testing.T) {
		fileTree := map[string]int64{
			"album.flac": 500000000,
			"cover.jpg":  100000,
		}
		result := ClassifyTorrent("Some Album 2024 FLAC", fileTree)
		if result.Category != "music" {
			t.Errorf("category = %q, want music", result.Category)
		}
	})
}

func TestClassifyFromDir(t *testing.T) {
	tmpDir := t.TempDir()

	// Brothers.in.Arms 模拟：2 集 E28 E29
	dir1 := filepath.Join(tmpDir, "Series.S01.1080p.WEB-DL-CHDWEB")
	os.MkdirAll(dir1, 0755)
	os.WriteFile(filepath.Join(dir1, "Series.S01E28.1080p.WEB-DL-CHDWEB.mkv"), []byte("x"), 0644)
	os.WriteFile(filepath.Join(dir1, "Series.S01E29.1080p.WEB-DL-CHDWEB.mkv"), []byte("xy"), 0644)

	cls, err := ClassifyFromDir(dir1, "Series.S01.1080p.WEB-DL-CHDWEB")
	if err != nil {
		t.Fatal(err)
	}
	if cls.Type.Form != "partial_pack" {
		t.Errorf("form = %q, want partial_pack", cls.Type.Form)
	}
	if len(cls.VideoFiles) != 2 {
		t.Errorf("video files = %d, want 2", len(cls.VideoFiles))
	}
	if cls.VideoFiles[0].Size < cls.VideoFiles[1].Size {
		t.Error("video files not sorted by size descending")
	}

	// 电影模拟：1 个 mkv 无 SxxExx
	dir2 := filepath.Join(tmpDir, "Movie.1999.1080p.BluRay")
	os.MkdirAll(dir2, 0755)
	os.WriteFile(filepath.Join(dir2, "Movie.1999.1080p.BluRay.x264.mkv"), []byte("x"), 0644)

	cls2, err := ClassifyFromDir(dir2, "Movie.1999.1080p.BluRay")
	if err != nil {
		t.Fatal(err)
	}
	if cls2.Type.Category != "movie" {
		t.Errorf("category = %q, want movie", cls2.Type.Category)
	}
}

func padEp(n int) string {
	if n < 10 {
		return "0" + string(rune('0'+n))
	}
	return string(rune('0'+n/10)) + string(rune('0'+n%10))
}
