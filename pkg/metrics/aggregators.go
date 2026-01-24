package metrics

import (
	"context"
	"errors"
	"sort"
	"strings"

	"github.com/grycap/oscar/v3/pkg/types"
)

type Aggregator struct {
	Sources Sources
}

var errUnsupportedMetric = errors.New("unsupported metric for value query")

func (a *Aggregator) MetricValue(ctx context.Context, tr TimeRange, serviceID string, metric types.MetricKey) (types.MetricValueResponse, error) {
	if !types.IsMetricKeyValid(metric) {
		return types.MetricValueResponse{}, errUnsupportedMetric
	}

	resp := types.MetricValueResponse{
		ServiceID: serviceID,
		Metric:    metric,
		Start:     tr.Start,
		End:       tr.End,
		Sources:   []types.SourceStatus{},
	}

	switch metric {
	case types.MetricCPUHours, types.MetricGPUHours:
		if a.Sources.UsageMetrics == nil {
			resp.Sources = append(resp.Sources, *missingStatus("usage-metrics", errors.New("usage metrics source missing")))
		} else {
			cpu, gpu, status, _ := a.Sources.UsageMetrics.UsageHours(ctx, tr, serviceID)
			if status != nil {
				resp.Sources = append(resp.Sources, *status)
			}
			if metric == types.MetricCPUHours {
				resp.Value = cpu
				resp.Unit = "hours"
			} else {
				resp.Value = gpu
				resp.Unit = "hours"
			}
		}
	case types.MetricRequestsSync, types.MetricRequestsAsync, types.MetricUsersPerService:
		if a.Sources.RequestLogs == nil {
			resp.Sources = append(resp.Sources, *missingStatus("request-logs", errors.New("request log source missing")))
		} else {
			records, status, _ := a.Sources.RequestLogs.ListRequests(ctx, tr, serviceID)
			if status != nil {
				resp.Sources = append(resp.Sources, *status)
			}
			switch metric {
			case types.MetricRequestsSync:
				resp.Value = float64(countRequests(records, RequestSync))
			case types.MetricRequestsAsync:
				resp.Value = float64(countRequests(records, RequestAsync))
			case types.MetricUsersPerService:
				resp.Value = float64(uniqueUsers(records))
			}
		}
	case types.MetricRequestsExposed:
		if a.Sources.ExposedRequestLogs == nil {
			resp.Sources = append(resp.Sources, *missingStatus("exposed-request-logs", errors.New("exposed request log source missing")))
		} else {
			records, status, _ := a.Sources.ExposedRequestLogs.ListRequests(ctx, tr, serviceID)
			if status != nil {
				resp.Sources = append(resp.Sources, *status)
			}
			resp.Value = float64(len(records))
		}
	default:
		return types.MetricValueResponse{}, errUnsupportedMetric
	}

	return resp, nil
}

func (a *Aggregator) Summary(ctx context.Context, tr TimeRange) (types.MetricsSummaryResponse, error) {
	resp := types.MetricsSummaryResponse{
		Start:   tr.Start,
		End:     tr.End,
		Totals:  types.SummaryTotals{},
		Sources: []types.SourceStatus{},
	}

	services, status, _ := a.Sources.ServiceInventory.ListServices(ctx, tr)
	if status != nil {
		resp.Sources = append(resp.Sources, *status)
	}
	resp.Totals.ServicesCountActive = len(services)

	cpuTotal, gpuTotal, usageStatus := a.sumUsage(ctx, tr, services)
	if usageStatus != nil {
		resp.Sources = append(resp.Sources, *usageStatus)
	}
	resp.Totals.CPUHoursTotal = cpuTotal
	resp.Totals.GPUHoursTotal = gpuTotal

	var requestRecords []RequestRecord
	if a.Sources.RequestLogs == nil {
		resp.Sources = append(resp.Sources, *missingStatus("request-logs", errors.New("request log source missing")))
	} else {
		records, requestStatus, _ := a.Sources.RequestLogs.ListRequests(ctx, tr, "")
		if requestStatus != nil {
			resp.Sources = append(resp.Sources, *requestStatus)
		}
		requestRecords = records
		countryStatus := a.processSummaryFromRecords(ctx, records, &resp)
		if countryStatus != nil {
			resp.Sources = append(resp.Sources, *countryStatus)
		}
	}

	var exposedRecords []RequestRecord
	if a.Sources.ExposedRequestLogs == nil {
		resp.Sources = append(resp.Sources, *missingStatus("exposed-request-logs", errors.New("exposed request log source missing")))
	} else {
		records, exposedStatus, _ := a.Sources.ExposedRequestLogs.ListRequests(ctx, tr, "")
		if exposedStatus != nil {
			resp.Sources = append(resp.Sources, *exposedStatus)
		}
		exposedRecords = records
		resp.Totals.RequestsCountExposed = len(records)
	}

	resp.Totals.ServicesCountTotal = uniqueServices(requestRecords, exposedRecords)

	return resp, nil
}

