package api

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

type LogEntry struct {
	Line      string    `json:"line"`
	Timestamp time.Time `json:"timestamp"`
}

type LogBroadcaster struct {
	mu          sync.RWMutex
	subscribers map[int]chan *LogEntry
	nextID      int
}

func NewLogBroadcaster() *LogBroadcaster {
	return &LogBroadcaster{
		subscribers: make(map[int]chan *LogEntry),
	}
}

func (b *LogBroadcaster) Subscribe(bufSize int) (int, <-chan *LogEntry) {
	if bufSize <= 0 {
		bufSize = 256
	}
	ch := make(chan *LogEntry, bufSize)
	b.mu.Lock()
	defer b.mu.Unlock()
	b.nextID++
	id := b.nextID
	b.subscribers[id] = ch
	return id, ch
}

func (b *LogBroadcaster) Unsubscribe(id int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if ch, ok := b.subscribers[id]; ok {
		delete(b.subscribers, id)
		close(ch)
	}
}

func (b *LogBroadcaster) Broadcast(line string) {
	entry := &LogEntry{Line: line, Timestamp: time.Now()}
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, ch := range b.subscribers {
		select {
		case ch <- entry:
		default:
			// subscriber buffer full, drop message
		}
	}
}

func (b *LogBroadcaster) SubscriberCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.subscribers)
}

type SSELogHandler struct {
	broadcaster *LogBroadcaster
	logger      *zap.Logger
}

func NewSSELogHandler(broadcaster *LogBroadcaster, logger *zap.Logger) *SSELogHandler {
	return &SSELogHandler{broadcaster: broadcaster, logger: logger}
}

func (h *SSELogHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	rc := http.NewResponseController(w)
	rc.SetWriteDeadline(time.Time{})

	levelFilter := strings.ToLower(r.URL.Query().Get("level"))

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	id, ch := h.broadcaster.Subscribe(256)
	defer h.broadcaster.Unsubscribe(id)

	h.logger.Debug("SSE log subscriber connected",
		zap.Int("subscriber_id", id),
		zap.Int("total_subscribers", h.broadcaster.SubscriberCount()))

	ctx := r.Context()

	heartbeat := time.NewTicker(30 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-ctx.Done():
			h.logger.Debug("SSE log subscriber disconnected",
				zap.Int("subscriber_id", id))
			return
		case <-heartbeat.C:
			_, err := w.Write([]byte(": heartbeat\n\n"))
			if err != nil {
				return
			}
			flusher.Flush()
		case entry, ok := <-ch:
			if !ok {
				return
			}
			if levelFilter != "" && !matchLogLevel(entry.Line, levelFilter) {
				continue
			}
			_, err := w.Write([]byte("data: " + entry.Line + "\n\n"))
			if err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func matchLogLevel(jsonLine, filter string) bool {
	if filter == "" || filter == "debug" {
		return true
	}
	priority := map[string]int{
		"debug":  0,
		"info":   1,
		"warn":   2,
		"error":  3,
		"dpanic": 4,
		"panic":  5,
		"fatal":  6,
	}
	lineLevel := extractLogLevel(jsonLine)
	if lineLevel == "" {
		return true
	}
	filterPrio, fok := priority[filter]
	linePrio, lok := priority[lineLevel]
	if !fok || !lok {
		return true
	}
	return linePrio >= filterPrio
}

func extractLogLevel(jsonLine string) string {
	idx := strings.Index(jsonLine, `"level":"`)
	if idx < 0 {
		return ""
	}
	start := idx + len(`"level":"`)
	end := strings.Index(jsonLine[start:], `"`)
	if end < 0 {
		return ""
	}
	return jsonLine[start : start+end]
}

var _ = context.Background
