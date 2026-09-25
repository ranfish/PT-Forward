package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"crypto/sha256"
	"encoding/hex"
	"runtime"
	"strings"
	"time"

	"github.com/ranfish/pt-forward/internal/setting"
	"go.uber.org/zap"
)

const githubAPI = "https://api.github.com/repos/ranfish/PT-Forward/releases/latest"

func (h *SystemHandler) getProxyFromSettings() string {
	repo := setting.NewRepository(h.db)
	val, err := repo.Get(context.Background(), "httpProxy")
	if err != nil || val == "" {
		return ""
	}
	return val
}

// otaProxyEnabled §59.280: OTA 更新启用代理开关（settings otaUseProxy，
// 默认 false）。ON=httpProxy 已配置时 OTA 检查+下载强制走代理——直连在部分
// 网络（fnos 案）对 GitHub 是挂起不报错，"直连优先失败回退"永不触发；
// OFF=维持直连优先回退行为。
func (h *SystemHandler) otaProxyEnabled() bool {
	repo := setting.NewRepository(h.db)
	val, err := repo.Get(context.Background(), "otaUseProxy")
	if err != nil {
		return false
	}
	return val == "true"
}

func (h *SystemHandler) newHTTPClientWithProxy(timeout time.Duration) *http.Client {
	tr := &http.Transport{}
	// §59.280: otaUseProxy 总开关——OFF 时 OTA 全直连（httpProxy 是站点采集
	// 等全局代理配置，OTA 不隐性搭车）；ON 且已配置才走代理
	if h.otaProxyEnabled() {
		if proxyStr := h.getProxyFromSettings(); proxyStr != "" {
			if u, err := url.Parse(proxyStr); err == nil {
				tr.Proxy = http.ProxyURL(u)
				h.logger.Info("OTA: using proxy", zap.String("proxy", proxyStr))
			}
		}
	}
	return &http.Client{Timeout: timeout, Transport: tr}
}

