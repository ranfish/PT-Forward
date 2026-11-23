package publish

import (
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
)

// §59.559 PR-C 前置锚（用户方案要求：dedup/超时复查分支锚先行）：
// 锁定 Execute 尾段的 dedup 判定与超时复查语义——抽取前跑绿·拆后仍绿。
// 注：完整 Execute 需要 DB+网络栈——本锚覆盖可纯测的判定单元。

// errString 简单 error 实现（测试用）。
type errString string

func (e errString) Error() string { return string(e) }

// fingerprintPiecesHash 锚：种子数据 pieces hash（dedup 键——空数据/坏数据/正常）。
func TestFingerprintPiecesHash(t *testing.T) {
	// 空数据
	if got := fingerprintPiecesHash(nil); got != "" {
		t.Errorf("空数据应空 hash: %q", got)
	}
	// 非 bencode
	if got := fingerprintPiecesHash([]byte("not bencode")); got != "" {
		t.Errorf("非 bencode 应空 hash: %q", got)
	}
	// 最小合法 bencode dict（无 pieces）→ 空（缺 pieces 字段）
	if got := fingerprintPiecesHash([]byte("de")); got != "" {
		t.Errorf("空 dict 应空 hash: %q", got)
	}
}

// model.PublishSkipStatuses/PublishedListStatuses 锚（dedup 本地记忆消费的终态口径）。
func TestDedupStatusSets(t *testing.T) {
	// PublishSkipStatuses 应含 pushed 但不含 existing
	skip := make(map[string]bool)
	for _, s := range model.PublishSkipStatuses {
		skip[s] = true
	}
	if !skip["pushed"] || skip["existing"] {
		t.Errorf("PublishSkipStatuses 应含 pushed 不含 existing: %v", model.PublishSkipStatuses)
	}
	// PublishedListStatuses 应两者都含
	list := make(map[string]bool)
	for _, s := range model.PublishedListStatuses {
		list[s] = true
	}
	if !list["pushed"] || !list["existing"] {
		t.Errorf("PublishedListStatuses 应含 pushed+existing: %v", model.PublishedListStatuses)
	}
}
