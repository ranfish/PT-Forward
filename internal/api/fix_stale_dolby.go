package api

// §59.541 附十 修4: 一次性 Dolby 键存量清洗（无前端·ops 调用）。
// 背景：§59.529（11-14）把 dict dolby_atmos 的 infer_scope 从 all 收窄到
// mi——改代码不洗存量：fetch 于 11-14 前的行若走了文本通道（标题/简介
// atmos 词），DB tags 里的 dolby_atmos/dolby_vision 键残留（piggo 57292
// 拒案实证：11-11 写入→11-17 发布只读 DB〔§59.319 定案〕→旧键投出）。
// 清洗语义（宁缺毋滥——§59.1438 音频族只信 MI 哲学）：
//   矛盾行 = tags 含 dolby_atmos/dolby_vision × MI 无对应证据
//   （atmos: MI 无 atmos 词；vision: MI 无 dolby vision/dvhe/dovi 词）
//   → JSON 删键（其余键保留）→ 数据完整行触发 inferFinalTags 重推
//   （走 §59.529 后推理链=顺便实战验证）；不完整行只删键不重推
//   （重推闸门本会跳过——避免半数据再产错键）。
// 幂等：二次调用清洗 0 行。

import (
	"encoding/json"
	"net/http"
	"strings"

	"go.uber.org/zap"

	"github.com/ranfish/pt-forward/internal/model"
)

func (h *PublishTorrentsHandler) handleFixStaleDolbyTags(w http.ResponseWriter, r *http.Request) {
	var metas []model.TorrentMetadata
	if err := h.db.WithContext(r.Context()).
		Where("tags LIKE ?", `%"dolby_atmos"%`).
		Or("tags LIKE ?", `%"dolby_vision"%`).
		Find(&metas).Error; err != nil {
		Error(w, http.StatusInternalServerError, 50000, "查询失败: "+err.Error())
		return
	}

	cleaned := 0
	reinferScheduled := 0
	skippedIncomplete := 0
	alreadyClean := 0

	for i := range metas {
		m := &metas[i]
		var tags []string
		if err := json.Unmarshal([]byte(m.Tags), &tags); err != nil {
			continue // 非法 JSON 保持原状（不扩大处置面）
		}
		mi := strings.ToLower(m.MediaInfo)
		hasAtmos := strings.Contains(mi, "atmos")
		hasVision := strings.Contains(mi, "dolby vision") ||
			strings.Contains(mi, "dvhe") || strings.Contains(mi, "dovi")

		var kept []string
		var removed []string
		for _, t := range tags {
			if t == "dolby_atmos" && !hasAtmos {
				removed = append(removed, t)
				continue
			}
			if t == "dolby_vision" && !hasVision {
				removed = append(removed, t)
				continue
			}
			kept = append(kept, t)
		}
		if len(removed) == 0 {
			alreadyClean++
			continue
		}
		data, err := json.Marshal(kept)
		if err != nil {
			continue
		}
		if err := h.db.WithContext(r.Context()).Model(&model.TorrentMetadata{}).
			Where("info_hash = ? AND site_name = ?", m.InfoHash, m.SiteName).
			Update("tags", string(data)).Error; err != nil {
			h.logger.Warn("fix stale dolby: update failed",
				zap.String("hash", m.InfoHash), zap.String("site", m.SiteName), zap.Error(err))
			continue
		}
		cleaned++
		// 数据完整才重推（与 inferFinalTags 闸门同语义——不完整行重推
		// 本会被跳过，预检减少无效调度）
		if missing := h.checkRequiredFields(m, detailSourceHasMusic(m.DetailSourceJSON)); len(missing) > 0 {
			skippedIncomplete++
			continue
		}
		h.inferFinalTags(r.Context(), 0, m.InfoHash, m.SiteName)
		reinferScheduled++
	}

	h.logger.Info("fix stale dolby tags completed",
		zap.Int("scanned", len(metas)),
		zap.Int("cleaned", cleaned),
		zap.Int("reinfer_scheduled", reinferScheduled),
		zap.Int("skipped_incomplete", skippedIncomplete),
		zap.Int("already_clean", alreadyClean))

	Success(w, map[string]interface{}{
		"message":            "Dolby 键存量清洗完成（§59.541 附十 修4·幂等）",
		"scanned":            len(metas),
		"cleaned":            cleaned,
		"reinfer_scheduled":  reinferScheduled,
		"skipped_incomplete": skippedIncomplete,
		"already_clean":      alreadyClean,
	})
}
