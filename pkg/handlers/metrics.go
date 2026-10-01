package handlers

import (
	"bytes"
	"encoding/csv"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/v4/pkg/metrics"
	"github.com/grycap/oscar/v4/pkg/types"
	"github.com/grycap/oscar/v4/pkg/utils"
	"github.com/grycap/oscar/v4/pkg/utils/auth"
)

type userBreakdownItem struct {
	Key             string               `json:"key"`
	Membership      string               `json:"membership,omitempty"`
	ExecutionsCount int                  `json:"executions_count"`
	Countries       []types.CountryCount `json:"countries"`
}

type userBreakdownResponse struct {
	Start   time.Time           `json:"start"`
	End     time.Time           `json:"end"`
	GroupBy string              `json:"group_by"`
	Items   []userBreakdownItem `json:"items"`
}

type serviceBreakdownItem struct {
	Key                  string               `json:"key"`
	Membership           string               `json:"membership,omitempty"`
	RequestsCountTotal   int                  `json:"requests_count_total"`
	RequestsCountSync    int                  `json:"requests_count_sync"`
	RequestsCountAsync   int                  `json:"requests_count_async"`
	RequestsCountExposed int                  `json:"requests_count_exposed"`
	UniqueUsersCount     int                  `json:"unique_users_count"`
	Users                []string             `json:"users,omitempty"`
	Countries            []types.CountryCount `json:"countries"`
}

type serviceBreakdownResponse struct {
	Start   time.Time              `json:"start"`
	End     time.Time              `json:"end"`
	GroupBy string                 `json:"group_by"`
	Items   []serviceBreakdownItem `json:"items"`
}

// MakeMetricValueHandler godoc
// @Summary Get metrics for a service
// @Description When metric is omitted, returns all supported per-service metrics.
// @Tags metrics
// @Produce json
// @Param serviceName path string true "Service name"
// @Param metric query string false "Metric key"
// @Param start query string false "RFC3339 start timestamp (defaults to end-24h)"
// @Param end query string false "RFC3339 end timestamp (defaults to now)"
// @Param owner query string false "Service owner filter (admin Basic Auth only)"
// @Success 200 {object} types.MetricValueResponse
// @Success 200 {object} types.ServiceMetricsResponse
// @Failure 400 {string} string "Invalid parameters"
// @Failure 500 {string} string "Internal Server Error"
// @Security BasicAuth
// @Security BearerAuth
// @Router /system/metrics/{serviceName} [get]
func MakeMetricValueHandler(cfg *types.Config, back types.ServerlessBackend, agg *metrics.Aggregator) gin.HandlerFunc {
	return func(c *gin.Context) {
		serviceName := strings.TrimSpace(c.Param("serviceName"))
		if serviceName == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "serviceName is required"})
			return
		}
		if isBearerRequest(c) {
			if !authorizeServiceMetricsQuery(c, back, serviceName) {
				return
			}
		}

		tr, ok := parseTimeRange(c)
		if !ok {
			return
		}

		scopedAgg, ok := scopedMetricsAggregator(c, cfg, back, agg)
		if !ok {
			return
		}

		metricRaw := strings.TrimSpace(c.Query("metric"))
		if metricRaw == "" {
			metricsList, err := loadAllServiceMetrics(c, cfg, scopedAgg, tr, serviceName)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, types.ServiceMetricsResponse{
				ServiceName: serviceName,
				Start:       tr.Start,
				End:         tr.End,
				Metrics:     metricsList,
			})
			return
		}

		metricKey := types.MetricKey(metricRaw)
		if !types.IsMetricKeyValid(metricKey) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid metric key"})
			return
		}

		resp, err := scopedAgg.MetricValue(c.Request.Context(), cfg, tr, serviceName, metricKey)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, resp)
	}
}

func loadAllServiceMetrics(c *gin.Context, cfg *types.Config, agg *metrics.Aggregator, tr metrics.TimeRange, serviceName string) ([]types.ServiceMetricValue, error) {
	keys := []types.MetricKey{
		types.MetricCPUHours,
		types.MetricGPUHours,
		types.MetricRequestsSync,
		types.MetricRequestsAsync,
		types.MetricRequestsExposed,
		types.MetricUsersPerService,
	}
	items := make([]types.ServiceMetricValue, 0, len(keys))
	for _, key := range keys {
		resp, err := agg.MetricValue(c.Request.Context(), cfg, tr, serviceName, key)
		if err != nil {
			return nil, err
		}
		items = append(items, types.ServiceMetricValue{
			Metric:  resp.Metric,
			Value:   resp.Value,
			Unit:    resp.Unit,
			Sources: resp.Sources,
		})
	}
	return items, nil
}

