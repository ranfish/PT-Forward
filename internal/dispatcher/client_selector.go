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
		return s.selectLeastLoad(ctx, candidates)
	case model.SelectionRoundRobin:
		return s.selectRoundRobin(sub.Name, candidates), nil
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

func (s *ClientSelector) selectLeastLoad(ctx context.Context, candidates []string) (string, error) {
	var bestClient string
	var bestCount = -1

	for _, c := range candidates {
		dl, err := s.clientProvider.Get(c)
		if err != nil {
			continue
		}
		torrents, err := dl.GetSeedingTorrents(ctx)
		if err != nil {
			continue
		}
		count := len(torrents)
		if bestCount < 0 || count < bestCount {
			bestCount = count
			bestClient = c
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

	key := subName
	idx := s.roundRobinIdx[key]
	if idx >= len(candidates) {
		idx = 0
	}
	selected := candidates[idx]
	s.roundRobinIdx[key] = idx + 1
	return selected
}
