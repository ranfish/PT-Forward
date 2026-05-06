package seeding

import (
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
)

func TestHasSameFileTorrent_NoMatch(t *testing.T) {
	torrent := &model.TorrentInfo{
		Hash: "aaa", Name: "movie.mkv", TotalSize: 1000, SavePath: "/data/movies",
	}
	others := []*model.TorrentInfo{
		{Hash: "bbb", Name: "other.mkv", TotalSize: 1000, SavePath: "/data/movies"},
		{Hash: "ccc", Name: "movie.mkv", TotalSize: 2000, SavePath: "/data/movies"},
		{Hash: "ddd", Name: "movie.mkv", TotalSize: 1000, SavePath: "/data/other"},
	}
	if HasSameFileTorrent(torrent, others) {
		t.Error("should not find match")
	}
}

func TestHasSameFileTorrent_SameNameSizePath(t *testing.T) {
	torrent := &model.TorrentInfo{
		Hash: "aaa", Name: "movie.mkv", TotalSize: 1000, SavePath: "/data/movies",
	}
	others := []*model.TorrentInfo{
		{Hash: "bbb", Name: "movie.mkv", TotalSize: 1000, SavePath: "/data/movies"},
	}
	if !HasSameFileTorrent(torrent, others) {
		t.Error("should find match with same name+size+path")
	}
}

func TestHasSameFileTorrent_SkipSelfHash(t *testing.T) {
	torrent := &model.TorrentInfo{
		Hash: "aaa", Name: "movie.mkv", TotalSize: 1000, SavePath: "/data/movies",
	}
	others := []*model.TorrentInfo{
		{Hash: "aaa", Name: "movie.mkv", TotalSize: 1000, SavePath: "/data/movies"},
	}
	if HasSameFileTorrent(torrent, others) {
		t.Error("should skip self hash")
	}
}

func TestHasSameFileTorrent_EmptyList(t *testing.T) {
	torrent := &model.TorrentInfo{Hash: "aaa", Name: "movie.mkv", TotalSize: 1000, SavePath: "/data"}
	if HasSameFileTorrent(torrent, nil) {
		t.Error("should not find match in empty list")
	}
}

func TestFindRelatedByTagOrPath_ByRepostTag(t *testing.T) {
	torrent := &model.TorrentInfo{
		Hash: "aaa", Name: "movie.mkv", SavePath: "/data",
		Tags: []string{"reseed", "REPOST_abc123"},
	}
	others := []*model.TorrentInfo{
		torrent,
		{Hash: "bbb", Name: "different.mkv", SavePath: "/data", Tags: []string{"reseed", "REPOST_abc123"}},
		{Hash: "ccc", Name: "unrelated.mkv", SavePath: "/data", Tags: []string{"reseed"}},
	}
	result := FindRelatedByTagOrPath(torrent, others, 1)
	if len(result) != 1 || result[0] != "bbb" {
		t.Errorf("expected [bbb], got %v", result)
	}
}

func TestFindRelatedByTagOrPath_BySavePathAndName(t *testing.T) {
	torrent := &model.TorrentInfo{
		Hash: "aaa", Name: "movie.mkv", SavePath: "/data/movies", Tags: []string{},
	}
	others := []*model.TorrentInfo{
		torrent,
		{Hash: "bbb", Name: "movie.mkv", SavePath: "/data/movies", Tags: []string{}},
	}
	result := FindRelatedByTagOrPath(torrent, others, 1)
	if len(result) != 1 || result[0] != "bbb" {
		t.Errorf("expected [bbb] via savepath+name match, got %v", result)
	}
}

func TestFindRelatedByTagOrPath_Depth1(t *testing.T) {
	torrent := &model.TorrentInfo{
		Hash: "a1", Tags: []string{"REPOST_chain"},
	}
	others := []*model.TorrentInfo{
		torrent,
		{Hash: "b1", Tags: []string{"REPOST_chain"}},
		{Hash: "c1", Tags: []string{"REPOST_c1"}},
	}
	result := FindRelatedByTagOrPath(torrent, others, 1)
	if len(result) != 1 {
		t.Errorf("depth=1 should find 1 related, got %d", len(result))
	}
}

func TestFindRelatedByTagOrPath_Depth2(t *testing.T) {
	torrent := &model.TorrentInfo{
		Hash: "a1", Tags: []string{"REPOST_chain"},
	}
	others := []*model.TorrentInfo{
		torrent,
		{Hash: "b1", Tags: []string{"REPOST_chain", "REPOST_c1"}},
		{Hash: "c1", Tags: []string{"REPOST_c1"}},
	}
	result := FindRelatedByTagOrPath(torrent, others, 2)
	if len(result) != 2 {
		t.Errorf("depth=2 should find 2 related, got %d: %v", len(result), result)
	}
}

func TestFindRelatedByTagOrPath_NoRelated(t *testing.T) {
	torrent := &model.TorrentInfo{
		Hash: "a1", Name: "unique.mkv", SavePath: "/unique", Tags: []string{},
	}
	others := []*model.TorrentInfo{
		torrent,
		{Hash: "b1", Name: "other.mkv", SavePath: "/other", Tags: []string{}},
	}
	result := FindRelatedByTagOrPath(torrent, others, 1)
	if len(result) != 0 {
		t.Errorf("expected no related, got %v", result)
	}
}

func TestFindRelatedByTagOrPath_SkipEmptyName(t *testing.T) {
	torrent := &model.TorrentInfo{
		Hash: "a1", Name: "", SavePath: "/data", Tags: []string{},
	}
	others := []*model.TorrentInfo{
		torrent,
		{Hash: "b1", Name: "", SavePath: "/data", Tags: []string{}},
	}
	result := FindRelatedByTagOrPath(torrent, others, 1)
	if len(result) != 0 {
		t.Errorf("empty name should not match, got %v", result)
	}
}

func TestExtractRepostTags(t *testing.T) {
	tags := []string{"reseed", "REPOST_abc123", "pt-forward", "repost_lower"}
	result := ExtractRepostTags(tags)
	if len(result) != 1 || result[0] != "REPOST_abc123" {
		t.Errorf("expected [REPOST_abc123], got %v", result)
	}
}

func TestHasAnyTag(t *testing.T) {
	if HasAnyTag([]string{"a", "b"}, nil) {
		t.Error("nil targets should return false")
	}
	if HasAnyTag([]string{"a", "b"}, []string{}) {
		t.Error("empty targets should return false")
	}
	if !HasAnyTag([]string{"a", "b", "c"}, []string{"b"}) {
		t.Error("should find b")
	}
	if HasAnyTag([]string{"a", "c"}, []string{"b"}) {
		t.Error("should not find b")
	}
}

func TestSplitRuleIDs(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"1,2,3", 3},
		{" 1 , 2 , 3 ", 3},
		{"1,,2", 2},
		{"1,abc,2", 2},
		{"", 0},
	}
	for _, tt := range tests {
		got := splitRuleIDs(tt.input)
		if len(got) != tt.want {
			t.Errorf("splitRuleIDs(%q) = %v, want %d items", tt.input, got, tt.want)
		}
	}
}
