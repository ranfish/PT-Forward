package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func (h *SeedingHandler) handleListRecords(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	clientUID := uint(0)
	if v := query.Get("clientUid"); v != "" {
		if n, err := strconv.ParseUint(v, 10, 64); err == nil {
			clientUID = uint(n)
		}
	}
	siteName := query.Get("site")
	status := query.Get("status")
	search := query.Get("search")
	page, _ := strconv.Atoi(query.Get("page"))
	size, _ := strconv.Atoi(query.Get("size"))
	if page <= 0 {
		page = 1
	}
	if size <= 0 {
		size = 50
	}

	if h.engine != nil && clientUID != 0 {
		records, err := h.engine.ListByClient(r.Context(), clientUID)
		if err != nil {
			Error(w, http.StatusInternalServerError, 50000, "查询刷流记录失败")
			return
		}

		filtered := records
		if siteName != "" {
			var tmp []model.SeedingTorrentRecord
			for _, r := range filtered {
				if r.SiteName == siteName {
					tmp = append(tmp, r)
				}
			}
			filtered = tmp
		}
		if status != "" {
			var tmp []model.SeedingTorrentRecord
			for _, r := range filtered {
				if string(r.Status) == status {
					tmp = append(tmp, r)
				}
			}
			filtered = tmp
		}
		if search != "" {
			var tmp []model.SeedingTorrentRecord
			for _, r := range filtered {
				if strings.Contains(r.InfoHash, search) || strings.Contains(r.TorrentID, search) || strings.Contains(r.SiteName, search) {
					tmp = append(tmp, r)
				}
			}
			filtered = tmp
		}

		total := len(filtered)
		start := (page - 1) * size
		if start > total {
			start = total
		}
		end := start + size
		if end > total {
			end = total
		}

		Success(w, map[string]interface{}{
			"items": filtered[start:end],
			"total": total,
			"page":  page,
			"size":  size,
		})
		return
	}

	q := h.db.WithContext(r.Context()).Model(&model.SeedingTorrentRecord{}).
		Where("status IN ?", []string{"seeding", "paused_free_end", "paused_rule"})
	if clientUID != 0 {
		q = q.Where("client_uid = ?", clientUID)
	}
	if siteName != "" {
		q = q.Where("site_name = ?", siteName)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if search != "" {
		q = q.Where("info_hash LIKE ? OR torrent_id LIKE ? OR site_name LIKE ?", "%"+search+"%", "%"+search+"%", "%"+search+"%")
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		Error(w, http.StatusInternalServerError, 50000, "查询种子记录总数失败")
		return
	}

	var records []model.SeedingTorrentRecord
	offset := (page - 1) * size
	if err := q.Session(&gorm.Session{}).Order("updated_at DESC").Offset(offset).Limit(size).Find(&records).Error; err != nil {
		Error(w, http.StatusInternalServerError, 50000, "查询种子记录失败")
		return
	}

	type seenTitle struct {
		SiteName  string
		TorrentID string
		Title     string
	}
	var titles []seenTitle
	h.db.WithContext(r.Context()).Model(&model.RSSTorrentSeen{}).
		Select("site_name, torrent_id, title").
		Find(&titles)
	titleMap := make(map[string]string, len(titles))
	for _, t := range titles {
		titleMap[t.SiteName+"|"+t.TorrentID] = t.Title
	}

	type siteInfo struct {
		BaseURL   string
		Framework string
	}
	var sites []model.Site
	if err := h.db.Select("name, base_url, framework").Find(&sites).Error; err != nil {
		h.logger.Warn("query failed", zap.Error(err))
	}

	siteMap := make(map[string]siteInfo, len(sites))
	for _, s := range sites {
		siteMap[s.Name] = siteInfo{BaseURL: s.BaseURL, Framework: s.Framework}
	}

	type recordItem struct {
		model.SeedingTorrentRecord
		Title     string `json:"title"`
		DetailURL string `json:"detail_url"`
	}

	items := make([]recordItem, len(records))
	for i := range records {
		items[i] = recordItem{SeedingTorrentRecord: records[i]}
		if title, ok := titleMap[records[i].SiteName+"|"+records[i].TorrentID]; ok {
			items[i].Title = title
		}
		if si, ok := siteMap[records[i].SiteName]; ok && si.BaseURL != "" && records[i].TorrentID != "" {
			items[i].DetailURL = buildDetailURL(si.BaseURL, si.Framework, records[i].TorrentID)
		}
	}

	Success(w, map[string]interface{}{
		"items": items,
		"total": total,
		"page":  page,
		"size":  size,
	})
}

func extractLastSegment(path, prefix string) string {
	remaining := strings.TrimPrefix(path, prefix)
	remaining = strings.TrimRight(remaining, "/")
	return remaining
}

// parseUintParam §59.552: path 尾段 ID 提取+委托 parseUintStrict（统一契约）。
func parseUintParam(path, prefix string) (uint, error) {
	remaining := strings.TrimPrefix(path, prefix)
	remaining = strings.TrimRight(remaining, "/")
	parts := strings.Split(remaining, "/")
	return parseUintStrict(parts[len(parts)-1])
}

func (h *SeedingHandler) handleResumeRecord(w http.ResponseWriter, r *http.Request, idStr string) {
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		Error(w, http.StatusBadRequest, 40001, "无效的记录 ID")
		return
	}

	if h.engine == nil {
		Error(w, http.StatusServiceUnavailable, 50001, "刷流引擎未初始化")
		return
	}

	if err := h.engine.UpdateStatus(r.Context(), uint(id), model.SeedingStatusSeeding, "manual_resume"); err != nil {
		Error(w, http.StatusInternalServerError, 50000, "恢复记录失败")
		return
	}

	h.logger.Info("seeding record resumed", zap.String("id", idStr))
	Success(w, map[string]interface{}{"message": "已恢复"})
}

