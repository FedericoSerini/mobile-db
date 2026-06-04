package admin

import (
	"fmt"
	"html/template"
	"net/http"
	"strings"

	dto "github.com/prometheus/client_model/go"
)

// MetricsSource is satisfied by *handlers.MetricsRegistry after adding Gather().
type MetricsSource interface {
	Gather() ([]*dto.MetricFamily, error)
}

type appMetricRow struct {
	AppID   string
	Syncs   int64
	DBBytes int64
}

type metricsHandler struct {
	src  MetricsSource
	tmpl *template.Template
}

func NewMetricsHandler(src MetricsSource, tmpl *template.Template) http.Handler {
	return &metricsHandler{src: src, tmpl: tmpl}
}

func (h *metricsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	mfs, err := h.src.Gather()
	if err != nil {
		http.Error(w, "gather metrics: "+err.Error(), http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"ActiveDevices":     int64(sumMetric(mfs, "mobiledb_active_devices")),
		"CRDTOpsTotal":      int64(sumMetric(mfs, "mobiledb_crdt_ops_total")),
		"SSEConnections":    int64(sumMetric(mfs, "mobiledb_sse_connections")),
		"CompactionRuns":    int64(sumMetric(mfs, "mobiledb_compaction_runs_total")),
		"SyncRequestsTotal": int64(sumMetric(mfs, "mobiledb_sync_requests_total")),
		"DBSizeMB":          fmt.Sprintf("%.1f", sumMetric(mfs, "mobiledb_db_size_bytes")/1024/1024),
		"PerApp":            perAppRows(mfs),
	}

	var content strings.Builder
	if err := h.tmpl.ExecuteTemplate(&content, "metrics", data); err != nil {
		http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	var page strings.Builder
	if err := h.tmpl.ExecuteTemplate(&page, "base", map[string]any{
		"Title":   "Metrics",
		"Content": template.HTML(content.String()),
	}); err != nil {
		http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(page.String()))
}

func sumMetric(mfs []*dto.MetricFamily, name string) float64 {
	for _, mf := range mfs {
		if mf.GetName() != name {
			continue
		}
		var sum float64
		for _, m := range mf.GetMetric() {
			if g := m.GetGauge(); g != nil {
				sum += g.GetValue()
			} else if c := m.GetCounter(); c != nil {
				sum += c.GetValue()
			}
		}
		return sum
	}
	return 0
}

func perAppRows(mfs []*dto.MetricFamily) []appMetricRow {
	syncs := map[string]int64{}
	dbBytes := map[string]int64{}

	for _, mf := range mfs {
		n := mf.GetName()
		if n != "mobiledb_sync_requests_total" && n != "mobiledb_db_size_bytes" {
			continue
		}
		for _, m := range mf.GetMetric() {
			var appID string
			for _, lp := range m.GetLabel() {
				if lp.GetName() == "app_id" {
					appID = lp.GetValue()
				}
			}
			if appID == "" {
				continue
			}
			switch n {
			case "mobiledb_sync_requests_total":
				if c := m.GetCounter(); c != nil {
					syncs[appID] += int64(c.GetValue())
				}
			case "mobiledb_db_size_bytes":
				if g := m.GetGauge(); g != nil {
					dbBytes[appID] += int64(g.GetValue())
				}
			}
		}
	}

	seen := map[string]bool{}
	var rows []appMetricRow
	for appID := range syncs {
		seen[appID] = true
		rows = append(rows, appMetricRow{AppID: appID, Syncs: syncs[appID], DBBytes: dbBytes[appID]})
	}
	for appID := range dbBytes {
		if !seen[appID] {
			rows = append(rows, appMetricRow{AppID: appID, DBBytes: dbBytes[appID]})
		}
	}
	return rows
}