// MakeMetricsSummaryHandler godoc
// @Summary Get metrics summary
// @Tags metrics
// @Produce json
// @Param start query string false "RFC3339 start timestamp (defaults to end-24h)"
// @Param end query string false "RFC3339 end timestamp (defaults to now)"
// @Param owner query string false "Service owner filter (admin Basic Auth only)"
// @Success 200 {object} types.MetricsSummaryResponse
// @Failure 400 {string} string "Invalid parameters"
// @Failure 500 {string} string "Internal Server Error"
// @Security BasicAuth
// @Security BearerAuth
// @Router /system/metrics [get]
func MakeMetricsSummaryHandler(cfg *types.Config, back types.ServerlessBackend, agg *metrics.Aggregator) gin.HandlerFunc {
	return func(c *gin.Context) {
		tr, ok := parseTimeRange(c)
		if !ok {
			return
		}

		scopedAgg, ok := scopedMetricsAggregator(c, cfg, back, agg)
		if !ok {
			return
		}

		resp, err := scopedAgg.Summary(c.Request.Context(), cfg, tr)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, resp)
	}
}

// MakeMetricsBreakdownHandler godoc
// @Summary Get metrics breakdown
// @Tags metrics
// @Produce json
// @Param start query string false "RFC3339 start timestamp (defaults to end-24h)"
// @Param end query string false "RFC3339 end timestamp (defaults to now)"
// @Param group_by query string true "Breakdown dimension" Enums(service,user,country)
// @Param include_users query bool false "Include user list when group_by=service"
// @Param format query string false "Response format" Enums(json,csv) default(json)
// @Param owner query string false "Service owner filter (admin Basic Auth only)"
// @Success 200 {object} types.MetricsBreakdownResponse
// @Success 200 {string} string "CSV response"
// @Failure 400 {string} string "Invalid parameters"
// @Failure 500 {string} string "Internal Server Error"
// @Security BasicAuth
// @Security BearerAuth
// @Router /system/metrics/breakdown [get]
func MakeMetricsBreakdownHandler(cfg *types.Config, back types.ServerlessBackend, agg *metrics.Aggregator) gin.HandlerFunc {
	return func(c *gin.Context) {
		tr, ok := parseTimeRange(c)
		if !ok {
			return
		}

		groupBy := strings.TrimSpace(c.Query("group_by"))
		if groupBy == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "group_by is required"})
			return
		}

		format := strings.ToLower(strings.TrimSpace(c.DefaultQuery("format", "json")))
		if format != "json" && format != "csv" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid format"})
			return
		}

		includeUsers := strings.ToLower(strings.TrimSpace(c.DefaultQuery("include_users", "false"))) == "true"
		scopedAgg, ok := scopedMetricsAggregator(c, cfg, back, agg)
		if !ok {
			return
		}

		resp, err := scopedAgg.Breakdown(c.Request.Context(), cfg, tr, groupBy, includeUsers)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if format == "csv" {
			if strings.EqualFold(groupBy, "service") && includeUsers {
				c.JSON(http.StatusBadRequest, gin.H{"error": "include_users is not supported for csv"})
				return
			}
			payload, err := renderBreakdownCSV(resp)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "csv export failed"})
				return
			}
			c.Data(http.StatusOK, "text/csv", payload)
			return
		}

		if strings.EqualFold(resp.GroupBy, "user") {
			items := make([]userBreakdownItem, 0, len(resp.Items))
			for _, item := range resp.Items {
				items = append(items, userBreakdownItem{
					Key:             item.Key,
					Membership:      item.Membership,
					ExecutionsCount: item.ExecutionsCount,
					Countries:       item.Countries,
				})
			}
			c.JSON(http.StatusOK, userBreakdownResponse{
				Start:   resp.Start,
				End:     resp.End,
				GroupBy: resp.GroupBy,
				Items:   items,
			})
			return
		}
		if strings.EqualFold(resp.GroupBy, "service") {
			items := make([]serviceBreakdownItem, 0, len(resp.Items))
			for _, item := range resp.Items {
				items = append(items, serviceBreakdownItem{
					Key:                  item.Key,
					Membership:           item.Membership,
					RequestsCountTotal:   item.RequestsCountTotal,
					RequestsCountSync:    item.RequestsCountSync,
					RequestsCountAsync:   item.RequestsCountAsync,
					RequestsCountExposed: item.RequestsCountExposed,
					UniqueUsersCount:     item.UniqueUsersCount,
					Users:                item.Users,
					Countries:            item.Countries,
				})
			}
			c.JSON(http.StatusOK, serviceBreakdownResponse{
				Start:   resp.Start,
				End:     resp.End,
				GroupBy: resp.GroupBy,
				Items:   items,
			})
			return
		}

		c.JSON(http.StatusOK, resp)
	}
}

