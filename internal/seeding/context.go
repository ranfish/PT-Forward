package seeding

import (
	"strings"
	"sync"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
)

type exprEnv struct {
	SiteName      string  `expr:"siteName"`
	Status        string  `expr:"status"`
	IsFree        bool    `expr:"isFree"`
	HasHR         bool    `expr:"hasHR"`
	HRSeedTimeH   int     `expr:"hrSeedTimeH"`
	Discount      string  `expr:"discount"`
	ClientID      string  `expr:"clientID"`
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
	SeedingTime   int64   `expr:"seedingTime"`
	Category      string  `expr:"category"`
	Tags          string  `expr:"tags"`
	SavePath      string  `expr:"savePath"`
	Seeder        int     `expr:"seeder"`
	Leecher       int     `expr:"leecher"`
	IsFinished    bool    `expr:"isFinished"`
	AddedTime     float64 `expr:"addedTime"`
	FreeRemainSec float64 `expr:"freeRemainSec"`
	HRRemainSec   int64   `expr:"hrRemainSec"`
	FreeSpace     int64   `expr:"freeSpace"`
	Hour          int     `expr:"hour"`

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
		ClientID:     rc.Record.ClientID,
		TorrentID:    rc.Record.TorrentID,
		FreeLevel:    rc.Record.FreeLevel,
		Source:       rc.Record.Source,
		LastActionBy: rc.Record.LastActionBy,
		FreeSpace:    rc.FreeSpace,
		Hour:         rc.Now.Hour(),

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
		return cached.(*vm.Program), nil
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
