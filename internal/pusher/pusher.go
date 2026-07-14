package pusher

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ranfish/pt-forward/internal/httpclient"
	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type PushRequest struct {
	ClientID      string
	SiteName      string
	TorrentID     string
	InfoHash      string
	Title         string
	HasHR         bool
	Discount      model.DiscountLevel
	IsFree        bool
	FreeEndAt     *time.Time
	SavePath      string
	Category      string
	Tags          string
	AddPaused     bool
	AutoTMM       bool
	UploadLimitKB int64
	DownloadLimitKB int64
}

type PushResult struct {
	Success      bool
	InfoHash     string
	AlreadyExist bool
	SkipReason   string
	Error        error
}

type PushedEvent struct {
	ClientID       string
	SiteName       string
	TorrentID      string
	InfoHash       string
	Title          string
	Size           int64
	Role           string
	Discount       model.DiscountLevel
	HasHR          bool
	IsFree         bool
	FreeEndAt      *time.Time
	AutoReseed     bool
	ReseedClientIDs []string
	PushedAt       time.Time
}

type SiteProvider interface {
	GetSiteConfig(ctx context.Context, siteName string) (*model.SiteConfig, error)
	GetAdapter(ctx context.Context, siteName string) (model.SiteAdapter, error)
}

type ClientProvider interface {
	Get(clientID string) (model.DownloaderClient, error)
}

type Pusher struct {
	db            *gorm.DB
	logger        *zap.Logger
	siteProvider  SiteProvider
	clientProvider ClientProvider
	mu            sync.Mutex
}

func NewPusher(db *gorm.DB, logger *zap.Logger) *Pusher {
	return &Pusher{
		db:     db,
		logger: logger.With(zap.String("component", "pusher")),
	}
}

func (p *Pusher) SetSiteProvider(sp SiteProvider) {
	p.siteProvider = sp
}

func (p *Pusher) SetClientProvider(cp ClientProvider) {
	p.clientProvider = cp
}

func (p *Pusher) Push(ctx context.Context, req *PushRequest) *PushResult {
	if req == nil {
		return &PushResult{Error: fmt.Errorf("nil push request")}
	}

	result := &PushResult{InfoHash: req.InfoHash}

	if p.clientProvider == nil {
		result.Error = fmt.Errorf("client provider not configured")
		return result
	}
	if p.siteProvider == nil {
		result.Error = fmt.Errorf("site provider not configured")
		return result
	}

	dlClient, err := p.clientProvider.Get(req.ClientID)
	if err != nil {
		result.Error = fmt.Errorf("get downloader client: %w", err)
		return result
	}

	if exists, err := dlClient.CheckExists(ctx, req.InfoHash); err == nil && exists {
		result.AlreadyExist = true
		result.SkipReason = "already exists in downloader"
		p.logger.Debug("push: torrent already exists",
			zap.String("client", req.ClientID),
			zap.String("info_hash", req.InfoHash))
		return result
	}

	siteCfg, err := p.siteProvider.GetSiteConfig(ctx, req.SiteName)
	if err != nil || siteCfg == nil {
		result.Error = fmt.Errorf("get site config for %s: %w", req.SiteName, err)
		return result
	}

	adapter, err := p.siteProvider.GetAdapter(ctx, req.SiteName)
	if err != nil || adapter == nil {
		result.Error = fmt.Errorf("get adapter for %s: %w", req.SiteName, err)
		return result
	}

	if req.IsFree && req.FreeEndAt != nil && req.FreeEndAt.Before(time.Now()) {
		result.SkipReason = "free period expired"
		return result
	}

	if req.IsFree && req.FreeEndAt == nil && req.Discount != model.DiscountAssumeFree {
		recheckCtx, recheckCancel := context.WithTimeout(ctx, 10*time.Second)
		discResult, discErr := adapter.DetectDiscount(recheckCtx, siteCfg, req.TorrentID)
		recheckCancel()
		if discErr != nil {
			result.SkipReason = "discount recheck failed"
			return result
		}
		if discResult == nil || !discResult.Level.IsFree() {
			result.SkipReason = "free status expired before push"
			return result
		}
	}

	if req.HasHR {
		hrStrategy := "protect"
		if siteCfg.HRStrategy == "skip" || siteCfg.HRStrategy == "ignore" {
			hrStrategy = siteCfg.HRStrategy
		}
		if hrStrategy == "protect" {
			result.SkipReason = "HR protected, skipping"
			return result
		}
	}

	dlCtx, dlCancel := context.WithTimeout(ctx, 30*time.Second)
	torrentData, err := adapter.DownloadTorrent(dlCtx, siteCfg, req.TorrentID)
	dlCancel()
	if err != nil || len(torrentData) == 0 {
		result.Error = fmt.Errorf("download torrent: %w", err)
		return result
	}

	opts := model.AddTorrentOptions{
		SavePath: req.SavePath,
		Category: req.Category,
		Paused:   req.AddPaused,
		AutoTMM:  req.AutoTMM,
	}
	if req.Tags != "" {
		opts.Tags = strings.Split(req.Tags, ",")
	}
	if req.UploadLimitKB > 0 {
		opts.UploadLimit = req.UploadLimitKB * 1024
	}
	if req.DownloadLimitKB > 0 {
		opts.DownloadLimit = req.DownloadLimitKB * 1024
	}

	addResult, err := dlClient.AddFromFile(ctx, torrentData, opts)
	if err != nil {
		result.Error = fmt.Errorf("add from file: %w", err)
		return result
	}

	if addResult != nil && addResult.InfoHash != "" && addResult.InfoHash != req.InfoHash {
		p.logger.Info("push: info_hash mismatch, using actual hash",
			zap.String("expected", req.InfoHash),
			zap.String("actual", addResult.InfoHash),
			zap.String("site", req.SiteName),
			zap.String("torrent_id", req.TorrentID))
		result.InfoHash = addResult.InfoHash
	} else if addResult != nil && addResult.InfoHash != "" {
		result.InfoHash = addResult.InfoHash
	}

	result.Success = true
	p.logger.Debug("push: torrent pushed successfully",
		zap.String("client_id", req.ClientID),
		zap.String("info_hash", result.InfoHash),
		zap.String("site", req.SiteName),
		zap.String("torrent_id", req.TorrentID))

	return result
}

func (p *Pusher) PushBatch(ctx context.Context, reqs []*PushRequest) []*PushResult {
	results := make([]*PushResult, 0, len(reqs))
	for _, req := range reqs {
		select {
		case <-ctx.Done():
			results = append(results, &PushResult{Error: ctx.Err()})
			return results
		default:
		}
		r := p.Push(ctx, req)
		results = append(results, r)
	}
	return results
}

func (p *Pusher) GetClientRole(ctx context.Context, clientID string) string {
	var cfg model.ClientConfig
	idNum, idErr := parseUint(clientID)
	if idErr == nil {
		if err := p.db.WithContext(ctx).Where("name = ? OR id = ?", clientID, idNum).First(&cfg).Error; err == nil {
			return cfg.Role
		}
	} else {
		if err := p.db.WithContext(ctx).Where("name = ?", clientID).First(&cfg).Error; err == nil {
			return cfg.Role
		}
	}
	return ""
}

func parseUint(s string) (uint, error) {
	var n uint
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("not a number")
		}
		n = n*10 + uint(c-'0')
	}
	return n, nil
}

var _ = httpclient.DrainBody
var _ = strings.TrimSpace