// MakeMetricsOwnersHandler godoc
// @Summary List owners available for administrative metrics filtering
// @Tags metrics
// @Produce json
// @Success 200 {object} types.MetricsOwnersResponse
// @Failure 403 {string} string "Forbidden"
// @Failure 500 {string} string "Internal Server Error"
// @Security BasicAuth
// @Router /system/metrics/owners [get]
func MakeMetricsOwnersHandler(cfg *types.Config, back types.ServerlessBackend) gin.HandlerFunc {
	return func(c *gin.Context) {
		if isBearerRequest(c) {
			c.Status(http.StatusForbidden)
			return
		}

		managedNamespaces, err := utils.ListManagedUserNamespaces(c.Request.Context(), back.GetKubeClientset())
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}

		namesByOwner := map[string]string{}
		services, err := back.ListServices()
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
		for _, service := range services {
			if service == nil || service.Owner == "" {
				continue
			}
			if ownerName := strings.TrimSpace(service.Labels["owner_name"]); ownerName != "" {
				namesByOwner[service.Owner] = strings.ReplaceAll(ownerName, "_", " ")
			}
		}

		owners := make([]types.MetricsOwner, 0, len(managedNamespaces)+1)
		owners = append(owners, types.MetricsOwner{
			ID:        types.DefaultOwner,
			Name:      "oscar",
			Namespace: cfg.ServicesNamespace,
		})
		for _, namespace := range managedNamespaces {
			name := namesByOwner[namespace.Owner]
			if name == "" {
				name = namespace.Owner
			}
			owners = append(owners, types.MetricsOwner{
				ID:        namespace.Owner,
				Name:      name,
				Namespace: namespace.Name,
			})
		}
		sort.Slice(owners, func(i, j int) bool {
			return strings.ToLower(owners[i].Name) < strings.ToLower(owners[j].Name)
		})
		c.JSON(http.StatusOK, types.MetricsOwnersResponse{Owners: owners})
	}
}

func scopedMetricsAggregator(c *gin.Context, cfg *types.Config, back types.ServerlessBackend, agg *metrics.Aggregator) (*metrics.Aggregator, bool) {
	scope, scoped, ok := metricsQueryScope(c, cfg, back)
	if !ok {
		return nil, false
	}
	if !scoped {
		return agg, true
	}
	return &metrics.Aggregator{
		Sources: metrics.ScopeSources(agg.Sources, scope),
	}, true
}

func metricsQueryScope(c *gin.Context, cfg *types.Config, back types.ServerlessBackend) (metrics.QueryScope, bool, bool) {
	ownerFilter := strings.TrimSpace(c.Query("owner"))
	if !isBearerRequest(c) {
		if ownerFilter == "" {
			return metrics.QueryScope{}, false, true
		}
		return adminOwnerMetricsQueryScope(c, cfg, back, ownerFilter)
	}
	if ownerFilter != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "owner filter is only available with admin authentication"})
		return metrics.QueryScope{}, false, false
	}
	uid, err := auth.GetUIDFromContext(c)
	if err != nil {
		c.String(http.StatusUnauthorized, err.Error())
		return metrics.QueryScope{}, false, false
	}

	services, ok := listAuthorizedServicesForMetrics(c, back)
	if !ok {
		return metrics.QueryScope{}, false, false
	}

	scope := metrics.QueryScope{
		OwnerNamespace: utils.BuildUserNamespace(cfg, uid),
		ActiveServices: make([]metrics.ServiceScope, 0, len(services)),
	}
	for _, service := range services {
		if service == nil {
			continue
		}
		namespace := service.Namespace
		if namespace == "" {
			namespace = utils.BuildUserNamespace(cfg, service.Owner)
		}
		scope.ActiveServices = append(scope.ActiveServices, metrics.ServiceScope{
			Name:      service.Name,
			Namespace: namespace,
		})
	}
	return scope, true, true
}

