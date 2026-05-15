package seeding

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type RuleEvaluator struct {
	db     *gorm.DB
	logger *zap.Logger
}

func NewRuleEvaluator(db *gorm.DB, logger *zap.Logger) *RuleEvaluator {
	return &RuleEvaluator{db: db, logger: logger}
}

type RuleMatch struct {
	Rule     model.DeleteRule
	Records  []model.SeedingTorrentRecord
	Priority int
}

type RuleContext struct {
	Record    *model.SeedingTorrentRecord
	Torrent   *model.TorrentInfo
	FreeSpace int64
	Now       time.Time

	ActiveUploads       int
	ActiveDownloads     int
	GlobalUploadSpeed   float64
	GlobalDownloadSpeed float64

	ScoringScore        float64
	ScoringRank         int
	ScoringTotalInCycle int
	LowScoreCount       int
}

func (rc *RuleContext) fieldValue(key string) (string, bool) {
	rec := rc.Record
	ti := rc.Torrent

	switch key {
	case "site_name":
		return rec.SiteName, true
	case "status":
		return string(rec.Status), true
	case "is_free":
		return fmt.Sprintf("%t", rec.IsFree), true
	case "has_hr":
		return fmt.Sprintf("%t", rec.HasHR), true
	case "hr_seed_time_h":
		return fmt.Sprintf("%d", rec.HRSeedTimeH), true
	case "discount":
		return string(rec.Discount), true
	case "client_id":
		return rec.ClientID, true
	case "torrent_id":
		return rec.TorrentID, true
	case "free_level":
		return rec.FreeLevel, true
	case "source":
		return rec.Source, true
	case "last_action_by":
		return rec.LastActionBy, true
	}

	if ti != nil {
		switch key {
		case "name":
			return ti.Name, true
		case "size":
			return fmt.Sprintf("%d", ti.TotalSize), true
		case "totalSize":
			return fmt.Sprintf("%d", ti.TotalSize), true
		case "progress":
			return fmt.Sprintf("%.4f", ti.Progress), true
		case "state":
			return ti.State, true
		case "uploaded":
			return fmt.Sprintf("%d", ti.Uploaded), true
		case "uploadSpeed":
			return fmt.Sprintf("%d", ti.UploadSpeed), true
		case "downloadSpeed":
			return fmt.Sprintf("%d", ti.DownloadSpeed), true
		case "ratio":
			return fmt.Sprintf("%.4f", ti.Ratio), true
		case "seedingTime", "seed_time":
			return fmt.Sprintf("%d", ti.SeedTime), true
		case "category":
			return ti.Category, true
		case "tags":
			return strings.Join(ti.Tags, ","), true
		case "savePath":
			return ti.SavePath, true
		case "seeder", "num_complete":
			return fmt.Sprintf("%d", ti.NumComplete), true
		case "leecher", "num_incomplete":
			return fmt.Sprintf("%d", ti.NumIncomplete), true
		case "is_finished":
			return fmt.Sprintf("%t", ti.IsFinished), true
		}

		if ti.TotalSize > 0 && key == "downloadUploadRatio" {
			if ti.UploadSpeed > 0 {
				return fmt.Sprintf("%.4f", float64(ti.DownloadSpeed)/float64(ti.UploadSpeed)), true
			}
			return "0", true
		}

		if !ti.AddedAt.IsZero() && key == "addedTime" {
			elapsed := rc.Now.Sub(ti.AddedAt).Seconds()
			return fmt.Sprintf("%.0f", elapsed), true
		}
	}

	switch key {
	case "freeRemainSec":
		if rec.FreeEndAt != nil {
			remain := rec.FreeEndAt.Sub(rc.Now).Seconds()
			if remain < 0 {
				remain = 0
			}
			return fmt.Sprintf("%.0f", remain), true
		}
		return "0", true
	case "hrRemainSec":
		if rec.HasHR && rec.HRSeedTimeH > 0 && ti != nil {
			required := int64(rec.HRSeedTimeH) * 3600
			remain := required - ti.SeedTime
			if remain < 0 {
				remain = 0
			}
			return fmt.Sprintf("%d", remain), true
		}
		return "0", true
	case "freeSpace":
		return fmt.Sprintf("%d", rc.FreeSpace), true
	case "hour":
		return fmt.Sprintf("%d", rc.Now.Hour()), true
	case "activeUploads":
		return fmt.Sprintf("%d", rc.ActiveUploads), true
	case "activeDownloads":
		return fmt.Sprintf("%d", rc.ActiveDownloads), true
	case "globalUploadSpeed":
		return fmt.Sprintf("%.0f", rc.GlobalUploadSpeed), true
	case "globalDownloadSpeed":
		return fmt.Sprintf("%.0f", rc.GlobalDownloadSpeed), true
	case "scoringScore":
		return fmt.Sprintf("%.2f", rc.ScoringScore), true
	case "scoringRank":
		return fmt.Sprintf("%d", rc.ScoringRank), true
	case "scoringCycleID":
		return "", false
	case "lowScoreCount":
		return fmt.Sprintf("%d", rc.LowScoreCount), true
	}

	return "", false
}

