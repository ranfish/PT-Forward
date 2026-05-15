package ptgen

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

type Provider struct {
	db     *gorm.DB
	logger *zap.Logger
	client *http.Client
}

func NewProvider(db *gorm.DB, logger *zap.Logger) *Provider {
	return &Provider{
		db:     db,
		logger: logger,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (p *Provider) Query(ctx context.Context, query string) (*model.PTGenResult, error) {
	if query == "" {
		return nil, &model.AppError{Code: 40001, Message: "query is empty"}
	}

	cached, err := p.getCache(ctx, query)
	if err == nil && cached != nil {
		result := &model.PTGenResult{
			ChineseTitle: cached.ChineseTitle,
			PosterURL:    cached.PosterURL,
			DoubanURL:    cached.DoubanURL,
			IMDBURL:      cached.IMDbURL,
			RawBBCode:    cached.BBCode,
			Source:       cached.Source,
			Cached:       true,
		}
		if cached.JSONData != "" {
			if jsonErr := json.Unmarshal([]byte(cached.JSONData), result); jsonErr != nil {
				return nil, fmt.Errorf("parse cached ptgen data: %w", jsonErr)
			}
			result.Cached = true
		}
		return result, nil
	}

	result, err := p.queryRemote(ctx, query)
	if err != nil {
		return nil, err
	}

	if err := p.saveCache(ctx, query, result); err != nil {
		p.logger.Warn("ptgen cache save failed", zap.Error(err))
	}

	return result, nil
}

func (p *Provider) SupportsNameSearch() bool {
	return true
}

func (p *Provider) queryRemote(ctx context.Context, query string) (*model.PTGenResult, error) {
	endpoints := []string{
		"https://ptgen.agsv.cc/api",
		"https://ptgen.click/api",
	}

	var lastErr error
	for _, endpoint := range endpoints {
		result, err := p.queryEndpoint(ctx, endpoint, query)
		if err != nil {
			p.logger.Debug("ptgen endpoint failed",
				zap.String("endpoint", endpoint),
				zap.Error(err),
			)
			lastErr = err
			continue
		}
		return result, nil
	}

	return nil, ptgenError(ErrPTGenRemote, "all PTGen endpoints failed", lastErr)
}

func (p *Provider) queryEndpoint(ctx context.Context, endpoint, query string) (*model.PTGenResult, error) {
	payload := map[string]string{"query": query}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, ptgenError(ErrPTGenRemote, fmt.Sprintf("request %s", endpoint), err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, ptgenError(ErrPTGenResponse, "read response", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, ptgenError(ErrPTGenResponse, fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(respBody)), nil)
	}

	var raw map[string]any
	if err := json.Unmarshal(respBody, &raw); err != nil {
		return nil, ptgenError(ErrPTGenResponse, "decode response", err)
	}

	result := &model.PTGenResult{Source: endpoint}

	if title, ok := raw["chinese_title"].(string); ok {
		result.ChineseTitle = title
	}
	if title, ok := raw["foreign_title"].(string); ok {
		result.ForeignTitle = title
	}
	if year, ok := raw["year"].(string); ok {
		result.Year = year
	}
	if poster, ok := raw["poster"].(string); ok {
		result.PosterURL = poster
	}
	if douban, ok := raw["douban_url"].(string); ok {
		result.DoubanURL = douban
	}
	if imdb, ok := raw["imdb_url"].(string); ok {
		result.IMDBURL = imdb
	}
	if imdbID, ok := raw["imdb_id"].(string); ok {
		result.IMDBID = imdbID
	}
	if doubanRating, ok := raw["douban_rating"].(string); ok {
		result.DoubanRating = doubanRating
	}
	if imdbRating, ok := raw["imdb_rating"].(string); ok {
		result.IMDBRating = imdbRating
	}
	if intro, ok := raw["introduction"].(string); ok {
		result.Introduction = intro
	}
	if bbcode, ok := raw["bbcode"].(string); ok {
		result.RawBBCode = bbcode
	}
	if region, ok := raw["region"].([]any); ok {
		for _, r := range region {
			if s, ok := r.(string); ok {
				result.Region = append(result.Region, s)
			}
		}
	}
	if genre, ok := raw["genre"].([]any); ok {
		for _, g := range genre {
			if s, ok := g.(string); ok {
				result.Genre = append(result.Genre, s)
			}
		}
	}
	if director, ok := raw["director"].([]any); ok {
		for _, d := range director {
			if s, ok := d.(string); ok {
				result.Director = append(result.Director, s)
			}
		}
	}

	return result, nil
}

func (p *Provider) getCache(ctx context.Context, query string) (*model.PTGenCache, error) {
	var cache model.PTGenCache
	err := p.db.WithContext(ctx).
		Where("query_key = ?", query).
		First(&cache).Error
	if err != nil {
		return nil, err
	}
	return &cache, nil
}

func (p *Provider) saveCache(ctx context.Context, query string, result *model.PTGenResult) error {
	jsonData, _ := json.Marshal(result)

	cache := &model.PTGenCache{
		QueryKey:     query,
		ChineseTitle: result.ChineseTitle,
		PosterURL:    result.PosterURL,
		DoubanURL:    result.DoubanURL,
		IMDbURL:      result.IMDBURL,
		BBCode:       result.RawBBCode,
		JSONData:     string(jsonData),
		Source:       result.Source,
	}

	var existing model.PTGenCache
	err := p.db.WithContext(ctx).Where("query_key = ?", query).First(&existing).Error
	if err == nil {
		cache.ID = existing.ID
	}

	return p.db.WithContext(ctx).Save(cache).Error
}

func (p *Provider) CleanExpiredCache(ctx context.Context, retainDays int) (int64, error) {
	if retainDays <= 0 {
		retainDays = 30
	}
	cutoff := time.Now().AddDate(0, 0, -retainDays)
	result := p.db.WithContext(ctx).
		Where("updated_at < ?", cutoff).
		Delete(&model.PTGenCache{})
	return result.RowsAffected, result.Error
}
