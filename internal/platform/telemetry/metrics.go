package telemetry

import (
	"context"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

const namespace = "fitcore"

// Metrics methods are nil-safe so adapters can be built without metrics.
type Metrics struct {
	Registry *prometheus.Registry

	HTTPRequestsTotal         *prometheus.CounterVec
	HTTPRequestDurationSec    *prometheus.HistogramVec
	ApplicationErrorsTotal    *prometheus.CounterVec
	DatabaseErrorsTotal       *prometheus.CounterVec
	membershipsPurchasedTotal prometheus.Counter
}

func New() *Metrics {
	reg := prometheus.NewRegistry()
	reg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	httpRequests := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "http_requests_total",
		Help:      "Total number of HTTP requests handled by the server.",
	}, []string{"method", "path", "status"})
	httpDuration := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: namespace,
		Name:      "http_request_duration_seconds",
		Help:      "HTTP request latency in seconds.",
		Buckets:   prometheus.DefBuckets,
	}, []string{"method", "path"})
	appErrors := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "application_errors_total",
		Help:      "Number of application-level errors returned to clients.",
	}, []string{"module", "operation", "status"})
	dbErrors := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "database_errors_total",
		Help:      "Number of database errors observed by repositories.",
	}, []string{"module", "operation"})
	membershipsPurchased := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "memberships_purchased_total",
		Help:      "Total number of memberships purchased.",
	})
	reg.MustRegister(httpRequests, httpDuration, appErrors, dbErrors, membershipsPurchased)

	return &Metrics{
		Registry:                  reg,
		HTTPRequestsTotal:         httpRequests,
		HTTPRequestDurationSec:    httpDuration,
		ApplicationErrorsTotal:    appErrors,
		DatabaseErrorsTotal:       dbErrors,
		membershipsPurchasedTotal: membershipsPurchased,
	}
}

func (m *Metrics) ObserveHTTPRequest(method, path string, status int, d time.Duration) {
	if m == nil {
		return
	}
	m.HTTPRequestsTotal.WithLabelValues(method, path, strconv.Itoa(status)).Inc()
	m.HTTPRequestDurationSec.WithLabelValues(method, path).Observe(d.Seconds())
}

func (m *Metrics) RecordApplicationError(module, operation string, status int) {
	if m == nil {
		return
	}
	m.ApplicationErrorsTotal.WithLabelValues(module, operation, strconv.Itoa(status)).Inc()
}

func (m *Metrics) RecordDatabaseError(module, operation string) {
	if m == nil {
		return
	}
	m.DatabaseErrorsTotal.WithLabelValues(module, operation).Inc()
}

func (m *Metrics) RecordMembershipPurchased(_ context.Context) {
	if m == nil {
		return
	}
	m.membershipsPurchasedTotal.Inc()
}
