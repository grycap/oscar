package metrics

import (
	"context"
	"testing"
	"time"

	"github.com/grycap/oscar/v4/pkg/types"
)

type mockServiceInventorySource struct {
	services []ServiceDescriptor
	err     error
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

func (m *mockRequestLogSource) ListRequests(ctx context.Context, tr TimeRange, serviceID string) ([]RequestRecord, *types.SourceStatus, error) {
	return m.records, &types.SourceStatus{}, m.err
}

func TestScopeSourcesNilAllowed(t *testing.T) {
	src := Sources{
		ServiceInventory: &mockServiceInventorySource{},
	}

	result := ScopeSources(src, nil)
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
		UsageMetrics:    &mockUsageMetricsSource{},
		RequestLogs:   &mockRequestLogSource{},
		ExposedRequestLogs: &mockRequestLogSource{},
	}

	allowed := map[string]struct{}{
		"svc1": {},
	}

	result := ScopeSources(src, allowed)
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

func TestCloneAllowedServices(t *testing.T) {
	allowed := map[string]struct{}{
		"svc1": {},
		"svc2": {},
	}

	cloned := cloneAllowedServices(allowed)
	if cloned == nil {
		t.Fatal("Expected non-nil")
	}
	if _, ok := cloned["svc1"]; !ok {
		t.Error("Expected svc1 in cloned")
	}
	if _, ok := cloned["svc2"]; !ok {
		t.Error("Expected svc2 in cloned")
	}
}

func TestCloneAllowedServicesNil(t *testing.T) {
	cloned := cloneAllowedServices(nil)
	if cloned != nil {
		t.Error("Expected nil for nil input")
	}
}

func TestFilterServiceDescriptors(t *testing.T) {
	services := []ServiceDescriptor{
		{ID: "svc1"},
		{ID: "svc2"},
		{ID: "svc3"},
	}
	allowed := map[string]struct{}{
		"svc1": {},
		"svc3": {},
	}

	filtered := filterServiceDescriptors(services, allowed)
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

	filtered := filterServiceDescriptors(services, nil)
	if len(filtered) != 1 {
		t.Errorf("Expected 1 service, got %d", len(filtered))
	}
}

func TestFilterRequestRecords(t *testing.T) {
	records := []RequestRecord{
		{ServiceID: "svc1"},
		{ServiceID: "svc2"},
		{ServiceID: "svc3"},
	}
	allowed := map[string]struct{}{
		"svc1": {},
		"svc3": {},
	}

	filtered := filterRequestRecords(records, allowed)
	if len(filtered) != 2 {
		t.Errorf("Expected 2 records, got %d", len(filtered))
	}
	if filtered[0].ServiceID != "svc1" {
		t.Errorf("Expected svc1, got %s", filtered[0].ServiceID)
	}
	if filtered[1].ServiceID != "svc3" {
		t.Errorf("Expected svc3, got %s", filtered[1].ServiceID)
	}
}

func TestFilterRequestRecordsNilAllowed(t *testing.T) {
	records := []RequestRecord{
		{ServiceID: "svc1"},
	}

	filtered := filterRequestRecords(records, nil)
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
			{ID: "svc1"},
			{ID: "svc2"},
		},
	}

	allowed := map[string]struct{}{
		"svc1": {},
	}

	src := &scopedServiceInventorySource{
		inner:   inner,
		allowed: allowed,
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
	src := &scopedUsageMetricsSource{inner: inner}

	cpu, mem, _, _ := src.UsageHours(ctx, tr, "svc1")
	if cpu != 1.0 {
		t.Errorf("Expected cpu=1.0, got %f", cpu)
	}
	if mem != 2.0 {
		t.Errorf("Expected mem=2.0, got %f", mem)
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

	allowed := map[string]struct{}{
		"svc1": {},
	}

	src := &scopedRequestLogSource{
		inner:   inner,
		allowed: allowed,
	}

	records, _, _ := src.ListRequests(ctx, tr, "svc1")
	if len(records) != 1 {
		t.Errorf("Expected 1 record, got %d", len(records))
	}
	if records[0].ServiceID != "svc1" {
		t.Errorf("Expected svc1, got %s", records[0].ServiceID)
	}
}