package metrics

import (
	"context"
	"errors"
	"regexp"

	"github.com/grycap/oscar/v4/pkg/types"
)

var errScopedUsageUnsupported = errors.New("scoped aggregate usage is not supported by this source")

// ServiceScope identifies an active service owned by the selected metrics owner.
// Namespace is required to keep services with the same name isolated.
type ServiceScope struct {
	Name      string
	Namespace string
}

// QueryScope limits metrics to an owner's namespace and active services.
// An empty OwnerNamespace represents an unscoped admin query.
type QueryScope struct {
	OwnerNamespace string
	ActiveServices []ServiceScope
}

type scopedServiceInventorySource struct {
	inner ServiceInventorySource
	scope QueryScope
}

func (s *scopedServiceInventorySource) Name() string {
	return s.inner.Name()
}

func (s *scopedServiceInventorySource) ListServices(ctx context.Context, tr TimeRange) ([]ServiceDescriptor, *types.SourceStatus, error) {
	services, status, err := s.inner.ListServices(ctx, tr)
	return filterServiceDescriptors(services, s.scope), status, err
}

type scopedUsageMetricsSource struct {
	inner UsageMetricsSource
	scope QueryScope
}

func (s *scopedUsageMetricsSource) Name() string {
	return s.inner.Name()
}

func (s *scopedUsageMetricsSource) UsageHours(ctx context.Context, tr TimeRange, serviceID string) (float64, float64, *types.SourceStatus, error) {
	prom, ok := s.inner.(*PrometheusUsageMetricsSource)
	if !ok {
		return s.inner.UsageHours(ctx, tr, serviceID)
	}

	namespaces := s.namespacesForService(serviceID)
	return sumPrometheusUsage(ctx, prom, tr, serviceID, namespaces)
}

// UsageHoursAll lets the aggregator issue one query for all services in the
// owner's namespace instead of one query per currently deployed service.
func (s *scopedUsageMetricsSource) UsageHoursAll(ctx context.Context, tr TimeRange) (float64, float64, *types.SourceStatus, error) {
	prom, ok := s.inner.(*PrometheusUsageMetricsSource)
	if !ok {
		return 0, 0, nil, errScopedUsageUnsupported
	}

	cpu, gpu, status, err := prom.UsageHoursInNamespace(
		ctx,
		tr,
		regexp.QuoteMeta(s.scope.OwnerNamespace),
		".*",
	)
	if err != nil {
		return cpu, gpu, status, err
	}

	for _, service := range s.scope.ActiveServices {
		if service.Namespace == "" || service.Namespace == s.scope.OwnerNamespace {
			continue
		}
		serviceCPU, serviceGPU, serviceStatus, serviceErr := prom.UsageHoursInNamespace(
			ctx,
			tr,
			regexp.QuoteMeta(service.Namespace),
			regexp.QuoteMeta(service.Name),
		)
		status = mergeSourceStatus(status, serviceStatus)
		if serviceErr != nil {
			return cpu, gpu, status, serviceErr
		}
		cpu += serviceCPU
		gpu += serviceGPU
	}

	return cpu, gpu, status, nil
}

func (s *scopedUsageMetricsSource) namespacesForService(serviceID string) []string {
	namespaces := []string{s.scope.OwnerNamespace}
	seen := map[string]struct{}{s.scope.OwnerNamespace: {}}
	for _, service := range s.scope.ActiveServices {
		if service.Name != serviceID || service.Namespace == "" {
			continue
		}
		if _, ok := seen[service.Namespace]; ok {
			continue
		}
		seen[service.Namespace] = struct{}{}
		namespaces = append(namespaces, service.Namespace)
	}
	return namespaces
}

func sumPrometheusUsage(ctx context.Context, prom *PrometheusUsageMetricsSource, tr TimeRange, serviceID string, namespaces []string) (float64, float64, *types.SourceStatus, error) {
	var cpuTotal float64
	var gpuTotal float64
	var combinedStatus *types.SourceStatus
	for _, namespace := range namespaces {
		cpu, gpu, status, err := prom.UsageHoursInNamespace(
			ctx,
			tr,
			regexp.QuoteMeta(namespace),
			regexp.QuoteMeta(serviceID),
		)
		combinedStatus = mergeSourceStatus(combinedStatus, status)
		if err != nil {
			return cpuTotal, gpuTotal, combinedStatus, err
		}
		cpuTotal += cpu
		gpuTotal += gpu
	}
	return cpuTotal, gpuTotal, combinedStatus, nil
}

