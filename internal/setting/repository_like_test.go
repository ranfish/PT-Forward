package setting

import (
	"context"
	"testing"
)

// 13 补刀（v0.0.1657）: ListByPrefix 前缀过滤通配符劫持回归锚——
// prefix 取自 URL 查询参数（settings_handler:95）·用户输入 % 时原
// 裸拼 `key LIKE ?, prefix+"%"` 命中全表（用户复审探针实证 3 行·
// 前缀语义应 0 行）。修复后走 util.LikeEscape 前缀锚定+ESCAPE。
//
// 阴性对照（08 篇方法论）：repository.go 退回裸拼（删 LikeEscape/
// LikeEscapeSuffix）→ 锚1（% 输入全命中）FAIL——可触发验证。
func TestRepository_ListByPrefix_WildcardLiteral(t *testing.T) {
	repo := NewRepository(setupTestDB(t))
	ctx := context.Background()

	// 伪数据：模拟敏感键形态（真实部署 setting 键命名域）
	keys := []string{
		"httpProxy",
		"httpProxyHost",
		"otaUseProxy",
		"websocketEnabled",
		"abc.lifecycle.x",
	}
	for _, k := range keys {
		if err := repo.Set(ctx, k, "v"); err != nil {
			t.Fatal(err)
		}
	}

	// 锚1：字面 % 输入——前缀语义应 0 行（无键以字面 % 开头）
	got, err := repo.ListByPrefix(ctx, "%")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("输入 %% 前缀应命中 0 行（前缀语义）·实得 %d 行——通配符劫持（裸拼回归）", len(got))
	}

	// 锚2：字面 _ 输入——前缀语义应 0 行
	got, err = repo.ListByPrefix(ctx, "_")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("输入 _ 前缀应命中 0 行·实得 %d 行", len(got))
	}

	// 锚3：正常前缀语义保留（转义不破坏原行为）
	got, err = repo.ListByPrefix(ctx, "httpProxy")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got["httpProxy"] != "v" || got["httpProxyHost"] != "v" {
		t.Errorf("前缀 httpProxy 应命中 2 行（httpProxy/httpProxyHost）·实得 %v", got)
	}

	// 锚4：空 prefix=全量（既有语义不变）
	got, err = repo.ListByPrefix(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(keys) {
		t.Errorf("空 prefix 应全量 %d 行·实得 %d 行", len(keys), len(got))
	}
}
