package api

// §59.559 PR-D: query 参数窄契约 helper（报告 §3.4——只覆盖明确场景·
// 不做全量机械替换·不改变默认值/上限/错误文案）。
// parsePagination/parseUintStrict 已有（§59.552）——本文件补三件：
// parseBoolQuery / parseUintQuery / parseIntQuery。

import (
	"net/http"
	"strconv"
)

// parseBoolQuery 布尔 query 解析——宽松口径："1" 或 "true" 为真。
// ⚠ 这是有意放宽的契约（§59.559 PR-D 备注定案 2026-11-23）：
// 部分旧调用点（如 exclude_forbidden）原本仅接受 "true"——收敛到本
// helper 后 "1" 也返回 true。并非逐字节零行为变更，实际风险很低
// （"1" 在旧写法里=缺省/假值之外的意外真值形态·现按宽松口径归一）。
// 后续维护者勿据此推断该调用点历史上接受 "1"。
// 现状双形态并存收敛：reseed video_only 用 "1"||"true"·client light
// 用 "true"。缺省返回 def。
func parseBoolQuery(r *http.Request, key string, def bool) bool {
	v := r.URL.Query().Get(key)
	switch v {
	case "1", "true":
		return true
	case "0", "false", "":
		if v == "" {
			return def
		}
		return false
	default:
		return def
	}
}

// parseUintQuery 无符号整型 query 解析——缺省/非法返回 def
// （err 静默·现状 client_id 等场景的惯用形态）。
func parseUintQuery(r *http.Request, key string, def uint) uint {
	v := r.URL.Query().Get(key)
	if v == "" {
		return def
	}
	n, err := parseUintStrict(v)
	if err != nil {
		return def
	}
	return n
}

// parseIntQuery 整型 query 解析——缺省/非法返回 def
// （limit 等场景的惯用形态）。
func parseIntQuery(r *http.Request, key string, def int) int {
	v := r.URL.Query().Get(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}
