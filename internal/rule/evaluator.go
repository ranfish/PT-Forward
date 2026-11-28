package rule

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
	"github.com/ranfish/pt-forward/internal/model"
)

type Context struct {
	InfoHash      string
	Name          string
	SavePath      string
	TotalSize     int64
	TrackerURL    string
	Category      string
	Tags          []string
	Ratio         float64
	Progress      float64
	UploadSpeed   int64
	DownloadSpeed int64
	Uploaded      int64
	Downloaded    int64
	SeedTime      int64
	State         string
	ErrorMsg      string
	NumComplete   int
	NumIncomplete int
	IsFinished    bool
	IsPaused      bool
	AddedAt       time.Time

	SiteName        string
	ClientUID       uint
	TorrentID       string
	Status          string
	Discount        string
	FreeLevel       string
	Source          string
	LastActionBy    string
	Unregistered    bool
	UnregisteredMsg string
	SubscriptionID  string
	IsFree          bool
	HasHR           bool
	HRSeedTimeH     int
	FreeEndAt       *time.Time

	FreeSpace           int64
	TotalSpace          int64
	Now                 time.Time
	ActiveUploads       int
	ActiveDownloads     int
	GlobalUploadSpeed   float64
	GlobalDownloadSpeed float64

	ScoringScore  float64
	ScoringRank   int
	LowScoreCount int
}

func (c *Context) fieldValue(key string) (string, bool) {
	switch key {
	case "site_name":
		return c.SiteName, true
	case "status":
		return c.Status, true
	case "is_free":
		return fmt.Sprintf("%t", c.IsFree), true
	case "has_hr":
		return fmt.Sprintf("%t", c.HasHR), true
	case "hr_seed_time_h":
		return fmt.Sprintf("%d", c.HRSeedTimeH), true
	case "discount":
		return c.Discount, true
	case "client_id":
		return fmt.Sprintf("%d", c.ClientUID), true
	case "torrent_id":
		return c.TorrentID, true
	case "free_level":
		return c.FreeLevel, true
	case "source":
		return c.Source, true
	case "last_action_by":
		return c.LastActionBy, true
	case "unregistered":
		return fmt.Sprintf("%t", c.Unregistered), true
	case "unregistered_msg":
		return c.UnregisteredMsg, true
	case "subscription_id":
		return c.SubscriptionID, true
	case "name":
		return c.Name, true
	case "size", "totalSize":
		return fmt.Sprintf("%d", c.TotalSize), true
	case "progress":
		return fmt.Sprintf("%.4f", c.Progress), true
	case "state":
		return c.State, true
	case "uploaded":
		return fmt.Sprintf("%d", c.Uploaded), true
	case "uploadSpeed":
		return fmt.Sprintf("%d", c.UploadSpeed), true
	case "downloadSpeed":
		return fmt.Sprintf("%d", c.DownloadSpeed), true
	case "ratio":
		return fmt.Sprintf("%.4f", c.Ratio), true
	case "seedingTime", "seed_time":
		return fmt.Sprintf("%d", c.SeedTime), true
	case "category":
		return c.Category, true
	case "tags":
		return strings.Join(c.Tags, ","), true
	case "savePath":
		return c.SavePath, true
	case "seeder", "num_complete":
		return fmt.Sprintf("%d", c.NumComplete), true
	case "leecher", "num_incomplete":
		return fmt.Sprintf("%d", c.NumIncomplete), true
	case "is_finished":
		return fmt.Sprintf("%t", c.IsFinished), true
	case "is_paused":
		return fmt.Sprintf("%t", c.IsPaused), true
	case "downloaded":
		return fmt.Sprintf("%d", c.Downloaded), true
	case "error":
		return c.ErrorMsg, true
	case "tracker_url":
		return c.TrackerURL, true
	case "downloadUploadRatio":
		// 15 刀1（v0.0.1661）: 零上传=+∞（契约 docs31:7757——黑车
		// 「只下不传」是该键存在的核心需求·原返回 "0" 语义完全反转：
		// downloadUploadRatio > 阈值 在黑车典型形态下恰不命中）。
		// +Inf 经 fmt "%.4f" 产出 "+Inf"·compareNumeric 的 ParseFloat
		// 天然解析（实测 +Inf>5=true）。去 TotalSize>0 门槛（文档
		// 未声明·速度比与种子大小无关——零大小种子〔刚建任务〕有
		// 上传速度原也被归零）。语义单点=DownloadUploadRatioOf。
		return fmt.Sprintf("%.4f", DownloadUploadRatioOf(c.DownloadSpeed, c.UploadSpeed)), true
	case "addedTime":
		if !c.AddedAt.IsZero() {
			return fmt.Sprintf("%.0f", c.Now.Sub(c.AddedAt).Seconds()), true
		}
		return "0", true
	case "freeRemainSec":
		if c.FreeEndAt != nil {
			remain := c.FreeEndAt.Sub(c.Now).Seconds()
			if remain < 0 {
				remain = 0
			}
			return fmt.Sprintf("%.0f", remain), true
		}
		return "0", true
	case "hrRemainSec":
		if c.HasHR && c.HRSeedTimeH > 0 {
			required := int64(c.HRSeedTimeH) * 3600
			remain := required - c.SeedTime
			if remain < 0 {
				remain = 0
			}
			return fmt.Sprintf("%d", remain), true
		}
		return "0", true
	case "freeSpace":
		return fmt.Sprintf("%d", c.FreeSpace), true
	case "totalSpace":
		return fmt.Sprintf("%d", c.TotalSpace), true
	case "diskUsedPercent":
		if c.TotalSpace > 0 {
			pct := float64(c.TotalSpace-c.FreeSpace) / float64(c.TotalSpace) * 100
			return fmt.Sprintf("%.2f", pct), true
		}
		return "0", true
	case "hour":
		return fmt.Sprintf("%d", c.Now.Hour()), true
	case "activeUploads":
		return fmt.Sprintf("%d", c.ActiveUploads), true
	case "activeDownloads":
		return fmt.Sprintf("%d", c.ActiveDownloads), true
	case "globalUploadSpeed":
		return fmt.Sprintf("%.0f", c.GlobalUploadSpeed), true
	case "globalDownloadSpeed":
		return fmt.Sprintf("%.0f", c.GlobalDownloadSpeed), true
	case "scoringScore":
		return fmt.Sprintf("%.2f", c.ScoringScore), true
	case "scoringRank":
		return fmt.Sprintf("%d", c.ScoringRank), true
	case "lowScoreCount":
		return fmt.Sprintf("%d", c.LowScoreCount), true
	}
	return "", false
}

