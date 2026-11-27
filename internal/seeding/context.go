package seeding

import (
	"fmt"
	"strings"
	"sync"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
	"github.com/ranfish/pt-forward/internal/rule"
)

type exprEnv struct {
	SiteName      string  `expr:"siteName"`
	Status        string  `expr:"status"`
	IsFree        bool    `expr:"isFree"`
	HasHR         bool    `expr:"hasHR"`
	HRSeedTimeH   int     `expr:"hrSeedTimeH"`
	Discount      string  `expr:"discount"`
	ClientUID     uint    `expr:"clientUID"`
	TorrentID     string  `expr:"torrentID"`
	FreeLevel     string  `expr:"freeLevel"`
	Source        string  `expr:"source"`
	LastActionBy  string  `expr:"lastActionBy"`
	Name          string  `expr:"name"`
	Size          int64   `expr:"size"`
	TotalSize     int64   `expr:"totalSize"`
	Progress      float64 `expr:"progress"`
	State         string  `expr:"state"`
	Uploaded      int64   `expr:"uploaded"`
	UploadSpeed   int64   `expr:"uploadSpeed"`
	DownloadSpeed int64   `expr:"downloadSpeed"`
	Ratio         float64 `expr:"ratio"`
	// 15 刀2（v0.0.1661）: V-02 根修——文档 42 键清单声明但 expr 环境
	// 缺失（Compile 实测 unknown name）。语义单点=rule.DownloadUploadRatioOf
	// （零上传=+Inf·V-01 同批·双路径一致）。
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
	// 15 刀2 补（v0.0.1661）: 双路径一致性缺口（fieldValue 有·expr
	// 缺·实施中覆盖探针抓出）——4 键。unregistered 29 生产规则在用
	// （声明式）；diskUsedPercent 为 fieldValue 键名（expr 旧键
	// diskUsedPct 保留兼容）。
	Unregistered    bool    `expr:"unregistered"`
	Downloaded      int64   `expr:"downloaded"`
	Error           string  `expr:"error"`
	DiskUsedPercent float64 `expr:"diskUsedPercent"`
	Hour            int     `expr:"hour"`

	ActiveUploads       int     `expr:"activeUploads"`
	ActiveDownloads     int     `expr:"activeDownloads"`
	GlobalUploadSpeed   float64 `expr:"globalUploadSpeed"`
	GlobalDownloadSpeed float64 `expr:"globalDownloadSpeed"`

	ScoringScore  float64 `expr:"scoringScore"`
	ScoringRank   int     `expr:"scoringRank"`
	LowScoreCount int     `expr:"lowScoreCount"`
}

