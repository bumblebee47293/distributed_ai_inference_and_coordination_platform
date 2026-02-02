package observability

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// InferenceRequestsTotal counts total inference requests
	InferenceRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "inference_requests_total",
			Help: "Total number of inference requests",
		},
		[]string{"model", "version", "type", "status"},
	)

	// InferenceRequestDuration tracks inference request latency
	InferenceRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "inference_request_duration_seconds",
			Help:    "Inference request latency in seconds",
			Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
		},
		[]string{"model", "version", "type"},
	)

	// BatchJobsSubmitted counts batch jobs submitted
	BatchJobsSubmitted = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "batch_jobs_submitted_total",
			Help: "Total number of batch jobs submitted",
		},
		[]string{"model", "version"},
	)
)

// InitMetrics initializes Prometheus metrics
func InitMetrics() {
	// Metrics are auto-registered via promauto
}
