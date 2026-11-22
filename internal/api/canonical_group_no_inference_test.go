package api

import (
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// §59.550 锚: 官组身份零推导——前缀回退退役后 miss 即 miss（诚实原词）·
// 变体必须映射表显式收编（开发者决定）。CMCTf 形态历史靠回退推导→CMCT·
// 退役后原词返回不推导。DSWeb 先例=深度官组显式收编（migration181）。
func TestCanonicalGroupNoPrefixInference(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.ReleaseGroupMapping{}); err != nil {
		t.Fatal(err)
	}
	h := &PublishTorrentsHandler{db: db, logger: zap.NewNop()}

	// ① miss 变体（前缀关系存在）→ 原词返回·不推导为词条
	db.Create(&model.ReleaseGroupMapping{GroupName: "CMCT", Domain: "springsunday.net", IsOfficial: true})
	if got := h.canonicalGroupName(t.Context(), "CMCTf"); got != "CMCTf" {
		t.Fatalf("① 前缀变体应原词返回（零推导）·got %q", got)
	}
	// ② 精确命中（含大小写形态——LOWER 归一·§59.550 实态确认生产链已有）
	db.Create(&model.ReleaseGroupMapping{GroupName: "PigoWeb", Domain: "piggo.me", IsOfficial: true})
	if got := h.canonicalGroupName(t.Context(), "PigoWEB"); got != "PigoWeb" {
		t.Fatalf("② 大小写形态应 LOWER 命中·got %q", got)
	}
	// ③ 显式收编的变体（DSWeb 形态）→ 精确命中
	db.Create(&model.ReleaseGroupMapping{GroupName: "DSWeb", Domain: "dstudio.me", IsOfficial: true})
	if got := h.canonicalGroupName(t.Context(), "DSWeb"); got != "DSWeb" {
		t.Fatalf("③ 显式收编变体应命中·got %q", got)
	}
}
