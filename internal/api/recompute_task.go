package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/titleparser"
	"go.uber.org/zap"
)

// §59.514 附八 P2: recompute 任务化（方案 A）——原同步端点全量单请求
// HTTP 超时不可靠；复用 batchFetchState 模式（§59.495 先例：异步任务+
// 进度轮询+graceful 停止）。范围参数：?site=X 限定站点（空=全量）。
type recomputeState struct {
	mu            sync.Mutex
	active        bool
	stopRequested bool
	total         int
	done          int
	failed        int
	startedAt     time.Time
	finishedAt    time.Time
	finished      bool
	error         string
}

// handleRecomputeProfilesTask POST /publish/seeds/recompute-profiles-task?site=X
// §59.514 附八 P2: 任务化重建（TechProfile 平铺列+重组标题列 FinalizeTitle
// 单点——逻辑升级后存量刷列的标准运维通道）。
func (h *PublishTorrentsHandler) handleRecomputeProfilesTask(w http.ResponseWriter, r *http.Request) {
	h.recompute.mu.Lock()
	if h.recompute.active {
		h.recompute.mu.Unlock()
		Error(w, http.StatusConflict, 40901, "已有重建任务在运行")
		return
	}
	site := r.URL.Query().Get("site")

	var rows []model.TorrentMetadata
	q := h.db.WithContext(r.Context()).Where("media_info != ''")
	if site != "" {
		q = q.Where("site_name = ?", site)
	}
	if err := q.Find(&rows).Error; err != nil {
		h.recompute.mu.Unlock()
		Error(w, http.StatusInternalServerError, 50000, "查询失败: "+err.Error())
		return
	}
	h.recompute.active = true
	h.recompute.stopRequested = false
	h.recompute.total = len(rows)
	h.recompute.done = 0
	h.recompute.failed = 0
	h.recompute.finished = false
	h.recompute.error = ""
	h.recompute.startedAt = time.Now()
	h.recompute.mu.Unlock()

	go h.runRecomputeTask(rows)
	Success(w, map[string]interface{}{"message": "已开始", "total": len(rows)})
}

// handleRecomputeProgress GET /publish/seeds/recompute-progress
func (h *PublishTorrentsHandler) handleRecomputeProgress(w http.ResponseWriter, r *http.Request) {
	h.recompute.mu.Lock()
	defer h.recompute.mu.Unlock()
	Success(w, map[string]interface{}{
		"active":        h.recompute.active,
		"total":         h.recompute.total,
		"done":          h.recompute.done,
		"failed":        h.recompute.failed,
		"stop_requested": h.recompute.stopRequested,
		"finished":      h.recompute.finished,
		"error":         h.recompute.error,
		"started_at":    h.recompute.startedAt,
		"finished_at":   h.recompute.finishedAt,
	})
}

// handleRecomputeCancel POST /publish/seeds/recompute-cancel（graceful·§59.495 同款）
func (h *PublishTorrentsHandler) handleRecomputeCancel(w http.ResponseWriter, r *http.Request) {
	h.recompute.mu.Lock()
	h.recompute.stopRequested = true
	h.recompute.mu.Unlock()
	Success(w, map[string]interface{}{"message": "停止请求已接受——当前行完成后终止"})
}

// runRecomputeTask 逐行：BuildTechProfile + FinalizeTitle + 平铺列+标题列 Updates。
// torrentData 尽力拉（快照定位·失败降级与 t0 同款）；graceful 检查点每行前。
func (h *PublishTorrentsHandler) runRecomputeTask(rows []model.TorrentMetadata) {
	ctx := context.Background()
	for _, m := range rows {
		h.recompute.mu.Lock()
		stopped := h.recompute.stopRequested
		h.recompute.mu.Unlock()
		if stopped {
			h.recompute.mu.Lock()
			h.recompute.error = "用户取消（已完成 " + itoa(h.recompute.done) + "/" + itoa(h.recompute.total) + "）"
			h.finishRecompute()
			h.recompute.mu.Unlock()
			return
		}
		mi := m.MediaInfo
		if mi == "" {
			mi = m.SourceMediaInfo
		}
		if mi == "" {
			h.recompute.mu.Lock()
			h.recompute.done++
			h.recompute.mu.Unlock()
			continue
		}
		h.recomputeRow(ctx, m, mi)
	}
	h.recompute.mu.Lock()
	h.finishRecompute()
	h.recompute.mu.Unlock()
}

