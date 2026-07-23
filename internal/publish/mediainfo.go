package publish

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"go.uber.org/zap"
)

// MediaInfoResult 封装 mediainfo 二进制的纯文本输出。
//
// 早期版本（v0.0.278 之前）曾用 --Output=JSON 解析成结构化字段
// （General/Video/Audio/Text），但下游消费者全部基于纯文本扫描
// （CorrectWithMediaInfo 找 "Format : xxx" 行、MediaTagInferer 关键字匹配、
// artifact_generator 取 RawOutput 原样下发到目标站 mediainfo 字段），
// 结构化分支从未被接通，且 JSON 输出会让上述文本扫描全部失效。
// v0.0.278 移除死代码，回归 mediainfo 默认的纯文本（树状）输出。
type MediaInfoResult struct {
	RawOutput string `json:"raw_output,omitempty"`
}

type MediaInfoAnalyzer struct {
	mediainfoPath string
	logger        *zap.Logger
}

func NewMediaInfoAnalyzer(logger *zap.Logger) *MediaInfoAnalyzer {
	return &MediaInfoAnalyzer{
		mediainfoPath: "mediainfo",
		logger:        logger,
	}
}

func (a *MediaInfoAnalyzer) Available() bool {
	_, err := exec.LookPath(a.mediainfoPath)
	return err == nil
}

// Analyze 调用 mediainfo 二进制获取纯文本（树状）格式的 MediaInfo。
//
// 不使用 --Output=JSON：mediainfo 默认输出即 PT 站点通用格式，
// 也是 CorrectWithMediaInfo / MediaTagInferer / 目标站 mediainfo 字段的预期输入。
func (a *MediaInfoAnalyzer) Analyze(ctx context.Context, filePath string) (*MediaInfoResult, error) {
	if !a.Available() {
		return nil, fmt.Errorf("mediainfo not found")
	}

	cmd := exec.CommandContext(ctx, a.mediainfoPath, filePath) //nolint:gosec // intentional subprocess
	cmd.Env = mediainfoEnv()
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("mediainfo execution: %w", err)
	}

	return &MediaInfoResult{RawOutput: strings.TrimSpace(string(output))}, nil
}

// mediainfoEnv 返回调用 mediainfo 用的环境变量。
// mediainfo 25.04 在 POSIX/C locale 下对非 ASCII 路径（中文/方括号）静默失败（media:null）。
// 设置 LC_ALL=C.UTF-8 让 mediainfo 正确处理 UTF-8 文件名。
func mediainfoEnv() []string {
	envs := []string{"LC_ALL=C.UTF-8", "LANG=C.UTF-8"}
	for _, kv := range execEnv() {
		if strings.HasPrefix(kv, "LC_ALL=") || strings.HasPrefix(kv, "LANG=") {
			continue
		}
		envs = append(envs, kv)
	}
	return envs
}

// execEnv 返回当前进程环境变量（不含 LC_ALL/LANG，由 mediainfoEnv 重设）。
func execEnv() []string {
	return append([]string(nil), os.Environ()...)
}
