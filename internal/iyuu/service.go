package iyuu

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type Service struct {
	db     *gorm.DB
	logger *zap.Logger
	client *http.Client
}

func NewService(db *gorm.DB, logger *zap.Logger) *Service {
	return &Service{
		db:     db,
		logger: logger,
		client: &http.Client{Timeout: 60 * time.Second},
	}
}

func (s *Service) getConfig(ctx context.Context) (*model.IYUUConfig, error) {
	var cfg model.IYUUConfig
	if err := s.db.WithContext(ctx).First(&cfg).Error; err != nil {
		return nil, fmt.Errorf("iyuu config not found: %w", err)
	}
	if cfg.Token == "" {
		return nil, fmt.Errorf("iyuu token is empty")
	}
	return &cfg, nil
}

func (s *Service) Ping(ctx context.Context) error {
	cfg, err := s.getConfig(ctx)
	if err != nil {
		return err
	}

	url := cfg.BaseURL + "/App.Api/sites?token=" + cfg.Token
	resp, err := s.doGet(ctx, url)
	if err != nil {
		return fmt.Errorf("ping failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("IYUU returned HTTP %d", resp.StatusCode)
	}

	var result iyuuResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	if result.Ret != 200 {
		return fmt.Errorf("IYUU error: %s", result.Msg)
	}

	return nil
}

func (s *Service) QueryReseed(ctx context.Context, infoHashes []string) ([]*model.IYUUReseedResult, error) {
	if len(infoHashes) == 0 {
		return nil, nil
	}

	cfg, err := s.getConfig(ctx)
	if err != nil {
		return nil, err
	}

	payload := map[string]any{
		"token":   cfg.Token,
		"hash":    infoHashes,
		"version": cfg.Version,
	}

	var resp iyuuReseedResponse
	if err := s.doPost(ctx, cfg.BaseURL+"/App.Api/reseed.Query", payload, &resp); err != nil {
		return nil, fmt.Errorf("query reseed: %w", err)
	}

	if resp.Ret != 200 {
		return nil, fmt.Errorf("IYUU query error: %s", resp.Msg)
	}

	results := make([]*model.IYUUReseedResult, 0, len(resp.Data))
	for hash, targets := range resp.Data {
		result := &model.IYUUReseedResult{
			SourceInfoHash: hash,
		}
		for _, t := range targets {
			result.Targets = append(result.Targets, model.IYUUTarget{
				Sid:       t.Sid,
				TorrentID: t.TorrentID,
				InfoHash:  t.InfoHash,
			})
		}
		results = append(results, result)
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

	url := cfg.BaseURL + "/App.Api/sites?token=" + cfg.Token
	resp, err := s.doGet(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("get site list: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var result iyuuSitesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode sites response: %w", err)
	}

	if result.Ret != 200 {
		return nil, fmt.Errorf("IYUU sites error: %s", result.Msg)
	}

	sites := make([]model.IYUUSite, 0, len(result.Data))
	for _, site := range result.Data {
		sites = append(sites, model.IYUUSite{
			Sid:      site.Sid,
			Nickname: site.Nickname,
			BaseURL:  site.BaseURL,
			Site:     site.Site,
		})
	}

	if err := s.syncSiteMappings(ctx, result.Data); err != nil {
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
		"token": cfg.Token,
		"sid":   sidList,
	}

	var resp iyuuResponse
	if err := s.doPost(ctx, cfg.BaseURL+"/App.Api/reseed.ReportExisting", payload, &resp); err != nil {
		return fmt.Errorf("report existing: %w", err)
	}

	if resp.Ret != 200 {
		return fmt.Errorf("IYUU report error: %s", resp.Msg)
	}

	return nil
}

func (s *Service) SendNotification(_ context.Context, text, desp string) error {
	cfg, err := s.getConfig(context.Background())
	if err != nil {
		return err
	}

	payload := map[string]any{
		"token": cfg.Token,
		"text":  text,
		"desp":  desp,
	}

	var resp iyuuResponse
	if err := s.doPost(context.Background(), cfg.BaseURL+"/+api/send", payload, &resp); err != nil {
		return fmt.Errorf("send notification: %w", err)
	}

	if resp.Ret != 200 {
		return fmt.Errorf("IYUU notification error: %s", resp.Msg)
	}

	return nil
}

func (s *Service) doGet(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return s.client.Do(req)
}

func (s *Service) doPost(ctx context.Context, url string, payload any, result any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	return json.Unmarshal(respBody, result)
}

func (s *Service) getSiteMappings(ctx context.Context) map[int]string {
	var mappings []model.IYUUSiteMapping
	s.db.WithContext(ctx).Find(&mappings)

	result := make(map[int]string, len(mappings))
	for _, m := range mappings {
		result[m.IYUUSid] = m.SiteDomain
	}
	return result
}

func (s *Service) syncSiteMappings(ctx context.Context, sites []iyuuSiteRaw) error {
	for _, site := range sites {
		var mapping model.IYUUSiteMapping
		err := s.db.WithContext(ctx).
			Where("iyuu_sid = ?", site.Sid).
			First(&mapping).Error
		if err == gorm.ErrRecordNotFound {
			mapping = model.IYUUSiteMapping{
				IYUUSid:    site.Sid,
				SiteDomain: site.BaseURL,
				SiteName:   site.Nickname,
			}
			if err := s.db.WithContext(ctx).Create(&mapping).Error; err != nil {
				s.logger.Warn("create site mapping failed",
					zap.Int("sid", site.Sid),
					zap.Error(err),
				)
			}
		}
	}
	return nil
}

type iyuuResponse struct {
	Ret  int    `json:"ret"`
	Msg  string `json:"msg"`
	Data any    `json:"data"`
}

type iyuuReseedResponse struct {
	Ret  int                           `json:"ret"`
	Msg  string                        `json:"msg"`
	Data map[string][]iyuuReseedTarget `json:"data"`
}

type iyuuReseedTarget struct {
	Sid       int    `json:"sid"`
	TorrentID int    `json:"torrent_id"`
	InfoHash  string `json:"info_hash"`
}

type iyuuSitesResponse struct {
	Ret  int           `json:"ret"`
	Msg  string        `json:"msg"`
	Data []iyuuSiteRaw `json:"data"`
}

type iyuuSiteRaw struct {
	Sid      int    `json:"sid"`
	Nickname string `json:"nickname"`
	BaseURL  string `json:"base_url"`
	Site     string `json:"site"`
}
