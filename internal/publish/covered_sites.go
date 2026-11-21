// §59.535 刀5/刀6: 簇已有站三面并集判据单点——选站置灰（单簇过滤）与批量
// 发布执行层逐对跳过（3b）共用。三面：
//
//	面1 tracker 快照实测——簇内副本 tracker_domain→站名（正在做种·最实时）
//	面2 覆盖表——info_hash×site（has/probably_has·站上有但没在做种）
//	面3 发布记录——我们自己的发布成功史（站上必有）
//
// 方向=宁可多置灰（站方 dedup 兜底重复发布无害）不漏放（漏置灰仅多一次
// 注定失败请求·two-way 安全）。
package publish

import (
	"context"

	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/site"
	"gorm.io/gorm"
)

// coveredPublishStatuses 面3 发布记录成功终态集——单点引用
// model.PublishSkipStatuses（§59.546 终态登记处·发布事实口径不含 existing：
// 保留用户重试/推种选择权·用户定案 2026-11-22）。
var coveredPublishStatuses = func() map[string]bool {
	m := make(map[string]bool, len(model.PublishSkipStatuses))
	for _, s := range model.PublishSkipStatuses {
		m[s] = true
	}
	return m
}()

// CoveredSitesOfCluster 返回簇已有站集合（站名→true）。
// infoHash 定位快照行→(client_uid,save_path,name) 同簇聚合；快照 miss（非
// 本地种）返回空集合——调用方零跳过（发布链自有 dedup 三路防线）。
func CoveredSitesOfCluster(ctx context.Context, db *gorm.DB, tm *site.TrackerMatcher, infoHash string) map[string]bool {
	out := map[string]bool{}
	if db == nil || infoHash == "" {
		return out
	}
	var snap model.TorrentSnapshot
	if err := db.WithContext(ctx).
		Where("hash = ?", infoHash).First(&snap).Error; err != nil {
		return out
	}
	if snap.ClientUID == 0 || snap.SavePath == "" || snap.Name == "" {
		// 簇三元组不全：按单 hash 判（快照行自身仍可参与三面）
		coveredOfHashes(ctx, db, tm, []string{infoHash}, []ClusterMember{{
			Hash: infoHash, TrackerDomain: snap.TrackerDomain,
		}}, out)
		return out
	}
	members := GetClusterComments(ctx, db, snap.ClientUID, snap.SavePath, snap.Name)
	if len(members) == 0 {
		coveredOfHashes(ctx, db, tm, []string{infoHash}, []ClusterMember{{
			Hash: infoHash, TrackerDomain: snap.TrackerDomain,
		}}, out)
		return out
	}
	hashes := make([]string, 0, len(members))
	for _, m := range members {
		hashes = append(hashes, m.Hash)
	}
	coveredOfHashes(ctx, db, tm, hashes, members, out)
	return out
}

func coveredOfHashes(ctx context.Context, db *gorm.DB, tm *site.TrackerMatcher, hashes []string, members []ClusterMember, out map[string]bool) {
	// 面1: 簇内快照 tracker_domain→站名（正在做种）
	if tm != nil {
		for _, m := range members {
			if m.TrackerDomain == "" {
				continue
			}
			if name := tm.Match(m.TrackerDomain); name != "" {
				out[name] = true
			}
		}
	}
	// 面2: 覆盖表 has 态（站上有但没在做种·获取/辅种/IYUU/发布史各源）
	var covRows []string
	db.WithContext(ctx).
		Model(&model.SiteCoverageCache{}).
		Where("info_hash IN ? AND status IN ?", hashes,
			[]string{model.CoverageConfirmedHas, model.CoverageProbablyHas}).
		Pluck("site_name", &covRows)
	for _, s := range covRows {
		if s != "" {
			out[s] = true
		}
	}
	// 面3: 发布记录成功终态（我们自己的发布史=站上必有）
	var pubHitRows []struct {
		TargetSite string
		Status     string
	}
	db.WithContext(ctx).
		Table("publish_result_records").
		Select("publish_result_records.target_site, publish_result_records.status").
		Joins("JOIN publish_candidates ON publish_result_records.candidate_id = publish_candidates.id").
		Where("publish_candidates.info_hash IN ?", hashes).
		Scan(&pubHitRows)
	for _, r := range pubHitRows {
		if r.TargetSite != "" && coveredPublishStatuses[r.Status] {
			out[r.TargetSite] = true
		}
	}
}

// HashCoveredBySite 轻量单对判定（刀6 执行层逐 (簇,站) 对）——站名在簇已有
// 站集合中。coveredSet 由 CoveredSitesOfCluster 预算（站点循环外一次）。
func HashCoveredBySite(coveredSet map[string]bool, siteName string) bool {
	return coveredSet != nil && coveredSet[siteName]
}