func buildExprEnv(rc *RuleContext) *exprEnv {
	env := &exprEnv{
		SiteName:     rc.Record.SiteName,
		Status:       string(rc.Record.Status),
		IsFree:       rc.Record.IsFree,
		HasHR:        rc.Record.HasHR,
		HRSeedTimeH:  rc.Record.HRSeedTimeH,
		Discount:     string(rc.Record.Discount),
		ClientUID:    rc.Record.ClientUID,
		TorrentID:    rc.Record.TorrentID,
		FreeLevel:    rc.Record.FreeLevel,
		Source:       rc.Record.Source,
		LastActionBy: rc.Record.LastActionBy,
		// 15 刀2 补（v0.0.1661）: 双路径一致性——unregistered 生产在用。
		Unregistered: rc.Record.Unregistered,
		FreeSpace:    rc.FreeSpace,
		TotalSpace:   rc.TotalSpace,
		DiskUsedPct: func() float64 {
			if rc.TotalSpace > 0 {
				return float64(rc.TotalSpace-rc.FreeSpace) / float64(rc.TotalSpace) * 100.0
			}
			return 0
		}(),
		// 15 刀2 补（v0.0.1661）: 双键名并存（diskUsedPercent=
		// fieldValue 键名·diskUsedPct=expr 旧键兼容）·同源同值。
		DiskUsedPercent: func() float64 {
			if rc.TotalSpace > 0 {
				return float64(rc.TotalSpace-rc.FreeSpace) / float64(rc.TotalSpace) * 100.0
			}
			return 0
		}(),
		Hour: rc.Now.Hour(),

		ActiveUploads:       rc.ActiveUploads,
		ActiveDownloads:     rc.ActiveDownloads,
		GlobalUploadSpeed:   rc.GlobalUploadSpeed,
		GlobalDownloadSpeed: rc.GlobalDownloadSpeed,

		ScoringScore:  rc.ScoringScore,
		ScoringRank:   rc.ScoringRank,
		LowScoreCount: rc.LowScoreCount,
	}

	if rc.Record.FreeEndAt != nil {
		remain := rc.Record.FreeEndAt.Sub(rc.Now).Seconds()
		if remain < 0 {
			remain = 0
		}
		env.FreeRemainSec = remain
	}

	if rc.Record.HasHR && rc.Record.HRSeedTimeH > 0 && rc.Torrent != nil {
		required := int64(rc.Record.HRSeedTimeH) * 3600
		remain := required - rc.Torrent.SeedTime
		if remain < 0 {
			remain = 0
		}
		env.HRRemainSec = remain
	}

	if ti := rc.Torrent; ti != nil {
		env.Name = ti.Name
		env.Size = ti.TotalSize
		env.TotalSize = ti.TotalSize
		env.Progress = ti.Progress
		env.State = ti.State
		env.Uploaded = ti.Uploaded
		env.UploadSpeed = ti.UploadSpeed
		env.DownloadSpeed = ti.DownloadSpeed
		env.Ratio = ti.Ratio
		// 15 刀2（v0.0.1661）: 语义单点填充（零上传=+Inf·V-01 同批）
		// + 双路径一致性补全（downloaded/error——fieldValue 有 expr 缺）。
		env.DownloadUploadRatio = rule.DownloadUploadRatioOf(ti.DownloadSpeed, ti.UploadSpeed)
		env.Downloaded = ti.Downloaded
		env.Error = ti.Error
		env.SeedingTime = ti.SeedTime
		env.Category = ti.Category
		env.Tags = strings.Join(ti.Tags, ",")
		env.SavePath = ti.SavePath
		env.Seeder = ti.NumComplete
		env.Leecher = ti.NumIncomplete
		env.IsFinished = ti.IsFinished

		if !ti.AddedAt.IsZero() {
			env.AddedTime = rc.Now.Sub(ti.AddedAt).Seconds()
		}
	}

	return env
}

var (
	programCache sync.Map
)

func getOrCompileProgram(exprStr string) (*vm.Program, error) {
	if cached, ok := programCache.Load(exprStr); ok {
		prog, ok2 := cached.(*vm.Program)
		if !ok2 {
			return nil, fmt.Errorf("invalid cached program type")
		}
		return prog, nil
	}

	env := &exprEnv{}
	program, err := expr.Compile(exprStr, expr.Env(env), expr.AsBool())
	if err != nil {
		return nil, seedingError(ErrSeedingParse, "expr compile failed", err)
	}

	programCache.Store(exprStr, program)
	return program, nil
}

func evalExpr(exprStr string, rc *RuleContext) (bool, error) {
	program, err := getOrCompileProgram(exprStr)
	if err != nil {
		return false, err
	}

	env := buildExprEnv(rc)
	output, err := expr.Run(program, env)
	if err != nil {
		return false, seedingError(ErrSeedingParse, "expr run failed", err)
	}

	result, ok := output.(bool)
	if !ok {
		return false, seedingError(ErrSeedingParse, "expr result is not bool", nil)
	}
	return result, nil
}

func ValidateExpr(exprStr string) error {
	_, err := getOrCompileProgram(exprStr)
	return err
}

func EvalExprForTest(exprStr string, rc *RuleContext) (bool, error) {
	return evalExpr(exprStr, rc)
}
