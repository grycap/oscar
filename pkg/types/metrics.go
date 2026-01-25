package types

import "time"

type MetricKey string

const (
	MetricServicesCount   MetricKey = "services-count"
	MetricCPUHours        MetricKey = "cpu-hours"
	MetricGPUHours        MetricKey = "gpu-hours"
	MetricRequestsSync    MetricKey = "requests-sync-per-service"
	MetricRequestsAsync   MetricKey = "requests-async-per-service"
	MetricRequestsExposed MetricKey = "requests-exposed-per-service"
	MetricRequestsPerUser MetricKey = "requests-per-user"
	MetricUsersPerService MetricKey = "users-per-service"
	MetricCountriesCount  MetricKey = "countries-count"
	MetricCountriesList   MetricKey = "countries-list"
)

var MetricKeySet = map[MetricKey]struct{}{
	MetricServicesCount:   {},
	MetricCPUHours:        {},
	MetricGPUHours:        {},
	MetricRequestsSync:    {},
	MetricRequestsAsync:   {},
	MetricRequestsExposed: {},
	MetricRequestsPerUser: {},
	MetricUsersPerService: {},
	MetricCountriesCount:  {},
	MetricCountriesList:   {},
}

func IsMetricKeyValid(key MetricKey) bool {
	_, ok := MetricKeySet[key]
	return ok
}

type SourceStatus struct {
	Name        string     `json:"name"`
	Status      string     `json:"status"`
	LastUpdated *time.Time `json:"last_updated,omitempty"`
	Notes       string     `json:"notes,omitempty"`
}

type MetricValueResponse struct {
	ServiceID string         `json:"service_id"`
	Metric    MetricKey      `json:"metric"`
	Start     time.Time      `json:"start"`
	End       time.Time      `json:"end"`
	Value     float64        `json:"value"`
	Unit      string         `json:"unit,omitempty"`
	Sources   []SourceStatus `json:"sources"`
}

type ServiceMetricValue struct {
	Metric  MetricKey      `json:"metric"`
	Value   float64        `json:"value"`
	Unit    string         `json:"unit,omitempty"`
	Sources []SourceStatus `json:"sources"`
}

type ServiceMetricsResponse struct {
	ServiceName string               `json:"service_name"`
	Start       time.Time            `json:"start"`
	End         time.Time            `json:"end"`
	Metrics     []ServiceMetricValue `json:"metrics"`
}

type CountryCount struct {
	Country      string `json:"country"`
	RequestCount int    `json:"request_count"`
}

type SummaryTotals struct {
	ServicesCountActive  int            `json:"services_count_active"`
	ServicesCountTotal   int            `json:"services_count_total"`
	CPUHoursTotal        float64        `json:"cpu_hours_total"`
	GPUHoursTotal        float64        `json:"gpu_hours_total"`
	RequestsCountTotal   int            `json:"requests_count_total"`
	RequestsCountSync    int            `json:"requests_count_sync"`
	RequestsCountAsync   int            `json:"requests_count_async"`
	RequestsCountExposed int            `json:"requests_count_exposed"`
	CountriesCount       int            `json:"countries_count"`
	Countries            []CountryCount `json:"countries"`
	UsersCount           int            `json:"users_count"`
	Users                []string       `json:"users"`
}

type MetricsSummaryResponse struct {
	Start   time.Time      `json:"start"`
	End     time.Time      `json:"end"`
	Totals  SummaryTotals  `json:"totals"`
	Sources []SourceStatus `json:"sources"`
}

type BreakdownItem struct {
	Key                  string         `json:"key"`
	Membership           string         `json:"membership,omitempty"`
	ExecutionsCount      int            `json:"executions_count,omitempty"`
	RequestsCountTotal   int            `json:"requests_count_total,omitempty"`
	RequestsCountSync    int            `json:"requests_count_sync,omitempty"`
	RequestsCountAsync   int            `json:"requests_count_async,omitempty"`
	RequestsCountExposed int            `json:"requests_count_exposed,omitempty"`
	UniqueUsersCount     int            `json:"unique_users_count"`
	Users                []string       `json:"users,omitempty"`
	Countries            []CountryCount `json:"countries"`
}

type MetricsBreakdownResponse struct {
	Start   time.Time       `json:"start"`
	End     time.Time       `json:"end"`
	GroupBy string          `json:"group_by"`
	Items   []BreakdownItem `json:"items"`
}
