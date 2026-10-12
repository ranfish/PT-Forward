package pusher

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
)

// §59.321 测试锚：复检失败（网络错）≠不免费——Retryable=true；
// 确认非免费——Retryable=false 终态跳过。

type recheckSiteProvider struct {
	adapter model.SiteAdapter
	cfg     *model.SiteConfig
}

func (p *recheckSiteProvider) GetSiteConfig(_ context.Context, _ string) (*model.SiteConfig, error) {
	return p.cfg, nil
}
func (p *recheckSiteProvider) GetAdapter(_ context.Context, _ string) (model.SiteAdapter, error) {
	return p.adapter, nil
}

type recheckAdapter struct {
	model.SiteAdapter // 嵌入接口——测试路径只走 DetectDiscount 即 return
	discResult        *model.DiscountResult
	discErr           error
}

func (a *recheckAdapter) DetectDiscount(_ context.Context, _ *model.SiteConfig, _ string) (*model.DiscountResult, error) {
	return a.discResult, a.discErr
}

type okClientProvider struct{}

func (okClientProvider) Get(uint) (model.DownloaderClient, error) { return nil, nil }

func newRecheckPusher(adapter model.SiteAdapter) *Pusher {
	p := NewPusher(nil, zap.NewNop())
	p.SetSiteProvider(&recheckSiteProvider{adapter: adapter, cfg: &model.SiteConfig{}})
	p.SetClientProvider(okClientProvider{})
	return p
}

func TestPush_RecheckFailed_Retryable(t *testing.T) {
	p := newRecheckPusher(&recheckAdapter{discErr: errors.New("context deadline exceeded")})
	result := p.Push(context.Background(), &PushRequest{
		ClientUID: 1,
		SiteName:  "s",
		InfoHash:  "h1",
		IsFree:    true, // FreeEndAt=nil 触发复检
	})
	if result.Success {
		t.Fatal("不应成功")
	}
	if result.SkipReason != "discount recheck failed" {
		t.Fatalf("SkipReason 不符: %s", result.SkipReason)
	}
	if !result.Retryable {
		t.Fatal("网络复检失败必须 Retryable=true（§59.321）")
	}
}

func TestPush_RecheckNotFree_Terminal(t *testing.T) {
	p := newRecheckPusher(&recheckAdapter{discResult: &model.DiscountResult{Level: model.DiscountNone}})
	result := p.Push(context.Background(), &PushRequest{
		ClientUID: 1,
		SiteName:  "s",
		InfoHash:  "h1",
		IsFree:    true,
	})
	if result.Success {
		t.Fatal("不应成功")
	}
	if result.SkipReason != "free status expired before push" {
		t.Fatalf("SkipReason 不符: %s", result.SkipReason)
	}
	if result.Retryable {
		t.Fatal("确认非免费是终态，Retryable 必须 false")
	}
}

func TestPush_FreeEndAtExpired_Terminal(t *testing.T) {
	p := newRecheckPusher(&recheckAdapter{})
	past := time.Now().Add(-time.Hour)
	result := p.Push(context.Background(), &PushRequest{
		ClientUID: 1,
		SiteName:  "s",
		InfoHash:  "h1",
		IsFree:    true,
		FreeEndAt: &past,
	})
	if result.SkipReason != "free period expired" {
		t.Fatalf("SkipReason 不符: %s", result.SkipReason)
	}
	if result.Retryable {
		t.Fatal("免费期已过是终态")
	}
}
