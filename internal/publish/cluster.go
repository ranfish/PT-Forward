// Package publish 簇读取与 comment 直达候选聚合（§59.61 O2 定案）。
package publish

import (
	"context"

	"github.com/ranfish/pt-forward/internal/comment"
	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/site"
	"gorm.io/gorm"
)

// ClusterMember 簇内副本（同 client_id+save_path+name 的互辅种子群一员）。
type ClusterMember struct {
	Hash    string
	Comment string
}

// GetClusterComments 读簇内全部副本及其 comment（三入口统一：batch-fetch / manual-forward / refresh）。
// 依赖 migration 19 的复合索引 idx_snapshots_cluster。
func GetClusterComments(ctx context.Context, db *gorm.DB, clientID, savePath, name string) []ClusterMember {
	if clientID == "" || savePath == "" || name == "" {
		return nil
	}
	var rows []ClusterMember
	db.WithContext(ctx).
		Model(&model.TorrentSnapshot{}).
		Select("hash, COALESCE(comment, '') as comment").
		Where("client_id = ? AND save_path = ? AND name = ? AND is_hidden = 0", clientID, savePath, name).
		Scan(&rows)
	return rows
}

// BuildClusterTargets 簇副本 + comment 解析 → 直达候选（去重，规则序保序）。
// hostResolver: 域名→站名（site.TrackerMatcher 语义）。
// trackerHostByHash: 各副本 tracker 首域（馒头纯数字/相对路径方言需要）。
func BuildClusterTargets(members []ClusterMember, hostResolver func(string) string, trackerHostByHash map[string]string) []comment.DirectTarget {
	var out []comment.DirectTarget
	seen := map[string]bool{}
	for _, m := range members {
		if m.Comment == "" {
			continue
		}
		host := trackerHostByHash[m.Hash]
		for _, tgt := range comment.Resolve(m.Comment, hostResolver, host) {
			key := tgt.SiteName + ":" + tgt.TorrentID
			if !seen[key] {
				seen[key] = true
				out = append(out, tgt)
			}
		}
	}
	return out
}

// HostResolverFromSites 从站点列表构建域名→站名解析器（复用 TrackerMatcher 索引语义）。
func HostResolverFromSites(sites []model.Site) func(string) string {
	m := site.NewTrackerMatcherFromSites(sites)
	return func(host string) string {
		if host == "" {
			return ""
		}
		return m.Match("http://" + host + "/")
	}
}
