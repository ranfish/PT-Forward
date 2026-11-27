package auth

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"
)

// 18 刀2（v0.0.1671）: persistWg 配对锚（V-02 防回归）——Stop() 必须
// 等待最后一次 persistRefreshTokens 落库完成（原零 Add 空等·落库脱离
// 停机生命周期）。用阻塞 SettingStore 实证：Stop 在 Set 阻塞期间
// 不得返回。
//
// 阴性对照：删 Add(1) → 本锚 FAIL（Stop 提前返回）。
type blockStore18 struct {
	blockCh   chan struct{}
	setCalled chan struct{}
}

func (s *blockStore18) Get(ctx context.Context, key string) (string, error) {
	return "", nil
}
func (s *blockStore18) Set(ctx context.Context, key, value string) error {
	// 18 刀2 锚：jwt_signing_key（构造期写入）立即返回——只阻塞
	// refresh_tokens（persistLoop 落库）——NewAuthManagerWithSettings
	// 构造即写 jwt key（V-02 场景在 refresh 落库不在构造）。
	if key == "refresh_tokens" {
		select {
		case s.setCalled <- struct{}{}:
		default:
		}
		<-s.blockCh
	}
	return nil
}

func TestStopWaitsForPersist(t *testing.T) {
	store := &blockStore18{
		blockCh:   make(chan struct{}),
		setCalled: make(chan struct{}, 1),
	}
	mgr, err := NewAuthManagerWithSettings(nil, store, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}

	// 触发一次 persist（persistCh 路径）
	select {
	case mgr.persistCh <- struct{}{}:
	default:
	}

	// 等 persistLoop 进入 Set（阻塞）
	select {
	case <-store.setCalled:
	case <-time.After(2 * time.Second):
		t.Fatal("persistLoop 未进入 Set")
	}

	// Stop() 必须 Set 解除后才返回（persistWg.Wait 真等待）
	stopDone := make(chan struct{})
	go func() {
		mgr.Stop()
		close(stopDone)
	}()
	select {
	case <-stopDone:
		t.Fatal("V-02 回归：Stop 在 Set 阻塞期间返回（persistWg 空等——Add(1) 被删）")
	case <-time.After(300 * time.Millisecond):
		// ✓ Stop 仍在等待
	}

	// 解除阻塞——Stop 应随即返回
	close(store.blockCh)
	select {
	case <-stopDone:
		// ✓ 落库完成后 Stop 返回
	case <-time.After(2 * time.Second):
		t.Fatal("Set 解除后 Stop 未返回")
	}
}
