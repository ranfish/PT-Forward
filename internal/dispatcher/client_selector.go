package dispatcher

import (
	"context"
	"sync"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
)

type ClientSelector struct {
	clientProvider model.DownloaderProvider
	logger         *zap.Logger
	mu             sync.Mutex
	roundRobinIdx  map[string]int
}

func NewClientSelector(cp model.DownloaderProvider, logger *zap.Logger) *ClientSelector {
	return &ClientSelector{
		clientProvider: cp,
		logger:         logger,
		roundRobinIdx:  make(map[string]int),
	}
}

func (s *ClientSelector) Select(ctx context.Context, sub *model.RSSSubscription) (string, error) {
	if len(sub.CandidateClients) == 0 || sub.ClientSelection == model.SelectionFixed || sub.ClientSelection == "" {
		return sub.ClientID, nil
	}

	candidates := s.filterHealthy(ctx, sub.CandidateClients)
	if len(candidates) == 0 {
		s.logger.Warn("client selector: all candidates unhealthy, falling back to fixed",
			zap.String("subscription", sub.Name),
			zap.Strings("candidates", sub.CandidateClients),
		)
		return sub.ClientID, nil
	}

	if len(candidates) == 1 {
		return candidates[0], nil
	}

	switch sub.ClientSelection {
	case model.SelectionMostSpace:
		return s.selectMostSpace(ctx, candidates)
	case model.SelectionLeastLoad:
		return s.selectLeastUpload(ctx, candidates)
	case model.SelectionRoundRobin:
		return s.selectRoundRobin(sub.Name, candidates), nil
	case model.SelectionBestFit:
		return s.selectBestFit(ctx, candidates)
	default:
		return candidates[0], nil
	}
}

func (s *ClientSelector) filterHealthy(ctx context.Context, candidates []string) []string {
	if s.clientProvider == nil {
		return candidates
	}
	var healthy []string
	for _, c := range candidates {
		if _, err := s.clientProvider.Get(c); err == nil {
			healthy = append(healthy, c)
		}
	}
	return healthy
}

func (s *ClientSelector) selectMostSpace(ctx context.Context, candidates []string) (string, error) {
	var bestClient string
	var bestSpace int64 = -1

	for _, c := range candidates {
		dl, err := s.clientProvider.Get(c)
		if err != nil {
			continue
		}
		md, err := dl.GetMainData(ctx)
		if err != nil {
			continue
		}
		if md.FreeSpace > bestSpace {
			bestSpace = md.FreeSpace
			bestClient = c
		}
	}

	if bestClient == "" {
		return candidates[0], nil
	}
	return bestClient, nil
}

func (s *ClientSelector) selectLeastUpload(ctx context.Context, candidates []string) (string, error) {
	var bestClient string
	var bestSpeed int64 = -1

	for _, c := range candidates {
		dl, err := s.clientProvider.Get(c)
		if err != nil {
			continue
		}
		md, err := dl.GetMainData(ctx)
		if err != nil {
			continue
		}
		speed := md.ServerState.UploadSpeed
		if bestSpeed < 0 || speed < bestSpeed {
			bestSpeed = speed
			bestClient = c
		}
	}

	if bestClient == "" {
		return candidates[0], nil
	}
	return bestClient, nil
}

func (s *ClientSelector) selectBestFit(ctx context.Context, candidates []string) (string, error) {
	type candidateScore struct {
		name        string
		uploadSpeed int64
		freeSpace   int64
	}

	scores := make([]candidateScore, 0, len(candidates))
	var maxUpload int64
	var maxSpace int64

	for _, c := range candidates {
		dl, err := s.clientProvider.Get(c)
		if err != nil {
			continue
		}
		md, err := dl.GetMainData(ctx)
		if err != nil {
			continue
		}
		uploadSpeed := md.ServerState.UploadSpeed
		scores = append(scores, candidateScore{name: c, uploadSpeed: uploadSpeed, freeSpace: md.FreeSpace})
		if uploadSpeed > maxUpload {
			maxUpload = uploadSpeed
		}
		if md.FreeSpace > maxSpace {
			maxSpace = md.FreeSpace
		}
	}

	if len(scores) == 0 {
		return candidates[0], nil
	}

	var bestClient string
	var bestScore float64 = -1
	for _, sc := range scores {
		var uploadNorm, spaceNorm float64
		if maxUpload > 0 {
			uploadNorm = 1.0 - float64(sc.uploadSpeed)/float64(maxUpload)
		} else {
			uploadNorm = 1.0
		}
		if maxSpace > 0 {
			spaceNorm = float64(sc.freeSpace) / float64(maxSpace)
		} else {
			spaceNorm = 1.0
		}
		score := 0.6*uploadNorm + 0.4*spaceNorm
		if score > bestScore {
			bestScore = score
			bestClient = sc.name
		}
	}

	if bestClient == "" {
		return candidates[0], nil
	}
	return bestClient, nil
}

func (s *ClientSelector) selectRoundRobin(subName string, candidates []string) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.roundRobinIdx) > 10000 {
		s.roundRobinIdx = make(map[string]int, len(s.roundRobinIdx)/2)
	}

	key := subName
	idx := s.roundRobinIdx[key]
	if idx >= len(candidates) {
		idx = 0
	}
	selected := candidates[idx]
	s.roundRobinIdx[key] = idx + 1
	return selected
}