func (a *Aggregator) Breakdown(ctx context.Context, tr TimeRange, groupBy string) (types.MetricsBreakdownResponse, error) {
	resp := types.MetricsBreakdownResponse{
		Start:   tr.Start,
		End:     tr.End,
		GroupBy: groupBy,
		Items:   []types.BreakdownItem{},
	}

	var records []RequestRecord
	if a.Sources.RequestLogs != nil {
		records, _, _ = a.Sources.RequestLogs.ListRequests(ctx, tr, "")
	}

	groupBy = strings.ToLower(groupBy)
	switch groupBy {
	case "service":
		var exposedRecords []RequestRecord
		if a.Sources.ExposedRequestLogs != nil {
			exposedRecords, _, _ = a.Sources.ExposedRequestLogs.ListRequests(ctx, tr, "")
		}
		resp.Items = breakdownByService(records, exposedRecords)
	case "user":
		if len(records) == 0 {
			return resp, nil
		}
		resp.Items = breakdownByUser(ctx, records, a.Sources.UserRoster)
	case "country":
		if len(records) == 0 {
			return resp, nil
		}
		resp.Items = breakdownByCountry(records)
	default:
		return types.MetricsBreakdownResponse{}, errors.New("unsupported group_by")
	}

	return resp, nil
}

func (a *Aggregator) sumUsage(ctx context.Context, tr TimeRange, services []ServiceDescriptor) (float64, float64, *types.SourceStatus) {
	if a.Sources.UsageMetrics == nil {
		return 0, 0, missingStatus("usage-metrics", errors.New("usage source missing"))
	}

	if promSource, ok := a.Sources.UsageMetrics.(*PrometheusUsageMetricsSource); ok {
		cpuTotal, gpuTotal, status, err := promSource.UsageHours(ctx, tr, ".*")
		if err != nil {
			return 0, 0, status
		}
		return cpuTotal, gpuTotal, status
	}

	var cpuTotal float64
	var gpuTotal float64
	hasError := false
	hasOK := false
	var status *types.SourceStatus

	for _, service := range services {
		cpu, gpu, srcStatus, err := a.Sources.UsageMetrics.UsageHours(ctx, tr, service.ID)
		if srcStatus != nil {
			status = srcStatus
		}
		if err != nil {
			hasError = true
			continue
		}
		hasOK = true
		cpuTotal += cpu
		gpuTotal += gpu
	}

	if status == nil {
		status = okStatus("usage-metrics", "")
	}
	if hasError && hasOK {
		status.Status = "partial"
	}
	if hasError && !hasOK {
		status.Status = "missing"
	}

	return cpuTotal, gpuTotal, status
}

func (a *Aggregator) processSummaryFromRecords(ctx context.Context, records []RequestRecord, resp *types.MetricsSummaryResponse) *types.SourceStatus {
	if len(records) == 0 {
		return nil
	}
	requestTotals := countRequests(records, "")
	resp.Totals.RequestsCountTotal = requestTotals
	resp.Totals.RequestsCountSync = countRequests(records, RequestSync)
	resp.Totals.RequestsCountAsync = countRequests(records, RequestAsync)

	userSet := make(map[string]struct{})
	countryCounts := make(map[string]int)

	var countryStatus *types.SourceStatus
	for _, record := range records {
		if record.UserID != "" {
			userSet[record.UserID] = struct{}{}
		}

		country, status, _ := a.Sources.CountrySource.CountryForRecord(ctx, record)
		if status != nil {
			countryStatus = status
		}
		if country == "" || strings.EqualFold(country, "unknown") {
			continue
		}
		countryCounts[country]++
	}

	resp.Totals.UsersCount = len(userSet)
	resp.Totals.Users = mapToUserList(userSet)
	resp.Totals.CountriesCount = len(countryCounts)
	resp.Totals.Countries = mapToCountryList(countryCounts)
	return countryStatus
}

func countRequests(records []RequestRecord, typ RequestType) int {
	total := 0
	for _, record := range records {
		if typ == "" || record.Type == typ {
			total++
		}
	}
	return total
}

func uniqueUsers(records []RequestRecord) int {
	users := map[string]struct{}{}
	for _, record := range records {
		if record.UserID == "" {
			continue
		}
		users[record.UserID] = struct{}{}
	}
	return len(users)
}

func uniqueServices(recordSets ...[]RequestRecord) int {
	services := map[string]struct{}{}
	for _, records := range recordSets {
		for _, record := range records {
			if record.ServiceID == "" {
				continue
			}
			services[record.ServiceID] = struct{}{}
		}
	}
	return len(services)
}

