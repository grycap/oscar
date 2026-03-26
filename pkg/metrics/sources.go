package metrics

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/grycap/oscar/v3/pkg/types"
	promapi "github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type TimeRange struct {
	Start time.Time
	End   time.Time
}

type ServiceDescriptor struct {
	ID    string
	Name  string
	Image string
}

type RequestType string

const (
	RequestSync  RequestType = "sync"
	RequestAsync RequestType = "async"
)

type RequestRecord struct {
	ServiceID  string
	UserID     string
	Country    string
	AuthMethod string
	Type       RequestType
	Timestamp  time.Time
}

type ServiceInventorySource interface {
	Name() string
	ListServices(ctx context.Context, tr TimeRange) ([]ServiceDescriptor, *types.SourceStatus, error)
}

type UsageMetricsSource interface {
	Name() string
	UsageHours(ctx context.Context, tr TimeRange, serviceID string) (float64, float64, *types.SourceStatus, error)
}

type RequestLogSource interface {
	Name() string
	ListRequests(ctx context.Context, tr TimeRange, serviceID string) ([]RequestRecord, *types.SourceStatus, error)
}

type UserRosterSource interface {
	Name() string
	Classification(ctx context.Context, userID string) (string, *types.SourceStatus, error)
}

type CountryAttributionSource interface {
	Name() string
	CountryForRecord(ctx context.Context, record RequestRecord) (string, *types.SourceStatus, error)
}

type Sources struct {
	ServiceInventory   ServiceInventorySource
	UsageMetrics       UsageMetricsSource
	RequestLogs        RequestLogSource
	ExposedRequestLogs RequestLogSource
	UserRoster         UserRosterSource
	CountrySource      CountryAttributionSource
}

func DefaultSources(cfg *types.Config, back types.ServerlessBackend, kubeClientset kubernetes.Interface) Sources {
	sources := Sources{
		ServiceInventory:   &BackendServiceInventorySource{Back: back},
		UsageMetrics:       &NoopUsageMetricsSource{},
		RequestLogs:        &NoopRequestLogSource{SourceName: "request-logs"},
		ExposedRequestLogs: &NoopRequestLogSource{SourceName: "exposed-request-logs"},
		UserRoster:         &NoopUserRosterSource{},
		CountrySource:      &RequestCountrySource{},
	}
	if cfg != nil {
		if cfg.LokiBaseURL != "" {
			sources.RequestLogs = &LokiRequestLogSource{
				BaseURL:               cfg.LokiBaseURL,
				QueryTemplate:         cfg.LokiQuery,
				Namespace:             cfg.Namespace,
				AppLabel:              "oscar",
				Client:                &http.Client{Timeout: 10 * time.Second},
				ServiceFilterTemplate: "/(job|run)/%s",
			}
			if cfg.LokiExposedQuery != "" {
				sources.ExposedRequestLogs = &LokiRequestLogSource{
					BaseURL:               cfg.LokiBaseURL,
					QueryTemplate:         cfg.LokiExposedQuery,
					Namespace:             cfg.LokiExposedNamespace,
					AppLabel:              cfg.LokiExposedAppLabel,
					Client:                &http.Client{Timeout: 10 * time.Second},
					ParseFunc:             parseIngressAccessLog,
					SourceName:            "exposed-request-logs",
					ServiceFilterTemplate: "/system/services/%s/exposed",
					ServiceFilterAll:      "/system/services/.+/exposed",
				}
			}
		} else if kubeClientset != nil {
			sources.RequestLogs = &KubeRequestLogSource{
				Client:     kubeClientset,
				Namespace:  cfg.Namespace,
				LabelKey:   "app",
				LabelValue: "oscar",
			}
			sources.ExposedRequestLogs = &KubeRequestLogSource{
				Client:     kubeClientset,
				Namespace:  "ingress-nginx",
				LabelKey:   "app.kubernetes.io/name",
				LabelValue: "ingress-nginx",
				ParseFunc:  parseIngressAccessLog,
				SourceName: "exposed-request-logs",
			}
		}
	}
	if cfg != nil && cfg.PrometheusBaseURL != "" {
		if promSource, err := NewPrometheusUsageMetricsSource(cfg.PrometheusBaseURL, cfg.PrometheusCPUQuery, cfg.PrometheusGPUQuery, cfg.ServicesNamespace); err == nil {
			sources.UsageMetrics = promSource
		}
	}
	if cfg != nil && cfg.PrometheusBaseURL == "" {
		defaultURL := "http://prometheus-server.monitoring.svc.cluster.local"
		if promSource, err := NewPrometheusUsageMetricsSource(defaultURL, cfg.PrometheusCPUQuery, cfg.PrometheusGPUQuery, cfg.ServicesNamespace); err == nil {
			sources.UsageMetrics = promSource
		}
	}
	return sources
}

