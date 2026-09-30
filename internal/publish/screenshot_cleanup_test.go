package publish

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// §59.300 回归审核：多时间点截图共用同一 tmpDir，captureFrameMPVEx 的清理循环
// 只允许删除 mpv 数字编号帧（00000001.jpg），不得误删前序迭代已重命名的 shot_00X.jpg。
// 用假 mpv 脚本端到端复现（--frames N 输出 N 个数字帧文件）。
func TestCaptureFrameCleanupKeepsRenamedShots(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh")
	}

	tmp := t.TempDir()
	// 假 mpv：向 outdir 写 N 个数字编号 jpg（N 取自 --frames）
	script := filepath.Join(tmp, "fake-mpv.sh")
	if err := os.WriteFile(script, []byte(`#!/bin/sh
OUT=""; N=1
for a in "$@"; do
  case "$a" in
    --frames=*) N="${a#--frames=}" ;;
    --vo-image-outdir=*) OUT="${a#--vo-image-outdir=}" ;;
  esac
done
i=1
while [ "$i" -le "$N" ]; do
  printf 'x' > "$OUT/$(printf %08d $i).jpg"
  i=$((i+1))
done
exit 0
`), 0755); err != nil {
		t.Fatal(err)
	}

	eng := NewScreenshotEngine(script, 2, 1, 85, nil)

	ctx := context.Background()
	// 模拟两次迭代写同一 tmpDir（与 engine.Capture 行为一致）
	dir := t.TempDir()
	p1 := filepath.Join(dir, "shot_000.jpg")
	p2 := filepath.Join(dir, "shot_001.jpg")
	if err := eng.captureFrameMPVEx(ctx, "dummy.mkv", 10.0, 0, false, false, 24.0, 0, p1); err != nil {
		t.Fatalf("iter1: %v", err)
	}
	if _, err := os.Stat(p1); err != nil {
		t.Fatalf("iter1 output missing: %v", err)
	}
	if err := eng.captureFrameMPVEx(ctx, "dummy.mkv", 40.0, 0, false, false, 24.0, 0, p2); err != nil {
		t.Fatalf("iter2: %v", err)
	}
	if _, err := os.Stat(p2); err != nil {
		t.Fatalf("iter2 output missing: %v", err)
	}
	// 关键断言：iter2 不得删掉 iter1 的产物
	if _, err := os.Stat(p1); err != nil {
		t.Fatalf("REGRESSION: iter2 deleted iter1 shot: %v", err)
	}

	// tmpDir 内不应残留数字编号帧
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		base := strings.TrimSuffix(e.Name(), ".jpg")
		if base != "" && isAllDigits(base) {
			t.Fatalf("REGRESSION: leftover numbered frame %s", e.Name())
		}
	}
}

func isAllDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return len(s) > 0
}
