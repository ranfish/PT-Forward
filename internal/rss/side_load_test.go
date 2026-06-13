package rss

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/ranfish/pt-forward/internal/mocks"
	"github.com/ranfish/pt-forward/internal/model"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestBencodeCache_GetSet(t *testing.T) {
	cache := NewBencodeCache(10, time.Hour)

	_, ok := cache.Get("site1:123")
	require.False(t, ok, "empty cache should miss")

	cache.Set("site1:123", "abc123", 1024)
	entry, ok := cache.Get("site1:123")
	require.True(t, ok)
	require.Equal(t, "abc123", entry.InfoHash)
	require.Equal(t, int64(1024), entry.Size)
}

func TestBencodeCache_Expired(t *testing.T) {
	cache := NewBencodeCache(10, 10*time.Millisecond)
	cache.Set("site1:123", "abc123", 1024)

	time.Sleep(20 * time.Millisecond)
	_, ok := cache.Get("site1:123")
	require.False(t, ok, "expired entry should miss")
}

func TestBencodeCache_Eviction(t *testing.T) {
	cache := NewBencodeCache(2, time.Hour)
	cache.Set("site1:1", "hash1", 100)
	cache.Set("site1:2", "hash2", 200)
	cache.Set("site1:3", "hash3", 300)

	count := 0
	for i := 1; i <= 3; i++ {
		if _, ok := cache.Get("site1:" + string(rune('0'+i))); ok {
			count++
		}
	}
	require.LessOrEqual(t, count, 2, "cache should evict when full")
}

func TestSideLoadManager_Enqueue(t *testing.T) {
	emitter := NewSideLoadEventEmitter()
	mgr := NewSideLoadManager(nil, emitter, zap.NewNop())

	ev := &model.TorrentEvent{
		SiteName:       "unittest",
		TorrentID:      "12345",
		InfoHash:       "",
		SideLoadStatus: model.SideLoadPending,
	}
	err := mgr.Enqueue(ev, "unittest")
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	select {
	case task := <-mgr.pendingQueue:
		require.Equal(t, "unittest", task.SiteName)
		require.Equal(t, "12345", task.TorrentEvent.TorrentID)
	case <-ctx.Done():
		t.Fatal("timed out waiting for task")
	}
}

func TestSideLoadManager_Enqueue_QueueFull(t *testing.T) {
	emitter := NewSideLoadEventEmitter()
	mgr := NewSideLoadManager(nil, emitter, zap.NewNop())
	mgr.pendingQueue = make(chan *SideLoadTask, 1)

	ev1 := &model.TorrentEvent{SiteName: "s", TorrentID: "1"}
	ev2 := &model.TorrentEvent{SiteName: "s", TorrentID: "2"}

	require.NoError(t, mgr.Enqueue(ev1, "s"))
	require.Error(t, mgr.Enqueue(ev2, "s"), "queue full should error")
}

func TestSideLoadManager_Worker_ProcessSuccess(t *testing.T) {
	adapter := &mocks.SiteAdapter{
		DownloadTorrentFn: func(ctx context.Context, config *model.SiteConfig, torrentID string) ([]byte, error) {
			return createTestTorrent(t), nil
		},
	}

	provider := &mocks.SiteInfoProvider{
		GetSiteConfigFn: func(ctx context.Context, domain string) (*model.SiteConfig, error) {
			return &model.SiteConfig{
				Domain:  "testsite",
				Passkey: "pk123",
				Cookie:  "cookie123",
			}, nil
		},
		GetAdapterFn: func(ctx context.Context, domain string) (model.SiteAdapter, error) {
			return adapter, nil
		},
	}

	emitter := NewSideLoadEventEmitter()
	eventCh := emitter.Subscribe()
	mgr := NewSideLoadManager(provider, emitter, zap.NewNop())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mgr.Start(ctx)

	ev := &model.TorrentEvent{
		SiteName:       "testsite",
		TorrentID:      "99999",
		InfoHash:       "",
		SideLoadStatus: model.SideLoadPending,
	}

	require.NoError(t, mgr.Enqueue(ev, "testsite"))

	waitCtx, waitCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer waitCancel()

	var evt SideLoadEvent
	for {
		select {
		case e := <-eventCh:
			if e.Status == model.SideLoadCompleted || e.Status == model.SideLoadFailed {
				evt = e
				goto gotFinal
			}
		case <-waitCtx.Done():
			t.Fatal("timed out waiting for side load event")
		}
	}
gotFinal:
	require.Equal(t, model.SideLoadCompleted, evt.Status)
	require.Equal(t, "99999", evt.TorrentID)
	require.NotEmpty(t, evt.TorrentEvent.InfoHash, "info hash should be populated")
	require.Equal(t, model.SideLoadCompleted, evt.TorrentEvent.SideLoadStatus)
}

