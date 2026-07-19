package description

import (
	"strings"
	"testing"
)

func TestFormatMusicDescription_Complete(t *testing.T) {
	data := &MusicAlbumData{
		Artists: []MusicArtist{
			{Name: "Artist A", Importance: "Main"},
			{Name: "Artist B", Importance: "Guest"},
		},
		Title:         "Test Album",
		Year:          "2024",
		RecordLabel:   "Test Label",
		CatalogNumber: "TL001",
		ReleaseType:   "Album",
		Format:        "FLAC",
		Encoding:      "Lossless",
		Media:         "CD",
		Tracklist: []MusicTrack{
			{Number: 1, Title: "Track One", Duration: "3:45"},
			{Number: 2, Title: "Track Two", Duration: "4:20"},
		},
		PosterURL:   "https://example.com/cover.jpg",
		Description: "专辑简介",
		Scene:       true,
	}

	result := FormatMusicDescription(data)
	if result == "" {
		t.Fatal("result should not be empty")
	}
	// 检查封面
	if !strings.Contains(result, "[img]https://example.com/cover.jpg[/img]") {
		t.Errorf("should contain poster")
	}
	// 检查艺术家（仅 Main）
	if !strings.Contains(result, "Artist A") {
		t.Errorf("should contain main artist")
	}
	if strings.Contains(result, "Artist B") {
		t.Errorf("should not contain guest artist in main field")
	}
	// 检查专辑信息
	if !strings.Contains(result, "Test Album") {
		t.Errorf("should contain album title")
	}
	if !strings.Contains(result, "2024") {
		t.Errorf("should contain year")
	}
	if !strings.Contains(result, "FLAC") {
		t.Errorf("should contain format")
	}
	// 检查曲目列表
	if !strings.Contains(result, "Tracklist") {
		t.Errorf("should contain tracklist header")
	}
	if !strings.Contains(result, "Track One") {
		t.Errorf("should contain track name")
	}
	if !strings.Contains(result, "3:45") {
		t.Errorf("should contain track duration")
	}
	// 检查 Scene（BBCode 格式 [b]Scene:[/b] Yes）
	if !strings.Contains(result, "Scene") || !strings.Contains(result, "Yes") {
		t.Errorf("should contain scene tag, got: %q", result)
	}
}

func TestFormatMusicDescription_Nil(t *testing.T) {
	if result := FormatMusicDescription(nil); result != "" {
		t.Errorf("nil data should return empty, got %q", result)
	}
}

func TestFormatMusicDescription_Minimal(t *testing.T) {
	data := &MusicAlbumData{
		Title:  "Minimal",
		Year:   "2024",
		Format: "MP3",
	}
	result := FormatMusicDescription(data)
	if !strings.Contains(result, "Minimal") {
		t.Errorf("should contain title")
	}
	if strings.Contains(result, "Tracklist") {
		t.Errorf("should not contain tracklist when empty")
	}
}

func TestFormatMusicDescription_TrackNumber(t *testing.T) {
	data := &MusicAlbumData{
		Title: "Test",
		Tracklist: []MusicTrack{
			{Number: 1, Title: "First"},
			{Number: 10, Title: "Tenth"},
		},
	}
	result := FormatMusicDescription(data)
	if !strings.Contains(result, "1. First") {
		t.Errorf("track 1 should be zero-padded or direct, got: %q", result)
	}
	if !strings.Contains(result, "Tenth") {
		t.Errorf("track 10 should be present")
	}
}
