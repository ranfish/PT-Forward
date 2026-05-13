package health

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"
)

var startTime = time.Now()

type DBPinger interface {
	Ping() error
}

type ComponentHealth struct {
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

type HealthResponse struct {
	Status     string                    `json:"status"`
	Version    string                    `json:"version"`
	Uptime     string                    `json:"uptime"`
	Components map[string]ComponentHealth `json:"components,omitempty"`
}

type HealthChecker struct {
	version    string
	dbPinger   DBPinger
}

func NewHealthChecker(version string) *HealthChecker {
	return &HealthChecker{version: version}
}

func (h *HealthChecker) SetDBPinger(p DBPinger) {
	h.dbPinger = p
}

func (h *HealthChecker) Handler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	resp := HealthResponse{
		Status:  "ok",
		Version: h.version,
		Uptime:  time.Since(startTime).Round(time.Second).String(),
	}

	if h.dbPinger != nil {
		if resp.Components == nil {
			resp.Components = make(map[string]ComponentHealth)
		}
		if err := h.dbPinger.Ping(); err != nil {
			resp.Status = "degraded"
			resp.Components["database"] = ComponentHealth{
				Status: "unhealthy",
				Error:  err.Error(),
			}
		} else {
			resp.Components["database"] = ComponentHealth{Status: "healthy"}
		}
	}

	if resp.Status == "degraded" {
		w.WriteHeader(http.StatusServiceUnavailable)
	} else {
		w.WriteHeader(http.StatusOK)
	}

	_ = json.NewEncoder(w).Encode(resp)
}

type sqlPinger struct {
	db *sql.DB
}

func NewSQLPinger(db *sql.DB) DBPinger {
	return &sqlPinger{db: db}
}

func (s *sqlPinger) Ping() error {
	return s.db.Ping()
}
