package admin_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	dto "github.com/prometheus/client_model/go"

	"github.com/federicoserini/mobile-db/admin"
)

type stubMetrics struct{}

func (s *stubMetrics) Gather() ([]*dto.MetricFamily, error) {
	gaugeName := "mobiledb_active_devices"
	gaugeType := dto.MetricType_GAUGE
	val := float64(7)
	return []*dto.MetricFamily{{
		Name: &gaugeName,
		Type: &gaugeType,
		Metric: []*dto.Metric{
			{Gauge: &dto.Gauge{Value: &val}},
		},
	}}, nil
}

func TestMetricsPageRenders(t *testing.T) {
	tmpl, err := admin.LoadTemplates()
	if err != nil {
		t.Fatal(err)
	}
	h := admin.NewMetricsHandler(&stubMetrics{}, tmpl)
	req := httptest.NewRequest("GET", "/admin/metrics", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rr.Code, rr.Body)
	}
	if !strings.Contains(rr.Body.String(), "Active Devices") {
		t.Error("metrics page must contain Active Devices card")
	}
}
