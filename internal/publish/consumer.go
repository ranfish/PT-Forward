package publish

import (
	"context"
	"fmt"

	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/pusher"
	"go.uber.org/zap"
)

func (p *Pipeline) OnPushed(ctx context.Context, event *pusher.PushedEvent) {
	if event == nil {
		return
	}

	if event.Role != "download" && event.Role != "source" {
		return
	}

	role := model.RoleDownload
	if event.Role == "source" {
		role = model.RoleSource
	}

	candidate := &model.PublishCandidate{
		SourceSite:      event.SiteName,
		SourceTorrentID: event.TorrentID,
		InfoHash:        event.InfoHash,
		TorrentName:     event.Title,
		Size:            event.Size,
		Discount:        event.Discount,
		HasHR:           event.HasHR,
		PublishStatus:   model.CandidatePending,
		Role:            role,
		ClientID:        event.ClientID,
	}

	if err := p.CreateCandidate(ctx, candidate); err != nil {
		p.logger.Warn("OnPushed: create publish candidate failed",
			zap.String("site", event.SiteName),
			zap.String("torrent_id", event.TorrentID),
			zap.Error(err))
		return
	}

	p.logger.Debug("OnPushed: publish candidate created",
		zap.String("client_id", event.ClientID),
		zap.String("info_hash", event.InfoHash),
		zap.String("role", event.Role))
}

var _ = fmt.Sprintf