func TestSideLoadManager_Worker_CacheHit(t *testing.T) {
	downloadCalled := false
	adapter := &mocks.SiteAdapter{
		DownloadTorrentFn: func(ctx context.Context, config *model.SiteConfig, torrentID string) ([]byte, error) {
			downloadCalled = true
			return createTestTorrent(t), nil
		},
	}

	provider := &mocks.SiteInfoProvider{
		GetSiteConfigFn: func(ctx context.Context, domain string) (*model.SiteConfig, error) {
			return &model.SiteConfig{Domain: "testsite"}, nil
		},
		GetAdapterFn: func(ctx context.Context, domain string) (model.SiteAdapter, error) {
			return adapter, nil
		},
	}

	emitter := NewSideLoadEventEmitter()
	mgr := NewSideLoadManager(provider, emitter, zap.NewNop())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mgr.Start(ctx)

	cacheKey := "testsite:88888"
	mgr.bencodeCache.Set(cacheKey, "cached_hash_1234567890123456789012345678", 2048)

	ev := &model.TorrentEvent{
		SiteName:       "testsite",
		TorrentID:      "88888",
		InfoHash:       "",
		SideLoadStatus: model.SideLoadPending,
	}

	eventCh := emitter.Subscribe()
	require.NoError(t, mgr.Enqueue(ev, "testsite"))

	waitCtx2, waitCancel2 := context.WithTimeout(context.Background(), 3*time.Second)
	defer waitCancel2()

	var evt SideLoadEvent
	for {
		select {
		case e := <-eventCh:
			if e.Status == model.SideLoadCompleted || e.Status == model.SideLoadFailed {
				evt = e
				goto gotFinal2
			}
		case <-waitCtx2.Done():
			t.Fatal("timed out")
		}
	}
gotFinal2:
	require.Equal(t, model.SideLoadCompleted, evt.Status)
	require.False(t, downloadCalled, "should use cache, not download")
	require.Equal(t, "cached_hash_1234567890123456789012345678", evt.TorrentEvent.InfoHash)
}

func createTestTorrent(t *testing.T) []byte {
	t.Helper()
	pieces := make([]byte, 20)
	info := map[string]any{
		"name":         "test.txt",
		"length":       int64(1024),
		"piece length": int64(16384),
		"pieces":       string(pieces),
	}
	encoded, err := encodeBencode(map[string]any{
		"info":     info,
		"announce": "http://tracker.example.com/announce",
	})
	require.NoError(t, err, "encode test torrent")
	return encoded
}

func encodeBencode(val any) ([]byte, error) {
	var buf bytes.Buffer
	if err := writeBencode(&buf, val); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func writeBencode(buf *bytes.Buffer, val any) error {
	switch v := val.(type) {
	case map[string]any:
		buf.WriteString("d")
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(buf, "%d:%s", len(k), k)
			if err := writeBencode(buf, v[k]); err != nil {
				return err
			}
		}
		buf.WriteString("e")
	case int64:
		fmt.Fprintf(buf, "i%de", v)
	case string:
		fmt.Fprintf(buf, "%d:%s", len(v), v)
	case []byte:
		fmt.Fprintf(buf, "%d:", len(v))
		buf.Write(v)
	default:
		return fmt.Errorf("unsupported type: %T", val)
	}
	return nil
}
