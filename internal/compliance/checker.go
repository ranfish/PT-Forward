package compliance

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type Result struct {
	Passed   bool   `json:"passed"`
	Reason   string `json:"reason"`
	Category string `json:"category"`
}

var Pass = &Result{Passed: true}

type Checker struct {
	db     *gorm.DB
	logger *zap.Logger

	mu               sync.RWMutex
	userKeywords     []string
	userKeywordsTS   time.Time
	siteBlacklist    map[string][]string
	siteBlacklistTS  time.Time

	// §56.21: DB 驱动的合规规则缓存
	rulesAdult     []string
	rulesForbidden []string
	rulesGroup     []string
	rulesTS        time.Time
}

func NewChecker(db *gorm.DB, logger *zap.Logger) *Checker {
	return &Checker{
		db:            db,
		logger:        logger,
		siteBlacklist: SiteCategoryBlacklist,
	}
}

// getComplianceRules §56.21: 从 DB 加载合规规则（5 分钟缓存）。
// DB 为空时 fallback 硬编码（keywords.go）。
func (c *Checker) getComplianceRules(ctx context.Context) (adult, forbidden, group []string) {
	c.mu.RLock()
	if c.rulesAdult != nil && time.Since(c.rulesTS) < userKeywordCacheTTL {
		result := c.rulesAdult
		c.mu.RUnlock()
		return result, c.rulesForbidden, c.rulesGroup
	}
	c.mu.RUnlock()

	var rules []model.ComplianceRule
	if c.db != nil {
		c.db.WithContext(ctx).Find(&rules)
	}

	if len(rules) == 0 {
		// fallback: 硬编码
		return AdultKeywords, ForbiddenTransferKeywords, ForbiddenGroups
	}

	for _, r := range rules {
		switch r.RuleType {
		case model.RuleTypeAdult:
			adult = append(adult, r.Pattern)
		case model.RuleTypeForbiddenKeyword:
			forbidden = append(forbidden, r.Pattern)
		case model.RuleTypeForbiddenGroup:
			group = append(group, r.Pattern)
		}
	}

	// DB 可能缺某些类型，补充硬编码
	if len(adult) == 0 {
		adult = AdultKeywords
	}
	if len(forbidden) == 0 {
		forbidden = ForbiddenTransferKeywords
	}
	if len(group) == 0 {
		group = ForbiddenGroups
	}

	c.mu.Lock()
	c.rulesAdult = adult
	c.rulesForbidden = forbidden
	c.rulesGroup = group
	c.rulesTS = time.Now()
	c.mu.Unlock()

	return adult, forbidden, group
}

// InvalidateCache §56.21 Q6: CRUD 触发缓存失效。
func (c *Checker) InvalidateCache() {
	c.mu.Lock()
	c.rulesAdult = nil
	c.rulesForbidden = nil
	c.rulesGroup = nil
	c.userKeywords = nil
	c.siteBlacklist = nil
	c.mu.Unlock()
}

func (c *Checker) Check(ctx context.Context, title string) *Result {
	if title == "" {
		return Pass
	}

	// §56.21: 从 DB 加载规则（缓存 5 分钟）
	adult, forbidden, group := c.getComplianceRules(ctx)

	for _, kw := range adult {
		if strings.Contains(title, kw) || strings.Contains(strings.ToLower(title), strings.ToLower(kw)) {
			return &Result{Passed: false, Reason: kw, Category: "adult"}
		}
	}

	// qui 精确正则检测（JAV 番号 / 日期格式 / XXX 关键词），补充关键词无法覆盖的结构化模式
	if matched, reason := DetectAdult(title, ""); matched {
		return &Result{Passed: false, Reason: reason, Category: "adult"}
	}

	for _, kw := range forbidden {
		if strings.Contains(title, kw) {
			return &Result{Passed: false, Reason: kw, Category: "forbidden_transfer"}
		}
	}

	for _, g := range group {
		if strings.Contains(title, g) {
			return &Result{Passed: false, Reason: g, Category: "forbidden_group"}
		}
	}

	userKws := c.getUserKeywords(ctx)
	for _, kw := range userKws {
		if kw == "" {
			continue
		}
		if strings.Contains(title, kw) || strings.Contains(strings.ToLower(title), strings.ToLower(kw)) {
			return &Result{Passed: false, Reason: kw, Category: "user_filter"}
		}
	}

	return Pass
}

func (c *Checker) CheckWithSite(ctx context.Context, title, siteName string) *Result {
	r := c.Check(ctx, title)
	if !r.Passed {
		return r
	}

	if siteName == "" {
		return Pass
	}

	blacklist := c.getSiteBlacklist(ctx)
	if cats, ok := blacklist[siteName]; ok && len(cats) > 0 {
		sourceCat := c.lookupSourceCategory(ctx, title, siteName)
		if sourceCat != "" {
			for _, blockedCat := range cats {
				if strings.Contains(sourceCat, blockedCat) {
					return &Result{Passed: false, Reason: siteName + ":" + blockedCat, Category: "site_blacklist"}
				}
			}
		}
	}

	return Pass
}

