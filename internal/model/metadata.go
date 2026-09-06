package model

import (
	"sort"
	"time"
)

// §48.2 — TorrentMetadata: 源种元数据（抓取-存储-映射体系）
// §56.8 扩展：加 4 个 JSON 列（三源原始 + 合并结果）+ LastMergeMode
type TorrentMetadata struct {
	ID                uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	InfoHash          string    `json:"info_hash" gorm:"uniqueIndex:idx_hash_site;size:40"`
	SiteName          string    `json:"site_name" gorm:"uniqueIndex:idx_hash_site;size:100"`
	TorrentID         string    `json:"torrent_id" gorm:"size:50"`
	Title             string    `json:"title" gorm:"size:500"`
	Subtitle          string    `json:"subtitle" gorm:"size:500"`
	SourceCategory    string    `json:"source_category" gorm:"size:100"`
	StandardType      string    `json:"standard_type" gorm:"size:50"`
	Tags              string    `json:"tags" gorm:"type:text"`
	Flags             string    `json:"flags" gorm:"type:text"`
	SourceDescription string    `json:"source_description" gorm:"type:text"`
	Description       string    `json:"description" gorm:"type:text"`
	Screenshots       string    `json:"screenshots" gorm:"type:text"`
	Poster            string    `json:"poster" gorm:"size:500"`
	MediaInfo         string    `json:"mediainfo" gorm:"type:text"`
	SourceMediaInfo   string    `json:"source_mediainfo" gorm:"type:text"`
	MediaInfoSource   string    `json:"mediainfo_source" gorm:"size:20;default:''"`
	IMDbURL           string    `json:"imdb_url" gorm:"column:im_db_url;size:200"`
	DoubanURL         string    `json:"douban_url" gorm:"column:douban_url;size:200"`
	TMDbURL           string    `json:"tmdb_url" gorm:"column:tm_db_url;size:200"`
	FetchSource       string    `json:"fetch_source" gorm:"size:20;default:''"`
	Reviewed          bool      `json:"reviewed" gorm:"default:false"`
	FetchedAt         time.Time `json:"fetched_at"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`

	// §56.8 三源 JSON 列（详情页/PTGen/本地产物原始结果）
	DetailSourceJSON string `json:"detail_source_json" gorm:"column:detail_source_json;type:TEXT"`
	PTGenSourceJSON  string `json:"ptgen_source_json"  gorm:"column:ptgen_source_json;type:TEXT"`
	LocalSourceJSON  string `json:"local_source_json"  gorm:"column:local_source_json;type:TEXT"`

	// §56.8 合并结果 JSON（前端直接读"上次合并结果"，toggle 切换时合并一次写回）
	MergedJSON string `json:"merged_json" gorm:"column:merged_json;type:TEXT"`

	// §56.8 合并模式记录（UI toggle 显示）
	LastMergeMode string `json:"last_merge_mode" gorm:"column:last_merge_mode;size:20;default:'ptgen_first'"`

	// §59.20 源站声明 + BDInfo 持久化
	// §59.162: 限时禁转让渡截止（keepfrds 24h 窗——added+24h+30m 余量；
	// 零值=无窗口[永久禁转或可转]；now<until 拦截，过期自动放行）
	NoTransferUntil *time.Time `json:"no_transfer_until,omitempty"`
	Statement string `json:"statement" gorm:"type:text"` // §59.20: 源站官组声明（仅"获取数据"时写入）
	BDInfo    string `json:"bdinfo" gorm:"type:text"`     // §59.20: 蓝光原盘 BDInfo（BDMV/M2TS）

	// §59.19 TechProfile 平铺字段（从 MergedJSON 提取，种子配置页展示/筛选用）
	Category      string `json:"category" gorm:"size:20;default:''"`       // movie/tv_series/music/animation/...
	Form          string `json:"form" gorm:"size:20;default:''"`           // season_pack/partial_pack/single_episode/unknown
	Resolution    string `json:"resolution" gorm:"size:20;default:''"`     // 2160p/1080p/720p
	VideoCodec    string `json:"video_codec" gorm:"size:20;default:''"`    // x264/x265/H265
	AudioCodec    string `json:"audio_codec" gorm:"size:20;default:''"`    // DTS/TrueHD/AAC
	AudioChannels string `json:"audio_channels" gorm:"size:20;default:''"`  // 5.1/7.1/2.0
	AudioTech     string `json:"audio_tech" gorm:"size:20;default:''"`     // Atmos/Auro3D
	AudioTracks   int    `json:"audio_tracks" gorm:"default:0"`             // §59.76: 音轨数（v1.05 #16，Encode 类强制）
	HDR           string `json:"hdr" gorm:"size:30;default:''"`            // DV/HDR/HDR10+
	BitDepth      string `json:"bit_depth" gorm:"size:10;default:''"`      // 8bit/10bit
	SourceType    string `json:"source_type" gorm:"size:30;default:''"`    // UHD Blu-ray/BluRay
	Specification string `json:"specification" gorm:"size:20;default:''"`  // Remux/WEB-DL
	SourcePlatform string `json:"source_platform" gorm:"size:30;default:''"` // NF/AMZN/DSNP
	EditionInfo   string `json:"edition_info" gorm:"size:30;default:''"`   // REPACK/REMUX
	RegionCode    string `json:"region_code" gorm:"size:10;default:''"`    // USA/JPN/ITA
}

func (TorrentMetadata) TableName() string { return "torrent_metadata" }

// SortMetasAuthoritative §59.171: 同名多行确定性排序——权威行（非 cluster 传播
// 副本）优先 → updated_at 新 → id 小。消除无序查询的"抽签"（PT31 MI 红叉实锤：
// 列表 item 的 hash 挂着 mi=14741 的源行，selectSourceMeta 却选中 cluster 空行
// ——SELECT 无 ORDER BY 时 SQLite 走 info_hash 索引序，"第一行"由哈希字母序决定）。
// fetch_source 是判别字段：传播会复制 torrent_id，"有 tid"不可判别（243 实证
// 46 行同 tid）；cluster=传播副本，非 cluster（rss_detail/analyze/空）=直获权威。
func SortMetasAuthoritative(metas []TorrentMetadata) {
	sort.SliceStable(metas, func(i, j int) bool {
		aAuth := metas[i].FetchSource != "cluster"
		bAuth := metas[j].FetchSource != "cluster"
		if aAuth != bAuth {
			return aAuth
		}
		if !metas[i].UpdatedAt.Equal(metas[j].UpdatedAt) {
			return metas[i].UpdatedAt.After(metas[j].UpdatedAt)
		}
		return metas[i].ID < metas[j].ID
	})
}