func mapToCountryList(counts map[string]int) []types.CountryCount {
	items := make([]types.CountryCount, 0, len(counts))
	for country, count := range counts {
		items = append(items, types.CountryCount{
			Country:      country,
			RequestCount: count,
		})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Country < items[j].Country
	})
	return items
}

func mapToUserList(users map[string]struct{}) []string {
	if len(users) == 0 {
		return []string{}
	}
	items := make([]string, 0, len(users))
	for user := range users {
		items = append(items, user)
	}
	sort.Strings(items)
	return items
}

func breakdownByService(records []RequestRecord, exposedRecords []RequestRecord) []types.BreakdownItem {
	type agg struct {
		executions int
		sync       int
		async      int
		exposed    int
		users      map[string]struct{}
		countries  map[string]int
	}
	index := map[string]*agg{}
	for _, record := range records {
		entry := index[record.ServiceID]
		if entry == nil {
			entry = &agg{users: map[string]struct{}{}, countries: map[string]int{}}
			index[record.ServiceID] = entry
		}
		entry.executions++
		switch record.Type {
		case RequestSync:
			entry.sync++
		case RequestAsync:
			entry.async++
		}
		if record.UserID != "" {
			entry.users[record.UserID] = struct{}{}
		}
		if record.Country != "" && !strings.EqualFold(record.Country, "unknown") {
			entry.countries[record.Country]++
		}
	}
	for _, record := range exposedRecords {
		if record.ServiceID == "" {
			continue
		}
		entry := index[record.ServiceID]
		if entry == nil {
			entry = &agg{users: map[string]struct{}{}, countries: map[string]int{}}
			index[record.ServiceID] = entry
		}
		entry.exposed++
		entry.executions++
	}

	items := make([]types.BreakdownItem, 0, len(index))
	for key, entry := range index {
		items = append(items, types.BreakdownItem{
			Key:                  key,
			ExecutionsCount:      entry.executions,
			RequestsCountTotal:   entry.sync + entry.async,
			RequestsCountSync:    entry.sync,
			RequestsCountAsync:   entry.async,
			RequestsCountExposed: entry.exposed,
			UniqueUsersCount:     len(entry.users),
			Countries:            mapToCountryList(entry.countries),
		})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Key < items[j].Key
	})
	return items
}

func breakdownByUser(ctx context.Context, records []RequestRecord, roster UserRosterSource) []types.BreakdownItem {
	type agg struct {
		executions int
		countries  map[string]int
		membership string
	}
	index := map[string]*agg{}
	for _, record := range records {
		entry := index[record.UserID]
		if entry == nil {
			entry = &agg{countries: map[string]int{}}
			index[record.UserID] = entry
		}
		entry.executions++
		country := record.Country
		if country != "" && !strings.EqualFold(country, "unknown") {
			entry.countries[country]++
		}
		if entry.membership == "" {
			entry.membership = classifyMembership(ctx, roster, record)
		}
	}

	items := make([]types.BreakdownItem, 0, len(index))
	for key, entry := range index {
		uniqueUsers := 0
		if key != "" {
			uniqueUsers = 1
		}
		items = append(items, types.BreakdownItem{
			Key:              key,
			Membership:       entry.membership,
			ExecutionsCount:  entry.executions,
			UniqueUsersCount: uniqueUsers,
			Countries:        mapToCountryList(entry.countries),
		})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Key < items[j].Key
	})
	return items
}

func breakdownByCountry(records []RequestRecord) []types.BreakdownItem {
	type agg struct {
		executions int
		users      map[string]struct{}
	}
	index := map[string]*agg{}
	for _, record := range records {
		country := record.Country
		if country == "" {
			country = "unknown"
		}
		entry := index[country]
		if entry == nil {
			entry = &agg{users: map[string]struct{}{}}
			index[country] = entry
		}
		entry.executions++
		if record.UserID != "" {
			entry.users[record.UserID] = struct{}{}
		}
	}

	items := make([]types.BreakdownItem, 0, len(index))
	for key, entry := range index {
		items = append(items, types.BreakdownItem{
			Key:              key,
			ExecutionsCount:  entry.executions,
			UniqueUsersCount: len(entry.users),
			Countries:        mapToCountryList(map[string]int{key: entry.executions}),
		})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Key < items[j].Key
	})
	return items
}

func classifyMembership(ctx context.Context, roster UserRosterSource, record RequestRecord) string {
	switch strings.ToLower(record.AuthMethod) {
	case "oidc":
		return "member"
	case "service_token":
		return "external"
	}
	if roster == nil || record.UserID == "" {
		return "unknown"
	}
	classification, _, err := roster.Classification(ctx, record.UserID)
	if err != nil {
		return "unknown"
	}
	if classification == "member" || classification == "external" {
		return classification
	}
	return "unknown"
}