// recomputeRow 单行重建（§59.514 附八 P6 从闭包提取为方法——sqlcheck 闭包
// 合并误报：立即调用闭包是一条 ExprStmt·快照查询 Where("hash") 与 Updates
// 链被 analyzeStmt 合并到 torrent_metadata 报 no such column: hash；
// 提取后各查询是独立语句零合并·行为零变化）。
func (h *PublishTorrentsHandler) recomputeRow(ctx context.Context, m model.TorrentMetadata, mi string) {
	defer func() {
		if p := recover(); p != nil {
			h.logger.Error("recompute panic", zap.String("hash", m.InfoHash), zap.Any("panic", p))
			h.recompute.mu.Lock()
			h.recompute.failed++
			h.recompute.done++
			h.recompute.mu.Unlock()
		}
	}()
	domMedium, domRes, domVideo, domAudio := titleparser.DOMFieldsFromDetailSource(m.DetailSourceJSON)
	profile := titleparser.BuildTechProfile(m.Title, mi, domMedium, domRes, domVideo, domAudio)
	profile.AudioTracks = titleparser.AdjustCommentaryTracks(profile.AudioTracks, m.Subtitle, mi)
	var torrentData []byte
	if h.clientMgr != nil && m.InfoHash != "" {
		var snap model.TorrentSnapshot
		if err := h.db.WithContext(ctx).Where("hash = ?", m.InfoHash).First(&snap).Error; err == nil {
			if cl, clErr := h.clientMgr.Get(snap.ClientUID); clErr == nil && cl != nil {
				fCtx, fCancel := context.WithTimeout(ctx, 15*time.Second)
				if b, bErr := cl.ExportTorrent(fCtx, m.InfoHash); bErr == nil {
					torrentData = b
				}
				fCancel()
			}
		}
	}
	reassembledTitle, reassembledNotes := h.executor.FinalizeTitle(ctx, &m, &profile, torrentData)
	components := titleparser.TechProfileToComponents(profile)
	category := titleparser.InferCategory(components, m.SourceCategory, "", "")
	updates := map[string]interface{}{
		"reassembled_title": reassembledTitle,
		"reassembled_notes": strings.Join(reassembledNotes, "；"),
		"category":          category,
		"resolution":        profile.Resolution,
		"video_codec":       profile.VideoCodec,
		"audio_codec":       profile.AudioCodec,
		"audio_channels":    profile.AudioChannels,
		"audio_tech":        profile.AudioTechnology,
		"audio_tracks":      profile.AudioTracks,
		"hdr":               profile.HDR,
		"bit_depth":         profile.BitDepth,
		"source_type":       profile.SourceType,
		"specification":     profile.Specification,
		"source_platform":   profile.SourcePlatform,
		"edition_info":      profile.EditionInfo,
		"region_code":       profile.RegionCode,
	}
	if err := h.db.WithContext(ctx).Model(&model.TorrentMetadata{}).
		Where("info_hash = ? AND site_name = ?", m.InfoHash, m.SiteName).
		Updates(updates).Error; err != nil {
		h.logger.Warn("recompute task row failed", zap.String("hash", m.InfoHash[:min(10, len(m.InfoHash))]), zap.Error(err))
		h.recompute.mu.Lock()
		h.recompute.failed++
		h.recompute.done++
		h.recompute.mu.Unlock()
		return
	}
	h.recompute.mu.Lock()
	h.recompute.done++
	h.recompute.mu.Unlock()
}

func (h *PublishTorrentsHandler) finishRecompute() {
	h.recompute.active = false
	h.recompute.finished = true
	h.recompute.finishedAt = time.Now()
}

func itoa(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}
