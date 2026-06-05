package seeding

import (
	"sync"
	"time"
)

type fitTimerKey struct {
	RuleID   uint
	InfoHash string
}

type FitTimer struct {
	mu      sync.RWMutex
	entries map[fitTimerKey]time.Time
}

func NewFitTimer() *FitTimer {
	return &FitTimer{
		entries: make(map[fitTimerKey]time.Time),
	}
}

func (ft *FitTimer) MarkMatched(ruleID uint, infoHash string, now time.Time) {
	key := fitTimerKey{RuleID: ruleID, InfoHash: infoHash}
	ft.mu.Lock()
	defer ft.mu.Unlock()
	if _, exists := ft.entries[key]; !exists {
		ft.entries[key] = now
	}
}

func (ft *FitTimer) MarkMatchedAndReturn(ruleID uint, infoHash string, now time.Time) bool {
	key := fitTimerKey{RuleID: ruleID, InfoHash: infoHash}
	ft.mu.Lock()
	defer ft.mu.Unlock()
	if _, exists := ft.entries[key]; !exists {
		ft.entries[key] = now
		return true
	}
	return false
}

func (ft *FitTimer) ClearUnmatched(ruleID uint, activeHashes map[string]bool) {
	ft.mu.Lock()
	defer ft.mu.Unlock()
	for key := range ft.entries {
		if key.RuleID == ruleID {
			if !activeHashes[key.InfoHash] {
				delete(ft.entries, key)
			}
		}
	}
}

func (ft *FitTimer) ClearUnmatchedAndGet(ruleID uint, activeHashes map[string]bool) []string {
	ft.mu.Lock()
	defer ft.mu.Unlock()
	var removed []string
	for key := range ft.entries {
		if key.RuleID == ruleID {
			if !activeHashes[key.InfoHash] {
				removed = append(removed, key.InfoHash)
				delete(ft.entries, key)
			}
		}
	}
	return removed
}

func (ft *FitTimer) IsFit(ruleID uint, infoHash string, fitTimeSeconds int, now time.Time) bool {
	if fitTimeSeconds <= 0 {
		return true
	}
	key := fitTimerKey{RuleID: ruleID, InfoHash: infoHash}
	ft.mu.RLock()
	matchedAt, exists := ft.entries[key]
	ft.mu.RUnlock()
	if !exists {
		return false
	}
	return now.Sub(matchedAt) >= time.Duration(fitTimeSeconds)*time.Second
}

func (ft *FitTimer) Remove(infoHash string) {
	ft.mu.Lock()
	defer ft.mu.Unlock()
	for key := range ft.entries {
		if key.InfoHash == infoHash {
			delete(ft.entries, key)
		}
	}
}

func (ft *FitTimer) GetMatchedAt(ruleID uint, infoHash string) *time.Time {
	key := fitTimerKey{RuleID: ruleID, InfoHash: infoHash}
	ft.mu.RLock()
	t, exists := ft.entries[key]
	ft.mu.RUnlock()
	if !exists {
		return nil
	}
	return &t
}