func (h *SeedingHandler) handlePauseRecord(w http.ResponseWriter, r *http.Request, idStr string) {
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		Error(w, http.StatusBadRequest, 40001, "无效的记录 ID")
		return
	}

	if h.engine == nil {
		Error(w, http.StatusServiceUnavailable, 50001, "刷流引擎未初始化")
		return
	}

	if err := h.engine.UpdateStatus(r.Context(), uint(id), model.SeedingStatusPausedFreeEnd, "manual_pause"); err != nil {
		Error(w, http.StatusInternalServerError, 50000, "暂停记录失败")
		return
	}

	h.logger.Info("seeding record paused", zap.String("id", idStr))
	Success(w, map[string]interface{}{"message": "已暂停"})
}

func (h *SeedingHandler) handleFreeWaitQueue(w http.ResponseWriter, r *http.Request) {
	if h.engine == nil {
		Success(w, map[string]interface{}{"items": []interface{}{}, "total": 0})
		return
	}
	items := h.engine.FreeWaitQueue()
	Success(w, map[string]interface{}{"items": items, "total": len(items)})
}

func (h *SeedingHandler) handleListTorrents(w http.ResponseWriter, r *http.Request) {
	q := h.db.WithContext(r.Context()).Model(&model.SeedingTorrentRecord{}).
		Where("status IN ?", []string{"seeding", "paused_free_end", "paused_rule"})

	if v := r.URL.Query().Get("client_id"); v != "" {
		if n, err := strconv.ParseUint(v, 10, 64); err == nil {
			q = q.Where("client_uid = ?", uint(n))
		}
	} else {
		// §56.40: 默认只返回 role=seeding 下载器的种子（聚焦刷流业务）
		q = q.Where("client_uid IN (SELECT id FROM clients WHERE role = 'seeding')")
	}

	page, pageSize := parsePagination(r) // §59.552 统一（100→200 有意放宽）
	offset := (page - 1) * pageSize

	var total int64
	if err := q.Count(&total).Error; err != nil {
		Error(w, http.StatusInternalServerError, 50000, "查询种子列表总数失败")
		return
	}

	var records []model.SeedingTorrentRecord
	if err := q.Session(&gorm.Session{}).Order("updated_at DESC").Offset(offset).Limit(pageSize).Find(&records).Error; err != nil {
		Error(w, http.StatusInternalServerError, 50000, "查询种子列表失败")
		return
	}

	type seenTitle struct {
		SiteName  string
		TorrentID string
		Title     string
	}
	var titles []seenTitle
	h.db.WithContext(r.Context()).Model(&model.RSSTorrentSeen{}).
		Select("site_name, torrent_id, title").
		Find(&titles)
	titleMap := make(map[string]string, len(titles))
	for _, t := range titles {
		titleMap[t.SiteName+":"+t.TorrentID] = t.Title
	}

	type siteInfo struct {
		BaseURL   string
		Framework string
	}
	var sites []model.Site
	if err := h.db.Select("name, base_url, framework").Find(&sites).Error; err != nil {
		h.logger.Warn("query failed", zap.Error(err))
	}

	siteMap := make(map[string]siteInfo, len(sites))
	for _, s := range sites {
		siteMap[s.Name] = siteInfo{BaseURL: s.BaseURL, Framework: s.Framework}
	}

	type torrentItem struct {
		model.SeedingTorrentRecord
		Title     string `json:"title"`
		DetailURL string `json:"detail_url"`
	}

	items := make([]torrentItem, len(records))
	for i := range records {
		items[i] = torrentItem{SeedingTorrentRecord: records[i]}
		if title, ok := titleMap[records[i].SiteName+":"+records[i].TorrentID]; ok {
			items[i].Title = title
		}
		if si, ok := siteMap[records[i].SiteName]; ok && si.BaseURL != "" && records[i].TorrentID != "" {
			items[i].DetailURL = buildDetailURL(si.BaseURL, si.Framework, records[i].TorrentID)
		}
	}

	Success(w, map[string]interface{}{
		"items": items,
		"total": total,
		"page":  page,
	})
}

