// Package adapter — mtorrentCore §59.460 附五（刀1）:
// mTorrent 协议核心——从 MTeamAdapter 抽取的协议层（M-Team 兼容面契约
// 的请求构造/解析，服务 mTeamAdapter 与 colaAdapter 两站组合复用；
// 馒头特有资产（分类树/角色表/RSS API feed）留 MTeamAdapter，可乐特有
// 行为（Web 面详情/promotion/peer 简名端点）归 colaAdapter——三列分层
// 见 docs/31 §59.460 附五）。
package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/ranfish/pt-forward/internal/httpclient"
	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
)

// mtorrentCore 持有协议层共享依赖（doer/logger 经 embed 提升给宿主 adapter）。
type mtorrentCore struct {
	doer   *HTTPDoer
	logger *zap.Logger
}

func (c *mtorrentCore) searchViaAPI(ctx context.Context, config *model.SiteConfig, keyword string, modes []string) ([]*model.SeedingSearchResult, error) {
	u := resolveBaseURL(config) + "/api/torrent/search"

	var all []*model.SeedingSearchResult
	for _, mode := range modes {
		payload, _ := json.Marshal(map[string]interface{}{
			"keyword":    keyword,
			"mode":       mode,
			"pageNumber": 1,
			"pageSize":   20,
		})

		req, err := http.NewRequestWithContext(ctx, "POST", u, bytes.NewReader(payload))
		if err != nil {
			return nil, searchError("构造搜索请求失败", err)
		}
		req.Header.Set("Content-Type", "application/json")
		c.setAPIHeaders(req, config.APIKey)

		resp, err := c.doer.Client.Do(req)
		if err != nil {
			if mode == "normal" {
				return nil, searchError("搜索请求失败", err)
			}
			continue // adult 轮失败不阻断——降级 normal-only
		}

		body, readErr := readBody(resp)
		drainBody(resp)
		if readErr != nil && mode == "normal" {
			return nil, readErr
		}
		if readErr != nil {
			continue
		}

		var result struct {
			Code json.Number `json:"code"`
			Data struct {
				Data []struct {
					// §59.460: json.Number 双兼容（M-Team 官方字符串 id /
					// 可乐 reelcore 数字 id——纯 string 在数字形态下 unmarshal 失败）
					ID     json.Number `json:"id"`
					Name   string      `json:"name"`
					Size   flexInt     `json:"size"`
					Status struct {
						Seeders  flexInt `json:"seeders"`
						Leechers flexInt `json:"leechers"`
						Discount string  `json:"discount"`
					} `json:"status"`
				} `json:"data"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &result); err != nil {
			if mode == "normal" {
				return nil, parseError("解析搜索结果失败", err)
			}
			continue
		}

		// §59.224: API 业务错误码（code≠0）——返回错误进入退避重试链
		// （savr 六连案：并发批量期 code=4 瞬时限流，单独重试即恢复——
		//  此前返回空结果=合法空页，§59.219 空页重试偶尔能兜住但不可靠）。
		// message 提取供日志+错误消息。
		if result.Code.String() != "0" {
			var errBody struct {
				Message string `json:"message"`
			}
			_ = json.Unmarshal(body, &errBody)
			if c.logger != nil {
				c.logger.Warn("mteam search API error",
					zap.String("mode", mode),
					zap.String("keyword", keyword),
					zap.String("code", result.Code.String()),
					zap.String("message", errBody.Message))
			}
			if mode == "normal" {
				return nil, &model.AppError{Code: 15002, Message: fmt.Sprintf("MTeam API error code=%s: %s", result.Code.String(), errBody.Message)}
			}
			// adult 轮 code≠0：记日志不阻断（降级 normal-only 结果）
			continue
		}
		if c.logger != nil {
			c.logger.Debug("mteam search resp",
				zap.String("mode", mode),
				zap.String("keyword", keyword),
				zap.Int("body_bytes", len(body)),
				zap.Int("parsed", len(result.Data.Data)))
		}

		for _, item := range result.Data.Data {
			r := &model.SeedingSearchResult{
				TorrentID: item.ID.String(),
				Title:     item.Name,
				Size:      int64(item.Size),
				Seeders:   int(item.Status.Seeders),
				Leechers:  int(item.Status.Leechers),
			}
			if mode == "adult" {
				r.Adult = true
			}
			all = append(all, r)
		}
	}
	return all, nil
}


func (c *mtorrentCore) setAPIHeaders(req *http.Request, apiKey string) {
	req.Header.Set("User-Agent", httpclient.BrowserUserAgent())
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("Accept", "application/json")
}


func (c *mtorrentCore) downloadViaAPI(ctx context.Context, config *model.SiteConfig, torrentID string) ([]byte, error) {
	tokenURL := resolveBaseURL(config) + "/api/torrent/genDlToken"
	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader("id="+torrentID))
	if err != nil {
		return nil, networkError("构造 genDlToken 请求失败", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	c.setAPIHeaders(req, config.APIKey)

	resp, err := c.doer.Client.Do(req)
	if err != nil {
		return nil, networkError("genDlToken 请求失败", err)
	}
	defer func() { drainBody(resp) }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, notFoundError("种子不存在或已被删除")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, httpError(fmtES("genDlToken HTTP %d", resp.StatusCode), nil)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 50*1024*1024))
	if err != nil {
		return nil, networkError("读取 genDlToken 响应失败", err)
	}
	var tokenResp struct {
		Data string `json:"data"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, parseError("解析 genDlToken 响应失败", err)
	}
	if tokenResp.Data == "" {
		return nil, downloadError("genDlToken 返回空下载链接", nil)
	}

	dlReq, err := http.NewRequestWithContext(ctx, "GET", tokenResp.Data, nil)
	if err != nil {
		return nil, networkError("构造下载请求失败", err)
	}
	// §59.460: 下载 URL 通道判定——按 URL 前缀（非签发来源）：
	// ==resolveBaseURL（API 兼容面同域，M-Team 形态）带 API 头；
	// 其它域（可乐 www /downloads Web 通道）无头——带 x-api-key 到非
	// 兼容面路径触发 scope 校验（实测两种惩罚形态：403 api_key_scope_
	// denied 响应 / CF 直接掐连接挂到超时），签名 URL 自含 exp+sig 鉴权，
	// 无头裸 GET 即 200 真种子（站方 MoviePilot 对接文档同款：下载换票
	// 之后的 GET 不带任何头）。
	if strings.HasPrefix(tokenResp.Data, resolveBaseURL(config)+"/") {
		c.setAPIHeaders(dlReq, config.APIKey)
	}

	dlResp, err := c.doer.Client.Do(dlReq)
	if err != nil {
		return nil, downloadError("下载种子失败", err)
	}

	// §59.460: 兜底——万一 API 同域下载带头仍 403（scope 收紧等），
	// 无头重试一次（签名 URL 自含鉴权；馒头常态带头 200 不触发）。
	if dlResp.StatusCode == http.StatusForbidden {
		drainBody(dlResp)
		bareReq, bareErr := http.NewRequestWithContext(ctx, "GET", tokenResp.Data, nil)
		if bareErr != nil {
			return nil, networkError("构造无头下载请求失败", bareErr)
		}
		dlResp, err = c.doer.Client.Do(bareReq)
		if err != nil {
			return nil, downloadError("下载种子失败", err)
		}
	}
	defer func() { drainBody(dlResp) }()

	if dlResp.StatusCode == http.StatusNotFound {
		return nil, notFoundError("种子不存在或已被删除")
	}
	if dlResp.StatusCode != http.StatusOK {
		return nil, httpError(fmtES("下载种子 HTTP %d", dlResp.StatusCode), nil)
	}

	data, err := io.ReadAll(io.LimitReader(dlResp.Body, 50*1024*1024))
	if err != nil {
		return nil, downloadError("读取种子数据失败", err)
	}
	if len(data) == 0 {
		return nil, &model.AppError{Code: 15001, Message: "种子数据为空"}
	}
	return data, nil
}


// mtorrentProfile §59.460 附五: profile 端点原始数据（core 只解析原始
// 字段；Role→等级名的映射是站特有资产由宿主 adapter 完成）。
type mtorrentProfile struct {
	Username      string
	Role          int
	UploadBytes   int64
	DownloadBytes int64
	Bonus         float64
	Ratio         float64
}

// fetchProfile POST /api/member/profile——json.Number 双兼容解析
// （§59.460：M-Team 字符串数字/reelcore 真数字同解）。
func (c *mtorrentCore) fetchProfile(ctx context.Context, config *model.SiteConfig) (*mtorrentProfile, error) {
	u := resolveBaseURL(config) + "/api/member/profile"
	req, err := http.NewRequestWithContext(ctx, "POST", u, nil)
	if err != nil {
		return nil, err
	}
	c.setAPIHeaders(req, config.APIKey)
	resp, err := c.doer.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { drainBody(resp) }()
	body, err := readBody(resp)
	if err != nil {
		return nil, err
	}
	var result struct {
		Data struct {
			Username    string  `json:"username"`
			Role        flexInt `json:"role"`
			MemberCount struct {
				Uploaded   json.Number `json:"uploaded"`
				Downloaded json.Number `json:"downloaded"`
				Bonus      json.Number `json:"bonus"`
				ShareRate  json.Number `json:"shareRate"`
			} `json:"memberCount"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析用户信息失败: %w", err)
	}
	prof := &mtorrentProfile{
		Username:      result.Data.Username,
		Role:          int(result.Data.Role),
		UploadBytes:   parseSizeString(result.Data.MemberCount.Uploaded.String()),
		DownloadBytes: parseSizeString(result.Data.MemberCount.Downloaded.String()),
	}
	prof.Bonus, _ = strconv.ParseFloat(result.Data.MemberCount.Bonus.String(), 64)
	prof.Ratio, _ = strconv.ParseFloat(result.Data.MemberCount.ShareRate.String(), 64)
	if prof.Ratio == 0 && prof.DownloadBytes > 0 {
		prof.Ratio = float64(prof.UploadBytes) / float64(prof.DownloadBytes)
	}
	return prof, nil
}