func (re *RuleEvaluator) EvaluateRules(ctx context.Context, clientID string, torrentMap map[string]*model.TorrentInfo, freeSpace int64) ([]RuleMatch, error) {
	var rules []model.DeleteRule
	if err := re.db.WithContext(ctx).
		Where("enabled = ?", true).
		Order("priority DESC").
		Find(&rules).Error; err != nil {
		return nil, seedingError(ErrSeedingDB, "query delete rules", err)
	}

	if len(rules) == 0 {
		return nil, nil
	}

	var records []model.SeedingTorrentRecord
	if err := re.db.WithContext(ctx).
		Where("client_id = ? AND status = ?", clientID, model.SeedingStatusSeeding).
		Find(&records).Error; err != nil {
		return nil, seedingError(ErrSeedingDB, "query seeding records", err)
	}

	if len(records) == 0 {
		return nil, nil
	}

	var globalUpSpeed, globalDownSpeed float64
	var state model.SeedingClientState
	if err := re.db.WithContext(ctx).Where("client_id = ?", clientID).First(&state).Error; err == nil {
		globalUpSpeed = state.AvgUploadSpeed
		globalDownSpeed = state.AvgDownloadSpeed
	}

	now := time.Now()
	var matches []RuleMatch
	for _, rule := range rules {
		matched := re.matchRule(ctx, rule, records, torrentMap, freeSpace, now, globalUpSpeed, globalDownSpeed)
		if len(matched) > 0 {
			matches = append(matches, RuleMatch{
				Rule:     rule,
				Records:  matched,
				Priority: rule.Priority,
			})
		}
	}

	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Priority > matches[j].Priority
	})

	return matches, nil
}

func (re *RuleEvaluator) EvaluateRulesSimple(ctx context.Context, clientID string) ([]RuleMatch, error) {
	return re.EvaluateRules(ctx, clientID, nil, -1)
}

type scoringCache struct {
	latestScore map[string]float64
	rankInCycle map[string]int
	totalInCycle map[string]int
	lowScoreCount map[string]int
}

