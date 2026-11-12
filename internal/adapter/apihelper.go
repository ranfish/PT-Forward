package adapter

import (
	"fmt"
	"io"
	"net/http"
)

// apihelper.go API 网关站共性机制层（2026-11-12 好大架构定案·hdarea-api.md §10）。
// 边界铁律：**共享机制·不共享方言**——此处只放与站点方言无关的机制
// （X-API-Key 注入/短效凭证下载流程骨架）；字段映射/字典/端点常量
// 全留各 adapter 方言层。包内私有（消费者仅 adapter 包内·三次法则
// 后再升 util——出现包外消费者或第三站证明流程骨架稳定时）。

// setXAPIKey X-API-Key 头注入（M-Team 兼容传递方式·大小写不敏感）。
// 可乐的 mtorrentCore.setAPIHeaders 将来可顺路迁入收敛（非本次必做）。
func setXAPIKey(req *http.Request, apiKey string) {
	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	}
}

// shortTokenDownload 短效凭证下载流程骨架（参数化——好大 genDlToken 与
// mt 同语义但请求/响应形态各方言自定）：
//   tokenReq  签发请求（调用方构造——方言层定端点/方法/body）
//   parseURL  从签发响应体解析出带签名的下载直链（方言层定字段）
// 流程：签发 → GET 直链（不带 API Key——URL 自含凭证）→ 403 过期重签
// 一次（1800s 短效凭证的标准失效路径·站方文档 §9）。
func shortTokenDownload(client *http.Client, tokenReq *http.Request, parseURL func([]byte) (string, error)) ([]byte, error) {
	for attempt := 0; attempt < 2; attempt++ {
		resp, err := client.Do(tokenReq)
		if err != nil {
			return nil, networkError("短效凭证签发请求失败: "+err.Error(), err)
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, httpError(fmt.Sprintf("短效凭证签发 HTTP %d", resp.StatusCode), nil)
		}
		dlURL, err := parseURL(body)
		if err != nil || dlURL == "" {
			return nil, parseError("短效凭证响应解析失败", err)
		}
		getReq, err := http.NewRequest("GET", dlURL, nil)
		if err != nil {
			return nil, networkError("构造下载请求失败", err)
		}
		getReq.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Safari/537.36")
		dlResp, err := client.Do(getReq)
		if err != nil {
			return nil, networkError("短效下载请求失败: "+err.Error(), err)
		}
		data, _ := io.ReadAll(io.LimitReader(dlResp.Body, 256<<20))
		dlResp.Body.Close()
		if dlResp.StatusCode == http.StatusOK {
			return data, nil
		}
		if dlResp.StatusCode == http.StatusForbidden && attempt == 0 {
			// 403=短效凭证过期（站方 FAQ）——重签一次
			continue
		}
		return nil, httpError(fmt.Sprintf("短效下载 HTTP %d", dlResp.StatusCode), nil)
	}
	return nil, httpError("短效下载重签后仍失败", nil)
}
