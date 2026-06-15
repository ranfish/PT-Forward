package reseed

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
)

type domainResolver struct {
	domainToName map[string]string
	nameToDomain map[string]string
}

func buildDomainResolver(ps *preloadedSites) *domainResolver {
	if ps == nil {
		return &domainResolver{
			domainToName: make(map[string]string),
			nameToDomain: make(map[string]string),
		}
	}
	dr := &domainResolver{
		domainToName: make(map[string]string),
		nameToDomain: make(map[string]string),
	}
	for name, site := range ps.siteLimits {
		if site == nil {
			continue
		}
		primary := normalizeDomain(site.Domain)
		if primary == "" {
			primary = normalizeDomain(site.BaseURL)
		}
		if primary != "" {
			dr.domainToName[primary] = name
			dr.nameToDomain[name] = primary
		}
		if site.BaseURL != "" {
			if d := normalizeDomain(site.BaseURL); d != "" && d != primary {
				dr.domainToName[d] = name
			}
		}
		if site.AlternativeDomains != "" {
			var alts []string
			if err := json.Unmarshal([]byte(site.AlternativeDomains), &alts); err == nil {
				for _, alt := range alts {
					if d := normalizeDomain(alt); d != "" {
						dr.domainToName[d] = name
					}
				}
			}
		}
	}
	return dr
}

func normalizeDomain(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		u, err := url.Parse(raw)
		if err == nil && u.Host != "" {
			return strings.ToLower(u.Host)
		}
	}
	return strings.ToLower(raw)
}

func (dr *domainResolver) toName(domain string) string {
	if dr == nil {
		return domain
	}
	if name, ok := dr.domainToName[strings.ToLower(domain)]; ok {
		return name
	}
	return domain
}

func (dr *domainResolver) toDomain(name string) string {
	if dr == nil {
		return name
	}
	if domain, ok := dr.nameToDomain[name]; ok {
		return domain
	}
	return name
}

type cloudFPCache struct {
	data    map[string][]model.CloudFPMatch
	deleted sync.Map
}

func (c *cloudFPCache) lookup(piecesHash, siteName string) *model.CloudFPMatch {
	if c == nil {
		return nil
	}
	matches, ok := c.data[piecesHash]
	if !ok {
		return nil
	}
	for i := range matches {
		m := &matches[i]
		if m.SiteName != siteName {
			continue
		}
		if _, deleted := c.deleted.Load(m.SiteName + ":" + m.TorrentID); deleted {
			continue
		}
		return m
	}
	return nil
}

func (c *cloudFPCache) markDeleted(site, torrentID string) {
	if c == nil {
		return
	}
	c.deleted.Store(site+":"+torrentID, true)
}

func (e *Engine) preloadCloudFingerprints(ctx context.Context, fc *fpCache, dr *domainResolver) *cloudFPCache {
	if e.cloudFPService == nil || !e.cloudFPService.IsEnabled() {
		return nil
	}

	seen := make(map[string]bool)
	var allHashes []string
	for _, fp := range fc.byKey {
		if fp.PiecesHash != "" && !seen[fp.PiecesHash] {
			seen[fp.PiecesHash] = true
			allHashes = append(allHashes, fp.PiecesHash)
		}
	}

	if len(allHashes) == 0 {
		return nil
	}

	matches, err := e.cloudFPService.BatchLookup(ctx, allHashes, nil)
	if err != nil {
		e.logger.Warn("L1 云端指纹查询失败", zap.Int("hashes", len(allHashes)), zap.Error(err))
		return nil
	}

	translated := make(map[string][]model.CloudFPMatch, len(matches))
	for hash, list := range matches {
		for _, m := range list {
			m.SiteName = dr.toName(m.SiteName)
			translated[hash] = append(translated[hash], m)
		}
	}

	e.logger.Info("L1 云端指纹预加载完成",
		zap.Int("hashes", len(allHashes)),
		zap.Int("matched", len(translated)),
	)
	return &cloudFPCache{data: translated}
}

func (e *Engine) matchLayer1FromCloudCache(sourceInfoHash, sourceSiteName, siteName string, fc *fpCache, cache *cloudFPCache) *model.Candidate {
	if cache == nil {
		return nil
	}
	sourceFP := fc.get(sourceInfoHash, sourceSiteName)
	if sourceFP == nil || sourceFP.PiecesHash == "" {
		return nil
	}

	match := cache.lookup(sourceFP.PiecesHash, siteName)
	if match == nil {
		return nil
	}

	return &model.Candidate{
		TargetSite:      match.SiteName,
		TargetTorrentID: match.TorrentID,
		Confidence:      0.95,
		MatchMethod:     "cloud_fingerprint",
	}
}

