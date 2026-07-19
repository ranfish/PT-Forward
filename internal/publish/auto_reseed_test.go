package publish

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/zap"
)

// mockDownloader 测试用 .torrent 下载器。
type mockDownloader struct {
	data []byte
	err  error
}

func (m *mockDownloader) DownloadTorrent(_ context.Context, _, _ string) ([]byte, error) {
	return m.data, m.err
}

// mockPusher 测试用推种器。
type mockPusher struct {
	err error
	called bool
}

func (m *mockPusher) Push(_ context.Context, _ string, _ []byte, _ string) error {
	m.called = true
	return m.err
}

func TestAutoReseed_Success(t *testing.T) {
	dl := &mockDownloader{data: []byte("torrent data")}
	pusher := &mockPusher{}

	result := AutoReseed(
		context.Background(), dl, pusher,
		"PTer", "123", "client-1", "/data/torrents",
		zap.NewNop(),
	)
	if !result.Seeded {
		t.Errorf("should be seeded, error: %s", result.Error)
	}
	if !pusher.called {
		t.Error("pusher should be called")
	}
}

func TestAutoReseed_DownloadFail(t *testing.T) {
	dl := &mockDownloader{err: errors.New("network error")}
	pusher := &mockPusher{}

	result := AutoReseed(
		context.Background(), dl, pusher,
		"PTer", "123", "client-1", "/data",
		zap.NewNop(),
	)
	if result.Seeded {
		t.Error("should not be seeded")
	}
	if result.Error == "" {
		t.Error("should have error message")
	}
	if pusher.called {
		t.Error("pusher should not be called on download fail")
	}
}

func TestAutoReseed_EmptyData(t *testing.T) {
	dl := &mockDownloader{data: []byte{}}
	pusher := &mockPusher{}

	result := AutoReseed(
		context.Background(), dl, pusher,
		"PTer", "123", "client-1", "/data",
		zap.NewNop(),
	)
	if result.Seeded {
		t.Error("should not be seeded for empty data")
	}
}

func TestAutoReseed_PushFail(t *testing.T) {
	dl := &mockDownloader{data: []byte("torrent")}
	pusher := &mockPusher{err: errors.New("push failed")}

	result := AutoReseed(
		context.Background(), dl, pusher,
		"PTer", "123", "client-1", "/data",
		zap.NewNop(),
	)
	if result.Seeded {
		t.Error("should not be seeded")
	}
	if !pusher.called {
		t.Error("pusher should be called")
	}
	if result.Error == "" {
		t.Error("should have error message")
	}
}

func TestAutoReseed_NilDownloader(t *testing.T) {
	pusher := &mockPusher{}
	result := AutoReseed(
		context.Background(), nil, pusher,
		"PTer", "123", "client-1", "/data",
		zap.NewNop(),
	)
	if result.Seeded {
		t.Error("should not be seeded with nil downloader")
	}
}

func TestAutoReseed_NilPusher(t *testing.T) {
	dl := &mockDownloader{data: []byte("torrent")}
	result := AutoReseed(
		context.Background(), dl, nil,
		"PTer", "123", "client-1", "/data",
		zap.NewNop(),
	)
	if result.Seeded {
		t.Error("should not be seeded with nil pusher")
	}
}

func TestMarkRecordSeeded_Success(t *testing.T) {
	seeded, seededAt, seedErr := MarkRecordSeeded(true, "")
	if !seeded {
		t.Error("should be true")
	}
	if seededAt == nil {
		t.Error("seededAt should not be nil")
	}
	if seedErr != "" {
		t.Errorf("seedErr should be empty, got %q", seedErr)
	}
}

func TestMarkRecordSeeded_Failure(t *testing.T) {
	seeded, seededAt, seedErr := MarkRecordSeeded(false, "push failed")
	if seeded {
		t.Error("should be false")
	}
	if seededAt != nil {
		t.Error("seededAt should be nil")
	}
	if seedErr != "push failed" {
		t.Errorf("seedErr mismatch, got %q", seedErr)
	}
}