func (re *RuleEvaluator) preloadScoringCache(ctx context.Context, clientID string, records []model.SeedingTorrentRecord) *scoringCache {
	cache := &scoringCache{
		latestScore:   make(map[string]float64, len(records)),
		rankInCycle:   make(map[string]int, len(records)),
		totalInCycle:  make(map[string]int, len(records)),
		lowScoreCount: make(map[string]int, len(records)),
	}
	if re.db == nil || len(records) == 0 {
		return cache
	}

	hashes := make([]string, 0, len(records))
	for _, r := range records {
		hashes = append(hashes, r.InfoHash)
	}

	type latestRow struct {
		InfoHash string
		Score    float64
		CycleID  string
	}
	var latestRows []latestRow
	re.db.WithContext(ctx).Model(&model.ScoringLog{}).
		Select("info_hash, score, cycle_id").
		Where("client_id = ? AND info_hash IN ? AND id IN (?)",
			clientID, hashes,
			re.db.WithContext(ctx).Model(&model.ScoringLog{}).
				Select("MAX(id)").Where("client_id = ? AND info_hash IN ?", clientID, hashes).
				Group("info_hash"),
		).
		Find(&latestRows)

	cycleIDs := make(map[string]struct{})
	for _, row := range latestRows {
		cache.latestScore[row.InfoHash] = row.Score
		if row.CycleID != "" {
			cycleIDs[row.CycleID] = struct{}{}
		}
	}

	if len(cycleIDs) > 0 {
		cidList := make([]string, 0, len(cycleIDs))
		for cid := range cycleIDs {
			cidList = append(cidList, cid)
		}

		type cycleTotal struct {
			CycleID string
			Count   int64
		}
		var totals []cycleTotal
		re.db.WithContext(ctx).Model(&model.ScoringLog{}).
			Select("cycle_id, COUNT(*) as count").
			Where("cycle_id IN ?", cidList).
			Group("cycle_id").
			Find(&totals)
		cycleCountMap := make(map[string]int, len(totals))
		for _, t := range totals {
			cycleCountMap[t.CycleID] = int(t.Count)
		}

		for _, row := range latestRows {
			if row.CycleID != "" {
				cache.totalInCycle[row.InfoHash] = cycleCountMap[row.CycleID]
			}
		}

		for _, row := range latestRows {
			if row.CycleID == "" {
				continue
			}
			var rank int64
			re.db.WithContext(ctx).Model(&model.ScoringLog{}).
				Where("cycle_id = ? AND score <= ?", row.CycleID, row.Score).
				Count(&rank)
			cache.rankInCycle[row.InfoHash] = int(rank)
		}
	}

	cutoff := time.Now().Add(-scoringCutoffHours)
	type lowRow struct {
		InfoHash string
		Count    int64
	}
	var lowRows []lowRow
	re.db.WithContext(ctx).Model(&model.ScoringLog{}).
		Select("info_hash, COUNT(*) as count").
		Where("client_id = ? AND info_hash IN ? AND score < ? AND created_at > ?",
			clientID, hashes, 5.0, cutoff).
		Group("info_hash").
		Find(&lowRows)
	for _, row := range lowRows {
		cache.lowScoreCount[row.InfoHash] = int(row.Count)
	}

	return cache
}

func (re *RuleEvaluator) fillScoringContext(_ context.Context, rc *RuleContext, cache *scoringCache) {
	if cache == nil {
		return
	}
	rc.ScoringScore = cache.latestScore[rc.Record.InfoHash]
	rc.ScoringRank = cache.rankInCycle[rc.Record.InfoHash]
	rc.ScoringTotalInCycle = cache.totalInCycle[rc.Record.InfoHash]
	rc.LowScoreCount = cache.lowScoreCount[rc.Record.InfoHash]
}

func (re *RuleEvaluator) matchRule(ctx context.Context, rule model.DeleteRule, records []model.SeedingTorrentRecord, torrentMap map[string]*model.TorrentInfo, freeSpace int64, now time.Time, globalUpSpeed, globalDownSpeed float64) []model.SeedingTorrentRecord {
	if rule.Conditions == "" && rule.Expr == "" {
		return nil
	}

	useExpr := rule.Type == "expr" && rule.Expr != ""
	conditions := ParseConditions(rule.Conditions)

	activeUploads, activeDownloads := countActiveStates(torrentMap)

	var clientID string
	if len(records) > 0 {
		clientID = records[0].ClientID
	}
	cache := re.preloadScoringCache(ctx, clientID, records)

	var matched []model.SeedingTorrentRecord
	for _, rec := range records {
		var ti *model.TorrentInfo
		if torrentMap != nil {
			ti = torrentMap[rec.InfoHash]
		}
		rc := &RuleContext{
			Record:              &rec,
			Torrent:             ti,
			FreeSpace:           freeSpace,
			Now:                 now,
			ActiveUploads:       activeUploads,
			ActiveDownloads:     activeDownloads,
			GlobalUploadSpeed:   globalUpSpeed,
			GlobalDownloadSpeed: globalDownSpeed,
		}

		re.fillScoringContext(ctx, rc, cache)

		if useExpr {
			ok, err := evalExpr(rule.Expr, rc)
			if err != nil {
				re.logger.Debug("expr eval failed", zap.String("expr", rule.Expr), zap.Error(err))
				continue
			}
			if ok {
				matched = append(matched, rec)
			}
		} else if MatchContext(rc, conditions) {
			matched = append(matched, rec)
		}
	}

	if rule.DeleteNum > 0 && len(matched) > rule.DeleteNum {
		matched = matched[:rule.DeleteNum]
	}

	return matched
}

