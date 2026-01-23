package handlers

import (
	"bytes"
	"encoding/csv"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/v3/pkg/metrics"
	"github.com/grycap/oscar/v3/pkg/types"
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

func MakeMetricValueHandler(agg *metrics.Aggregator) gin.HandlerFunc {
	return func(c *gin.Context) {
		serviceID := strings.TrimSpace(c.Query("service_id"))
		if serviceID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "service_id is required"})
			return
		}

		metricKey := types.MetricKey(strings.TrimSpace(c.Query("metric")))
		if !types.IsMetricKeyValid(metricKey) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid metric key"})
			return
		}

		tr, ok := parseTimeRange(c)
		if !ok {
			return
		}

		resp, err := agg.MetricValue(c.Request.Context(), tr, serviceID, metricKey)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, resp)
	}
}

func MakeMetricsSummaryHandler(agg *metrics.Aggregator) gin.HandlerFunc {
	return func(c *gin.Context) {
		tr, ok := parseTimeRange(c)
		if !ok {
			return
		}

		resp, err := agg.Summary(c.Request.Context(), tr)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, resp)
	}
}

func MakeMetricsBreakdownHandler(agg *metrics.Aggregator) gin.HandlerFunc {
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

		resp, err := agg.Breakdown(c.Request.Context(), tr, groupBy)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if format == "csv" {
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

		c.JSON(http.StatusOK, resp)
	}
}

func parseTimeRange(c *gin.Context) (metrics.TimeRange, bool) {
	startRaw := strings.TrimSpace(c.Query("start"))
	endRaw := strings.TrimSpace(c.Query("end"))
	if startRaw == "" || endRaw == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "start and end are required"})
		return metrics.TimeRange{}, false
	}

	start, err := time.Parse(time.RFC3339, startRaw)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid start timestamp"})
		return metrics.TimeRange{}, false
	}
	end, err := time.Parse(time.RFC3339, endRaw)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid end timestamp"})
		return metrics.TimeRange{}, false
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
