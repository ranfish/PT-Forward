// Package publish 簇读取与 comment 直达候选聚合（§59.61 O2 定案）。
package publish

import (
	"context"
	"encoding/json"
	"sort"

	"github.com/ranfish/pt-forward/internal/comment"
	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/site"
	"gorm.io/gorm"
)

// ClusterMember 簇内副本（同 client_id+save_path+name 的互辅种子群一员）。
type ClusterMember struct {
	Hash    string
	Comment string
	TrackerDomain string // §59.296: 下载来源站首域（纯数字/相对路径方言站点上下文）
}

// GetClusterComments 读簇内全部副本及其 comment（三入口统一：batch-fetch / manual-forward / refresh）。
// 依赖 migration 19 的复合索引 idx_snapshots_cluster。
func GetClusterComments(ctx context.Context, db *gorm.DB, clientUID uint, savePath, name string) []ClusterMember {
	if clientUID == 0 || savePath == "" || name == "" {
		return nil
	}
	var rows []ClusterMember
	db.WithContext(ctx).
		Model(&model.TorrentSnapshot{}).
		Select("hash, COALESCE(comment, '') as comment, COALESCE(tracker_domain, '') as tracker_domain").
		Where("client_uid = ? AND save_path = ? AND name = ? AND is_hidden = 0", clientUID, savePath, name).
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
		// §59.296: 全簇副本 tracker 域——快照列直读（馒头纯数字方言
		// 上下文），外部 map 仅作缺列兜底（新装/未同步窗口）
		host := m.TrackerDomain
		if host == "" {
			host = trackerHostByHash[m.Hash]
		}
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

// SortTargetsByFetchPriority §59.341: 簇凭证按"获取站点优先级"排序——
// 配置内站按序在前，未配置站按原序（快照序）在后（稳定排序保相对序）。
// 用户语义定案：源站优先（无视排名）→ 同簇注释直达（内部按本排序）→
// 其它按排队。原实现快照插入序（From S01 案：PT地带副本先插入恒先中
// ——用户配置猫 56 < PT地带 66 被无视——设计与实现分叉修复）。
// priority 为空时零变化（无配置环境保持原序）。
func SortTargetsByFetchPriority(targets []comment.DirectTarget, priority []string) []comment.DirectTarget {
	if len(priority) == 0 || len(targets) < 2 {
		return targets
	}
	rank := make(map[string]int, len(priority))
	for i, s := range priority {
		rank[s] = i
	}
	sorted := make([]comment.DirectTarget, len(targets))
	copy(sorted, targets)
	sort.SliceStable(sorted, func(i, j int) bool {
		ri, oki := rank[sorted[i].SiteName]
		rj, okj := rank[sorted[j].SiteName]
		if oki && okj {
			return ri < rj
		}
		if oki != okj {
			return oki // 配置内优先于未配置
		}
		return false // 都未配置：稳定排序保持原序
	})
	return sorted
}

// FetchPriorityList §59.341: 读 system_settings.fetch_priority（层选 ② 层
// 同款存储）。空配置返回 nil。
func FetchPriorityList(ctx context.Context, db *gorm.DB) []string {
	if db == nil {
		return nil
	}
	var val string
	if err := db.WithContext(ctx).Raw("SELECT value FROM system_settings WHERE key = 'fetch_priority' LIMIT 1").Row().Scan(&val); err != nil {
		return nil
	}
	var priority []string
	if err := json.Unmarshal([]byte(val), &priority); err != nil {
		return nil
	}
	return priority
}