type BackendServiceInventorySource struct {
	Back types.ServerlessBackend
}

func (s *BackendServiceInventorySource) Name() string {
	return "service-inventory"
}

func (s *BackendServiceInventorySource) ListServices(ctx context.Context, tr TimeRange) ([]ServiceDescriptor, *types.SourceStatus, error) {
	services, err := s.Back.ListServices()
	if err != nil {
		status := missingStatus(s.Name(), err)
		return nil, status, err
	}
	result := make([]ServiceDescriptor, 0, len(services))
	for _, service := range services {
		if service == nil {
			continue
		}
		result = append(result, ServiceDescriptor{
			ID:    service.Name,
			Name:  service.Name,
			Image: service.Image,
		})
	}
	status := okStatus(s.Name(), "")
	return result, status, nil
}

type NoopUsageMetricsSource struct{}

func (s *NoopUsageMetricsSource) Name() string {
	return "usage-metrics"
}

func (s *NoopUsageMetricsSource) UsageHours(ctx context.Context, tr TimeRange, serviceID string) (float64, float64, *types.SourceStatus, error) {
	err := errors.New("usage metrics source not configured")
	status := missingStatus(s.Name(), err)
	return 0, 0, status, err
}

type PrometheusUsageMetricsSource struct {
	API               v1.API
	CPUQuery          string
	GPUQuery          string
	ServicesNamespace string
}

func NewPrometheusUsageMetricsSource(baseURL, cpuQuery, gpuQuery, servicesNamespace string) (*PrometheusUsageMetricsSource, error) {
	client, err := promapi.NewClient(promapi.Config{Address: baseURL})
	if err != nil {
		return nil, err
	}
	return &PrometheusUsageMetricsSource{
		API:               v1.NewAPI(client),
		CPUQuery:          cpuQuery,
		GPUQuery:          gpuQuery,
		ServicesNamespace: servicesNamespace,
	}, nil
}

func (s *PrometheusUsageMetricsSource) Name() string {
	return "prometheus"
}

func (s *PrometheusUsageMetricsSource) UsageHours(ctx context.Context, tr TimeRange, serviceID string) (float64, float64, *types.SourceStatus, error) {
	if s.API == nil {
		err := errors.New("prometheus client not configured")
		return 0, 0, missingStatus(s.Name(), err), err
	}

	rangeSeconds := int64(tr.End.Sub(tr.Start).Seconds())
	if rangeSeconds <= 0 {
		err := errors.New("invalid time range")
		return 0, 0, missingStatus(s.Name(), err), err
	}
	rangeLiteral := fmt.Sprintf("%ds", rangeSeconds)

	cpuValue, cpuErr := s.queryValue(ctx, s.CPUQuery, serviceID, rangeLiteral, tr.End)
	gpuValue, gpuErr := s.queryValue(ctx, s.GPUQuery, serviceID, rangeLiteral, tr.End)

	status := okStatus(s.Name(), "")
	if cpuErr != nil || gpuErr != nil {
		status.Status = "partial"
		status.Notes = joinErrors(cpuErr, gpuErr)
	}
	if cpuErr != nil && gpuErr != nil {
		status.Status = "missing"
	}

	if cpuErr != nil && gpuErr != nil {
		return 0, 0, status, cpuErr
	}

	return cpuValue, gpuValue, status, nil
}