func countActiveStates(torrentMap map[string]*model.TorrentInfo) (uploads, downloads int) {
	if torrentMap == nil {
		return 0, 0
	}
	for _, ti := range torrentMap {
		switch ti.State {
		case "uploading", "stalledUP", "forcedUP":
			uploads++
		case "downloading", "stalledDL", "forcedDL", "metaDL", "forcedMetaDL":
			downloads++
		}
	}
	return
}

type ruleCondition struct {
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Value    string `json:"value"`
}

func ParseConditions(conditionsJSON string) []ruleCondition {
	if conditionsJSON == "" {
		return nil
	}
	var conditions []ruleCondition
	if err := json.Unmarshal([]byte(conditionsJSON), &conditions); err != nil {
		return nil
	}
	return conditions
}

func MatchContext(rc *RuleContext, conditions []ruleCondition) bool {
	for _, cond := range conditions {
		if !evalCondition(rc, cond) {
			return false
		}
	}
	return len(conditions) > 0
}

func evalCondition(rc *RuleContext, cond ruleCondition) bool {
	fv, known := rc.fieldValue(cond.Field)
	if !known {
		return true
	}

	switch cond.Operator {
	case "equals", "==":
		return fv == cond.Value
	case "not_equals", "!=":
		return fv != cond.Value
	case "contains":
		return strings.Contains(fv, cond.Value)
	case "not_contains":
		return !strings.Contains(fv, cond.Value)
	case "bigger", ">":
		return compareNumeric(fv, cond.Value) > 0
	case "smaller", "<":
		return compareNumeric(fv, cond.Value) < 0
	case "bigger_eq", ">=":
		return compareNumeric(fv, cond.Value) >= 0
	case "smaller_eq", "<=":
		return compareNumeric(fv, cond.Value) <= 0
	case "includeIn":
		return includeIn(fv, cond.Value)
	case "notIncludeIn":
		return !includeIn(fv, cond.Value)
	case "regExp":
		return matchRegExp(fv, cond.Value)
	case "notRegExp":
		return !matchRegExp(fv, cond.Value)
	default:
		return true
	}
}

func compareNumeric(fieldVal, condVal string) int {
	fv, err1 := parseFloat(fieldVal)
	cv, err2 := parseFloat(condVal)
	if err1 != nil || err2 != nil {
		return strings.Compare(fieldVal, condVal)
	}
	switch {
	case fv < cv:
		return -1
	case fv > cv:
		return 1
	default:
		return 0
	}
}

func parseFloat(s string) (float64, error) {
	return strconv.ParseFloat(s, 64)
}

func includeIn(val, list string) bool {
	for _, item := range strings.Split(list, ",") {
		item = strings.TrimSpace(item)
		if item == val {
			return true
		}
	}
	return false
}

var regexCache struct {
	sync.RWMutex
	m map[string]*regexp.Regexp
}

func matchRegExp(s, pattern string) bool {
	regexCache.RLock()
	if regexCache.m == nil {
		regexCache.RUnlock()
		regexCache.Lock()
		if regexCache.m == nil {
			regexCache.m = make(map[string]*regexp.Regexp)
		}
		regexCache.Unlock()
		regexCache.RLock()
	}
	re, ok := regexCache.m[pattern]
	regexCache.RUnlock()
	if !ok {
		var err error
		re, err = regexp.Compile(pattern)
		if err != nil {
			return false
		}
		regexCache.Lock()
		regexCache.m[pattern] = re
		regexCache.Unlock()
	}
	return re.MatchString(s)
}
