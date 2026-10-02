package telemetry

import (
	"context"
	"database/sql"
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
	dbPoolOpen                prometheus.Gauge
	dbPoolInUse               prometheus.Gauge
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
	dbPoolOpen := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "db_pool_open",
		Help:      "Current number of open database connections in the app pool.",
	})
	dbPoolInUse := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "db_pool_in_use",
		Help:      "Current number of in-use database connections in the app pool.",
	})
	reg.MustRegister(httpRequests, httpDuration, appErrors, dbErrors, membershipsPurchased, dbPoolOpen, dbPoolInUse)

	return &Metrics{
		Registry:                  reg,
		HTTPRequestsTotal:         httpRequests,
		HTTPRequestDurationSec:    httpDuration,
		ApplicationErrorsTotal:    appErrors,
		DatabaseErrorsTotal:       dbErrors,
		membershipsPurchasedTotal: membershipsPurchased,
		dbPoolOpen:                dbPoolOpen,
		dbPoolInUse:               dbPoolInUse,
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

// ObservePoolStats records the database connection pool state. It is called
// by a background sampler in main so dashboards can see pool saturation.
func (m *Metrics) ObservePoolStats(stats sql.DBStats) {
	if m == nil {
		return
	}
	m.dbPoolOpen.Set(float64(stats.OpenConnections))
	m.dbPoolInUse.Set(float64(stats.InUse))
}
