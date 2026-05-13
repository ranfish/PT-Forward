package publish

import (
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/zap"
)

func tempCacheDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(os.TempDir(), "pt-forward-cache-test")
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

func TestArtifactCache_SetAndGet(t *testing.T) {
	dir := tempCacheDir(t)
	cache := NewArtifactCache(dir, zap.NewNop())

	if err := cache.Set("key1", []byte("hello")); err != nil {
		t.Fatalf("set: %v", err)
	}

	data, ok := cache.Get("key1")
	if !ok {
		t.Fatal("expected to find key1")
	}
	if string(data) != "hello" {
		t.Errorf("got %q, want %q", string(data), "hello")
	}
}

func TestArtifactCache_GetMissing(t *testing.T) {
	dir := tempCacheDir(t)
	cache := NewArtifactCache(dir, zap.NewNop())

	_, ok := cache.Get("nonexistent")
	if ok {
		t.Error("should not find nonexistent key")
	}
}

func TestArtifactCache_Delete(t *testing.T) {
	dir := tempCacheDir(t)
	cache := NewArtifactCache(dir, zap.NewNop())

	cache.Set("key1", []byte("data"))
	cache.Delete("key1")

	_, ok := cache.Get("key1")
	if ok {
		t.Error("should not find deleted key")
	}
}

func TestArtifactCache_Overwrite(t *testing.T) {
	dir := tempCacheDir(t)
	cache := NewArtifactCache(dir, zap.NewNop())

	cache.Set("key1", []byte("v1"))
	cache.Set("key1", []byte("v2"))

	data, ok := cache.Get("key1")
	if !ok || string(data) != "v2" {
		t.Errorf("expected v2, got %q, ok=%v", string(data), ok)
	}
}

func TestTorrentCache_SetAndGet(t *testing.T) {
	dir := filepath.Join(os.TempDir(), "pt-forward-tcache-test")
	t.Cleanup(func() { os.RemoveAll(dir) })

	cache := NewTorrentCache(dir, zap.NewNop())

	if err := cache.Set("hash1", []byte("torrent-data")); err != nil {
		t.Fatalf("set: %v", err)
	}

	data, ok := cache.Get("hash1")
	if !ok || string(data) != "torrent-data" {
		t.Errorf("expected torrent-data, got %q, ok=%v", string(data), ok)
	}
}

func TestTorrentCache_Delete(t *testing.T) {
	dir := filepath.Join(os.TempDir(), "pt-forward-tcache-test2")
	t.Cleanup(func() { os.RemoveAll(dir) })

	cache := NewTorrentCache(dir, zap.NewNop())
	cache.Set("hash1", []byte("data"))
	cache.Delete("hash1")

	_, ok := cache.Get("hash1")
	if ok {
		t.Error("should not find deleted key")
	}
}
