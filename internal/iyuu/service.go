package iyuu

import (
	"bytes"
	"context"
	"crypto/sha1" //nolint:gosec // SHA1 used for IYUU token hashing
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ranfish/pt-forward/internal/httpclient"
	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type Service struct {
	db         *gorm.DB
	logger     *zap.Logger
	client     *http.Client
	sidSha1    string
	sidSha1Mux sync.RWMutex
	sidSha1Exp time.Time
}

func NewService(db *gorm.DB, logger *zap.Logger) *Service {
	return &Service{
		db:     db,
		logger: logger,
		client: &http.Client{
			Timeout: 60 * time.Second,
			Transport: &http.Transport{
				Proxy: nil,
			},
		},
	}
}

func (s *Service) getConfig(ctx context.Context) (*model.IYUUConfig, error) {
	var cfg model.IYUUConfig
	if err := s.db.WithContext(ctx).First(&cfg).Error; err != nil {
		return nil, iyuuError(ErrIYUUConfig, "iyuu config not found", err)
	}
	if cfg.Token == "" {
		return nil, iyuuError(ErrIYUUConfig, "iyuu token is empty", nil)
	}
	return &cfg, nil
}

func (s *Service) Ping(ctx context.Context) error {
	cfg, err := s.getConfig(ctx)
	if err != nil {
		return err
	}

	u := cfg.BaseURL + "/reseed/sites/index"
	resp, err := s.doGetWithToken(ctx, u, cfg.Token)
	if err != nil {
		return iyuuError(ErrIYUUHTTP, "ping failed", err)
	}
	body, err := httpclient.ReadBody(resp)
	httpclient.DrainBody(resp)

	if err != nil {
		return iyuuError(ErrIYUUAPI, "read response body", err)
	}

	if resp.StatusCode != http.StatusOK {
		return iyuuError(ErrIYUUHTTP, fmt.Sprintf("IYUU returned HTTP %d: %s", resp.StatusCode, string(body)), nil)
	}

	var result iyuuRestResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return iyuuError(ErrIYUUAPI, "decode response", err)
	}

	if result.Code != 0 {
		return iyuuError(ErrIYUUAPI, fmt.Sprintf("IYUU error: %s", result.Msg), nil)
	}

	return nil
}

func (s *Service) QueryReseed(ctx context.Context, infoHashes []string) ([]*model.IYUUReseedResult, error) {
	var cleaned []string
	for _, h := range infoHashes {
		if h != "" {
			cleaned = append(cleaned, h)
		}
	}
	if len(cleaned) == 0 {
		return nil, nil
	}

	cfg, err := s.getConfig(ctx)
	if err != nil {
		return nil, err
	}

	if sidSha1 := s.getSidSha1(); sidSha1 == "" {
		if err := s.ensureSidSha1(ctx); err != nil {
			s.logger.Warn("failed to ensure sid_sha1, querying without it", zap.Error(err))
		}
	}
	sidSha1 := s.getSidSha1()

	infoHashes = cleaned

	const batchSize = 200
	var allResults []*model.IYUUReseedResult

	for i := 0; i < len(infoHashes); i += batchSize {
		end := i + batchSize
		if end > len(infoHashes) {
			end = len(infoHashes)
		}
		batch := make([]string, end-i)
		copy(batch, infoHashes[i:end])
		sort.Strings(batch)

		hashJSON, err := json.Marshal(batch)
		if err != nil {
			return nil, iyuuError(ErrIYUUAPI, "marshal info_hashes batch", err)
		}

		sha1Hash := fmt.Sprintf("%x", sha1.Sum(hashJSON)) //nolint:gosec // SHA1 required by IYUU API protocol
		ts := strconv.FormatInt(time.Now().Unix(), 10)

		form := url.Values{}
		form.Set("hash", string(hashJSON))
		form.Set("sha1", sha1Hash)
		form.Set("timestamp", ts)
		form.Set("version", cfg.Version)
		if sidSha1 != "" {
			form.Set("sid_sha1", sidSha1)
		}

		u := cfg.BaseURL + "/reseed/index/index"
		var resp iyuuRestReseedResponse
		if err := s.doPostFormWithToken(ctx, u, cfg.Token, form, &resp); err != nil {
			s.logger.Warn("IYUU batch query failed",
				zap.Int("batch", i/batchSize+1),
				zap.Int("from", i),
				zap.Int("to", end),
				zap.Error(err),
			)
			continue
		}

		if resp.Code == 404 {
			s.logger.Info("IYUU batch returned 404 (no matches)",
				zap.Int("batch", i/batchSize+1),
				zap.Int("batchSize", len(batch)),
			)
			continue
		}

		if resp.Code != 0 {
			s.logger.Warn("IYUU batch query error",
				zap.Int("batch", i/batchSize+1),
				zap.String("msg", resp.Msg),
				zap.Int("code", resp.Code),
			)
			continue
		}

		batchResults, err := s.parseReseedData(resp.Data)
		if err != nil {
			s.logger.Warn("IYUU batch decode failed",
				zap.Int("batch", i/batchSize+1),
				zap.Error(err),
			)
			continue
		}

		s.logger.Info("IYUU batch result",
			zap.Int("batch", i/batchSize+1),
			zap.Int("batchSize", len(batch)),
			zap.Int("results", len(batchResults)),
		)

		allResults = append(allResults, batchResults...)
	}

	return allResults, nil
}