func (s *PrometheusUsageMetricsSource) queryValue(ctx context.Context, template, serviceID, rangeLiteral string, ts time.Time) (float64, error) {
	if template == "" {
		return 0, errors.New("prometheus query template not configured")
	}
	query := strings.ReplaceAll(template, "{{service}}", serviceID)
	query = strings.ReplaceAll(query, "{{range}}", rangeLiteral)
	if strings.Contains(query, "{{services_namespace}}") {
		if s.ServicesNamespace == "" {
			return 0, errors.New("prometheus services namespace not configured")
		}
		query = strings.ReplaceAll(query, "{{services_namespace}}", s.ServicesNamespace)
	}

	result, warnings, err := s.API.Query(ctx, query, ts)
	if err != nil {
		return 0, err
	}
	if len(warnings) > 0 {
		return 0, fmt.Errorf("prometheus warnings: %s", strings.Join(warnings, "; "))
	}
	return parsePromValue(result)
}

func parsePromValue(value model.Value) (float64, error) {
	switch v := value.(type) {
	case model.Vector:
		var sum float64
		for _, sample := range v {
			sum += float64(sample.Value)
		}
		return sum, nil
	case *model.Scalar:
		return float64(v.Value), nil
	case model.Matrix:
		var sum float64
		for _, stream := range v {
			if len(stream.Values) == 0 {
				continue
			}
			last := stream.Values[len(stream.Values)-1]
			sum += float64(last.Value)
		}
		return sum, nil
	default:
		return 0, fmt.Errorf("unsupported prometheus result type %T", value)
	}
}

func joinErrors(errs ...error) string {
	parts := make([]string, 0, len(errs))
	for _, err := range errs {
		if err == nil {
			continue
		}
		parts = append(parts, err.Error())
	}
	return strings.Join(parts, "; ")
}

type NoopRequestLogSource struct {
	SourceName string
}

func (s *NoopRequestLogSource) Name() string {
	if s.SourceName != "" {
		return s.SourceName
	}
	return "request-logs"
}

func (s *NoopRequestLogSource) ListRequests(ctx context.Context, tr TimeRange, serviceID string) ([]RequestRecord, *types.SourceStatus, error) {
	err := errors.New("request log source not configured")
	status := missingStatus(s.Name(), err)
	return nil, status, err
}

type LokiRequestLogSource struct {
	BaseURL               string
	QueryTemplate         string
	Namespace             string
	AppLabel              string
	Client                *http.Client
	ParseFunc             func(string) (RequestRecord, bool)
	SourceName            string
	ServiceFilterTemplate string
	ServiceFilterAll      string
}

func (s *LokiRequestLogSource) Name() string {
	if s.SourceName != "" {
		return s.SourceName
	}
	return "loki"
}

