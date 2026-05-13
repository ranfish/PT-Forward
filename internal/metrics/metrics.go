package metrics

import (
	"runtime"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var once sync.Once

func Init(version string) {
	once.Do(func() {
		promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "ptforward_build_info",
			Help: "Build information",
		}, []string{"version", "go_version"}).WithLabelValues(version, runtime.Version()).Set(1)
	})
}

var HTTPRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "ptforward_http_requests_total",
	Help: "Total number of HTTP requests",
}, []string{"method", "path", "status"})

var HTTPRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Name:    "ptforward_http_request_duration_seconds",
	Help:    "HTTP request duration in seconds",
	Buckets: prometheus.DefBuckets,
}, []string{"method", "path"})

var Goroutines = promauto.NewGaugeFunc(prometheus.GaugeOpts{
	Name: "ptforward_goroutines",
	Help: "Current number of goroutines",
}, func() float64 {
	return float64(runtime.NumGoroutine())
})

var RSSTorrentsFetched = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "ptforward_rss_torrents_fetched_total",
	Help: "Total RSS torrents fetched",
}, []string{"site"})

var RSSTorrentsFiltered = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "ptforward_rss_torrents_filtered_total",
	Help: "Total RSS torrents filtered out",
}, []string{"site", "reason"})

var PublishTasksTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "ptforward_publish_tasks_total",
	Help: "Total publish tasks",
}, []string{"site", "status"})

var PublishDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Name:    "ptforward_publish_duration_seconds",
	Help:    "Publish task duration in seconds",
	Buckets: []float64{1, 5, 10, 30, 60, 120, 300},
}, []string{"site"})

var SiteRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "ptforward_site_adapter_requests_total",
	Help: "Total site adapter requests",
}, []string{"site", "method"})

var SiteRequestErrors = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "ptforward_site_adapter_request_errors_total",
	Help: "Total site adapter request errors",
}, []string{"site", "method"})
