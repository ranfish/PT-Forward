package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestInit_Idempotent(t *testing.T) {
	Init("test-version")
	Init("test-version-2")

	mfs, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("Gather failed: %v", err)
	}

	found := false
	for _, mf := range mfs {
		if mf.GetName() == "ptforward_build_info" {
			found = true
			if len(mf.GetMetric()) != 1 {
				t.Errorf("expected exactly 1 build_info metric, got %d", len(mf.GetMetric()))
			}
			label := mf.GetMetric()[0].GetLabel()
			hasVersion := false
			for _, l := range label {
				if l.GetName() == "version" && l.GetValue() == "test-version" {
					hasVersion = true
				}
			}
			if !hasVersion {
				t.Error("expected first Init version to be preserved")
			}
		}
	}
	if !found {
		t.Error("ptforward_build_info metric not found")
	}
}

func TestMetricsUsable(t *testing.T) {
	HTTPRequestsTotal.WithLabelValues("GET", "/", "200").Inc()
	HTTPRequestDuration.WithLabelValues("GET", "/").Observe(0.1)
	RSSTorrentsFetched.WithLabelValues("site1").Inc()
	RSSTorrentsFiltered.WithLabelValues("site1", "size").Inc()
	PublishTasksTotal.WithLabelValues("site1", "success").Inc()
	PublishDuration.WithLabelValues("site1").Observe(1.0)
	SiteRequestsTotal.WithLabelValues("site1", "GET").Inc()
	SiteRequestErrors.WithLabelValues("site1", "GET").Inc()

	mfs, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("Gather failed: %v", err)
	}

	registered := make(map[string]bool)
	for _, mf := range mfs {
		registered[mf.GetName()] = true
	}

	expected := []string{
		"ptforward_http_requests_total",
		"ptforward_http_request_duration_seconds",
		"ptforward_rss_torrents_fetched_total",
		"ptforward_rss_torrents_filtered_total",
		"ptforward_publish_tasks_total",
		"ptforward_publish_duration_seconds",
		"ptforward_site_adapter_requests_total",
		"ptforward_site_adapter_request_errors_total",
		"ptforward_goroutines",
	}
	for _, name := range expected {
		if !registered[name] {
			t.Errorf("metric %s not found in Gather output", name)
		}
	}
}
