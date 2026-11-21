package api

// §59.541 附十 修4（v0.0.1541 终版）: 一次性 Dolby 键存量清洗（无前端·幂等）。
//
// 演进史（三次判定修正的方法论收敛）：
//   v1: 只查 media_info 列 → 误删 source MI 真键行（馒头 1141959 案）
//   v2: local 空→source 兜底 → 仍误删 BDInfo 真键行（优堡 341301 案——
//       extractBDInfoHDR 通道：MI 无 Video 段时 NFO 是 DoVi/Atmos 铁证）
//   v3（本版·终版）: 判定直接复用推理引擎——对候选行跑 InferFull（与
//       inferFinalTags 完全同源的输入构造），产物含 Dolby 键=推理认可=
//       真键保留；不含=文本通道时代残留=假键清洗。**与推理同源 = 永不
//       漏列**——未来推理链新增证据源判定自动跟随。
//
// 背景：§59.529（11-14）把 dict dolby_atmos 的 infer_scope 从 all 收窄到
// mi——改代码不洗存量：fetch 于 11-14 前的行若走了文本通道，DB tags 里
// 的 Dolby 键残留（piggo 57292 拒案实证）。清后触发 inferFinalTags 重推
// （顺便实战验证收窄后链路）。幂等：二次调用清洗 0 行。

import (
	"encoding/json"
	"net/http"

	"go.uber.org/zap"

	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/publish"
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

	inferer := publish.NewMediaTagInferer()
	cleaned := 0
	reinferScheduled := 0
	skippedIncomplete := 0
	keptByInfer := 0

	for i := range metas {
		m := &metas[i]
		var tags []string
		if err := json.Unmarshal([]byte(m.Tags), &tags); err != nil {
			continue // 非法 JSON 保持原状（不扩大处置面）
		}
		hasAtmosKey, hasVisionKey := false, false
		for _, t := range tags {
			if t == "dolby_atmos" {
				hasAtmosKey = true
			}
			if t == "dolby_vision" {
				hasVisionKey = true
			}
		}
		if !hasAtmosKey && !hasVisionKey {
			continue
		}

		// §59.541 附十 v3：推理引擎同源判定——与 inferFinalTags 完全
		// 相同的输入构造（MI local→source 兜底 + NFO=BDInfo + 标题/
		// 副标/简介/声明/产区）。产物含 Dolby=真键；不含=文本残留。
		var snapSize int64
		h.db.WithContext(r.Context()).Model(&model.TorrentSnapshot{}).
			Where("hash = ?", m.InfoHash).Select("COALESCE(MAX(size),0)").Scan(&snapSize)
		miForInfer := m.MediaInfo
		if miForInfer == "" {
			miForInfer = m.SourceMediaInfo
		}
		inferred := inferer.InferFull(publish.TagInput{
			MediaInfo:   miForInfer,
			Title:       m.Title,
			Subtitle:    m.Subtitle,
			Description: m.Description,
			NFO:         m.BDInfo,
			Size:        snapSize,
			Region:      m.RegionCode,
			Statement:   m.Statement,
		})
		inferHasAtmos, inferHasVision := false, false
		for _, t := range inferred {
			if t == "dolby_atmos" {
				inferHasAtmos = true
			}
			if t == "dolby_vision" {
				inferHasVision = true
			}
		}

		var kept []string
		var removed []string
		for _, t := range tags {
			if t == "dolby_atmos" && !inferHasAtmos {
				removed = append(removed, t)
				continue
			}
			if t == "dolby_vision" && !inferHasVision {
				removed = append(removed, t)
				continue
			}
			kept = append(kept, t)
		}
		if len(removed) == 0 {
			keptByInfer++ // 推理认可的真键行
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
		// 数据完整才重推（与 inferFinalTags 闸门同语义）
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
		zap.Int("kept_by_infer", keptByInfer))

	Success(w, map[string]interface{}{
		"message":            "Dolby 键存量清洗完成（§59.541 附十 v3 推理同源判定·幂等）",
		"scanned":            len(metas),
		"cleaned":            cleaned,
		"reinfer_scheduled":  reinferScheduled,
		"skipped_incomplete": skippedIncomplete,
		"kept_by_infer":      keptByInfer,
	})
}
