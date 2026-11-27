package api

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 13 刀3（v0.0.1656）: LIKE 转义静态锚——源码扫描「用户输入进入 LIKE
// 模式」的接线纪律：所有含 LIKE ? 的 SQL 字符串必须带 ESCAPE 子句
// （util.LikeEscapeSuffix），或属于字面量模式白名单（模式为内部常量·
// 不含用户输入·无需转义）。
//
// 白名单登记制（09 篇例外登记先例）：新增用户输入 LIKE 必须走
// util.LikeEscape+LikeEscapeSuffix；新增字面量模式 LIKE 在此登记
// file+fragment 并注释「字面量模式」理由。
//
// 阴性对照：删任一处 ESCAPE 子句（如 seeddata 的 LikeEscapeSuffix）
// → 本锚 FAIL（可触发验证）。
type likeWhitelistEntry struct {
	file     string
	fragment string
	reason   string
}

var likeLiteralWhitelist = []likeWhitelistEntry{
	{"callbacks.go", "NOT LIKE ?", "字面量模式：密文前缀探针（ciphertextPrefix+%·内部常量）"},
	{"no_transfer_recheck.go", "flags LIKE ?", "字面量模式：禁转词表 JSON 探针（8 固定串）"},
	{"fix_stale_dolby.go", "tags LIKE ?", "字面量模式：dolby_atmos/dolby_vision JSON 探针"},
	{"migration62_subtitle_shell_cleanup.go", "subtitle LIKE ?", "字面量模式：一次性迁移固定模式"},
	{"migration.go", "subtitle LIKE ?", "字面量模式：方括号残留清理固定模式"},
	{"migration.go", "save_path LIKE ?", "字面量模式：尾斜杠归一迁移固定模式（%/）"},
	{"migration.go", "flags LIKE ?", "字面量模式：禁转词重刷迁移固定模式（6 词）"},
	{"seed_list.go", "detail_source_json LIKE ?", "字面量模式：release_group JSON 探针"},
	{"declaration_backfill_task.go", "release_group", "字面量模式：JSON 探针（转义分支已带 ESCAPE）"},
	{"lifecycle.go", "key LIKE ?", "字面量模式：lifecycle.% 前缀探针（定义处即消费处·单调用方自洽）"},
	{"engine_store.go", "client_ids LIKE ?", "内部 ID 拼接：clientID 源自 uint 转字符串·无通配符劫持面"},
	// 白名单登记纪律（13 补刀 v0.0.1657 教训）：登记前必须 grep 该
	// 函数全部调用方并核验输入源——repository.ListByPrefix 原登记
	// 「调用方传常量 key 前缀」仅凭定义行猜测·漏查 settings_handler:95
	// 用户输入直达调用方·给通配符劫持开绿灯（用户复审探针实证
	// prefix=% 命中 3 行）。新登记条目理由必须附调用方清单。
}

func isLikeWhitelisted(file, line string) bool {
	base := filepath.Base(file)
	for _, w := range likeLiteralWhitelist {
		if base == w.file && strings.Contains(line, w.fragment) {
			return true
		}
	}
	return false
}

func TestNoBareLikeEscapePatterns(t *testing.T) {
	root := filepath.Join("..", "..", "internal")
	var violations []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for i, line := range strings.Split(string(data), "\n") {
			// 行注释截断（注释里的 LIKE ? 非代码——seed_list 注释误报）
			if idx := strings.Index(line, "//"); idx >= 0 {
				line = line[:idx]
			}
			if !strings.Contains(line, "LIKE ?") {
				continue
			}
			// 09 篇工程坑：Walk root 自身 SkipDir（internal/ 目录本身非 .go 已被 HasSuffix 过滤）
			// ESCAPE 判定含两形态：字面子句 ESCAPE 与 util.LikeEscapeSuffix 标识符
			// （常量值即 " ESCAPE '\\'"——源码以标识符引用）。
			if strings.Contains(line, "ESCAPE") || strings.Contains(line, "LikeEscapeSuffix") || isLikeWhitelisted(path, line) {
				continue
			}
			rel, _ := filepath.Rel(root, path)
			violations = append(violations, rel+":"+itoa(i+1)+" "+strings.TrimSpace(line))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if len(violations) > 0 {
		t.Errorf("发现未带 ESCAPE 且未登记白名单的 LIKE ?（用户输入须 util.LikeEscape+LikeEscapeSuffix·字面量模式须白名单登记）:\n%s",
			strings.Join(violations, "\n"))
	}
}

func itoa(n int) string {
	if n < 10 {
		return string(rune('0' + n))
	}
	return itoa(n/10) + string(rune('0'+n%10))
}
