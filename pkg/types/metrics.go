package types

import "time"

type MetricKey string

const (
	MetricServicesCount   MetricKey = "services-count"
	MetricCPUHours        MetricKey = "cpu-hours"
	MetricGPUHours        MetricKey = "gpu-hours"
	MetricRequestsSync    MetricKey = "requests-sync-per-service"
	MetricRequestsAsync   MetricKey = "requests-async-per-service"
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

type CountryCount struct {
	Country      string `json:"country"`
	RequestCount int    `json:"request_count"`
}

type SummaryTotals struct {
	ServicesCount     int            `json:"services_count"`
	CPUHoursTotal     float64        `json:"cpu_hours_total"`
	GPUHoursTotal     float64        `json:"gpu_hours_total"`
	RequestCountTotal int            `json:"request_count_total"`
	RequestCountSync  int            `json:"request_count_sync"`
	RequestCountAsync int            `json:"request_count_async"`
	CountriesCount    int            `json:"countries_count"`
	Countries         []CountryCount `json:"countries"`
	UsersCount        int            `json:"users_count"`
	Users             []string       `json:"users"`
}

type MetricsSummaryResponse struct {
	Start   time.Time      `json:"start"`
	End     time.Time      `json:"end"`
	Totals  SummaryTotals  `json:"totals"`
	Sources []SourceStatus `json:"sources"`
}

type BreakdownItem struct {
	Key              string         `json:"key"`
	Membership       string         `json:"membership,omitempty"`
	ExecutionsCount  int            `json:"executions_count"`
	UniqueUsersCount int            `json:"unique_users_count"`
	Countries        []CountryCount `json:"countries"`
}

type MetricsBreakdownResponse struct {
	Start   time.Time       `json:"start"`
	End     time.Time       `json:"end"`
	GroupBy string          `json:"group_by"`
	Items   []BreakdownItem `json:"items"`
}