func (s *LokiRequestLogSource) ListRequests(ctx context.Context, tr TimeRange, serviceID string) ([]RequestRecord, *types.SourceStatus, error) {
	if s.BaseURL == "" {
		err := errors.New("loki base URL not configured")
		return nil, missingStatus(s.Name(), err), err
	}
	query := s.buildQuery(serviceID)
	if query == "" {
		err := errors.New("loki query template not configured")
		return nil, missingStatus(s.Name(), err), err
	}

	values := url.Values{}
	values.Set("query", query)
	values.Set("start", strconv.FormatInt(tr.Start.UTC().UnixNano(), 10))
	values.Set("end", strconv.FormatInt(tr.End.UTC().UnixNano(), 10))
	values.Set("limit", "5000")
	values.Set("direction", "forward")

	endpoint := strings.TrimRight(s.BaseURL, "/") + "/loki/api/v1/query_range?" + values.Encode()
	client := s.Client
	if client == nil {
		client = http.DefaultClient
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, missingStatus(s.Name(), err), err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, missingStatus(s.Name(), err), err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("loki query failed: %s", resp.Status)
		return nil, missingStatus(s.Name(), err), err
	}

	result, err := decodeLokiResponse(resp.Body)
	if err != nil {
		return nil, missingStatus(s.Name(), err), err
	}

	parseFn := s.ParseFunc
	if parseFn == nil {
		parseFn = parseGinExecutionLog
	}

	records := make([]RequestRecord, 0)
	for _, stream := range result {
		for _, entry := range stream.Values {
			lokiTime := time.Unix(0, entry.Timestamp)
			record, ok := parseFn(entry.Line)
			if !ok {
				continue
			}
			if record.Country == "" || strings.EqualFold(record.Country, "unknown") {
				if country := countryFromLokiLabels(stream.Labels); country != "" {
					record.Country = country
				}
			}
			if record.Timestamp.IsZero() {
				record.Timestamp = lokiTime
			}
			if record.Timestamp.Before(tr.Start) || record.Timestamp.After(tr.End) {
				continue
			}
			if serviceID != "" && record.ServiceID != serviceID {
				continue
			}
			records = append(records, record)
		}
	}

	status := okStatus(s.Name(), "")
	return records, status, nil
}

func (s *LokiRequestLogSource) buildQuery(serviceID string) string {
	if s.QueryTemplate == "" {
		return ""
	}
	query := strings.ReplaceAll(s.QueryTemplate, "{{namespace}}", s.Namespace)
	query = strings.ReplaceAll(query, "{{app}}", s.AppLabel)
	if strings.Contains(query, "{{service}}") {
		serviceReplacement := serviceID
		if serviceReplacement == "" {
			serviceReplacement = ".*"
		}
		query = strings.ReplaceAll(query, "{{service}}", serviceReplacement)
		return query
	}

	if serviceID != "" && s.ServiceFilterTemplate != "" {
		query += fmt.Sprintf(" |~ \"%s\"", fmt.Sprintf(s.ServiceFilterTemplate, regexp.QuoteMeta(serviceID)))
	} else if serviceID == "" && s.ServiceFilterAll != "" {
		query += fmt.Sprintf(" |~ \"%s\"", s.ServiceFilterAll)
	}
	return query
}

type lokiStreamEntry struct {
	Timestamp int64
	Line      string
}

type lokiStream struct {
	Labels map[string]string
	Values []lokiStreamEntry
}

func decodeLokiResponse(r io.Reader) ([]lokiStream, error) {
	var payload struct {
		Status string `json:"status"`
		Data   struct {
			ResultType string `json:"resultType"`
			Result     []struct {
				Stream map[string]string `json:"stream"`
				Values [][]string        `json:"values"`
			} `json:"result"`
		} `json:"data"`
	}

	decoder := json.NewDecoder(r)
	if err := decoder.Decode(&payload); err != nil {
		return nil, err
	}
	if payload.Status != "success" {
		return nil, fmt.Errorf("loki response status: %s", payload.Status)
	}

	streams := make([]lokiStream, 0, len(payload.Data.Result))
	for _, result := range payload.Data.Result {
		stream := lokiStream{Labels: result.Stream, Values: []lokiStreamEntry{}}
		for _, value := range result.Values {
			if len(value) < 2 {
				continue
			}
			ts, err := strconv.ParseInt(value[0], 10, 64)
			if err != nil {
				continue
			}
			stream.Values = append(stream.Values, lokiStreamEntry{
				Timestamp: ts,
				Line:      value[1],
			})
		}
		streams = append(streams, stream)
	}
	return streams, nil
}

func countryFromLokiLabels(labels map[string]string) string {
	if labels == nil {
		return ""
	}
	for _, key := range []string{
		"geoip_country_code",
		"geoip_country_name",
		"country",
		"country_code",
	} {
		if val := strings.TrimSpace(labels[key]); val != "" {
			return val
		}
	}
	return ""
}

