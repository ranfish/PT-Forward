package api

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/publish"
	"go.uber.org/zap"
)

// handleExecutePublish §59.156 切片 2: 新发布执行器入口（DB 供给——切读 publish_form_config）。
// body: {info_hash, target_site, anonymous, tag_overrides: [], dry_run}
// handleBDInfoBackfill §59.319 P3/D6：存量原盘批量补扫（手动运维端点，
// §59.275 先例）——扫描指定本地下载器下全部簇：磁盘识别门命中且簇内
// 无 bd_info → 入队扫描队列。返回统计（enqueued/已有产物跳过/非原盘/
// 文件不可见）。
func (h *PublishTorrentsHandler) handleBDInfoBackfill(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ClientUID uint `json:"clientId"`
	}
	if !decodeJSONBody(w, r, &req) {
		return
	}
	if req.ClientUID == 0 {
		Error(w, http.StatusBadRequest, 40001, "clientId 必填")
		return
	}
	var client model.ClientConfig
	if err := h.db.WithContext(r.Context()).Where("id = ?", req.ClientUID).First(&client).Error; err != nil {
		Error(w, http.StatusBadRequest, 40001, "下载器不存在")
		return
	}
	if !client.IsLocal {
		Error(w, http.StatusBadRequest, 40001, "该下载器非本地（is_local=false 不支持 BD 扫描）")
		return
	}
	if h.bdinfoQueue == nil || h.bdinfoScanner == nil {
		Error(w, http.StatusServiceUnavailable, 50001, "BDInfo 扫描队列未配置")
		return
	}

	// 簇枚举：distinct (save_path, name) + max(size)（§59.361 B: 簇内最大快照
	// size 供超时自适应——同簇同数据取代表值）
	type clusterRow struct {
		SavePath, Name string
		MaxSize        int64
	}
	var clusters []clusterRow
	if err := h.db.WithContext(r.Context()).Model(&model.TorrentSnapshot{}).
		Where("client_uid = ? AND is_hidden = 0", req.ClientUID).
		Distinct().Select("save_path, name, max(size) as max_size").Find(&clusters).Error; err != nil {
		Error(w, http.StatusInternalServerError, 50000, "簇枚举失败: "+err.Error())
		return
	}

	result := map[string]interface{}{
		"enqueued": 0, "has_bdinfo": 0, "not_disc": 0, "missing": 0,
		"enqueued_names": []string{},
	}
	enqNames := []string{}
	for _, c := range clusters {
		if c.SavePath == "" || c.Name == "" {
			continue
		}
		discPath := h.bdinfoScanner.DetectDiscPath(c.SavePath, c.Name)
		if discPath == "" {
			// 区分文件不可见与确定非原盘：种子目录存在但无盘结构=not_disc，
			// 目录不存在=missing（挂载/路径问题可观测）
			if _, err := os.Stat(filepath.Join(c.SavePath, c.Name)); err != nil {
				result["missing"] = result["missing"].(int) + 1
			} else {
				result["not_disc"] = result["not_disc"].(int) + 1
			}
			continue
		}
		if clusterHasBDInfoDB(h.db, r.Context(), req.ClientUID, c.SavePath, c.Name) {
			result["has_bdinfo"] = result["has_bdinfo"].(int) + 1
			continue
		}
		if h.bdinfoQueue.Enqueue(publish.BDInfoScanTask{
			ClientUID: req.ClientUID, SavePath: c.SavePath, Name: c.Name, DiscPath: discPath,
			Size: c.MaxSize, // §59.361 B: 超时自适应
		}) {
			result["enqueued"] = result["enqueued"].(int) + 1
			enqNames = append(enqNames, c.Name)
		}
	}
	result["enqueued_names"] = enqNames
	result["clusters_total"] = len(clusters)
	h.logger.Info("bdinfo backfill",
		zap.Uint("client", req.ClientUID),
		zap.Int("clusters", len(clusters)),
		zap.Int("enqueued", result["enqueued"].(int)))
	Success(w, result)
}

// handleBDInfoRescan §59.361 A: 单颗重扫——失败态行的手动入口（清失败
// 记录+重新入队；预算自适应 §59.361 B）。pt29 案：30min 超时失败后
// 无重试无入口，失败态永挂。
func (h *PublishTorrentsHandler) handleBDInfoRescan(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ClientUID uint   `json:"clientId"`
		SavePath  string `json:"savePath"`
		Name      string `json:"name"`
	}
	if !decodeJSONBody(w, r, &req) {
		return
	}
	if req.ClientUID == 0 || req.SavePath == "" || req.Name == "" {
		Error(w, http.StatusBadRequest, 40001, "clientId/savePath/name 必填")
		return
	}
	if h.bdinfoQueue == nil || h.bdinfoScanner == nil {
		Error(w, http.StatusServiceUnavailable, 50001, "BDInfo 扫描队列未配置")
		return
	}
	discPath := h.bdinfoScanner.DetectDiscPath(req.SavePath, req.Name)
	if discPath == "" {
		Error(w, http.StatusBadRequest, 40001, "磁盘未检测到原盘结构（BDMV/ISO）")
		return
	}
	// 簇内已有产物=无需重扫
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	if h.clusterHasBDInfo(ctx, req.ClientUID, req.SavePath, req.Name) {
		Success(w, map[string]interface{}{"enqueued": false, "reason": "簇内已有 BDInfo"})
		return
	}
	// 快照 size（超时自适应）+ 清失败记录（重新进入扫描中）
	var size int64
	h.db.WithContext(ctx).Model(&model.TorrentSnapshot{}).
		Where("client_uid = ? AND save_path = ? AND name = ?", req.ClientUID, req.SavePath, req.Name).
		Select("COALESCE(MAX(size),0)").Scan(&size)
	hashes := h.clusterHashes(ctx, req.ClientUID, req.SavePath, req.Name)
	if len(hashes) > 0 {
		if err := h.db.WithContext(ctx).Model(&model.TorrentMetadata{}).
			Where("info_hash IN ?", hashes).
			Updates(map[string]interface{}{
				"bd_scan_failed_at": nil,
				"bd_scan_error":     "",
			}).Error; err != nil {
			h.logger.Warn("bdinfo rescan clear-failed failed",
				zap.String("name", req.Name), zap.Error(err))
		}
	}
	ok := h.bdinfoQueue.Enqueue(publish.BDInfoScanTask{
		ClientUID: req.ClientUID, SavePath: req.SavePath, Name: req.Name,
		DiscPath: discPath, Size: size,
	})
	h.logger.Info("bdinfo rescan requested",
		zap.String("name", req.Name), zap.Bool("enqueued", ok))
	Success(w, map[string]interface{}{"enqueued": ok})
}

// handleBDInfoQueueStatus §59.319 P3/D6：扫描队列状态（running/pending——
// 前端/运维轮询补扫进度）。
func (h *PublishTorrentsHandler) handleBDInfoQueueStatus(w http.ResponseWriter, r *http.Request) {
	if h.bdinfoQueue == nil {
		Error(w, http.StatusServiceUnavailable, 50001, "BDInfo 扫描队列未配置")
		return
	}
	Success(w, h.bdinfoQueue.Status())
}
