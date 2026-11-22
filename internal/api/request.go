package api

import (
	"encoding/json"
	"net/http"
)

// decodeJSONBody §59.553: JSON 请求体解码单点——失败统一 40001「请求格式错误」。
// 收敛 108 处 json.NewDecoder 样板（五种文案漂移：请求格式错误/invalid body/
// invalid request body/参数解析失败/请求参数错误——§59.553 定案统一多数派文案）。
//
// 刻意例外六处（§59.553 定性清单·新增例外须在此登记）：
//   1. publish_torrents_handler.go DryRun 端点：40002+err 详情
//      （DryRun 400 案教训的刻意设计——解码失败曾被"必填"文案掩盖）
//   2. form_config_handler.go ×2：writeJSON 自定义响应结构+字段校验合并
//      ——改本 helper 会改变响应 shape（上游=响应风格统一议题·单独评审）
//   3. image_host_handler / download_handler ×2：body 可选·`_ =` 忽略
//      decode 错误（缺失是合法态·空值走默认逻辑）
//   4. site_handler 批量审核端点：`_ =` 忽略·坏 body 走后续 400
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
