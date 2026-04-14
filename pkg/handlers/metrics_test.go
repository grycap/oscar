package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/v3/pkg/metrics"
	"github.com/grycap/oscar/v3/pkg/types"
)

type fakeServiceInventory struct {
	services []metrics.ServiceDescriptor
}

func (f *fakeServiceInventory) Name() string {
	return "service-inventory"
}

func (f *fakeServiceInventory) ListServices(ctx context.Context, tr metrics.TimeRange) ([]metrics.ServiceDescriptor, *types.SourceStatus, error) {
	return f.services, &types.SourceStatus{Name: f.Name(), Status: "ok"}, nil
}

type fakeUsageMetrics struct {
	cpu float64
	gpu float64
}

func (f *fakeUsageMetrics) Name() string {
	return "usage-metrics"
}

func (f *fakeUsageMetrics) UsageHours(ctx context.Context, tr metrics.TimeRange, serviceID string) (float64, float64, *types.SourceStatus, error) {
	return f.cpu, f.gpu, &types.SourceStatus{Name: f.Name(), Status: "ok"}, nil
}

type fakeRequestLogs struct {
	records []metrics.RequestRecord
}

func (f *fakeRequestLogs) Name() string {
	return "request-logs"
}

func (f *fakeRequestLogs) ListRequests(ctx context.Context, tr metrics.TimeRange, serviceID string) ([]metrics.RequestRecord, *types.SourceStatus, error) {
	return f.records, &types.SourceStatus{Name: f.Name(), Status: "ok"}, nil
}

type fakeCountrySource struct{}

func (f *fakeCountrySource) Name() string {
	return "country-attribution"
}

func (f *fakeCountrySource) CountryForRecord(ctx context.Context, record metrics.RequestRecord) (string, *types.SourceStatus, error) {
	if record.Country == "" {
		return "unknown", &types.SourceStatus{Name: f.Name(), Status: "ok"}, nil
	}
	return record.Country, &types.SourceStatus{Name: f.Name(), Status: "ok"}, nil
}

func setupMetricsRouter(agg *metrics.Aggregator) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/system/metrics", MakeMetricsSummaryHandler(agg))
	router.GET("/system/metrics/", MakeMetricsSummaryHandler(agg))
	router.GET("/system/metrics/breakdown", MakeMetricsBreakdownHandler(agg))
	router.GET("/system/metrics/:serviceName", MakeMetricValueHandler(agg))
	return router
}

func TestMetricValueHandler(t *testing.T) {
	agg := &metrics.Aggregator{
		Sources: metrics.Sources{
			UsageMetrics: &fakeUsageMetrics{cpu: 2.5, gpu: 1.0},
		},
	}
	router := setupMetricsRouter(agg)

	req := httptest.NewRequest(http.MethodGet, "/system/metrics/svc-a?metric=cpu-hours&start=2026-01-01T00:00:00Z&end=2026-01-02T00:00:00Z", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp types.MetricValueResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unexpected json error: %v", err)
	}
	if resp.Value != 2.5 {
		t.Fatalf("expected cpu value 2.5, got %v", resp.Value)
	}
}

func TestMetricValueHandlerAllMetrics(t *testing.T) {
	agg := &metrics.Aggregator{
		Sources: metrics.Sources{
			UsageMetrics: &fakeUsageMetrics{cpu: 2.5, gpu: 1.0},
			RequestLogs: &fakeRequestLogs{records: []metrics.RequestRecord{
				{ServiceID: "svc-a", UserID: "u1", Type: metrics.RequestSync},
				{ServiceID: "svc-a", UserID: "u2", Type: metrics.RequestAsync},
			}},
		},
	}
	router := setupMetricsRouter(agg)

	req := httptest.NewRequest(http.MethodGet, "/system/metrics/svc-a?start=2026-01-01T00:00:00Z&end=2026-01-02T00:00:00Z", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp types.ServiceMetricsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unexpected json error: %v", err)
	}
	if resp.ServiceName != "svc-a" {
		t.Fatalf("expected service_name svc-a, got %s", resp.ServiceName)
	}
	if len(resp.Metrics) == 0 {
		t.Fatalf("expected metrics list to be populated")
	}
	foundCPU := false
	for _, metric := range resp.Metrics {
		if metric.Metric == types.MetricCPUHours {
			foundCPU = true
		}
	}
	if !foundCPU {
		t.Fatalf("expected cpu-hours metric in response")
	}
}