func adminOwnerMetricsQueryScope(c *gin.Context, cfg *types.Config, back types.ServerlessBackend, owner string) (metrics.QueryScope, bool, bool) {
	namespace := ""
	if owner == types.DefaultOwner {
		namespace = cfg.ServicesNamespace
	} else {
		managedNamespaces, err := utils.ListManagedUserNamespaces(c.Request.Context(), back.GetKubeClientset())
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return metrics.QueryScope{}, false, false
		}
		for _, candidate := range managedNamespaces {
			if candidate.Owner == owner {
				namespace = candidate.Name
				break
			}
		}
	}
	if namespace == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "metrics owner not found"})
		return metrics.QueryScope{}, false, false
	}

	services, err := back.ListServices()
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return metrics.QueryScope{}, false, false
	}
	scope := metrics.QueryScope{OwnerNamespace: namespace}
	for _, service := range services {
		if service == nil || service.Owner != owner {
			continue
		}
		serviceNamespace := service.Namespace
		if serviceNamespace == "" {
			serviceNamespace = namespace
		}
		scope.ActiveServices = append(scope.ActiveServices, metrics.ServiceScope{
			Name:      service.Name,
			Namespace: serviceNamespace,
		})
	}
	return scope, true, true
}

func parseTimeRange(c *gin.Context) (metrics.TimeRange, bool) {
	startRaw := strings.TrimSpace(c.Query("start"))
	endRaw := strings.TrimSpace(c.Query("end"))

	now := time.Now().UTC()
	end := now
	if endRaw != "" {
		parsedEnd, err := time.Parse(time.RFC3339, endRaw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid end timestamp"})
			return metrics.TimeRange{}, false
		}
		end = parsedEnd
	}

	start := end.Add(-24 * time.Hour)
	if startRaw != "" {
		parsedStart, err := time.Parse(time.RFC3339, startRaw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid start timestamp"})
			return metrics.TimeRange{}, false
		}
		start = parsedStart
	}

	if end.Before(start) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "end must be after start"})
		return metrics.TimeRange{}, false
	}

	return metrics.TimeRange{Start: start, End: end}, true
}

func renderBreakdownCSV(resp types.MetricsBreakdownResponse) ([]byte, error) {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)
	if strings.EqualFold(resp.GroupBy, "user") {
		if err := writer.Write([]string{"key", "membership", "executions_count", "countries"}); err != nil {
			return nil, err
		}
	} else if strings.EqualFold(resp.GroupBy, "service") {
		if err := writer.Write([]string{"key", "membership", "requests_count_total", "requests_count_sync", "requests_count_async", "requests_count_exposed", "unique_users_count", "countries"}); err != nil {
			return nil, err
		}
	} else {
		if err := writer.Write([]string{"key", "membership", "executions_count", "unique_users_count", "countries"}); err != nil {
			return nil, err
		}
	}
	for _, item := range resp.Items {
		countries := make([]string, 0, len(item.Countries))
		for _, c := range item.Countries {
			countries = append(countries, c.Country+":"+itoa(c.RequestCount))
		}
		if strings.EqualFold(resp.GroupBy, "user") {
			if err := writer.Write([]string{
				item.Key,
				item.Membership,
				itoa(item.ExecutionsCount),
				strings.Join(countries, "|"),
			}); err != nil {
				return nil, err
			}
		} else if strings.EqualFold(resp.GroupBy, "service") {
			if err := writer.Write([]string{
				item.Key,
				item.Membership,
				itoa(item.RequestsCountTotal),
				itoa(item.RequestsCountSync),
				itoa(item.RequestsCountAsync),
				itoa(item.RequestsCountExposed),
				itoa(item.UniqueUsersCount),
				strings.Join(countries, "|"),
			}); err != nil {
				return nil, err
			}
		} else {
			if err := writer.Write([]string{
				item.Key,
				item.Membership,
				itoa(item.ExecutionsCount),
				itoa(item.UniqueUsersCount),
				strings.Join(countries, "|"),
			}); err != nil {
				return nil, err
			}
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func itoa(value int) string {
	return strconv.Itoa(value)
}
