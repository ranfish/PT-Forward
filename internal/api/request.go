package api

import (
	"encoding/json"
	"net/http"
)

// decodeJSONBody §59.553: JSON 请求体解码单点——失败统一 40001「请求格式错误」。
// 收敛 108 处 json.NewDecoder 样板（五种文案漂移：请求格式错误/invalid body/
// invalid request body/参数解析失败/请求参数错误——§59.553 定案统一多数派文案）。
// 例外保留：publish_torrents DryRun 端点 40002+err 详情（DryRun 400 案教训的
// 刻意设计——解码失败曾被"必填"文案掩盖·注释在案）。
func decodeJSONBody(w http.ResponseWriter, r *http.Request, dst any) bool {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		Error(w, http.StatusBadRequest, 40001, "请求格式错误")
		return false
	}
	return true
}

// methodNotAllowed §59.553: 405 响应单点——收敛 128 处手写三件套
// （StatusMethodNotAllowed+40001+"方法不允许"）。
func methodNotAllowed(w http.ResponseWriter) {
	Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
}
