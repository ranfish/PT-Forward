package api

import (
	"context"

	"go.uber.org/zap"

	"github.com/ranfish/pt-forward/internal/comment"
	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/publish"
	"github.com/ranfish/pt-forward/internal/site"
)

// buildClusterTargets §59.61: 簇副本 comment → 直达候选。
// 数据源管道 b（snapshots.comment，syncer 10min 同步）；tracker 首域取自本副本
// 下载器实时数据不可得时回退缩省形态（仅 URL/ob_tid/HDHx 方言，纯数字方言依赖
// tracker 上下文自动跳过——降级语义安全）。
func (h *PublishTorrentsHandler) buildClusterTargets(ctx context.Context, clientID, savePath, name, selfHash string) []comment.DirectTarget {
	if h.db == nil || clientID == "" || savePath == "" || name == "" {
		return nil
	}
	members := publish.GetClusterComments(ctx, h.db, clientID, savePath, name)
	if len(members) == 0 {
		return nil
	}
	// 域名解析器：全量站点四列域名（domain/alt/tracker/base_url）
	var sites []model.Site
	h.db.WithContext(ctx).Find(&sites)
	resolver := publish.HostResolverFromSites(sites)
	// tracker 首域：本副本 hash 的 TrackerMatcher 解析（快照无 tracker 列，
	// 用覆盖缓存 tracker 来源域近似——仅纯数字/相对路径方言需要，缺失则跳过）
	trackerHost := h.lookupTrackerHost(ctx, selfHash)
	hosts := map[string]string{selfHash: trackerHost}
	return publish.BuildClusterTargets(members, resolver, hosts)
}

// lookupTrackerHost 从覆盖缓存取该 hash 已知 tracker 域（近似上下文）。
func (h *PublishTorrentsHandler) lookupTrackerHost(ctx context.Context, hash string) string {
	if h.coverage == nil {
		return ""
	}
	cs, err := h.coverage.GetCachedCoverage(ctx, hash)
	if err != nil {
		return ""
	}
	for _, c := range cs {
		if c.Source == model.CoverageSourceTracker && c.SiteName != "" {
			var s model.Site
			if err := h.db.WithContext(ctx).Where("name = ?", c.SiteName).First(&s).Error; err == nil {
				return s.Domain
			}
		}
	}
	return ""
}

var _ = site.NewTrackerMatcher // 保持 site 包引用（HostResolverFromSites 内部使用）

// §59.61 第 4 步: 簇共享三方法——批内跳过 / 元数据传播 / 截图二次传播

// hasCompleteMetadata 该 hash 是否已有完整数据行（poster/description 任一非空）
// ——runBatchFetch 批内跳过判据（簇内先前副本已获取或传播）。
func (h *PublishTorrentsHandler) hasCompleteMetadata(ctx context.Context, hash string) bool {
	if hash == "" {
		return false
	}
	var n int64
	h.db.WithContext(ctx).Model(&model.TorrentMetadata{}).
		Where("info_hash = ? AND (poster != '' OR description != '')", hash).
		Count(&n)
	return n > 0
}

// propagateClusterMetadata 获取成功后（fetchSingleTorrent 末尾）：站点侧数据 + MI
// 复制到簇内无数据副本（已有行不覆盖——尊重显式获取/编辑）。fetch_source=cluster 审计可辨。
func (h *PublishTorrentsHandler) propagateClusterMetadata(ctx context.Context, clientID, savePath, name, selfHash, siteName string) {
	if h.db == nil || clientID == "" || savePath == "" || name == "" {
		return
	}
	// 源行（本站数据最全——TechProfile/tags/MI 均已后处理完）
	var src model.TorrentMetadata
	if err := h.db.WithContext(ctx).
		Where("info_hash = ? AND site_name = ?", selfHash, siteName).
		First(&src).Error; err != nil {
		return
	}
	// 簇内缺行副本
	var siblingHashes []string
	h.db.WithContext(ctx).Model(&model.TorrentSnapshot{}).
		Where("client_id = ? AND save_path = ? AND name = ? AND is_hidden = 0 AND hash != ?",
			clientID, savePath, name, selfHash).
		Pluck("hash", &siblingHashes)
	if len(siblingHashes) == 0 {
		return
	}
	var have []string
	h.db.WithContext(ctx).Model(&model.TorrentMetadata{}).
		Where("info_hash IN ?", siblingHashes).Pluck("info_hash", &have)
	haveSet := map[string]bool{}
	for _, hh := range have {
		haveSet[hh] = true
	}
	n := 0
	for _, hh := range siblingHashes {
		if haveSet[hh] {
			continue
		}
		cpy := src
		cpy.ID = 0
		cpy.InfoHash = hh
		cpy.FetchSource = "cluster"
		if err := h.db.WithContext(ctx).Create(&cpy).Error; err == nil {
			n++
		}
	}
	if n > 0 {
		h.logger.Info("cluster metadata propagated",
			zap.String("hash", selfHash[:min(10, len(selfHash))]),
			zap.Int("replicas", n))
	}
}

// propagateClusterScreenshots 截图策略完成后：补簇内空截图行
// （策略是分钟级异步任务，元数据传播时截图未就绪——此为二次传播）。
func (h *PublishTorrentsHandler) propagateClusterScreenshots(ctx context.Context, clientID, savePath, name, selfHash, screenshotsJSON string) {
	if h.db == nil || clientID == "" || savePath == "" || name == "" || screenshotsJSON == "" || screenshotsJSON == "[]" {
		return
	}
	var siblingHashes []string
	h.db.WithContext(ctx).Model(&model.TorrentSnapshot{}).
		Where("client_id = ? AND save_path = ? AND name = ? AND is_hidden = 0 AND hash != ?",
			clientID, savePath, name, selfHash).
		Pluck("hash", &siblingHashes)
	if len(siblingHashes) == 0 {
		return
	}
	res := h.db.WithContext(ctx).Model(&model.TorrentMetadata{}).
		Where("info_hash IN ? AND (screenshots = '' OR screenshots = '[]')", siblingHashes).
		Update("screenshots", screenshotsJSON)
	if res.RowsAffected > 0 {
		h.logger.Info("cluster screenshots propagated",
			zap.String("hash", selfHash[:min(10, len(selfHash))]),
			zap.Int64("rows", res.RowsAffected))
	}
}