func (s *Service) parseReseedData(data json.RawMessage) ([]*model.IYUUReseedResult, error) {
	if len(data) == 0 || data[0] != '{' {
		return nil, nil
	}

	var dataMap map[string]json.RawMessage
	if err := json.Unmarshal(data, &dataMap); err != nil {
		return nil, iyuuError(ErrIYUUAPI, "decode reseed data outer", err)
	}

	results := make([]*model.IYUUReseedResult, 0, len(dataMap))
	for hash, raw := range dataMap {
		result := &model.IYUUReseedResult{
			SourceInfoHash: hash,
		}

		var targets []iyuuReseedTarget
		if err := json.Unmarshal(raw, &targets); err == nil && len(targets) > 0 {
			for _, t := range targets {
				tid, _ := t.TorrentID.Int64()
				result.Targets = append(result.Targets, model.IYUUTarget{
					Sid:       t.Sid,
					TorrentID: int(tid),
					InfoHash:  t.InfoHash,
					Group:     t.Group,
				})
			}
		} else {
			var wrapper iyuuReseedDataWrapper
			if err := json.Unmarshal(raw, &wrapper); err != nil {
				s.logger.Info("skip unparseable reseed entry",
					zap.String("hash", hash),
					zap.Error(err),
				)
				continue
			}
			for _, t := range wrapper.Torrent {
				tid, _ := t.TorrentID.Int64()
				result.Targets = append(result.Targets, model.IYUUTarget{
					Sid:       t.Sid,
					TorrentID: int(tid),
					InfoHash:  t.InfoHash,
					Group:     t.Group,
				})
			}
		}

		if len(result.Targets) > 0 {
			results = append(results, result)
		}
	}

	return results, nil
}

func (s *Service) GetSeededSites(ctx context.Context, infoHash string) ([]string, error) {
	results, err := s.QueryReseed(ctx, []string{infoHash})
	if err != nil {
		return nil, err
	}

	if len(results) == 0 {
		return nil, nil
	}

	mappings := s.getSiteMappings(ctx)
	var sites []string
	for _, target := range results[0].Targets {
		if domain, ok := mappings[target.Sid]; ok {
			sites = append(sites, domain)
		}
	}

	return sites, nil
}

func (s *Service) GetSiteList(ctx context.Context) ([]model.IYUUSite, error) {
	cfg, err := s.getConfig(ctx)
	if err != nil {
		return nil, err
	}

	u := cfg.BaseURL + "/reseed/sites/index"
	resp, err := s.doGetWithToken(ctx, u, cfg.Token)
	if err != nil {
		return nil, iyuuError(ErrIYUUHTTP, "get site list", err)
	}
	body, err := httpclient.ReadBody(resp)
	httpclient.DrainBody(resp)
	if err != nil {
		return nil, iyuuError(ErrIYUUAPI, "read sites response body", err)
	}

	var result iyuuRestSitesResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, iyuuError(ErrIYUUAPI, "decode sites response", err)
	}

	if result.Code != 0 {
		return nil, iyuuError(ErrIYUUAPI, fmt.Sprintf("IYUU sites error: %s", result.Msg), nil)
	}

	var sitesData iyuuSitesData
	if len(result.Data) > 0 && result.Data[0] == '{' {
		if err := json.Unmarshal(result.Data, &sitesData); err != nil {
			return nil, iyuuError(ErrIYUUAPI, "decode sites data", err)
		}
	}

	sites := make([]model.IYUUSite, 0, len(sitesData.Sites))
	for _, site := range sitesData.Sites {
		sites = append(sites, model.IYUUSite{
			Sid:      site.ID,
			Nickname: site.Nickname,
			BaseURL:  site.BaseURL,
			Site:     site.Site,
		})
	}

	if err := s.syncSiteMappings(ctx, sitesData.Sites); err != nil {
		s.logger.Warn("failed to sync site mappings", zap.Error(err))
	}

	return sites, nil
}

func (s *Service) ReportExisting(ctx context.Context, sidList []int) error {
	if len(sidList) == 0 {
		return nil
	}

	cfg, err := s.getConfig(ctx)
	if err != nil {
		return err
	}

	payload := map[string]any{
		"sid_list": sidList,
	}

	u := cfg.BaseURL + "/reseed/sites/reportExisting"
	var resp iyuuReportResponse
	if err := s.doPostJSONWithToken(ctx, u, cfg.Token, payload, &resp); err != nil {
		return iyuuError(ErrIYUUHTTP, "report existing", err)
	}

	if resp.Code != 0 {
		return iyuuError(ErrIYUUAPI, fmt.Sprintf("IYUU report error: %s", resp.Msg), nil)
	}

	if resp.Data.SidSha1 != "" {
		s.sidSha1Mux.Lock()
		s.sidSha1 = resp.Data.SidSha1
		s.sidSha1Exp = time.Now().Add(7 * 24 * time.Hour)
		s.sidSha1Mux.Unlock()
	}

	return nil
}