func TestMetricsSummaryHandler(t *testing.T) {
	agg := &metrics.Aggregator{
		Sources: metrics.Sources{
			ServiceInventory: &fakeServiceInventory{services: []metrics.ServiceDescriptor{{ID: "svc-a"}}},
			UsageMetrics:     &fakeUsageMetrics{cpu: 1.0, gpu: 0.5},
			RequestLogs: &fakeRequestLogs{records: []metrics.RequestRecord{
				{ServiceID: "svc-a", UserID: "u1", Type: metrics.RequestSync, Country: "ES"},
			}},
			CountrySource: &fakeCountrySource{},
		},
	}
	router := setupMetricsRouter(agg)

	req := httptest.NewRequest(http.MethodGet, "/system/metrics?start=2026-01-01T00:00:00Z&end=2026-01-02T00:00:00Z", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp types.MetricsSummaryResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unexpected json error: %v", err)
	}
	if resp.Totals.ServicesCountActive != 1 {
		t.Fatalf("expected services_count_active 1, got %d", resp.Totals.ServicesCountActive)
	}
}

func TestMetricsSummaryDefaultsToLastDay(t *testing.T) {
	agg := &metrics.Aggregator{
		Sources: metrics.Sources{
			ServiceInventory: &fakeServiceInventory{services: []metrics.ServiceDescriptor{{ID: "svc-a"}}},
			UsageMetrics:     &fakeUsageMetrics{cpu: 1.0, gpu: 0.5},
			RequestLogs:      &fakeRequestLogs{records: []metrics.RequestRecord{}},
			CountrySource:    &fakeCountrySource{},
		},
	}
	router := setupMetricsRouter(agg)

	before := time.Now().UTC()
	req := httptest.NewRequest(http.MethodGet, "/system/metrics", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp types.MetricsSummaryResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unexpected json error: %v", err)
	}

	after := time.Now().UTC()
	if resp.End.Before(before) || resp.End.After(after.Add(2*time.Second)) {
		t.Fatalf("unexpected end timestamp: %s", resp.End.Format(time.RFC3339))
	}
	if resp.End.Sub(resp.Start) < 23*time.Hour || resp.End.Sub(resp.Start) > 25*time.Hour {
		t.Fatalf("unexpected default range: %s", resp.End.Sub(resp.Start))
	}
}

func TestMetricsBreakdownCSVExport(t *testing.T) {
	agg := &metrics.Aggregator{
		Sources: metrics.Sources{
			RequestLogs: &fakeRequestLogs{records: []metrics.RequestRecord{
				{ServiceID: "svc-a", UserID: "u1", Type: metrics.RequestSync, Country: "ES"},
			}},
			CountrySource: &fakeCountrySource{},
		},
	}
	router := setupMetricsRouter(agg)

	req := httptest.NewRequest(http.MethodGet, "/system/metrics/breakdown?start=2026-01-01T00:00:00Z&end=2026-01-02T00:00:00Z&group_by=service&format=csv", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/csv" {
		t.Fatalf("expected text/csv content type, got %s", ct)
	}
	if len(rec.Body.Bytes()) == 0 {
		t.Fatalf("expected csv body")
	}
}

func TestMetricsBreakdownInvalidTimeRange(t *testing.T) {
	agg := &metrics.Aggregator{}
	router := setupMetricsRouter(agg)
	req := httptest.NewRequest(http.MethodGet, "/system/metrics/breakdown?start=bad&end=2026-01-02T00:00:00Z&group_by=service", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestMetricsValueMissingServiceID(t *testing.T) {
	agg := &metrics.Aggregator{}
	router := setupMetricsRouter(agg)
	req := httptest.NewRequest(http.MethodGet, "/system/metrics/%20?metric=cpu-hours&start=2026-01-01T00:00:00Z&end=2026-01-02T00:00:00Z", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestMetricsSummaryTimeRangeOrder(t *testing.T) {
	agg := &metrics.Aggregator{}
	router := setupMetricsRouter(agg)
	req := httptest.NewRequest(http.MethodGet, "/system/metrics?start=2026-01-02T00:00:00Z&end=2026-01-01T00:00:00Z", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func BenchmarkMetricsBreakdownCSVExport(b *testing.B) {
	resp := types.MetricsBreakdownResponse{
		GroupBy: "service",
		Items:   benchmarkBreakdownItems(250, 6),
	}
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		payload, err := renderBreakdownCSV(resp)
		if err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
		if len(payload) == 0 {
			b.Fatalf("expected csv payload")
		}
	}
}

func benchmarkBreakdownItems(itemCount int, countriesPerItem int) []types.BreakdownItem {
	items := make([]types.BreakdownItem, 0, itemCount)
	for i := 0; i < itemCount; i++ {
		countries := make([]types.CountryCount, 0, countriesPerItem)
		for j := 0; j < countriesPerItem; j++ {
			countries = append(countries, types.CountryCount{
				Country:      "C" + strconv.Itoa(j),
				RequestCount: (i + 1) * (j + 2),
			})
		}
		items = append(items, types.BreakdownItem{
			Key:              "svc-" + strconv.Itoa(i),
			Membership:       "member",
			ExecutionsCount:  100 + i,
			UniqueUsersCount: 10 + (i % 10),
			Countries:        countries,
		})
	}
	return items
}