type Condition struct {
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Value    string `json:"value"`
	// §59.596: 前端时长单位键（minutes/hours/days）——求值零消费（value 恒
	// 字段底层单位）·仅供回显还原「数值+单位」UI 原子。老版本 json.Unmarshal
	// 忽略未知键·双向版本兼容。
	Unit string `json:"unit,omitempty"`
}

func ParseConditions(conditionsJSON string) []Condition {
	if conditionsJSON == "" {
		return nil
	}
	var conditions []Condition
	if err := json.Unmarshal([]byte(conditionsJSON), &conditions); err != nil {
		return nil
	}
	return conditions
}

func MatchRule(c *Context, r *model.DeleteRule) bool {
	if r.Type == "expr" && r.Expr != "" {
		ok, err := evalExpr(r.Expr, c)
		if err != nil {
			return false
		}
		return ok
	}
	conditions := ParseConditions(r.Conditions)
	if len(conditions) == 0 {
		return false
	}
	if r.Logic == "or" {
		for _, cond := range conditions {
			if evalCondition(c, cond) {
				return true
			}
		}
		return false
	}
	for _, cond := range conditions {
		if !evalCondition(c, cond) {
			return false
		}
	}
	return true
}

func evalCondition(c *Context, cond Condition) bool {
	fv, known := c.fieldValue(cond.Field)
	if !known {
		// 15 刀3 F-01（v0.0.1661）: 未知字段统一拒绝（原 default:
		// return true 放行——拼错操作符落入 default 致条件恒真·
		// 规则静默生效〔删种规则即误删风险〕）。29 实态取证：6 条
		// 规则字段/操作符全已知——改拒绝零兼容风险。配置期拦截=
		// seeding.ValidateConditions（字段+操作符白名单校验·拼错
		// 当场 400 回显）；本分支为运行期兜底（DB 直改绕过防）。
		return false
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
		// 15 刀3 F-01（v0.0.1661）: 未知操作符拒绝（原 true 放行——
		// 拼错操作符恒真·删种规则误删风险）。与 seeding 包统一口径；
		// 配置期拦截=seeding.ValidateConditions 操作符白名单。
		return false
	}
}

