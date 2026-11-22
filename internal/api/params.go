package api

import (
	"fmt"
	"strconv"
	"strings"
)

// §59.552 ID 解析契约（P1-05 窄版·用户定案）：
//   - path/query ID 统一 strconv.ParseUint(s, 10, 64) 转 uint（GORM 模型
//     ID 均 uint·64 位平台无损）
//   - 非法值返回 ErrAPIInvalidID（统一错误码·不暴露 strconv 原始错误串）
//   - 空/带空白先 TrimSpace；Sscanf 形态已废止（"42abc" 静默取 42 的宽契约
//     改为严格拒绝——消费面自家前端·零实际影响）
//   - 新增 handler 的 path/query ID 解析优先用本文件 helper；存量标准库
//     直调不强制批量改（P2 handler 样板治理时按模块顺手收敛）
//
// 收敛三个既有 helper：filter parseUint（Sscanf→严格）/seeding
// parseUintParam（path 尾段）/reseed parseMatchID——均委托 parseUintStrict。

// parseUintStrict 严格十进制 ID 解析（API 语义·错误统一 ErrAPIInvalidID）。
func parseUintStrict(s string) (uint, error) {
	trimmed := strings.TrimSpace(s)
	v, err := strconv.ParseUint(trimmed, 10, 64)
	if err != nil {
		return 0, apiError(ErrAPIInvalidID, fmt.Sprintf("invalid ID: %s", trimmed), nil)
	}
	return uint(v), nil
}
