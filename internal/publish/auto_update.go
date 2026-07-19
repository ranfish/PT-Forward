// Package publish AutoUpdate 编排（§56.23 决策 5）。
//
// IsExisting=true 后按 ExistingStrategy 分支:
//   skip: 记录跳过
//   update: CheckEditPermission + GetEditForm + MergeEditFields + SubmitEdit
//   force: 直接标记成功
package publish

import (
	"context"
	"fmt"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
)

// EditPublisher 编辑发布器接口（adapter 实现）。
// 与 SitePublisher 分离，便于 adapter 逐步实现。
type EditPublisher interface {
	GetEditForm(ctx context.Context, torrentID string, config *model.SiteConfig) (*model.EditForm, error)
	SubmitEdit(ctx context.Context, req *model.EditRequest) error
}

// AutoUpdateResult 自动更新结果。
type AutoUpdateResult struct {
	TorrentID string           `json:"torrent_id"`
	Strategy  model.ExistingStrategy `json:"strategy"`
	Skipped   bool             `json:"skipped"`
	Edited    bool             `json:"edited"`
	Reason    string           `json:"reason"`
}

// AutoUpdate 自动更新已存在的种子。
//   publisher: 已实现 EditPublisher 的 adapter
//   detailHTML: 详情页 HTML（权限校验用）
//   torrentID: 目标种子 ID
//   newValues: 新表单字段（描述/MediaInfo 等）
//   strategy: 处理策略
//   siteFramework: 站点框架（权限校验路由）
func AutoUpdate(
	ctx context.Context,
	publisher EditPublisher,
	config *model.SiteConfig,
	detailHTML, torrentID string,
	newValues map[string]string,
	strategy model.ExistingStrategy,
	siteFramework string,
	logger *zap.Logger,
) (*AutoUpdateResult, error) {
	result := &AutoUpdateResult{
		TorrentID: torrentID,
		Strategy:  strategy,
	}

	switch strategy {
	case model.ExistingSkip:
		result.Skipped = true
		result.Reason = "策略=skip，跳过已存在种子"
		return result, nil

	case model.ExistingForce:
		result.Edited = true
		result.Reason = "策略=force，直接标记成功"
		return result, nil

	case model.ExistingUpdate:
		// 继续 update 流程
	default:
		result.Skipped = true
		result.Reason = "未知策略: " + string(strategy)
		return result, nil
	}

	// update 策略: 4 步流程

	// 1. 权限校验
	allowed, permReason := CheckEditPermission(detailHTML, torrentID, siteFramework)
	if !allowed {
		result.Skipped = true
		result.Reason = "权限校验失败: " + permReason
		if logger != nil {
			logger.Info("auto update: permission denied",
				zap.String("torrent_id", torrentID),
				zap.String("reason", permReason))
		}
		return result, nil
	}

	// 2. 获取编辑表单
	editForm, err := publisher.GetEditForm(ctx, torrentID, config)
	if err != nil {
		return result, fmt.Errorf("get edit form: %w", err)
	}

	// 3. 合并字段
	merged := MergeEditFields(editForm.Fields, newValues)

	// 4. 提交编辑
	editReq := &model.EditRequest{
		TorrentID:  torrentID,
		FormFields: merged,
	}
	if config != nil {
		editReq.Cookie = config.Cookie
		editReq.BaseURL = config.BaseURL
		editReq.Referer = config.BaseURL + "/details.php?id=" + torrentID
	}
	if err := publisher.SubmitEdit(ctx, editReq); err != nil {
		return result, fmt.Errorf("submit edit: %w", err)
	}

	result.Edited = true
	result.Reason = "编辑成功（" + permReason + "）"
	return result, nil
}
