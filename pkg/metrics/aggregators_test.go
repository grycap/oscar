package metrics

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/grycap/oscar/v3/pkg/types"
)

type fakeServiceInventory struct {
	services []ServiceDescriptor
	status   *types.SourceStatus
	err      error
}

func (f *fakeServiceInventory) Name() string {
	return "service-inventory"
}

func (f *fakeServiceInventory) ListServices(ctx context.Context, tr TimeRange) ([]ServiceDescriptor, *types.SourceStatus, error) {
	if f.status != nil {
		return f.services, f.status, f.err
	}
	return f.services, okStatus(f.Name(), ""), f.err
}

type fakeUsageMetrics struct {
	cpuByService map[string]float64
	gpuByService map[string]float64
	err          error
}

func (f *fakeUsageMetrics) Name() string {
	return "usage-metrics"
}

func (f *fakeUsageMetrics) UsageHours(ctx context.Context, tr TimeRange, serviceID string) (float64, float64, *types.SourceStatus, error) {
	if f.err != nil {
		return 0, 0, missingStatus(f.Name(), f.err), f.err
	}
	return f.cpuByService[serviceID], f.gpuByService[serviceID], okStatus(f.Name(), ""), nil
}

type fakeRequestLogs struct {
	records []RequestRecord
	err     error
	name    string
}

func (f *fakeRequestLogs) Name() string {
	if f.name != "" {
		return f.name
	}
	return "request-logs"
}

func (f *fakeRequestLogs) ListRequests(ctx context.Context, tr TimeRange, serviceID string) ([]RequestRecord, *types.SourceStatus, error) {
	if f.err != nil {
		return nil, missingStatus(f.Name(), f.err), f.err
	}
	if serviceID == "" {
		return f.records, okStatus(f.Name(), ""), nil
	}
	filtered := make([]RequestRecord, 0, len(f.records))
	for _, record := range f.records {
		if record.ServiceID == serviceID {
			filtered = append(filtered, record)
		}
	}
	return filtered, okStatus(f.Name(), ""), nil
}

type fakeRoster struct {
	class map[string]string
}

func (f *fakeRoster) Name() string {
	return "user-roster"
}

func (f *fakeRoster) Classification(ctx context.Context, userID string) (string, *types.SourceStatus, error) {
	if value, ok := f.class[userID]; ok {
		return value, okStatus(f.Name(), ""), nil
	}
	return "unknown", okStatus(f.Name(), ""), nil
}

type fakeCountrySource struct{}

func (f *fakeCountrySource) Name() string {
	return "country-attribution"
}

func (f *fakeCountrySource) CountryForRecord(ctx context.Context, record RequestRecord) (string, *types.SourceStatus, error) {
	if record.Country == "" {
		return "unknown", okStatus(f.Name(), ""), nil
	}
	return record.Country, okStatus(f.Name(), ""), nil
}