func (s *Service) getSidSha1() string {
	s.sidSha1Mux.RLock()
	defer s.sidSha1Mux.RUnlock()
	if s.sidSha1 != "" && time.Now().Before(s.sidSha1Exp) {
		return s.sidSha1
	}
	return ""
}

func (s *Service) ensureSidSha1(ctx context.Context) error {
	var mappings []model.IYUUSiteMapping
	if err := s.db.WithContext(ctx).Where("enabled = ?", true).Find(&mappings).Error; err != nil {
		return err
	}
	if len(mappings) == 0 {
		return nil
	}
	sidList := make([]int, 0, len(mappings))
	for _, m := range mappings {
		sidList = append(sidList, m.IYUUSid)
	}
	return s.ReportExisting(ctx, sidList)
}

func (s *Service) SendNotification(ctx context.Context, text, desp string) error {
	cfg, err := s.getConfig(ctx)
	if err != nil {
		return err
	}

	form := url.Values{}
	form.Set("text", text)
	form.Set("desp", desp)

	u := cfg.BaseURL + "/+api/send"
	var resp iyuuRestResponse
	if err := s.doPostFormWithToken(ctx, u, cfg.Token, form, &resp); err != nil {
		return iyuuError(ErrIYUUHTTP, "send notification", err)
	}

	if resp.Code != 0 {
		return iyuuError(ErrIYUUAPI, fmt.Sprintf("IYUU notification error: %s", resp.Msg), nil)
	}

	return nil
}

func (s *Service) doGetWithToken(ctx context.Context, rawURL, token string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Token", token)
	return s.client.Do(req)
}

func (s *Service) doPostFormWithToken(ctx context.Context, rawURL, token string, form url.Values, result any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Token", token)

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	respBody, err := httpclient.ReadBody(resp)
	httpclient.DrainBody(resp)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return iyuuError(ErrIYUUHTTP, fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(respBody)), nil)
	}

	return json.Unmarshal(respBody, result)
}

func (s *Service) doPostJSONWithToken(ctx context.Context, rawURL, token string, payload any, result any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Token", token)

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	respBody, err := httpclient.ReadBody(resp)
	httpclient.DrainBody(resp)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return iyuuError(ErrIYUUHTTP, fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(respBody)), nil)
	}

	return json.Unmarshal(respBody, result)
}

func (s *Service) getSiteMappings(ctx context.Context) map[int]string {
	var mappings []model.IYUUSiteMapping
	if err := s.db.WithContext(ctx).Find(&mappings).Error; err != nil {
		s.logger.Warn("getSiteMappings failed", zap.Error(err))
		return make(map[int]string)
	}

	result := make(map[int]string, len(mappings))
	for _, m := range mappings {
		result[m.IYUUSid] = m.SiteDomain
	}
	return result
}

func (s *Service) syncSiteMappings(ctx context.Context, sites []iyuuSiteRaw) error {
	if len(sites) == 0 {
		return nil
	}

	sids := make([]int, 0, len(sites))
	for _, site := range sites {
		sids = append(sids, site.ID)
	}

	var existing []model.IYUUSiteMapping
	s.db.WithContext(ctx).Where("iyuu_sid IN ?", sids).Find(&existing)
	existingSet := make(map[int]bool, len(existing))
	for _, m := range existing {
		existingSet[m.IYUUSid] = true
	}

	for _, site := range sites {
		if existingSet[site.ID] {
			continue
		}
		mapping := model.IYUUSiteMapping{
			IYUUSid:    site.ID,
			SiteDomain: site.BaseURL,
			SiteName:   site.Nickname,
		}
		if err := s.db.WithContext(ctx).Create(&mapping).Error; err != nil {
			s.logger.Warn("create site mapping failed",
				zap.Int("sid", site.ID),
				zap.Error(err),
			)
		}
	}
	return nil
}

type iyuuRestResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data"`
}

type iyuuReportResponse struct {
	Code int            `json:"code"`
	Msg  string         `json:"msg"`
	Data iyuuReportData `json:"data"`
}

type iyuuReportData struct {
	SidSha1 string `json:"sid_sha1"`
}

type iyuuRestReseedResponse struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

type iyuuRestSitesResponse struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

type iyuuSitesData struct {
	Count int           `json:"count"`
	Sites []iyuuSiteRaw `json:"sites"`
}

type iyuuReseedTarget struct {
	Sid       int            `json:"sid"`
	TorrentID json.Number    `json:"torrent_id"`
	InfoHash  string         `json:"info_hash"`
	Group     int            `json:"group"`
}

type iyuuReseedDataWrapper struct {
	Torrent []iyuuReseedTarget `json:"torrent"`
}

type iyuuSiteRaw struct {
	ID       int    `json:"id"`
	Nickname string `json:"nickname"`
	BaseURL  string `json:"base_url"`
	Site     string `json:"site"`
}
