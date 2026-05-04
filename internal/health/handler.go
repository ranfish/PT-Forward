package health

import (
	"encoding/json"
	"net/http"
	"runtime"
	"time"
)

var startTime = time.Now()

type HealthResponse struct {
	Status  string `json:"status"`
	Version string `json:"version"`
	Uptime  string `json:"uptime"`
}

type HealthChecker struct {
	version string
}

func NewHealthChecker(version string) *HealthChecker {
	return &HealthChecker{version: version}
}

func (h *HealthChecker) Handler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	resp := HealthResponse{
		Status:  "ok",
		Version: h.version,
		Uptime:  time.Since(startTime).Round(time.Second).String(),
	}
	_ = json.NewEncoder(w).Encode(resp)
}
