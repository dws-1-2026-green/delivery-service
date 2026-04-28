package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	MessagesReceived = promauto.NewCounter(prometheus.CounterOpts{
		Name: "delivery_messages_received_total",
		Help: "Total number of delivery tasks received from Kafka",
	})

	DeliveryAttempts = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "delivery_attempts_total",
		Help: "Individual delivery attempt outcomes",
	}, []string{"status"}) // success | failure

	DeliveryFinalStatus = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "delivery_final_status_total",
		Help: "Final delivery outcome after all retries",
	}, []string{"status"}) // success | exhausted

	DeliveryAttemptDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "delivery_attempt_duration_seconds",
		Help:    "Duration of a single HTTP delivery attempt",
		Buckets: prometheus.DefBuckets,
	})

	DeliveryRetriesTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "delivery_retries_total",
		Help: "Total retry attempts (excludes the first delivery attempt)",
	})
)
