package fingerprint

import (
	"context"
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupFingerprintDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.ContentFingerprint{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return db
}

func TestBencodeDecodeInt(t *testing.T) {
	v, err := decodeBencode([]byte("i42e"))
	if err != nil {
		t.Fatal(err)
	}
	if v.(int64) != 42 {
		t.Errorf("expected 42, got %v", v)
	}
}

func TestBencodeDecodeString(t *testing.T) {
	v, err := decodeBencode([]byte("4:spam"))
	if err != nil {
		t.Fatal(err)
	}
	if v.(string) != "spam" {
		t.Errorf("expected 'spam', got %v", v)
	}
}

func TestBencodeDecodeList(t *testing.T) {
	v, err := decodeBencode([]byte("l4:spam4:eggse"))
	if err != nil {
		t.Fatal(err)
	}
	list := v.([]any)
	if len(list) != 2 {
		t.Fatalf("expected 2 items, got %d", len(list))
	}
	if list[0].(string) != "spam" {
		t.Errorf("expected 'spam', got %v", list[0])
	}
	if list[1].(string) != "eggs" {
		t.Errorf("expected 'eggs', got %v", list[1])
	}
}

func TestBencodeDecodeDict(t *testing.T) {
	v, err := decodeBencode([]byte("d3:cow3:moo4:spam4:eggse"))
	if err != nil {
		t.Fatal(err)
	}
	d := v.(map[string]any)
	if d["cow"].(string) != "moo" {
		t.Errorf("expected 'moo', got %v", d["cow"])
	}
	if d["spam"].(string) != "eggs" {
		t.Errorf("expected 'eggs', got %v", d["spam"])
	}
}

func TestBencodeDecodeNested(t *testing.T) {
	data := []byte("d4:infod6:lengthi1024e4:name4:teste12:announce-url26:http://tracker.example.come")
	v, err := decodeBencode(data)
	if err != nil {
		t.Fatal(err)
	}
	root := v.(map[string]any)
	info := root["info"].(map[string]any)
	if info["length"].(int64) != 1024 {
		t.Errorf("expected 1024, got %v", info["length"])
	}
	if info["name"].(string) != "test" {
		t.Errorf("expected 'test', got %v", info["name"])
	}
}

func TestBencodeDecodeInvalid(t *testing.T) {
	_, err := decodeBencode([]byte("x"))
	if err == nil {
		t.Error("expected error for invalid data")
	}

	_, err = decodeBencode([]byte{})
	if err == nil {
		t.Error("expected error for empty data")
	}

	_, err = decodeBencode([]byte("i"))
	if err == nil {
		t.Error("expected error for unterminated int")
	}
}

func TestBencodeEncodeRoundtrip(t *testing.T) {
	original := map[string]any{
		"name":   "test.torrent",
		"length": int64(2048),
		"files": []any{
			map[string]any{
				"path":   []any{"dir", "file.txt"},
				"length": int64(2048),
			},
		},
	}

	encoded, err := encodeBencode(original)
	if err != nil {
		t.Fatal(err)
	}

	decoded, err := decodeBencode(encoded)
	if err != nil {
		t.Fatal(err)
	}

	d := decoded.(map[string]any)
	if d["name"].(string) != "test.torrent" {
		t.Errorf("roundtrip name mismatch: %v", d["name"])
	}
	if d["length"].(int64) != 2048 {
		t.Errorf("roundtrip length mismatch: %v", d["length"])
	}
}

func TestComputeFromTorrent_SingleFile(t *testing.T) {
	torrentData := buildTestTorrentSingleFile(t)
	meta, err := ComputeFromTorrent(torrentData)
	if err != nil {
		t.Fatal(err)
	}

	if meta.InfoHash == "" {
		t.Error("info_hash should not be empty")
	}
	if meta.PiecesHash == "" {
		t.Error("pieces_hash should not be empty")
	}
	if meta.TotalSize != 1024 {
		t.Errorf("expected total_size=1024, got %d", meta.TotalSize)
	}
	if meta.FileCount != 1 {
		t.Errorf("expected file_count=1, got %d", meta.FileCount)
	}
	if meta.LargestFile != 1024 {
		t.Errorf("expected largest_file=1024, got %d", meta.LargestFile)
	}
	if meta.FilesHash == "" {
		t.Error("files_hash should not be empty")
	}
	if _, ok := meta.FileTree["test.txt"]; !ok {
		t.Errorf("expected 'test.txt' in file tree, got %v", meta.FileTree)
	}
}

func TestComputeFromTorrent_MultiFile(t *testing.T) {
	torrentData := buildTestTorrentMultiFile(t)
	meta, err := ComputeFromTorrent(torrentData)
	if err != nil {
		t.Fatal(err)
	}

	if meta.TotalSize != 3072 {
		t.Errorf("expected total_size=3072, got %d", meta.TotalSize)
	}
	if meta.FileCount != 3 {
		t.Errorf("expected file_count=3, got %d", meta.FileCount)
	}
	if meta.LargestFile != 2048 {
		t.Errorf("expected largest_file=2048, got %d", meta.LargestFile)
	}

	expectedFiles := map[string]int64{
		"dir/file1.txt": 1024,
		"dir/file2.txt": 2048,
		"dir/file3.txt": 0,
	}
	for path, size := range expectedFiles {
		if got, ok := meta.FileTree[path]; !ok || got != size {
			t.Errorf("file tree[%q]: expected %d, got %d (exists=%v)", path, size, got, ok)
		}
	}
}

func TestComputeFromTorrent_Empty(t *testing.T) {
	_, err := ComputeFromTorrent([]byte{})
	if err == nil {
		t.Error("expected error for empty data")
	}
}

func TestComputeFromTorrent_Invalid(t *testing.T) {
	_, err := ComputeFromTorrent([]byte("not bencoded"))
	if err == nil {
		t.Error("expected error for invalid data")
	}
}

func TestComputeFromTorrent_NoInfo(t *testing.T) {
	data := []byte("d3:foo3:bare")
	_, err := ComputeFromTorrent(data)
	if err == nil {
		t.Error("expected error for missing info dict")
	}
}

func TestRepository_SaveAndGet(t *testing.T) {
	db := setupFingerprintDB(t)
	repo := NewRepository(db, zap.NewNop())
	ctx := context.Background()

	fp := &model.ContentFingerprint{
		InfoHash:   "abc123",
		SiteName:   "testsite",
		TorrentID:  "42",
		PiecesHash: "def456",
		TotalSize:  1024,
		FileCount:  1,
	}
	if err := repo.Save(ctx, fp); err != nil {
		t.Fatal(err)
	}

	got, err := repo.GetByInfoHash(ctx, "abc123")
	if err != nil {
		t.Fatal(err)
	}
	if got.PiecesHash != "def456" {
		t.Errorf("expected def456, got %s", got.PiecesHash)
	}
	if got.SiteName != "testsite" {
		t.Errorf("expected testsite, got %s", got.SiteName)
	}
}

func TestRepository_GetBySiteAndTorrentID(t *testing.T) {
	db := setupFingerprintDB(t)
	repo := NewRepository(db, zap.NewNop())
	ctx := context.Background()

	fp := &model.ContentFingerprint{
		InfoHash:   "abc123",
		SiteName:   "site1",
		TorrentID:  "42",
		PiecesHash: "def456",
	}
	if err := repo.Save(ctx, fp); err != nil {
		t.Fatal(err)
	}

	got, err := repo.GetBySiteAndTorrentID(ctx, "site1", "42")
	if err != nil {
		t.Fatal(err)
	}
	if got.InfoHash != "abc123" {
		t.Errorf("expected abc123, got %s", got.InfoHash)
	}
}

func TestRepository_ComputeAndSave(t *testing.T) {
	db := setupFingerprintDB(t)
	repo := NewRepository(db, zap.NewNop())
	ctx := context.Background()

	torrentData := buildTestTorrentSingleFile(t)

	fp, err := repo.ComputeAndSave(ctx, "site1", "42", torrentData, "Test Torrent")
	if err != nil {
		t.Fatal(err)
	}

	if fp.InfoHash == "" {
		t.Error("info_hash should not be empty")
	}
	if fp.SiteName != "site1" {
		t.Errorf("expected site1, got %s", fp.SiteName)
	}
	if fp.TorrentID != "42" {
		t.Errorf("expected 42, got %s", fp.TorrentID)
	}
	if fp.TotalSize != 1024 {
		t.Errorf("expected 1024, got %d", fp.TotalSize)
	}
	if fp.Title != "Test Torrent" {
		t.Errorf("expected 'Test Torrent', got %s", fp.Title)
	}

	got, err := repo.GetByInfoHash(ctx, fp.InfoHash)
	if err != nil {
		t.Fatal(err)
	}
	if got.PiecesHash != fp.PiecesHash {
		t.Errorf("pieces_hash mismatch: expected %s, got %s", fp.PiecesHash, got.PiecesHash)
	}
}

func TestRepository_ComputeAndSaveIdempotent(t *testing.T) {
	db := setupFingerprintDB(t)
	repo := NewRepository(db, zap.NewNop())
	ctx := context.Background()

	torrentData := buildTestTorrentSingleFile(t)

	fp1, err := repo.ComputeAndSave(ctx, "site1", "42", torrentData, "Test")
	if err != nil {
		t.Fatal(err)
	}

	fp2, err := repo.ComputeAndSave(ctx, "site1", "42", torrentData, "Test Updated")
	if err != nil {
		t.Fatal(err)
	}

	if fp1.ID != fp2.ID {
		t.Errorf("expected same ID for idempotent save: %d vs %d", fp1.ID, fp2.ID)
	}
}

func TestRepository_FindByPiecesHash(t *testing.T) {
	db := setupFingerprintDB(t)
	repo := NewRepository(db, zap.NewNop())
	ctx := context.Background()

	if err := repo.Save(ctx, &model.ContentFingerprint{
		InfoHash: "h1", SiteName: "s1", TorrentID: "1", PiecesHash: "shared_hash", TotalSize: 100,
	}); err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(ctx, &model.ContentFingerprint{
		InfoHash: "h2", SiteName: "s2", TorrentID: "2", PiecesHash: "shared_hash", TotalSize: 200,
	}); err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(ctx, &model.ContentFingerprint{
		InfoHash: "h3", SiteName: "s3", TorrentID: "3", PiecesHash: "different_hash", TotalSize: 300,
	}); err != nil {
		t.Fatal(err)
	}

	results, err := repo.FindByPiecesHash(ctx, "shared_hash")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
}

func TestFilesHash_Deterministic(t *testing.T) {
	tree1 := map[string]int64{"a.txt": 100, "b.txt": 200}
	tree2 := map[string]int64{"b.txt": 200, "a.txt": 100}

	h1 := computeFilesHash(tree1)
	h2 := computeFilesHash(tree2)

	if h1 != h2 {
		t.Errorf("files hash should be deterministic regardless of map iteration order: %s vs %s", h1, h2)
	}
}

func buildTestTorrentSingleFile(t *testing.T) []byte {
	t.Helper()
	pieces := make([]byte, 20)
	info := map[string]any{
		"name":   "test.txt",
		"length": int64(1024),
		"pieces": string(pieces),
	}
	encoded, err := encodeBencode(map[string]any{
		"info":     info,
		"announce": "http://tracker.example.com/announce",
	})
	if err != nil {
		t.Fatalf("encode test torrent: %v", err)
	}
	return encoded
}

func buildTestTorrentMultiFile(t *testing.T) []byte {
	t.Helper()
	pieces := make([]byte, 20)
	info := map[string]any{
		"name": "testdir",
		"files": []any{
			map[string]any{"path": []any{"dir", "file1.txt"}, "length": int64(1024)},
			map[string]any{"path": []any{"dir", "file2.txt"}, "length": int64(2048)},
			map[string]any{"path": []any{"dir", "file3.txt"}, "length": int64(0)},
		},
		"pieces": string(pieces),
	}
	encoded, err := encodeBencode(map[string]any{
		"info":     info,
		"announce": "http://tracker.example.com/announce",
	})
	if err != nil {
		t.Fatalf("encode test torrent: %v", err)
	}
	return encoded
}
