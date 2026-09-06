// Package publish 资源视图（虚拟种子，§59.44）。
//
// 数据层向种子配置页与未来发布管线输出的正式资源接口：
// 资源键 = (client_id, save_path, name)（与列表去重键同构），元数据挂靠不动、
// 读时按资源圈 hash 聚合。修复"挂载 hash ≠ 保留行 hash"场景下 detail/PUT 的
// 行级直查 404（49% 资源组，MAX(id) 滚动漂移）。
package publish

import (
	"context"

	"github.com/ranfish/pt-forward/internal/model"
	"gorm.io/gorm"
)

// ResourceView 一个资源的完整数据视图。
type ResourceView struct {
	ClientID string   // 资源键：下载器
	SavePath string   // 资源键：目录（Clean 形态）
	Name     string   // 资源键：下载器种子名
	Hashes   []string // 该资源键下全部活跃（hidden=0）hash
	// Meta 挂 metadata 的行经选行后的结果（selectSourceMeta 语义：制作组映射
	// 源站优先，否则 updated_at 最新）。nil = 资源无元数据（真未获取）。
	Meta *model.TorrentMetadata
	// AllMetas 资源名下全部 metadata 行（多站各一份，发布层按站选择的原料）
	AllMetas []model.TorrentMetadata
	// FromFallback hidden 兜底命中（资源已从下载器消失，按 hash 直查防线）
	FromFallback bool
}

// ResourceResolver 资源视图解析器（数据层正式出口）。
type ResourceResolver struct {
	db *gorm.DB
}

// NewResourceResolver 创建解析器。
func NewResourceResolver(db *gorm.DB) *ResourceResolver {
	return &ResourceResolver{db: db}
}

// ResolveResource 按任意 hash 解析其所属资源的完整视图。
//
// 主路径：hash → 活跃快照行的 (client, path, name) → 圈该三元组全部活跃 hash
//        → 挂 metadata 的行 → 选行。
// 兜底（FromFallback）：hash 无活跃快照行（编辑期间种子被删，hidden）→
//        hash 直查 metadata（"资源已消失但数据还在"语义，允许查看/保存）。
//
// 返回 nil = 主路径与兜底都无数据（真未获取且不在下载器）。
func (rr *ResourceResolver) ResolveResource(ctx context.Context, infoHash string) *ResourceView {
	if infoHash == "" {
		return nil
	}

	// 主路径：定位活跃快照行（同 hash 多 client 取最新）
	var snap model.TorrentSnapshot
	err := rr.db.WithContext(ctx).
		Where("hash = ? AND is_hidden = ?", infoHash, false).
		Order("updated_at DESC").
		First(&snap).Error
	if err == nil && snap.Name != "" {
		rv := &ResourceView{
			ClientID: snap.ClientID,
			SavePath: snap.SavePath,
			Name:     snap.Name,
		}
		// 圈资源键下全部活跃 hash
		rr.db.WithContext(ctx).Model(&model.TorrentSnapshot{}).
			Where("client_id = ? AND save_path = ? AND name = ? AND is_hidden = ?",
				snap.ClientID, snap.SavePath, snap.Name, false).
			Pluck("hash", &rv.Hashes)
		if len(rv.Hashes) > 0 {
			rr.db.WithContext(ctx).
				Where("info_hash IN ?", rv.Hashes).
				Find(&rv.AllMetas)
			rv.Meta = selectSourceMetaRow(rv.AllMetas)
		}
		return rv
	}

	// 兜底：hash 直查 metadata（资源可能已 hidden，或 hash 来自历史 detail 返回值）
	var metas []model.TorrentMetadata
	rr.db.WithContext(ctx).Where("info_hash = ?", infoHash).Find(&metas)
	if len(metas) == 0 {
		return nil
	}
	rv := &ResourceView{
		Hashes:       []string{infoHash},
		AllMetas:     metas,
		Meta:         selectSourceMetaRow(metas),
		FromFallback: true,
	}
	return rv
}

// selectSourceMetaRow 从同资源多行 metadata 中选源站行（与列表 selectSourceMeta 同语义：
// 制作组映射命中优先，否则 updated_at 最新）。纯函数版（handler 内同款逻辑的正式出口）。
func selectSourceMetaRow(metas []model.TorrentMetadata) *model.TorrentMetadata {
	if len(metas) == 0 {
		return nil
	}
	// §59.171: 确定性排序（与 handler selectSourceMeta 同款）——排序即选择：
	// 权威行（非 cluster）优先 → 新者优先 → 小 id 优先。原 updated_at 全量扫描
	// 已废弃（传播 bump 副本时间戳，"最新"常是副本行）。
	model.SortMetasAuthoritative(metas)
	return &metas[0]
}
