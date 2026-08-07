// Package metrics tracks the metrics
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// RequestCounter tracks the total number of HTTP requests
// var RequestCounter = promauto.NewCounter(
// 	prometheus.CounterOpts{
// 		Name: "app_http_requests_total",
// 		Help: "Total number of HTTP requests processed",
// 	},
// )
// TODO: Think about http lvl metrics later.

// JobsProcessed tracks the total number of processed jobs
var JobsProcessed = promauto.NewCounter(
	prometheus.CounterOpts{
		Name: "job_processed_total",
		Help: "Total number of successful jobs processed",
	},
)

// JobsFailed tracks the total number of failed jobs
var JobsFailed = promauto.NewCounter(
	prometheus.CounterOpts{
		Name: "job_failed_total",
		Help: "Total number of failed jobs",
	},
)

// JobsRetry tracks the total number of jobs retry
var JobsRetry = promauto.NewCounter(
	prometheus.CounterOpts{
		Name: "job_retry_total",
		Help: "Total number of jobs retried",
	},
)

// QueueDepth tracks the number of jobs waiting in the queue
var QueueDepth = promauto.NewGauge(prometheus.GaugeOpts{
	Name: "queue_depth",
	Help: "Current num of jobs in queue",
})

// JobDuration tracks how long does it take to process a job
var JobDuration = promauto.NewHistogram(
	prometheus.HistogramOpts{
		Name: "job_duration_ms",
		Help: "Duration of job processing in milliseconds",
	},
)

// JobsProcessedByType tracks number of processed jobs by type
var JobsProcessedByType = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "jobs_processed_by_type_total",
		Help: "number of jobs processed of a type",
	},
	[]string{"job_type"},
)

// DBErrors tracks the total number of db failures
var DBErrors = promauto.NewCounter(
	prometheus.CounterOpts{
		Name: "db_errors_total",
		Help: "Total number of database operation errors",
	},
)

// JobPersistenceFailures tracks the number of times job's state didn't change to fail or success
var JobPersistenceFailures = promauto.NewCounter(
	prometheus.CounterOpts{
		Name: "job_persistence_failures_total",
		Help: "Number of times job state could not be persisted",
	},
)
