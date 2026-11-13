package api

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
)

// §59.516 v0.0.1461: 简介引用清洗任务（用户定案产品化——「版本词表演进 ↔
// 获取期过滤」固有时滞的自助闭环：开发者发新过滤词 → 用户 OTA → 点一次
// 原地清洗存量获取数据）。模式镜像 recompute 任务（§59.514 附八 P2）：
// 异步任务+进度轮询+graceful 停止+逐行 panic 隔离+逐行提方法（P6 sqlcheck
// 闭包合并教训）。清洗语义与获取期完全同源：DeclarationFilter.Filter 单点
// （引用块感知·整块删）+ GetPatterns 生效集（内置 ∪ 自定义）。幂等——
// 只更新有变化的行。
type declBackfillState struct {
	mu            sync.Mutex
	active        bool
	stopRequested bool
	total         int
	done          int
	failed        int
	changed       int
	startedAt     time.Time
	finishedAt    time.Time
	finished      bool
	error         string
}

// likeEscape LIKE 通配符转义（用户自定义词可能含 %/_——miss 即漏清洗）。
func likeEscape(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}

// handleDeclBackfillTask POST /publish/seeds/declaration-backfill-task
// 预筛：生效词任一 LIKE 命中 description/statement 的行（超集——Filter
// 引用块感知为最终仲裁者）。
func (h *PublishTorrentsHandler) handleDeclBackfillTask(w http.ResponseWriter, r *http.Request) {
	if h.declFilter == nil {
		Error(w, http.StatusServiceUnavailable, 50001, "声明过滤器未初始化")
		return
	}
	h.declBackfill.mu.Lock()
	if h.declBackfill.active {
		h.declBackfill.mu.Unlock()
		Error(w, http.StatusConflict, 40901, "已有清洗任务在运行")
		return
	}
	patterns := h.declFilter.GetPatterns(r.Context())
	if len(patterns) == 0 {
		h.declBackfill.mu.Unlock()
		Error(w, http.StatusBadRequest, 40001, "无生效过滤规则")
		return
	}
	conds := make([]string, 0, len(patterns))
	args := make([]interface{}, 0, len(patterns)*2)
	for _, p := range patterns {
		esc := "%" + likeEscape(p) + "%"
		conds = append(conds, "(description LIKE ? ESCAPE '\\' OR statement LIKE ? ESCAPE '\\')")
		args = append(args, esc, esc)
	}
	var rows []model.TorrentMetadata
	if err := h.db.WithContext(r.Context()).
		Where(strings.Join(conds, " OR "), args...).
		Select("info_hash", "description", "statement").
		Find(&rows).Error; err != nil {
		h.declBackfill.mu.Unlock()
		Error(w, http.StatusInternalServerError, 50000, "查询失败: "+err.Error())
		return
	}
	h.declBackfill.active = true
	h.declBackfill.stopRequested = false
	h.declBackfill.total = len(rows)
	h.declBackfill.done = 0
	h.declBackfill.failed = 0
	h.declBackfill.changed = 0
	h.declBackfill.finished = false
	h.declBackfill.error = ""
	h.declBackfill.startedAt = time.Now()
	h.declBackfill.mu.Unlock()

	go h.runDeclBackfillTask(rows, patterns)
	Success(w, map[string]interface{}{"message": "已开始", "total": len(rows)})
}

// handleDeclBackfillProgress GET /publish/seeds/declaration-backfill-progress
func (h *PublishTorrentsHandler) handleDeclBackfillProgress(w http.ResponseWriter, r *http.Request) {
	h.declBackfill.mu.Lock()
	defer h.declBackfill.mu.Unlock()
	Success(w, map[string]interface{}{
		"active":         h.declBackfill.active,
		"total":          h.declBackfill.total,
		"done":           h.declBackfill.done,
		"failed":         h.declBackfill.failed,
		"changed":        h.declBackfill.changed,
		"stop_requested": h.declBackfill.stopRequested,
		"finished":       h.declBackfill.finished,
		"error":          h.declBackfill.error,
		"started_at":     h.declBackfill.startedAt,
		"finished_at":    h.declBackfill.finishedAt,
	})
}

