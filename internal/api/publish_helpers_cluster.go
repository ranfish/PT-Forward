package api

import (
	"context"

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