func (h *SeedingHandler) handleListClients(w http.ResponseWriter, r *http.Request) {
	var configs []model.SeedingClientConfig
	if err := h.db.WithContext(r.Context()).Find(&configs).Error; err != nil {
		Error(w, http.StatusInternalServerError, 50000, "查询做种客户端配置失败")
		return
	}
	Success(w, map[string]interface{}{
		"items": configs,
		"total": len(configs),
	})
}

func (h *SeedingHandler) handleListHistory(w http.ResponseWriter, r *http.Request) {
	page, size := parsePagination(r)
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 200 {
		size = 50
	}

	q := h.db.WithContext(r.Context()).Model(&model.SeedingTorrentRecord{}).
		Where("status = ?", "deleted")

	if v := r.URL.Query().Get("client_id"); v != "" {
		if n, err := strconv.ParseUint(v, 10, 64); err == nil {
			q = q.Where("client_uid = ?", uint(n))
		}
	}
	if siteName := r.URL.Query().Get("site_name"); siteName != "" {
		q = q.Where("site_name = ?", siteName)
	}
	if search := r.URL.Query().Get("search"); search != "" {
		q = q.Where("info_hash LIKE ? OR torrent_id LIKE ? OR site_name LIKE ?", "%"+search+"%", "%"+search+"%", "%"+search+"%")
	}
	if actionBy := r.URL.Query().Get("action_by"); actionBy != "" {
		q = q.Where("last_action_by LIKE ?", "%"+actionBy+"%")
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		Error(w, http.StatusInternalServerError, 50000, "查询历史记录总数失败")
		return
	}

	var records []model.SeedingTorrentRecord
	offset := (page - 1) * size
	if err := q.Session(&gorm.Session{}).Order("updated_at DESC").Offset(offset).Limit(size).Find(&records).Error; err != nil {
		Error(w, http.StatusInternalServerError, 50000, "查询历史记录失败")
		return
	}

	type seenTitle struct {
		SiteName  string
		TorrentID string
		Title     string
	}
	var titles []seenTitle
	h.db.WithContext(r.Context()).Model(&model.RSSTorrentSeen{}).
		Select("site_name, torrent_id, title").
		Find(&titles)
	titleMap := make(map[string]string, len(titles))
	for _, t := range titles {
		titleMap[t.SiteName+"|"+t.TorrentID] = t.Title
	}

	type trafficInfo struct {
		InfoHash    string
		Uploaded    int64
		Downloaded  int64
		Ratio       float64
		UploadSpeed int64
	}
	var traffics []trafficInfo
	if len(records) > 0 {
		hashes := make([]string, len(records))
		for i, r := range records {
			hashes[i] = r.InfoHash
		}
		h.db.WithContext(r.Context()).Model(&model.TorrentTraffic{}).
			Select("info_hash, uploaded, downloaded, ratio, upload_speed").
			Where("info_hash IN ?", hashes).
			Find(&traffics)
	}
	trafficMap := make(map[string]trafficInfo, len(traffics))
	for _, t := range traffics {
		trafficMap[t.InfoHash] = t
	}

	type siteInfo struct {
		BaseURL   string
		Framework string
	}
	var sites []model.Site
	if err := h.db.Select("name, base_url, framework").Find(&sites).Error; err != nil {
		h.logger.Warn("query failed", zap.Error(err))
	}

	siteMap := make(map[string]siteInfo, len(sites))
	for _, s := range sites {
		siteMap[s.Name] = siteInfo{BaseURL: s.BaseURL, Framework: s.Framework}
	}

	type historyItem struct {
		model.SeedingTorrentRecord
		Title      string  `json:"title"`
		DetailURL  string  `json:"detail_url"`
		Uploaded   int64   `json:"uploaded"`
		Downloaded int64   `json:"downloaded"`
		Ratio      float64 `json:"ratio"`
	}
	items := make([]historyItem, len(records))
	for i := range records {
		items[i] = historyItem{SeedingTorrentRecord: records[i]}
		if title, ok := titleMap[records[i].SiteName+"|"+records[i].TorrentID]; ok {
			items[i].Title = title
		}
		if si, ok := siteMap[records[i].SiteName]; ok && si.BaseURL != "" && records[i].TorrentID != "" {
			items[i].DetailURL = buildDetailURL(si.BaseURL, si.Framework, records[i].TorrentID)
		}
		if t, ok := trafficMap[records[i].InfoHash]; ok {
			items[i].Uploaded = t.Uploaded
			items[i].Downloaded = t.Downloaded
			items[i].Ratio = t.Ratio
		}
	}

	Success(w, map[string]interface{}{
		"items": items,
		"total": total,
		"page":  page,
		"size":  size,
	})
}