// handleDeclBackfillCancel POST /publish/seeds/declaration-backfill-cancel（graceful）
func (h *PublishTorrentsHandler) handleDeclBackfillCancel(w http.ResponseWriter, r *http.Request) {
	h.declBackfill.mu.Lock()
	h.declBackfill.stopRequested = true
	h.declBackfill.mu.Unlock()
	Success(w, map[string]interface{}{"message": "停止请求已接受——当前行完成后终止"})
}

// runDeclBackfillTask 逐行清洗（graceful 检查点每行前）。
func (h *PublishTorrentsHandler) runDeclBackfillTask(rows []model.TorrentMetadata, patterns []string) {
	ctx := context.Background()
	for _, row := range rows {
		h.declBackfill.mu.Lock()
		stopped := h.declBackfill.stopRequested
		h.declBackfill.mu.Unlock()
		if stopped {
			h.declBackfill.mu.Lock()
			h.declBackfill.error = "用户取消（已完成 " + itoa(h.declBackfill.done) + "/" + itoa(h.declBackfill.total) + "）"
			h.finishDeclBackfill()
			h.declBackfill.mu.Unlock()
			return
		}
		h.declBackfillRow(ctx, row, patterns)
	}
	h.declBackfill.mu.Lock()
	h.finishDeclBackfill()
	h.declBackfill.mu.Unlock()
}

func (h *PublishTorrentsHandler) finishDeclBackfill() {
	h.declBackfill.active = false
	h.declBackfill.finished = true
	h.declBackfill.finishedAt = time.Now()
}

// declBackfillRow 单行清洗（§59.514 附八 P6 同款提方法——防闭包 sqlcheck
// 合并误报）。Filter 引用块感知为仲裁者：预筛命中但引用块不匹配的行零
// 变化零 UPDATE（幂等）。panic 隔离计 failed 不中断任务。
func (h *PublishTorrentsHandler) declBackfillRow(ctx context.Context, row model.TorrentMetadata, patterns []string) {
	defer func() {
		if p := recover(); p != nil {
			if h.logger != nil {
				h.logger.Error("decl-backfill panic", zap.String("hash", row.InfoHash), zap.Any("panic", p))
			}
			h.declBackfill.mu.Lock()
			h.declBackfill.failed++
			h.declBackfill.done++
			h.declBackfill.mu.Unlock()
		}
	}()
	updates := map[string]interface{}{}
	if row.Description != "" {
		if fr := h.declFilter.Filter(row.Description, patterns); fr.CleanedText != row.Description {
			updates["description"] = fr.CleanedText
		}
	}
	if row.Statement != "" {
		if fs := h.declFilter.Filter(row.Statement, patterns); fs.CleanedText != row.Statement {
			updates["statement"] = fs.CleanedText
		}
	}
	if len(updates) > 0 {
		if err := h.db.WithContext(ctx).Model(&model.TorrentMetadata{}).
			Where("info_hash = ?", row.InfoHash).
			Updates(updates).Error; err != nil {
			if h.logger != nil {
				h.logger.Error("decl-backfill update failed", zap.String("hash", row.InfoHash), zap.Error(err))
			}
			h.declBackfill.mu.Lock()
			h.declBackfill.failed++
			h.declBackfill.done++
			h.declBackfill.mu.Unlock()
			return
		}
		h.declBackfill.mu.Lock()
		h.declBackfill.changed++
		h.declBackfill.done++
		h.declBackfill.mu.Unlock()
		return
	}
	h.declBackfill.mu.Lock()
	h.declBackfill.done++
	h.declBackfill.mu.Unlock()
}