type githubRelease struct {
	TagName string `json:"tag_name"`
	Name    string `json:"name"`
	Body    string `json:"body"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
		Size               int64  `json:"size"`
	} `json:"assets"`
}

func (h *SystemHandler) handleCheckUpdate(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", githubAPI, nil)
	req.Header.Set("Accept", "application/vnd.github+json")

	// §59.280: otaUseProxy 开关——ON=强制代理（直连挂起不报错的网络下
	// "直连优先失败回退"永不回退——fnos 案）；OFF=纯直连（隐性代理回退
	// 由显式开关取代）
	resp, err := h.newHTTPClientWithProxy(15 * time.Second).Do(req)
	if err != nil {
		h.logger.Warn("check update: github api failed", zap.Error(err))
		Success(w, map[string]interface{}{
			"has_update":      false,
			"current_version": h.version,
			"error":           "无法连接 GitHub API",
		})
		return
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		h.logger.Warn("check update: github api status", zap.Int("status", resp.StatusCode))
		Success(w, map[string]interface{}{
			"has_update":      false,
			"current_version": h.version,
			"error":           "GitHub API 返回非 200",
		})
		return
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		h.logger.Warn("check update: decode failed", zap.Error(err))
		Success(w, map[string]interface{}{
			"has_update":      false,
			"current_version": h.version,
			"error":           "解析 Release 信息失败",
		})
		return
	}

	assetName := fmt.Sprintf("pt-forward-linux-%s", runtime.GOARCH)
	var downloadURL string
	for _, a := range release.Assets {
		if a.Name == assetName || a.Name == "pt-forward" {
			downloadURL = a.BrowserDownloadURL
			break
		}
	}

	if downloadURL == "" {
		Success(w, map[string]interface{}{
			"has_update":      false,
			"current_version": h.version,
			"latest_version":  release.TagName,
			"error":           "Release 中未找到适配的二进制文件",
		})
		return
	}

	hasUpdate := release.TagName != h.version
	Success(w, map[string]interface{}{
		"has_update":      hasUpdate,
		"current_version": h.version,
		"latest_version":  release.TagName,
		"release_notes":   release.Body,
		"download_url":    downloadURL,
	})
}

func (h *SystemHandler) handleUpdate(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Step 1: Fetch latest release info
	req, _ := http.NewRequestWithContext(ctx, "GET", githubAPI, nil)
	req.Header.Set("Accept", "application/vnd.github+json")

	// §59.280: 同 check——开关统一走 newHTTPClientWithProxy
	resp, err := h.newHTTPClientWithProxy(10 * time.Second).Do(req)
	if err != nil {
		Error(w, http.StatusServiceUnavailable, 50001, "无法连接 GitHub API")
		return
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		_ = resp.Body.Close()
		Error(w, http.StatusInternalServerError, 50002, "解析 Release 失败")
		return
	}
	_ = resp.Body.Close()

	assetName := fmt.Sprintf("pt-forward-linux-%s", runtime.GOARCH)
	var downloadURL string
	for _, a := range release.Assets {
		if a.Name == assetName || a.Name == "pt-forward" {
			downloadURL = a.BrowserDownloadURL
			break
		}
	}
	if downloadURL == "" {
		Error(w, http.StatusNotFound, 40401, "Release 中未找到二进制文件")
		return
	}

	// Step 2: Send response before starting download
	Success(w, map[string]interface{}{
		"status":          "downloading",
		"latest_version":  release.TagName,
		"current_version": h.version,
	})

	// Step 3: Download and replace in background
	go func() {
		if err := h.downloadAndReplace(downloadURL); err != nil {
			h.logger.Error("OTA update failed", zap.Error(err))
			return
		}
		h.logger.Info("OTA update complete, exiting for restart",
			zap.String("old_version", h.version),
			zap.String("new_version", release.TagName))
		// Graceful exit — systemd/Docker will restart
		time.Sleep(500 * time.Millisecond)
		os.Exit(0)
	}()
}

func (h *SystemHandler) downloadAndReplace(downloadURL string) error {
	// Get current executable path
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get exe path: %w", err)
	}
	exePath, _ = filepath.EvalSymlinks(exePath)
	exeDir := filepath.Dir(exePath)

	// Download to temp file in same directory (for atomic rename)
	tmpPath := filepath.Join(exeDir, ".pt-forward.new")
	h.logger.Info("OTA: downloading", zap.String("url", downloadURL), zap.String("dest", tmpPath))

	dlCtx, dlCancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer dlCancel()

	req, _ := http.NewRequestWithContext(dlCtx, "GET", downloadURL, nil)
	client := h.newHTTPClientWithProxy(5 * time.Minute)
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		return fmt.Errorf("download HTTP %d", resp.StatusCode)
	}

	tmpFile, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o700) //nolint:gosec // 可执行文件需 owner x 位
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}

	written, err := io.Copy(tmpFile, resp.Body)
	_ = tmpFile.Close()
	if err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("write temp file: %w", err)
	}

	h.logger.Info("OTA: download complete", zap.Int64("bytes", written))

	// SHA256 校验（强制）
	expectedHash, err := h.downloadSHA256(downloadURL + ".sha256")
	if err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("download SHA256 checksum: %w", err)
	}
	actualHash, err := computeFileSHA256(tmpPath)
	if err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("compute SHA256: %w", err)
	}
	if actualHash != expectedHash {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("SHA256 mismatch: expected %s, got %s", expectedHash, actualHash)
	}
	h.logger.Info("OTA: SHA256 verified", zap.String("hash", actualHash[:16]+"..."))

	// Verify the downloaded file is executable
	if err := os.Chmod(tmpPath, 0o700); err != nil { //nolint:gosec // OTA 二进制需 owner 执行位
		return fmt.Errorf("chmod: %w", err)
	}

	// Quick sanity check: run --version
	cmd := exec.CommandContext(context.Background(), tmpPath, "--version") //nolint:gosec // OTA 机制本体：执行的是已校验的下载二进制
	versionOutput, err := cmd.CombinedOutput()
	if err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("verify binary: %w (output: %s)", err, string(versionOutput))
	}
	h.logger.Info("OTA: binary verified", zap.String("version_output", strings.TrimSpace(string(versionOutput))))

	// Backup current binary
	backupPath := exePath + ".bak"
	_ = os.Remove(backupPath)
	if err := os.Rename(exePath, backupPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("backup current binary: %w", err)
	}

	// Atomic replace
	if err := os.Rename(tmpPath, exePath); err != nil {
		// Rollback
		_ = os.Rename(backupPath, exePath) // 回滚失败：备份路径残留，人工介入
		return fmt.Errorf("replace binary: %w", err)
	}

	_ = os.Remove(backupPath)
	return nil
}

func (h *SystemHandler) downloadSHA256(url string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	client := h.newHTTPClientWithProxy(30 * time.Second)
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("download checksum: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("checksum download HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024))
	if err != nil {
		return "", fmt.Errorf("read checksum: %w", err)
	}

	line := strings.TrimSpace(string(body))
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return "", fmt.Errorf("empty checksum file")
	}

	hash := parts[0]
	if len(hash) != 64 {
		return "", fmt.Errorf("invalid SHA256 length: %d", len(hash))
	}
	return hash, nil
}

func computeFileSHA256(path string) (string, error) {
	f, err := os.Open(path) //nolint:gosec // OTA 校验目标路径
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
