package metrics

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/grycap/oscar/v4/pkg/types"
)

func newScopedPrometheusSource(t *testing.T, queries *[]string) *PrometheusUsageMetricsSource {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("ParseForm() error: %v", err)
		}
		query := r.Form.Get("query")
		*queries = append(*queries, query)
		value := "1"
		if strings.Contains(query, "gpu:") {
			value = "2"
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"status":"success","data":{"resultType":"vector","result":[{"metric":{},"value":[1,%q]}]}}`, value)
	}))
	t.Cleanup(server.Close)
	source, err := NewPrometheusUsageMetricsSource(
		server.URL,
		"cpu:{{namespace}}:{{service}}:{{range}}",
		"gpu:{{namespace}}:{{service}}:{{range}}",
		"unused",
	)
	if err != nil {
		t.Fatalf("NewPrometheusUsageMetricsSource() error: %v", err)
	}
	return source
}

type mockServiceInventorySource struct {
	services []ServiceDescriptor
	err      error
}

func (m *mockServiceInventorySource) Name() string {
	return "mock"
}

func (m *mockServiceInventorySource) ListServices(ctx context.Context, tr TimeRange) ([]ServiceDescriptor, *types.SourceStatus, error) {
	return m.services, &types.SourceStatus{}, m.err
}

type mockUsageMetricsSource struct{}

func (m *mockUsageMetricsSource) Name() string {
	return "mock"
}

func (m *mockUsageMetricsSource) UsageHours(ctx context.Context, tr TimeRange, serviceID string) (float64, float64, *types.SourceStatus, error) {
	return 1.0, 2.0, &types.SourceStatus{}, nil
}

type mockRequestLogSource struct {
	records []RequestRecord
	err     error
}

func (m *mockRequestLogSource) Name() string {
	return "mock"
}

func (m *mockRequestLogSource) ListRequests(ctx context.Context, cfg *types.Config, tr TimeRange, serviceID string) ([]RequestRecord, *types.SourceStatus, error) {
	return m.records, &types.SourceStatus{}, m.err
}

func TestScopeSourcesNilAllowed(t *testing.T) {
	src := Sources{
		ServiceInventory: &mockServiceInventorySource{},
	}

	result := ScopeSources(src, QueryScope{})
	if result.ServiceInventory == nil {
		t.Error("Expected ServiceInventory to be unchanged")
	}
}

func TestScopeSourcesWithAllowed(t *testing.T) {
	src := Sources{
		ServiceInventory: &mockServiceInventorySource{
			services: []ServiceDescriptor{
				{ID: "svc1"},
				{ID: "svc2"},
			},
		},
		UsageMetrics:       &mockUsageMetricsSource{},
		RequestLogs:        &mockRequestLogSource{},
		ExposedRequestLogs: &mockRequestLogSource{},
	}

	scope := QueryScope{
		OwnerNamespace: "oscar-svc-user",
		ActiveServices: []ServiceScope{{Name: "svc1"}},
	}

	result := ScopeSources(src, scope)
	if result.ServiceInventory == nil {
		t.Error("Expected ServiceInventory to be set")
	}
	if result.UsageMetrics == nil {
		t.Error("Expected UsageMetrics to be set")
	}
	if result.RequestLogs == nil {
		t.Error("Expected RequestLogs to be set")
	}
	if result.ExposedRequestLogs == nil {
		t.Error("Expected ExposedRequestLogs to be set")
	}
}

func TestFilterServiceDescriptors(t *testing.T) {
	services := []ServiceDescriptor{
		{ID: "svc1", Namespace: "oscar-svc-user"},
		{ID: "svc2", Namespace: "oscar-svc-other"},
		{ID: "svc3", Namespace: "oscar-svc-shared"},
	}
	scope := QueryScope{
		OwnerNamespace: "oscar-svc-user",
		ActiveServices: []ServiceScope{{Name: "svc3", Namespace: "oscar-svc-shared"}},
	}

	filtered := filterServiceDescriptors(services, scope)
	if len(filtered) != 2 {
		t.Errorf("Expected 2 services, got %d", len(filtered))
	}
	if filtered[0].ID != "svc1" {
		t.Errorf("Expected svc1, got %s", filtered[0].ID)
	}
	if filtered[1].ID != "svc3" {
		t.Errorf("Expected svc3, got %s", filtered[1].ID)
	}
}

func TestFilterServiceDescriptorsNilAllowed(t *testing.T) {
	services := []ServiceDescriptor{
		{ID: "svc1"},
	}

	filtered := filterServiceDescriptors(services, QueryScope{})
	if len(filtered) != 1 {
		t.Errorf("Expected 1 service, got %d", len(filtered))
	}
}

func TestFilterRequestRecords(t *testing.T) {
	records := []RequestRecord{
		{ServiceID: "deleted", ServiceNamespace: "oscar-svc-user"},
		{ServiceID: "svc2", ServiceNamespace: "oscar-svc-other"},
		{ServiceID: "svc3", ServiceNamespace: "oscar-svc-shared"},
	}
	scope := QueryScope{
		OwnerNamespace: "oscar-svc-user",
		ActiveServices: []ServiceScope{{Name: "svc3", Namespace: "oscar-svc-shared"}},
	}

	filtered := filterRequestRecords(records, scope)
	if len(filtered) != 2 {
		t.Errorf("Expected 2 records, got %d", len(filtered))
	}
	if filtered[0].ServiceID != "deleted" {
		t.Errorf("Expected deleted, got %s", filtered[0].ServiceID)
	}
	if filtered[1].ServiceID != "svc3" {
		t.Errorf("Expected svc3, got %s", filtered[1].ServiceID)
	}
}

func TestFilterRequestRecordsNilAllowed(t *testing.T) {
	records := []RequestRecord{
		{ServiceID: "svc1"},
	}

	filtered := filterRequestRecords(records, QueryScope{})
	if len(filtered) != 1 {
		t.Errorf("Expected 1 record, got %d", len(filtered))
	}
}

func TestScopedServiceInventorySource(t *testing.T) {
	ctx := context.Background()
	tr := TimeRange{
		Start: time.Now().Add(-time.Hour),
		End:   time.Now(),
	}

	inner := &mockServiceInventorySource{
		services: []ServiceDescriptor{
			{ID: "svc1", Namespace: "oscar-svc-user"},
			{ID: "svc2", Namespace: "oscar-svc-other"},
		},
	}

	src := &scopedServiceInventorySource{
		inner: inner,
		scope: QueryScope{OwnerNamespace: "oscar-svc-user"},
	}

	services, _, _ := src.ListServices(ctx, tr)
	if len(services) != 1 {
		t.Errorf("Expected 1 service, got %d", len(services))
	}
	if services[0].ID != "svc1" {
		t.Errorf("Expected svc1, got %s", services[0].ID)
	}
}

func TestScopedUsageMetricsSource(t *testing.T) {
	ctx := context.Background()
	tr := TimeRange{
		Start: time.Now().Add(-time.Hour),
		End:   time.Now(),
	}

	inner := &mockUsageMetricsSource{}
	src := &scopedUsageMetricsSource{inner: inner, scope: QueryScope{OwnerNamespace: "oscar-svc-user"}}

	cpu, mem, _, _ := src.UsageHours(ctx, tr, "svc1")
	if cpu != 1.0 {
		t.Errorf("Expected cpu=1.0, got %f", cpu)
	}
	if mem != 2.0 {
		t.Errorf("Expected mem=2.0, got %f", mem)
	}
}

func TestScopedPrometheusUsageMetricsSource(t *testing.T) {
	queries := []string{}
	source := newScopedPrometheusSource(t, &queries)
	scoped := &scopedUsageMetricsSource{
		inner: source,
		scope: QueryScope{
			OwnerNamespace: "owner.ns",
			ActiveServices: []ServiceScope{
				{Name: "svc+one", Namespace: "owner.ns"},
				{Name: "svc+one", Namespace: "legacy.ns"},
				{Name: "svc+one", Namespace: "legacy.ns"},
				{Name: "other", Namespace: "ignored.ns"},
			},
		},
	}
	tr := TimeRange{Start: time.Unix(0, 0), End: time.Unix(3600, 0)}

	cpu, gpu, status, err := scoped.UsageHours(t.Context(), tr, "svc+one")
	if err != nil {
		t.Fatalf("UsageHours() error: %v", err)
	}
	if cpu != 2 || gpu != 4 {
		t.Fatalf("UsageHours() = (%v, %v), want (2, 4); queries: %v", cpu, gpu, queries)
	}
	if status == nil || status.Status != "ok" {
		t.Fatalf("UsageHours() status = %#v, want ok", status)
	}
	if len(queries) != 4 {
		t.Fatalf("query count = %d, want 4", len(queries))
	}
	joined := strings.Join(queries, "\n")
	for _, want := range []string{`owner\.ns`, `legacy\.ns`, `svc\+one`} {
		if !strings.Contains(joined, want) {
			t.Errorf("queries do not contain %q: %s", want, joined)
		}
	}
}

func TestScopedPrometheusUsageHoursAll(t *testing.T) {
	queries := []string{}
	source := newScopedPrometheusSource(t, &queries)
	scoped := &scopedUsageMetricsSource{
		inner: source,
		scope: QueryScope{
			OwnerNamespace: "owner-ns",
			ActiveServices: []ServiceScope{
				{Name: "same", Namespace: "owner-ns"},
				{Name: "legacy", Namespace: "legacy-ns"},
				{Name: "empty"},
			},
		},
	}

	cpu, gpu, status, err := scoped.UsageHoursAll(t.Context(), TimeRange{Start: time.Unix(0, 0), End: time.Unix(60, 0)})
	if err != nil {
		t.Fatalf("UsageHoursAll() error: %v", err)
	}
	if cpu != 2 || gpu != 4 || status == nil || status.Status != "ok" {
		t.Fatalf("UsageHoursAll() = (%v, %v, %#v), want (2, 4, ok)", cpu, gpu, status)
	}
	if len(queries) != 4 {
		t.Fatalf("query count = %d, want 4", len(queries))
	}
	joined := strings.Join(queries, "\n")
	if !strings.Contains(joined, "owner-ns:.*") || !strings.Contains(joined, "legacy-ns:legacy") {
		t.Fatalf("unexpected queries: %s", joined)
	}
}

func TestScopedUsageHoursAllUnsupported(t *testing.T) {
	scoped := &scopedUsageMetricsSource{inner: &mockUsageMetricsSource{}}
	if _, _, _, err := scoped.UsageHoursAll(t.Context(), TimeRange{}); err != errScopedUsageUnsupported {
		t.Fatalf("UsageHoursAll() error = %v, want %v", err, errScopedUsageUnsupported)
	}
}

func TestScopedUsageHelpers(t *testing.T) {
	scoped := &scopedUsageMetricsSource{scope: QueryScope{
		OwnerNamespace: "owner",
		ActiveServices: []ServiceScope{
			{Name: "svc", Namespace: ""},
			{Name: "other", Namespace: "other"},
			{Name: "svc", Namespace: "owner"},
			{Name: "svc", Namespace: "legacy"},
			{Name: "svc", Namespace: "legacy"},
		},
	}}
	namespaces := scoped.namespacesForService("svc")
	if got := strings.Join(namespaces, ","); got != "owner,legacy" {
		t.Fatalf("namespacesForService() = %q, want %q", got, "owner,legacy")
	}

	left := &types.SourceStatus{Status: "ok", Notes: "first"}
	right := &types.SourceStatus{Status: "partial", Notes: "second"}
	merged := mergeSourceStatus(left, right)
	if merged.Status != "partial" || merged.Notes != "first; second" {
		t.Fatalf("mergeSourceStatus() = %#v", merged)
	}
	if mergeSourceStatus(nil, right) != right || mergeSourceStatus(left, nil) != left {
		t.Fatal("mergeSourceStatus() should preserve non-nil status")
	}
	missing := mergeSourceStatus(&types.SourceStatus{Status: "ok"}, &types.SourceStatus{Status: "missing"})
	if missing.Status != "missing" {
		t.Fatalf("mergeSourceStatus() status = %q, want missing", missing.Status)
	}
}

func TestScopedRequestLogSource(t *testing.T) {
	ctx := context.Background()
	tr := TimeRange{
		Start: time.Now().Add(-time.Hour),
		End:   time.Now(),
	}

	inner := &mockRequestLogSource{
		records: []RequestRecord{
			{ServiceID: "svc1"},
			{ServiceID: "svc2"},
		},
	}

	src := &scopedRequestLogSource{
		inner: inner,
		scope: QueryScope{
			OwnerNamespace: "oscar-svc-user",
			ActiveServices: []ServiceScope{{Name: "svc1"}},
		},
	}

	records, _, _ := src.ListRequests(ctx, &types.Config{}, tr, "svc1")
	if len(records) != 1 {
		t.Errorf("Expected 1 record, got %d", len(records))
	}
	if records[0].ServiceID != "svc1" {
		t.Errorf("Expected svc1, got %s", records[0].ServiceID)
	}
}