func TestSummaryAggregationTotals(t *testing.T) {
	tr := TimeRange{Start: time.Now().Add(-time.Hour), End: time.Now()}
	services := []ServiceDescriptor{
		{ID: "svc-a"}, {ID: "svc-b"},
	}
	requests := []RequestRecord{
		{ServiceID: "svc-a", UserID: "u1", Type: RequestSync, Country: "ES", AuthMethod: "oidc"},
		{ServiceID: "svc-a", UserID: "u1", Type: RequestAsync, Country: "ES", AuthMethod: "oidc"},
		{ServiceID: "svc-b", UserID: "u2", Type: RequestSync, Country: "US", AuthMethod: "service_token"},
		{ServiceID: "svc-b", UserID: "u3", Type: RequestSync, Country: "", AuthMethod: "oidc"},
	}

	agg := Aggregator{
		Sources: Sources{
			ServiceInventory: &fakeServiceInventory{services: services},
			UsageMetrics: &fakeUsageMetrics{
				cpuByService: map[string]float64{"svc-a": 1.5, "svc-b": 2},
				gpuByService: map[string]float64{"svc-a": 0.5, "svc-b": 1},
			},
			RequestLogs:   &fakeRequestLogs{records: requests},
			UserRoster:    &fakeRoster{class: map[string]string{}},
			CountrySource: &fakeCountrySource{},
		},
	}

	resp, err := agg.Summary(context.Background(), tr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Totals.ServicesCountActive != 2 {
		t.Fatalf("expected services_count_active 2, got %d", resp.Totals.ServicesCountActive)
	}
	if resp.Totals.ServicesCountTotal != 2 {
		t.Fatalf("expected services_count_total 2, got %d", resp.Totals.ServicesCountTotal)
	}
	if resp.Totals.CPUHoursTotal != 3.5 || resp.Totals.GPUHoursTotal != 1.5 {
		t.Fatalf("unexpected CPU/GPU totals: %v/%v", resp.Totals.CPUHoursTotal, resp.Totals.GPUHoursTotal)
	}
	if resp.Totals.RequestsCountTotal != 4 || resp.Totals.RequestsCountSync != 3 || resp.Totals.RequestsCountAsync != 1 {
		t.Fatalf("unexpected request totals: %+v", resp.Totals)
	}
	if resp.Totals.UsersCount != 3 {
		t.Fatalf("expected users_count 3, got %d", resp.Totals.UsersCount)
	}
	if len(resp.Totals.Users) != 3 {
		t.Fatalf("expected 3 users, got %d", len(resp.Totals.Users))
	}
	if resp.Totals.CountriesCount != 2 {
		t.Fatalf("expected countries_count 2, got %d", resp.Totals.CountriesCount)
	}
	if len(resp.Totals.Countries) != 2 {
		t.Fatalf("expected 2 countries, got %d", len(resp.Totals.Countries))
	}
}

func TestSummaryIncludesExposedRequests(t *testing.T) {
	tr := TimeRange{Start: time.Now().Add(-time.Hour), End: time.Now()}
	requests := []RequestRecord{
		{ServiceID: "svc-a", UserID: "u1", Type: RequestSync},
	}
	exposed := []RequestRecord{
		{ServiceID: "svc-b"},
		{ServiceID: "svc-a"},
	}

	agg := Aggregator{
		Sources: Sources{
			ServiceInventory:   &fakeServiceInventory{services: []ServiceDescriptor{{ID: "svc-a"}, {ID: "svc-b"}}},
			UsageMetrics:       &fakeUsageMetrics{cpuByService: map[string]float64{}, gpuByService: map[string]float64{}},
			RequestLogs:        &fakeRequestLogs{records: requests},
			ExposedRequestLogs: &fakeRequestLogs{records: exposed, name: "exposed-request-logs"},
			CountrySource:      &fakeCountrySource{},
		},
	}

	resp, err := agg.Summary(context.Background(), tr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Totals.RequestsCountExposed != 2 {
		t.Fatalf("expected requests_count_exposed 2, got %d", resp.Totals.RequestsCountExposed)
	}
	if resp.Totals.ServicesCountTotal != 2 {
		t.Fatalf("expected services_count_total 2, got %d", resp.Totals.ServicesCountTotal)
	}
}

func TestCountryAttributionUnknownExcluded(t *testing.T) {
	tr := TimeRange{Start: time.Now().Add(-time.Hour), End: time.Now()}
	requests := []RequestRecord{
		{ServiceID: "svc-a", UserID: "u1", Type: RequestSync, Country: ""},
	}

	agg := Aggregator{
		Sources: Sources{
			ServiceInventory: &fakeServiceInventory{services: []ServiceDescriptor{{ID: "svc-a"}}},
			UsageMetrics:     &fakeUsageMetrics{cpuByService: map[string]float64{"svc-a": 0}, gpuByService: map[string]float64{"svc-a": 0}},
			RequestLogs:      &fakeRequestLogs{records: requests},
			CountrySource:    &fakeCountrySource{},
		},
	}

	resp, err := agg.Summary(context.Background(), tr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Totals.CountriesCount != 0 {
		t.Fatalf("expected countries_count 0, got %d", resp.Totals.CountriesCount)
	}
}

func TestBreakdownMembershipClassification(t *testing.T) {
	tr := TimeRange{Start: time.Now().Add(-time.Hour), End: time.Now()}
	requests := []RequestRecord{
		{ServiceID: "svc-a", UserID: "u1", Type: RequestSync, Country: "ES", AuthMethod: "oidc"},
		{ServiceID: "svc-a", UserID: "u2", Type: RequestSync, Country: "ES", AuthMethod: "service_token"},
	}

	agg := Aggregator{
		Sources: Sources{
			RequestLogs:   &fakeRequestLogs{records: requests},
			UserRoster:    &fakeRoster{class: map[string]string{}},
			CountrySource: &fakeCountrySource{},
		},
	}

	resp, err := agg.Breakdown(context.Background(), tr, "user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(resp.Items))
	}
	for _, item := range resp.Items {
		if item.Key == "u1" && item.Membership != "member" {
			t.Fatalf("expected u1 member, got %s", item.Membership)
		}
		if item.Key == "u2" && item.Membership != "external" {
			t.Fatalf("expected u2 external, got %s", item.Membership)
		}
	}
}

func TestMetricValueSyncCount(t *testing.T) {
	tr := TimeRange{Start: time.Now().Add(-time.Hour), End: time.Now()}
	requests := []RequestRecord{
		{ServiceID: "svc-a", UserID: "u1", Type: RequestSync},
		{ServiceID: "svc-a", UserID: "u2", Type: RequestAsync},
		{ServiceID: "svc-b", UserID: "u3", Type: RequestSync},
	}

	agg := Aggregator{
		Sources: Sources{
			RequestLogs: &fakeRequestLogs{records: requests},
		},
	}

	resp, err := agg.MetricValue(context.Background(), tr, "svc-a", types.MetricRequestsSync)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Value != 1 {
		t.Fatalf("expected sync count 1, got %v", resp.Value)
	}
}

func TestBuildLokiQuery(t *testing.T) {
	source := LokiRequestLogSource{
		QueryTemplate:         "{namespace=\"{{namespace}}\", app=\"{{app}}\"}",
		Namespace:             "oscar",
		AppLabel:              "oscar",
		ServiceFilterTemplate: "/(job|run)/%s",
	}
	query := source.buildQuery("svc-a")
	if !strings.Contains(query, "namespace=\"oscar\"") || !strings.Contains(query, "app=\"oscar\"") {
		t.Fatalf("expected namespace/app labels in query, got %s", query)
	}
	if !strings.Contains(query, "/(job|run)/svc-a") {
		t.Fatalf("expected service filter in query, got %s", query)
	}
}

func TestParseGinExecutionLogFromGinPrefix(t *testing.T) {
	line := "[GIN] 2026/01/20 - 10:00:00 | 200 |  12.345ms | 10.0.0.1 | POST    /job/test-service | user@example.com"
	record, ok := parseGinExecutionLog(line)
	if !ok {
		t.Fatal("expected log line to parse")
	}
	if record.ServiceID != "test-service" {
		t.Fatalf("expected serviceID test-service, got %s", record.ServiceID)
	}
	if record.Type != RequestAsync {
		t.Fatalf("expected async request type, got %s", record.Type)
	}
	if record.UserID != "user@example.com" {
		t.Fatalf("expected user@example.com, got %s", record.UserID)
	}
}

func TestParseIngressAccessLog(t *testing.T) {
	line := "172.18.0.1 - - [23/Jan/2026:18:13:07 +0000] \"GET /system/services/gmolto-nginx/exposed/ HTTP/1.1\" 200 17 \"-\" \"curl/8.7.1\" 109 0.003 [oscar-svc-gmolto-nginx-svc-80] [] 10.244.0.223:80 17 0.002 200 a72c147a794286b864361ecca7a31075"
	record, ok := parseIngressAccessLog(line)
	if !ok {
		t.Fatal("expected ingress log line to parse")
	}
	if record.ServiceID != "gmolto-nginx" {
		t.Fatalf("expected serviceID gmolto-nginx, got %s", record.ServiceID)
	}
	if record.Timestamp.IsZero() {
		t.Fatal("expected timestamp to be parsed")
	}
}

func TestSummaryCompletenessMissingSource(t *testing.T) {
	tr := TimeRange{Start: time.Now().Add(-time.Hour), End: time.Now()}

	agg := Aggregator{
		Sources: Sources{
			ServiceInventory: &fakeServiceInventory{services: []ServiceDescriptor{{ID: "svc-a"}}},
			UsageMetrics:     &fakeUsageMetrics{cpuByService: map[string]float64{"svc-a": 1}, gpuByService: map[string]float64{"svc-a": 1}},
			RequestLogs:      &fakeRequestLogs{err: errors.New("missing logs")},
			CountrySource:    &fakeCountrySource{},
		},
	}

	resp, err := agg.Summary(context.Background(), tr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	foundMissing := false
	for _, status := range resp.Sources {
		if status.Name == "request-logs" && status.Status == "missing" {
			foundMissing = true
		}
	}
	if !foundMissing {
		t.Fatalf("expected missing request-logs status")
	}
}

func TestSummaryBreakdownReconciliation(t *testing.T) {
	tr := TimeRange{Start: time.Now().Add(-time.Hour), End: time.Now()}
	requests := []RequestRecord{
		{ServiceID: "svc-a", UserID: "u1", Type: RequestSync, Country: "ES"},
		{ServiceID: "svc-a", UserID: "u2", Type: RequestSync, Country: "ES"},
		{ServiceID: "svc-b", UserID: "u1", Type: RequestAsync, Country: "US"},
	}

	agg := Aggregator{
		Sources: Sources{
			ServiceInventory: &fakeServiceInventory{services: []ServiceDescriptor{{ID: "svc-a"}, {ID: "svc-b"}}},
			UsageMetrics:     &fakeUsageMetrics{cpuByService: map[string]float64{"svc-a": 0, "svc-b": 0}, gpuByService: map[string]float64{"svc-a": 0, "svc-b": 0}},
			RequestLogs:      &fakeRequestLogs{records: requests},
			CountrySource:    &fakeCountrySource{},
		},
	}

	summary, err := agg.Summary(context.Background(), tr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	breakdown, err := agg.Breakdown(context.Background(), tr, "service")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	breakdownTotal := 0
	for _, item := range breakdown.Items {
		breakdownTotal += item.ExecutionsCount
	}
	if breakdownTotal != summary.Totals.RequestsCountTotal {
		t.Fatalf("expected summary total %d to match breakdown total %d", summary.Totals.RequestsCountTotal, breakdownTotal)
	}
}

func TestCountryAttributionPercentage(t *testing.T) {
	records := []RequestRecord{
		{Country: "ES"},
		{Country: "US"},
		{Country: ""},
		{Country: "US"},
	}

	known := 0
	for _, record := range records {
		if record.Country != "" {
			known++
		}
	}
	percentage := float64(known) / float64(len(records)) * 100
	if percentage != 75 {
		t.Fatalf("expected 75%% known countries, got %v", percentage)
	}
}

func BenchmarkSummaryAggregation(b *testing.B) {
	tr := TimeRange{Start: time.Now().Add(-time.Hour), End: time.Now()}
	requests := make([]RequestRecord, 1000)
	for i := range requests {
		requests[i] = RequestRecord{
			ServiceID: "svc-a",
			UserID:    "u1",
			Type:      RequestSync,
			Country:   "ES",
		}
	}

	agg := Aggregator{
		Sources: Sources{
			ServiceInventory: &fakeServiceInventory{services: []ServiceDescriptor{{ID: "svc-a"}}},
			UsageMetrics:     &fakeUsageMetrics{cpuByService: map[string]float64{"svc-a": 0}, gpuByService: map[string]float64{"svc-a": 0}},
			RequestLogs:      &fakeRequestLogs{records: requests},
			CountrySource:    &fakeCountrySource{},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = agg.Summary(context.Background(), tr)
	}
}
