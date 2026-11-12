package pusher

import (
	"context"
	"errors"
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
)

// §59.515 刀4 D2 测试锚：TorrentData 非空=跳过 DownloadTorrent 直用；
// 空=原行为走下载（其他站/DryRun/超时复查路径零回归）。
// 注：okClientProvider 返回 nil client——直用路径会走到 AddTorrent 对
// nil client panic（下载段之后）；recover 包裹保护·断言点在 panic 前
// 已完成（DownloadTorrent 调用计数）。

type dlCountAdapter struct {
	model.SiteAdapter
	calls int
	fail  bool
}

func (a *dlCountAdapter) DownloadTorrent(_ context.Context, _ *model.SiteConfig, _ string) ([]byte, error) {
	a.calls++
	if a.fail {
		return nil, errors.New("download unavailable")
	}
	return []byte("downloaded-torrent"), nil
}

// TestPush_TorrentDataDirect 非空直用——DownloadTorrent 零调用。
func TestPush_TorrentDataDirect(t *testing.T) {
	adapter := &dlCountAdapter{}
	p := newRecheckPusher(adapter)
	req := &PushRequest{
		ClientUID:   1,
		SiteName:    "好大",
		TorrentID:   "123",
		TorrentData: []byte("site-returned-final-torrent"),
	}
	func() {
		defer func() { _ = recover() }() // nil client AddTorrent panic——断言语义已达成
		p.Push(context.Background(), req)
	}()
	if adapter.calls != 0 {
		t.Errorf("DownloadTorrent 调用=%d·TorrentData 非空应为 0（直用回传数据）", adapter.calls)
	}
}

// TestPush_TorrentDataEmptyFallback 空字段——原行为走下载重试链
// （fail 形态：4 轮退避后返回错误·验证完整原路径未被 D2 改动）。
func TestPush_TorrentDataEmptyFallback(t *testing.T) {
	adapter := &dlCountAdapter{fail: true}
	p := newRecheckPusher(adapter)
	req := &PushRequest{
		ClientUID: 1,
		SiteName:  "其他站",
		TorrentID: "456",
	}
	result := p.Push(context.Background(), req)
	if adapter.calls != 4 {
		t.Errorf("DownloadTorrent 调用=%d·空字段应走 4 轮退避原路径（want 4）", adapter.calls)
	}
	if result == nil || result.Error == nil {
		t.Error("fail 形态应返回下载错误")
	}
}
