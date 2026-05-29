package filter

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
)

var (
	regexCache   = make(map[string]*regexp.Regexp)
	regexCacheMu sync.RWMutex
)

func compileRegex(pattern string) (*regexp.Regexp, error) {
	regexCacheMu.RLock()
	re, ok := regexCache[pattern]
	regexCacheMu.RUnlock()
	if ok {
		return re, nil
	}

	if strings.Contains(pattern, "(.+)+") || strings.Contains(pattern, "(.*)*") ||
		strings.Contains(pattern, "(.+)*)") || strings.Contains(pattern, "(.*)+)") {
		return nil, fmt.Errorf("potentially catastrophic regex pattern rejected: %s", pattern)
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	if len(regexCache) >= 5000 {
		for k := range regexCache {
			delete(regexCache, k)
			break
		}
	}
	regexCache[pattern] = re
	return re, nil
}

type Engine struct {
	repo   *Repository
	logger *zap.Logger
}

func NewEngine(repo *Repository, logger *zap.Logger) *Engine {
	return &Engine{repo: repo, logger: logger}
}

type MatchResult struct {
	Matched  bool
	RuleName string
	RuleType string
	SavePath string
	Category string
	Tags     string
	Reject   bool
}

type EvalContext struct {
	Title         string
	Size          int64
	Category      string
	Tags          []string
	Uploader      string
	SiteName      string
	Free          bool
	DiscountLevel string
}

func (e *Engine) Match(ctx context.Context, evalCtx *EvalContext) (*MatchResult, error) {
	rules, err := e.repo.ListEnabled(ctx, "")
	if err != nil {
		return nil, err
	}

	if len(rules) == 0 {
		return &MatchResult{Matched: false}, nil
	}

	for i := range rules {
		rule := &rules[i]
		if matchRule(rule, evalCtx) {
			isReject := rule.RuleType != "accept"
			return &MatchResult{
				Matched:  true,
				RuleName: rule.Name,
				RuleType: rule.RuleType,
				SavePath: rule.SavePath,
				Category: rule.Category,
				Tags:     rule.Tags,
				Reject:   isReject,
			}, nil
		}
	}

	return &MatchResult{Matched: false}, nil
}

func (e *Engine) MatchByIDs(ctx context.Context, ruleIDs []uint, evalCtx *EvalContext) (*MatchResult, error) {
	for _, id := range ruleIDs {
		rule, err := e.repo.GetByID(ctx, id)
		if err != nil || rule == nil {
			continue
		}
		if matchRule(rule, evalCtx) {
			return &MatchResult{
				Matched:  true,
				RuleName: rule.Name,
				RuleType: rule.RuleType,
				SavePath: rule.SavePath,
				Category: rule.Category,
				Tags:     rule.Tags,
				Reject:   rule.RuleType == "reject",
			}, nil
		}
	}
	return &MatchResult{Matched: false}, nil
}

func matchRule(rule *model.FilterRule, ctx *EvalContext) bool {
	if len(rule.Conditions) == 0 {
		return false
	}

	for _, cond := range rule.Conditions {
		if !matchCondition(cond, ctx) {
			return false
		}
	}
	return true
}

func matchCondition(cond model.RuleCondition, ctx *EvalContext) bool {
	return MatchConditionExport(cond, ctx)
}

func MatchConditionExport(cond model.RuleCondition, ctx *EvalContext) bool {
	actual := getField(ctx, cond.Key)

	switch cond.CompareType {
	case model.CompareEquals:
		return strings.EqualFold(actual, cond.Value)
	case model.CompareContain:
		return strings.Contains(strings.ToLower(actual), strings.ToLower(cond.Value))
	case model.CompareNotContain:
		return !strings.Contains(strings.ToLower(actual), strings.ToLower(cond.Value))
	case model.CompareBigger:
		return compareNumbers(actual, cond.Value) > 0
	case model.CompareSmaller:
		return compareNumbers(actual, cond.Value) < 0
	case model.CompareRegExp:
		re, err := compileRegex(cond.Value)
		if err != nil {
			return false
		}
		return matchRegexWithTimeout(re, actual)
	case model.CompareNotRegExp:
		re, err := compileRegex(cond.Value)
		if err != nil {
			return false
		}
		return !matchRegexWithTimeout(re, actual)
	case model.CompareIncludeIn:
		values := strings.Split(cond.Value, ",")
		for _, v := range values {
			if strings.EqualFold(strings.TrimSpace(v), actual) {
				return true
			}
		}
		return false
	case model.CompareNotIncludeIn:
		values := strings.Split(cond.Value, ",")
		for _, v := range values {
			if strings.EqualFold(strings.TrimSpace(v), actual) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func getField(ctx *EvalContext, key string) string {
	switch strings.ToLower(key) {
	case "title":
		return ctx.Title
	case "size":
		return fmt.Sprintf("%d", ctx.Size)
	case "category":
		return ctx.Category
	case "uploader":
		return ctx.Uploader
	case "sitename", "site":
		return ctx.SiteName
	case "free":
		return strconv.FormatBool(ctx.Free)
	case "tags":
		return strings.Join(ctx.Tags, ",")
	case "discount_level":
		return ctx.DiscountLevel
	default:
		return ""
	}
}

func compareNumbers(a, b string) int {
	aNum, aErr := strconv.ParseFloat(a, 64)
	bNum, bErr := strconv.ParseFloat(b, 64)
	if aErr != nil || bErr != nil {
		return strings.Compare(a, b)
	}
	if aNum < bNum {
		return -1
	}
	if aNum > bNum {
		return 1
	}
	return 0
}

func matchRegexWithTimeout(re *regexp.Regexp, s string) bool {
	return re.MatchString(s)
}
