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

	siteBlacklistTS time.Time
}

func NewChecker(db *gorm.DB, logger *zap.Logger) *Checker {
	return &Checker{
		db:            db,
		logger:        logger,
		siteBlacklist: SiteCategoryBlacklist,
	}
}

func (c *Checker) Check(ctx context.Context, title string) *Result {
	if title == "" {
		return Pass
	}

	for _, kw := range AdultKeywords {
		if strings.Contains(title, kw) || strings.Contains(strings.ToLower(title), strings.ToLower(kw)) {
			return &Result{Passed: false, Reason: kw, Category: "adult"}
		}
	}

	for _, kw := range ForbiddenTransferKeywords {
		if strings.Contains(title, kw) {
			return &Result{Passed: false, Reason: kw, Category: "forbidden_transfer"}
		}
	}

	for _, g := range ForbiddenGroups {
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
