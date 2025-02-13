package gateway

import "github.com/prometheus/client_golang/prometheus"

const (
	metricNamespace = "prometheus_mimic"
	metricSubsystem = "gateway"
)

var (
	metricWriteBatchesRequests = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: metricNamespace,
			Subsystem: metricSubsystem,
			Name:      "write_batches_requests_total",
		},
	)
	metricWriteBatchesReceivedBytes = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: metricNamespace,
			Subsystem: metricSubsystem,
			Name:      "write_batches_received_bytes_total",
		},
	)
	metricWriteBatchesReceivedUncompressedBytes = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: metricNamespace,
			Subsystem: metricSubsystem,
			Name:      "write_batches_received_uncompressed_bytes_total",
		},
	)
	metricsWriteBatchesRequestsDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: metricNamespace,
			Subsystem: metricSubsystem,
			Name:      "write_batches_requests_duration_seconds",
			Help:      "Duration of write batches requests",
			Buckets:   prometheus.DefBuckets,
		},
	)
	metricWriteBatchesRequestsEncoding = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: metricNamespace,
			Subsystem: metricSubsystem,
			Name:      "write_batches_requests_encoding_total",
		},
		[]string{"encoding"},
	)
	metricWriteKafkaMessages = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: metricNamespace,
			Subsystem: metricSubsystem,
			Name:      "write_kafka_messages_total",
		},
		[]string{"topic"},
	)
)

func init() {
	prometheus.MustRegister(metricWriteBatchesRequests)
	prometheus.MustRegister(metricWriteBatchesReceivedBytes)
	prometheus.MustRegister(metricWriteBatchesReceivedUncompressedBytes)
	prometheus.MustRegister(metricsWriteBatchesRequestsDuration)
	prometheus.MustRegister(metricWriteBatchesRequestsEncoding)
	prometheus.MustRegister(metricWriteKafkaMessages)
}