type KubeRequestLogSource struct {
	Client     kubernetes.Interface
	Namespace  string
	LabelKey   string
	LabelValue string
	ParseFunc  func(string) (RequestRecord, bool)
	SourceName string
}

func (s *KubeRequestLogSource) Name() string {
	if s.SourceName != "" {
		return s.SourceName
	}
	return "request-logs"
}

func (s *KubeRequestLogSource) ListRequests(ctx context.Context, tr TimeRange, serviceID string) ([]RequestRecord, *types.SourceStatus, error) {
	if s.Client == nil {
		err := errors.New("kubernetes client not configured")
		return nil, missingStatus(s.Name(), err), err
	}
	if s.Namespace == "" {
		err := errors.New("namespace not configured for request logs")
		return nil, missingStatus(s.Name(), err), err
	}

	listOpts := metav1.ListOptions{}
	if s.LabelKey != "" && s.LabelValue != "" {
		listOpts.LabelSelector = fmt.Sprintf("%s=%s", s.LabelKey, s.LabelValue)
	}

	pods, err := s.Client.CoreV1().Pods(s.Namespace).List(ctx, listOpts)
	if err != nil {
		return nil, missingStatus(s.Name(), err), err
	}

	records := make([]RequestRecord, 0)
	var hadError bool
	var hadData bool
	var notes []string

	for _, pod := range pods.Items {
		podRecords, podErr := s.readPodLogs(ctx, pod, tr, serviceID)
		if podErr != nil {
			hadError = true
			notes = append(notes, podErr.Error())
			continue
		}
		if len(podRecords) > 0 {
			hadData = true
			records = append(records, podRecords...)
		}
	}

	status := okStatus(s.Name(), "")
	if hadError && hadData {
		status.Status = "partial"
		status.Notes = strings.Join(notes, "; ")
	}
	if hadError && !hadData {
		status.Status = "missing"
		status.Notes = strings.Join(notes, "; ")
	}

	return records, status, nil
}

func (s *KubeRequestLogSource) readPodLogs(ctx context.Context, pod corev1.Pod, tr TimeRange, serviceID string) ([]RequestRecord, error) {
	opts := &corev1.PodLogOptions{
		SinceTime: &metav1.Time{Time: tr.Start},
	}
	req := s.Client.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, opts)
	stream, err := req.Stream(ctx)
	if err != nil {
		return nil, err
	}
	defer stream.Close()

	records := make([]RequestRecord, 0)
	scanner := bufio.NewScanner(stream)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	parseFn := s.ParseFunc
	if parseFn == nil {
		parseFn = parseGinExecutionLog
	}
	for scanner.Scan() {
		line := scanner.Text()
		record, ok := parseFn(line)
		if !ok {
			continue
		}
		if record.Timestamp.Before(tr.Start) || record.Timestamp.After(tr.End) {
			continue
		}
		if serviceID != "" && record.ServiceID != serviceID {
			continue
		}
		records = append(records, record)
	}
	if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) {
		return records, err
	}
	return records, nil
}

const ginExecutionPrefix = "[GIN-EXECUTIONS-LOGGER]"
const ginPrefix = "[GIN]"

func parseGinExecutionLog(line string) (RequestRecord, bool) {
	payload := ""
	if idx := strings.Index(line, ginExecutionPrefix); idx != -1 {
		payload = strings.TrimSpace(line[idx+len(ginExecutionPrefix):])
	} else if idx := strings.Index(line, ginPrefix); idx != -1 {
		payload = strings.TrimSpace(line[idx+len(ginPrefix):])
	} else {
		payload = strings.TrimSpace(line)
	}
	parts := strings.Split(payload, "|")
	if len(parts) < 6 {
		return RequestRecord{}, false
	}

	timestampRaw := strings.TrimSpace(parts[0])
	timestamp, err := time.ParseInLocation("2006/01/02 - 15:04:05", timestampRaw, time.UTC)
	if err != nil {
		return RequestRecord{}, false
	}

	ip := strings.TrimSpace(parts[3])
	methodPath := strings.Fields(strings.TrimSpace(parts[4]))
	path := ""
	if len(methodPath) > 1 {
		path = methodPath[len(methodPath)-1]
	}
	if path == "" {
		return RequestRecord{}, false
	}

	path = strings.Trim(path, "\"")
	path = strings.SplitN(path, "?", 2)[0]
	serviceID, reqType := parseServiceFromPath(path)

	return RequestRecord{
		ServiceID:  serviceID,
		UserID:     strings.TrimSpace(parts[5]),
		Country:    "",
		AuthMethod: "",
		Type:       reqType,
		Timestamp:  timestamp,
	}, ip != ""
}

