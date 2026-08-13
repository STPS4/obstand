package obs

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type Metrics struct {
	// Входящие запросы (server-side).
	RequestsTotal   *prometheus.CounterVec
	RequestDuration *prometheus.HistogramVec

	// Исходящие запросы (client-side). Нужны сервисам, которые сами
	// ходят в другие сервисы — у нас это пока только gateway.
	ClientRequestsTotal   *prometheus.CounterVec
	ClientRequestDuration *prometheus.HistogramVec
}

func NewRegistry() *prometheus.Registry {
	reg := prometheus.NewRegistry()
	reg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
	return reg
}

func NewMetrics(reg prometheus.Registerer) *Metrics {
	f := promauto.With(reg)

	return &Metrics{
		RequestsTotal: f.NewCounterVec(prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total HTTP requests served, by method/path/status.",
		}, []string{"method", "path", "status"}),

		RequestDuration: f.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds.",
			Buckets: prometheus.DefBuckets,
		}, []string{"method", "path"}),

		ClientRequestsTotal: f.NewCounterVec(prometheus.CounterOpts{
			Name: "http_client_requests_total",
			Help: "Total outgoing HTTP requests, by target/method/status.",
		}, []string{"target", "method", "status"}),

		ClientRequestDuration: f.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "http_client_request_duration_seconds",
			Help:    "Outgoing HTTP request duration in seconds, by target.",
			Buckets: prometheus.DefBuckets,
		}, []string{"target", "method"}),
	}
}