func (c *Checker) CheckWithTask(ctx context.Context, title string, task *model.ReseedTask) *Result {
	r := c.Check(ctx, title)
	if !r.Passed {
		return r
	}

	if task == nil {
		return Pass
	}

	var titleExcludes []string
	if task.TitleKeywordExcludes != "" {
		_ = json.Unmarshal([]byte(task.TitleKeywordExcludes), &titleExcludes)
	}
	for _, kw := range titleExcludes {
		if kw == "" {
			continue
		}
		if strings.Contains(title, kw) || strings.Contains(strings.ToLower(title), strings.ToLower(kw)) {
			return &Result{Passed: false, Reason: kw, Category: "task_exclude:keyword"}
		}
	}

	var groupExcludes []string
	if task.ReleaseGroupExcludes != "" {
		_ = json.Unmarshal([]byte(task.ReleaseGroupExcludes), &groupExcludes)
	}
	if len(groupExcludes) > 0 {
		groupName := extractReleaseGroup(title)
		for _, g := range groupExcludes {
			if g == "" {
				continue
			}
			if strings.EqualFold(groupName, g) {
				return &Result{Passed: false, Reason: g, Category: "task_exclude:group"}
			}
		}
	}

	var catExcludes []string
	if task.CategoryExcludes != "" {
		_ = json.Unmarshal([]byte(task.CategoryExcludes), &catExcludes)
	}
	if len(catExcludes) > 0 {
		sourceCat := c.lookupSourceCategory(ctx, title, "")
		if sourceCat != "" {
			for _, cat := range catExcludes {
				if cat == "" {
					continue
				}
				if strings.Contains(sourceCat, cat) {
					return &Result{Passed: false, Reason: cat, Category: "task_exclude:category"}
				}
			}
		}
	}

	return Pass
}

const userKeywordCacheTTL = 5 * time.Minute

func (c *Checker) getUserKeywords(ctx context.Context) []string {
	if c.db == nil {
		return nil
	}
	c.mu.RLock()
	if time.Since(c.userKeywordsTS) < userKeywordCacheTTL && c.userKeywords != nil {
		result := c.userKeywords
		c.mu.RUnlock()
		return result
	}
	c.mu.RUnlock()

	var raw string
	if err := c.db.WithContext(ctx).
		Table("system_settings").
		Where("key = ?", "compliance_user_keywords").
		Select("value").Row().Scan(&raw); err != nil {
		return nil
	}

	var kws []string
	if raw != "" {
		_ = json.Unmarshal([]byte(raw), &kws)
	}

	c.mu.Lock()
	c.userKeywords = kws
	c.userKeywordsTS = time.Now()
	c.mu.Unlock()

	return kws
}

func (c *Checker) getSiteBlacklist(ctx context.Context) map[string][]string {
	c.mu.RLock()
	if c.siteBlacklist != nil && time.Since(c.siteBlacklistTS) < userKeywordCacheTTL {
		result := c.siteBlacklist
		c.mu.RUnlock()
		return result
	}
	c.mu.RUnlock()

	result := make(map[string][]string)
	for k, v := range SiteCategoryBlacklist {
		result[k] = v
	}

	var raw string
	if err := c.db.WithContext(ctx).
		Table("system_settings").
		Where("key = ?", "compliance_site_blacklist").
		Select("value").Row().Scan(&raw); err == nil && raw != "" {
		var userBL map[string][]string
		if json.Unmarshal([]byte(raw), &userBL) == nil {
			for site, cats := range userBL {
				result[site] = cats
			}
		}
	}

	c.mu.Lock()
	c.siteBlacklist = result
	c.siteBlacklistTS = time.Now()
	c.mu.Unlock()

	return result
}

func (c *Checker) lookupSourceCategory(ctx context.Context, title, siteName string) string {
	if c.db == nil {
		return ""
	}
	var sourceCat string
	if siteName != "" {
		c.db.WithContext(ctx).Model(&model.RSSTorrentSeen{}).
			Where("site_name = ? AND title = ?", siteName, title).
			Select("source_category").Limit(1).Row().Scan(&sourceCat)
	}
	if sourceCat == "" {
		c.db.WithContext(ctx).Model(&model.RSSTorrentSeen{}).
			Where("title = ?", title).
			Select("source_category").Limit(1).Row().Scan(&sourceCat)
	}
	return sourceCat
}

func extractReleaseGroup(title string) string {
	idx := strings.LastIndex(title, "-")
	if idx < 0 || idx == len(title)-1 {
		return ""
	}
	group := title[idx+1:]
	dotIdx := strings.LastIndex(group, ".")
	if dotIdx >= 0 {
		group = group[:dotIdx]
	}
	return strings.TrimSpace(group)
}