type scopedRequestLogSource struct {
	inner RequestLogSource
	scope QueryScope
}

func (s *scopedRequestLogSource) Name() string {
	return s.inner.Name()
}

func (s *scopedRequestLogSource) ListRequests(ctx context.Context, cfg *types.Config, tr TimeRange, serviceID string) ([]RequestRecord, *types.SourceStatus, error) {
	records, status, err := s.inner.ListRequests(ctx, cfg, tr, serviceID)
	if err != nil {
		return records, status, err
	}
	return filterRequestRecords(records, s.scope), status, nil
}

func ScopeSources(src Sources, scope QueryScope) Sources {
	if scope.OwnerNamespace == "" {
		return src
	}

	scoped := src
	if src.ServiceInventory != nil {
		scoped.ServiceInventory = &scopedServiceInventorySource{inner: src.ServiceInventory, scope: scope}
	}
	if src.UsageMetrics != nil {
		scoped.UsageMetrics = &scopedUsageMetricsSource{inner: src.UsageMetrics, scope: scope}
	}
	if src.RequestLogs != nil {
		inner := scopeLokiSource(src.RequestLogs, scope)
		scoped.RequestLogs = &scopedRequestLogSource{inner: inner, scope: scope}
	}
	if src.ExposedRequestLogs != nil {
		inner := scopeLokiSource(src.ExposedRequestLogs, scope)
		scoped.ExposedRequestLogs = &scopedRequestLogSource{inner: inner, scope: scope}
	}
	return scoped
}

func scopeLokiSource(source RequestLogSource, scope QueryScope) RequestLogSource {
	loki, ok := source.(*LokiRequestLogSource)
	if !ok {
		return source
	}
	cloned := *loki
	clonedScope := scope
	cloned.scope = &clonedScope
	return &cloned
}

func filterServiceDescriptors(services []ServiceDescriptor, scope QueryScope) []ServiceDescriptor {
	if scope.OwnerNamespace == "" {
		return services
	}
	filtered := make([]ServiceDescriptor, 0, len(services))
	for _, service := range services {
		if service.Namespace == scope.OwnerNamespace || activeServiceAllowed(scope, service.ID, service.Namespace) {
			filtered = append(filtered, service)
		}
	}
	return filtered
}

func filterRequestRecords(records []RequestRecord, scope QueryScope) []RequestRecord {
	if scope.OwnerNamespace == "" {
		return records
	}
	filtered := make([]RequestRecord, 0, len(records))
	for _, record := range records {
		if record.ServiceNamespace == scope.OwnerNamespace || activeServiceAllowed(scope, record.ServiceID, record.ServiceNamespace) {
			filtered = append(filtered, record)
			continue
		}
		// Legacy execution records have no namespace. Preserve access only for
		// services that are still owned; deleted services require the new field.
		if record.ServiceNamespace == "" && activeServiceAllowed(scope, record.ServiceID, "") {
			filtered = append(filtered, record)
		}
	}
	return filtered
}

func activeServiceAllowed(scope QueryScope, name, namespace string) bool {
	for _, service := range scope.ActiveServices {
		if service.Name != name {
			continue
		}
		if namespace == "" || service.Namespace == "" || service.Namespace == namespace {
			return true
		}
	}
	return false
}

func mergeSourceStatus(left, right *types.SourceStatus) *types.SourceStatus {
	if left == nil {
		return right
	}
	if right == nil {
		return left
	}
	if left.Status == "missing" || right.Status == "missing" {
		left.Status = "missing"
	} else if left.Status == "partial" || right.Status == "partial" {
		left.Status = "partial"
	}
	if right.Notes != "" {
		if left.Notes != "" {
			left.Notes += "; "
		}
		left.Notes += right.Notes
	}
	return left
}
