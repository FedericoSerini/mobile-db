package handlers

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	dto "github.com/prometheus/client_model/go"
)

type MetricsRegistry struct {
	SyncRequests   *prometheus.CounterVec
	CRDTOpsTotal   prometheus.Counter
	ActiveDevices  prometheus.Gauge
	SSEConnections prometheus.Gauge
	CompactionRuns prometheus.Counter
	DBSizeBytes    *prometheus.GaugeVec
	reg            *prometheus.Registry
}

func NewMetricsRegistry() *MetricsRegistry {
	reg := prometheus.NewRegistry()
	m := &MetricsRegistry{reg: reg}

	m.SyncRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "mobiledb_sync_requests_total",
		Help: "Total sync requests by app_id.",
	}, []string{"app_id"})

	m.CRDTOpsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "mobiledb_crdt_ops_total",
		Help: "Total CRDT ops merged.",
	})

	m.ActiveDevices = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "mobiledb_active_devices",
		Help: "Active (non-revoked) devices.",
	})

	m.SSEConnections = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "mobiledb_sse_connections",
		Help: "Active SSE connections.",
	})

	m.CompactionRuns = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "mobiledb_compaction_runs_total",
		Help: "Total compaction runs.",
	})

	m.DBSizeBytes = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mobiledb_db_size_bytes",
		Help: "SQLite file size per tenant.",
	}, []string{"app_id"})

	reg.MustRegister(m.SyncRequests, m.CRDTOpsTotal, m.ActiveDevices,
		m.SSEConnections, m.CompactionRuns, m.DBSizeBytes)
	return m
}

func NewMetricsHandler(m *MetricsRegistry) http.Handler {
	return promhttp.HandlerFor(m.reg, promhttp.HandlerOpts{})
}

// Gather exposes the internal registry for admin UI consumption.
func (m *MetricsRegistry) Gather() ([]*dto.MetricFamily, error) {
	return m.reg.Gather()
}
