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

func (h *SystemHandler) newHTTPClientWithProxy(timeout time.Duration) *http.Client {
	proxyStr := h.getProxyFromSettings()
	tr := &http.Transport{}
	if proxyStr != "" {
		if u, err := url.Parse(proxyStr); err == nil {
			tr.Proxy = http.ProxyURL(u)
			h.logger.Info("OTA: using proxy", zap.String("proxy", proxyStr))
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

	client := h.newHTTPClientWithProxy(15 * time.Second)
	resp, err := client.Do(req)
	if err != nil {
		h.logger.Warn("check update: github api failed", zap.Error(err))
		Success(w, map[string]interface{}{
			"has_update":      false,
			"current_version": h.version,
			"error":           "无法连接 GitHub API",
		})
		return
	}
	defer resp.Body.Close()

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

	client := h.newHTTPClientWithProxy(10 * time.Second)
	resp, err := client.Do(req)
	if err != nil {
		Error(w, http.StatusServiceUnavailable, 50001, "无法连接 GitHub API")
		return
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		resp.Body.Close()
		Error(w, http.StatusInternalServerError, 50002, "解析 Release 失败")
		return
	}
	resp.Body.Close()

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
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("download HTTP %d", resp.StatusCode)
	}

	tmpFile, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}

	written, err := io.Copy(tmpFile, resp.Body)
	tmpFile.Close()
	if err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("write temp file: %w", err)
	}

	h.logger.Info("OTA: download complete", zap.Int64("bytes", written))

	// Verify the downloaded file is executable
	if err := os.Chmod(tmpPath, 0755); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("chmod: %w", err)
	}

	// Quick sanity check: run --version
	cmd := exec.CommandContext(context.Background(), tmpPath, "--version")
	versionOutput, err := cmd.CombinedOutput()
	if err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("verify binary: %w (output: %s)", err, string(versionOutput))
	}
	h.logger.Info("OTA: binary verified", zap.String("version_output", strings.TrimSpace(string(versionOutput))))

	// Backup current binary
	backupPath := exePath + ".bak"
	os.Remove(backupPath)
	if err := os.Rename(exePath, backupPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("backup current binary: %w", err)
	}

	// Atomic replace
	if err := os.Rename(tmpPath, exePath); err != nil {
		// Rollback
		os.Rename(backupPath, exePath)
		return fmt.Errorf("replace binary: %w", err)
	}

	os.Remove(backupPath)
	return nil
}
