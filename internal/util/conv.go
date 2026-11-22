package util

import (
	"strconv"
	"strings"
)

// ParseUint §59.552: 通用十进制无符号解析（TrimSpace·平台位宽·溢出报错）。
// 收敛手写逐字符版（pusher/syncer）——手写版溢出静默回绕是 bug 面
// （99999999999999999999 → 静默错值；本实现返回 strconv ErrRange）。
// API 语义的错误包装不放这里（见 internal/api/params.go 契约）。
func ParseUint(s string) (uint, error) {
	v, err := strconv.ParseUint(strings.TrimSpace(s), 10, strconv.IntSize)
	return uint(v), err
}