type deleteReporter struct {
	service model.CloudFPService
	logger  *zap.Logger
	ch      chan model.CloudFPDeleteReport
	done    chan struct{}
	dedup   map[string]bool
	mu      sync.Mutex
}

func newDeleteReporter(svc model.CloudFPService, logger *zap.Logger) *deleteReporter {
	r := &deleteReporter{
		service: svc,
		logger:  logger.With(zap.String("component", "delete_reporter")),
		ch:      make(chan model.CloudFPDeleteReport, 1000),
		done:    make(chan struct{}),
		dedup:   make(map[string]bool),
	}
	go r.run()
	return r
}

func (r *deleteReporter) Report(site, torrentID string) {
	key := site + ":" + torrentID
	r.mu.Lock()
	if r.dedup[key] {
		r.mu.Unlock()
		return
	}
	r.dedup[key] = true
	r.mu.Unlock()

	select {
	case r.ch <- model.CloudFPDeleteReport{SiteName: site, TorrentID: torrentID}:
	default:
		r.logger.Warn("删除上报队列已满，丢弃")
	}
}

func (r *deleteReporter) run() {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	var batch []model.CloudFPDeleteReport
	for {
		select {
		case report := <-r.ch:
			batch = append(batch, report)
			if len(batch) >= 50 {
				r.flush(batch)
				batch = batch[:0]
			}
		case <-ticker.C:
			if len(batch) > 0 {
				r.flush(batch)
				batch = batch[:0]
			}
		case <-r.done:
			for {
				select {
				case report := <-r.ch:
					batch = append(batch, report)
				default:
					if len(batch) > 0 {
						r.flush(batch)
					}
					return
				}
			}
		}
	}
}

func (r *deleteReporter) flush(batch []model.CloudFPDeleteReport) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := r.service.ReportDeleted(ctx, batch); err != nil {
		r.logger.Warn("删除上报 flush 失败", zap.Int("count", len(batch)), zap.Error(err))
	} else {
		r.logger.Info("删除上报 flush 完成", zap.Int("count", len(batch)))
	}
}

func (r *deleteReporter) Close() {
	close(r.done)
}

type contributeReporter struct {
	service    model.CloudFPService
	logger     *zap.Logger
	ch         chan model.CloudFPContribute
	done       chan struct{}
	dedup      map[string]bool
	dedupReset time.Time
	mu         sync.Mutex
}

func newContributeReporter(svc model.CloudFPService, logger *zap.Logger) *contributeReporter {
	r := &contributeReporter{
		service:    svc,
		logger:     logger.With(zap.String("component", "contribute_reporter")),
		ch:         make(chan model.CloudFPContribute, 5000),
		done:       make(chan struct{}),
		dedup:      make(map[string]bool),
		dedupReset: time.Now(),
	}
	go r.run()
	return r
}

func (r *contributeReporter) Upload(records []model.CloudFPContribute) {
	r.mu.Lock()
	if time.Since(r.dedupReset) > 24*time.Hour {
		r.dedup = make(map[string]bool)
		r.dedupReset = time.Now()
	}
	var newRecords []model.CloudFPContribute
	for _, rec := range records {
		key := rec.SiteName + ":" + rec.TorrentID
		if r.dedup[key] {
			continue
		}
		r.dedup[key] = true
		newRecords = append(newRecords, rec)
	}
	r.mu.Unlock()

	if len(newRecords) == 0 {
		return
	}
	for _, rec := range newRecords {
		select {
		case r.ch <- rec:
		default:
			r.logger.Warn("贡献数据队列已满，丢弃")
			return
		}
	}
}

func (r *contributeReporter) run() {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	var batch []model.CloudFPContribute
	for {
		select {
		case record := <-r.ch:
			batch = append(batch, record)
			if len(batch) >= 500 {
				r.flush(batch)
				batch = batch[:0]
			}
		case <-ticker.C:
			if len(batch) > 0 {
				r.flush(batch)
				batch = batch[:0]
			}
		case <-r.done:
			for {
				select {
				case record := <-r.ch:
					batch = append(batch, record)
				default:
					if len(batch) > 0 {
						r.flush(batch)
					}
					return
				}
			}
		}
	}
}

func (r *contributeReporter) flush(batch []model.CloudFPContribute) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := r.service.UploadRecords(ctx, batch); err != nil {
		r.logger.Warn("贡献数据 flush 失败", zap.Int("count", len(batch)), zap.Error(err))
	} else {
		r.logger.Info("贡献数据 flush 完成", zap.Int("count", len(batch)))
	}
}

func (r *contributeReporter) Close() {
	close(r.done)
}