func parseServiceFromPath(path string) (string, RequestType) {
	if strings.HasPrefix(path, "/job/") {
		return strings.TrimPrefix(path, "/job/"), RequestAsync
	}
	if strings.HasPrefix(path, "/run/") {
		return strings.TrimPrefix(path, "/run/"), RequestSync
	}
	return "", ""
}

func parseExposedServiceFromPath(path string) (string, bool) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", false
	}
	path = strings.SplitN(path, "?", 2)[0]
	const prefix = "/system/services/"
	if !strings.HasPrefix(path, prefix) {
		return "", false
	}
	rest := strings.TrimPrefix(path, prefix)
	parts := strings.SplitN(rest, "/", 3)
	if len(parts) < 2 {
		return "", false
	}
	if parts[0] == "" || parts[1] != "exposed" {
		return "", false
	}
	return parts[0], true
}

func parseIngressAccessLog(line string) (RequestRecord, bool) {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return RequestRecord{}, false
	}

	var timestamp time.Time
	openIdx := strings.Index(line, "[")
	closeIdx := strings.Index(line, "]")
	if openIdx != -1 && closeIdx > openIdx+1 {
		raw := line[openIdx+1 : closeIdx]
		parsed, err := time.Parse("02/Jan/2006:15:04:05 -0700", raw)
		if err == nil {
			timestamp = parsed
		}
	}

	firstQuote := strings.Index(line, "\"")
	if firstQuote == -1 {
		return RequestRecord{}, false
	}
	rest := line[firstQuote+1:]
	secondQuote := strings.Index(rest, "\"")
	if secondQuote == -1 {
		return RequestRecord{}, false
	}
	requestLine := rest[:secondQuote]
	reqParts := strings.Fields(requestLine)
	if len(reqParts) < 2 {
		return RequestRecord{}, false
	}
	path := reqParts[1]
	serviceID, ok := parseExposedServiceFromPath(path)
	if !ok {
		return RequestRecord{}, false
	}

	return RequestRecord{
		ServiceID:  serviceID,
		UserID:     "",
		Country:    "",
		AuthMethod: "",
		Type:       "",
		Timestamp:  timestamp,
	}, true
}

type NoopUserRosterSource struct{}

func (s *NoopUserRosterSource) Name() string {
	return "user-roster"
}

func (s *NoopUserRosterSource) Classification(ctx context.Context, userID string) (string, *types.SourceStatus, error) {
	err := errors.New("user roster source not configured")
	status := missingStatus(s.Name(), err)
	return "unknown", status, err
}

type RequestCountrySource struct{}

func (s *RequestCountrySource) Name() string {
	return "country-attribution"
}

func (s *RequestCountrySource) CountryForRecord(ctx context.Context, record RequestRecord) (string, *types.SourceStatus, error) {
	country := record.Country
	if country == "" {
		country = "unknown"
	}
	status := okStatus(s.Name(), "")
	return country, status, nil
}

func okStatus(name, notes string) *types.SourceStatus {
	return &types.SourceStatus{
		Name:   name,
		Status: "ok",
		Notes:  notes,
	}
}

func missingStatus(name string, err error) *types.SourceStatus {
	notes := ""
	if err != nil {
		notes = err.Error()
	}
	return &types.SourceStatus{
		Name:   name,
		Status: "missing",
		Notes:  notes,
	}
}