func compareNumeric(fieldVal, condVal string) int {
	fv, err1 := strconv.ParseFloat(fieldVal, 64)
	cv, err2 := strconv.ParseFloat(condVal, 64)
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

func includeIn(val, list string) bool {
	for _, item := range strings.Split(list, ",") {
		if strings.TrimSpace(item) == val {
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

type ExprEnv struct {
	SiteName      string  `expr:"siteName"`
	Status        string  `expr:"status"`
	IsFree        bool    `expr:"isFree"`
	HasHR         bool    `expr:"hasHR"`
	HRSeedTimeH   int     `expr:"hrSeedTimeH"`
	Discount      string  `expr:"discount"`
	ClientUID     uint    `expr:"clientUID"`
	Name          string  `expr:"name"`
	Size          int64   `expr:"size"`
	TotalSize     int64   `expr:"totalSize"`
	Progress      float64 `expr:"progress"`
	State         string  `expr:"state"`
	Uploaded      int64   `expr:"uploaded"`
	UploadSpeed   int64   `expr:"uploadSpeed"`
	DownloadSpeed int64   `expr:"downloadSpeed"`
	Ratio         float64 `expr:"ratio"`
	// 15 刀2（v0.0.1661）: V-02 根修——文档 42 键清单声明该字段但
	// expr 环境缺失（expr.Compile 实测 unknown name·按文档配置的
	// 规则编译期即失败）。与声明式路径（fieldValue）同语义：零上传
	// =+Inf（V-01 同批）。
	DownloadUploadRatio float64 `expr:"downloadUploadRatio"`
	SeedingTime         int64   `expr:"seedingTime"`
	Category            string  `expr:"category"`
	Tags                string  `expr:"tags"`
	SavePath            string  `expr:"savePath"`
	Seeder              int     `expr:"seeder"`
	Leecher             int     `expr:"leecher"`
	IsFinished          bool    `expr:"isFinished"`
	AddedTime           float64 `expr:"addedTime"`
	FreeRemainSec       float64 `expr:"freeRemainSec"`
	HRRemainSec         int64   `expr:"hrRemainSec"`
	FreeSpace           int64   `expr:"freeSpace"`
	TotalSpace          int64   `expr:"totalSpace"`
	DiskUsedPct         float64 `expr:"diskUsedPct"`
	Hour                int     `expr:"hour"`
	ActiveUploads       int     `expr:"activeUploads"`
	ActiveDownloads     int     `expr:"activeDownloads"`
	GlobalUploadSpeed   float64 `expr:"globalUploadSpeed"`
	GlobalDownloadSpeed float64 `expr:"globalDownloadSpeed"`
	ScoringScore        float64 `expr:"scoringScore"`
	ScoringRank         int     `expr:"scoringRank"`
	LowScoreCount       int     `expr:"lowScoreCount"`
}

func buildExprEnv(c *Context) *ExprEnv {
	env := &ExprEnv{
		SiteName:      c.SiteName,
		Status:        c.Status,
		IsFree:        c.IsFree,
		HasHR:         c.HasHR,
		HRSeedTimeH:   c.HRSeedTimeH,
		Discount:      c.Discount,
		ClientUID:     c.ClientUID,
		Name:          c.Name,
		Size:          c.TotalSize,
		TotalSize:     c.TotalSize,
		Progress:      c.Progress,
		State:         c.State,
		Uploaded:      c.Uploaded,
		UploadSpeed:   c.UploadSpeed,
		DownloadSpeed: c.DownloadSpeed,
		Ratio:         c.Ratio,
		// 15 刀2（v0.0.1661）: 与 fieldValue 同语义填充（零上传
		// =+Inf·黑车检测）。
		DownloadUploadRatio: downloadUploadRatioOf(c.DownloadSpeed, c.UploadSpeed),
		SeedingTime:         c.SeedTime,
		Category:            c.Category,
		Tags:                strings.Join(c.Tags, ","),
		SavePath:            c.SavePath,
		Seeder:              c.NumComplete,
		Leecher:             c.NumIncomplete,
		IsFinished:          c.IsFinished,
		FreeSpace:           c.FreeSpace,
		TotalSpace:          c.TotalSpace,
		Hour:                c.Now.Hour(),
		ActiveUploads:       c.ActiveUploads,
		ActiveDownloads:     c.ActiveDownloads,
		GlobalUploadSpeed:   c.GlobalUploadSpeed,
		GlobalDownloadSpeed: c.GlobalDownloadSpeed,
		ScoringScore:        c.ScoringScore,
		ScoringRank:         c.ScoringRank,
		LowScoreCount:       c.LowScoreCount,
	}
	if c.TotalSpace > 0 {
		env.DiskUsedPct = float64(c.TotalSpace-c.FreeSpace) / float64(c.TotalSpace) * 100.0
	}
	if !c.AddedAt.IsZero() {
		env.AddedTime = c.Now.Sub(c.AddedAt).Seconds()
	}
	if c.FreeEndAt != nil {
		remain := c.FreeEndAt.Sub(c.Now).Seconds()
		if remain < 0 {
			remain = 0
		}
		env.FreeRemainSec = remain
	}
	if c.HasHR && c.HRSeedTimeH > 0 {
		required := int64(c.HRSeedTimeH) * 3600
		remain := required - c.SeedTime
		if remain < 0 {
			remain = 0
		}
		env.HRRemainSec = remain
	}
	return env
}

var programCache sync.Map

func getOrCompileProgram(exprStr string) (*vm.Program, error) {
	if cached, ok := programCache.Load(exprStr); ok {
		return cached.(*vm.Program), nil
	}
	env := &ExprEnv{}
	program, err := expr.Compile(exprStr, expr.Env(env), expr.AsBool())
	if err != nil {
		return nil, fmt.Errorf("expr compile failed: %w", err)
	}
	programCache.Store(exprStr, program)
	return program, nil
}

func evalExpr(exprStr string, c *Context) (bool, error) {
	program, err := getOrCompileProgram(exprStr)
	if err != nil {
		return false, err
	}
	env := buildExprEnv(c)
	output, err := expr.Run(program, env)
	if err != nil {
		return false, fmt.Errorf("expr run failed: %w", err)
	}
	result, ok := output.(bool)
	if !ok {
		return false, fmt.Errorf("expr result is not bool")
	}
	return result, nil
}

func ValidateExpr(exprStr string) error {
	_, err := getOrCompileProgram(exprStr)
	return err
}

// ValidateExprForTest 15 刀2: 测试别名（语义同 ValidateExpr）。
func ValidateExprForTest(exprStr string) error {
	return ValidateExpr(exprStr)
}

func ContextFromTorrentInfo(ti *model.TorrentInfo, siteName string, clientUID uint, now time.Time) *Context {
	return &Context{
		InfoHash:      ti.Hash,
		Name:          ti.Name,
		SavePath:      ti.SavePath,
		TotalSize:     ti.TotalSize,
		TrackerURL:    ti.TrackerURL,
		Category:      ti.Category,
		Tags:          ti.Tags,
		Ratio:         ti.Ratio,
		Progress:      ti.Progress,
		UploadSpeed:   ti.UploadSpeed,
		DownloadSpeed: ti.DownloadSpeed,
		Uploaded:      ti.Uploaded,
		Downloaded:    ti.Downloaded,
		SeedTime:      ti.SeedTime,
		State:         ti.State,
		ErrorMsg:      ti.Error,
		NumComplete:   ti.NumComplete,
		NumIncomplete: ti.NumIncomplete,
		IsFinished:    ti.IsFinished,
		IsPaused:      ti.IsPaused,
		AddedAt:       ti.AddedAt,
		SiteName:      siteName,
		ClientUID:     clientUID,
		Now:           now,
	}
}

// DownloadUploadRatioOf 15 刀2（v0.0.1661）: 下载/上传速度比——全库
// 唯一语义单点（声明式 fieldValue 与双 expr 环境填充三处共用·
// 双路径口径一致）。零上传=+Inf（契约 docs31:7757：黑车「只下不传」
// 是该键存在的核心需求·V-01——原返回 0 语义反转：>阈值 恰不命中）。
func DownloadUploadRatioOf(downloadSpeed, uploadSpeed int64) float64 {
	if uploadSpeed > 0 {
		return float64(downloadSpeed) / float64(uploadSpeed)
	}
	return math.Inf(1)
}

// downloadUploadRatioOf 包内别名（fieldValue 与 buildExprEnv 共用）。
func downloadUploadRatioOf(downloadSpeed, uploadSpeed int64) float64 {
	return DownloadUploadRatioOf(downloadSpeed, uploadSpeed)
}
