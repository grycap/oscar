package types

import (
	"testing"
	"time"
)

func TestIsMetricKeyValid(t *testing.T) {
	tests := []struct {
		name     string
		key     MetricKey
		expected bool
	}{
		{"Valid services count", MetricServicesCount, true},
		{"Valid cpu hours", MetricCPUHours, true},
		{"Valid gpu hours", MetricGPUHours, true},
		{"Valid requests sync", MetricRequestsSync, true},
		{"Valid requests async", MetricRequestsAsync, true},
		{"Valid requests exposed", MetricRequestsExposed, true},
		{"Valid requests per user", MetricRequestsPerUser, true},
		{"Valid users per service", MetricUsersPerService, true},
		{"Valid countries count", MetricCountriesCount, true},
		{"Valid countries list", MetricCountriesList, true},
		{"Invalid key", MetricKey("invalid"), false},
		{"Empty key", MetricKey(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsMetricKeyValid(tt.key)
			if result != tt.expected {
				t.Errorf("IsMetricKeyValid(%q) = %v, want %v", tt.key, result, tt.expected)
			}
		})
	}
}

func TestSourceStatus(t *testing.T) {
	status := SourceStatus{
		Name:        "test",
		Status:      "ok",
		LastUpdated: nil,
		Notes:       "test note",
	}

	if status.Name != "test" {
		t.Errorf("Expected Name = test, got %s", status.Name)
	}
	if status.Status != "ok" {
		t.Errorf("Expected Status = ok, got %s", status.Status)
	}
	if status.Notes != "test note" {
		t.Errorf("Expected Notes = test note, got %s", status.Notes)
	}

	now := time.Now()
	status.LastUpdated = &now
	if status.LastUpdated.IsZero() {
		t.Error("Expected LastUpdated to be set")
	}
}

func TestMetricValueResponse(t *testing.T) {
	start := time.Now()
	end := time.Now().Add(time.Hour)

	response := MetricValueResponse{
		ServiceID: "test-service",
		Metric:    MetricCPUHours,
		Start:     start,
		End:       end,
		Value:     100.5,
		Unit:      "hours",
		Sources:   []SourceStatus{{Name: "test", Status: "ok"}},
	}

	if response.ServiceID != "test-service" {
		t.Errorf("Expected ServiceID = test-service, got %s", response.ServiceID)
	}
	if response.Metric != MetricCPUHours {
		t.Errorf("Expected Metric = cpu-hours, got %s", response.Metric)
	}
	if response.Value != 100.5 {
		t.Errorf("Expected Value = 100.5, got %f", response.Value)
	}
	if len(response.Sources) != 1 {
		t.Errorf("Expected 1 source, got %d", len(response.Sources))
	}
}

func TestServiceMetricsResponse(t *testing.T) {
	response := ServiceMetricsResponse{
		ServiceName: "test-service",
		Start:       time.Now(),
		End:         time.Now().Add(time.Hour),
		Metrics: []ServiceMetricValue{
			{Metric: MetricCPUHours, Value: 100},
			{Metric: MetricGPUHours, Value: 50},
		},
	}

	if response.ServiceName != "test-service" {
		t.Errorf("Expected ServiceName = test-service, got %s", response.ServiceName)
	}
	if len(response.Metrics) != 2 {
		t.Errorf("Expected 2 metrics, got %d", len(response.Metrics))
	}
}

func TestSummaryTotals(t *testing.T) {
	totals := SummaryTotals{
		ServicesCountActive:   10,
		ServicesCountTotal:   20,
		CPUHoursTotal:         100,
		GPUHoursTotal:        50,
		RequestsCountTotal:     1000,
		RequestsCountSync:    600,
		RequestsCountAsync:   300,
		RequestsCountExposed: 100,
		CountriesCount:      5,
		Countries: []CountryCount{
			{Country: "US", RequestCount: 500},
			{Country: "ES", RequestCount: 300},
		},
		UsersCount: 50,
		Users:     []string{"user1", "user2"},
	}

	if totals.ServicesCountActive != 10 {
		t.Errorf("Expected ServicesCountActive = 10, got %d", totals.ServicesCountActive)
	}
	if totals.CPUHoursTotal != 100 {
		t.Errorf("Expected CPUHoursTotal = 100, got %f", totals.CPUHoursTotal)
	}
	if len(totals.Countries) != 2 {
		t.Errorf("Expected 2 countries, got %d", len(totals.Countries))
	}
	if len(totals.Users) != 2 {
		t.Errorf("Expected 2 users, got %d", len(totals.Users))
	}
}

func TestMetricsBreakdownResponse(t *testing.T) {
	response := MetricsBreakdownResponse{
		Start:   time.Now(),
		End:     time.Now().Add(time.Hour),
		GroupBy: "service",
		Items: []BreakdownItem{
			{
				Key:                "service-1",
				ExecutionsCount:     100,
				RequestsCountTotal:  100,
				UniqueUsersCount:    10,
				Users:             []string{"user1", "user2"},
				Countries:         []CountryCount{{Country: "US", RequestCount: 50}},
			},
		},
	}

	if response.GroupBy != "service" {
		t.Errorf("Expected GroupBy = service, got %s", response.GroupBy)
	}
	if len(response.Items) != 1 {
		t.Errorf("Expected 1 item, got %d", len(response.Items))
	}
	if response.Items[0].Key != "service-1" {
		t.Errorf("Expected Key = service-1, got %s", response.Items[0].Key)
	}
	if response.Items[0].UniqueUsersCount != 10 {
		t.Errorf("Expected UniqueUsersCount = 10, got %d", response.Items[0].UniqueUsersCount)
	}
}