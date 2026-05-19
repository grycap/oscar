package metrics

import (
	"context"

	"github.com/grycap/oscar/v4/pkg/types"
)

type scopedServiceInventorySource struct {
	inner   ServiceInventorySource
	allowed map[string]struct{}
}

func (s *scopedServiceInventorySource) Name() string {
	return s.inner.Name()
}

func (s *scopedServiceInventorySource) ListServices(ctx context.Context, tr TimeRange) ([]ServiceDescriptor, *types.SourceStatus, error) {
	services, status, err := s.inner.ListServices(ctx, tr)
	return filterServiceDescriptors(services, s.allowed), status, err
}

type scopedUsageMetricsSource struct {
	inner UsageMetricsSource
}

func (s *scopedUsageMetricsSource) Name() string {
	return s.inner.Name()
}

func (s *scopedUsageMetricsSource) UsageHours(ctx context.Context, tr TimeRange, serviceID string) (float64, float64, *types.SourceStatus, error) {
	return s.inner.UsageHours(ctx, tr, serviceID)
}

type scopedRequestLogSource struct {
	inner   RequestLogSource
	allowed map[string]struct{}
}

func (s *scopedRequestLogSource) Name() string {
	return s.inner.Name()
}

func (s *scopedRequestLogSource) ListRequests(ctx context.Context, tr TimeRange, serviceID string) ([]RequestRecord, *types.SourceStatus, error) {
	records, status, err := s.inner.ListRequests(ctx, tr, serviceID)
	if err != nil {
		return records, status, err
	}
	return filterRequestRecords(records, s.allowed), status, nil
}

func ScopeSources(src Sources, allowedServiceIDs map[string]struct{}) Sources {
	if allowedServiceIDs == nil {
		return src
	}

	scoped := src
	if src.ServiceInventory != nil {
		scoped.ServiceInventory = &scopedServiceInventorySource{
			inner:   src.ServiceInventory,
			allowed: cloneAllowedServices(allowedServiceIDs),
		}
	}
	if src.UsageMetrics != nil {
		// Wrap usage metrics so sumUsage falls back to per-service iteration
		// instead of issuing a cluster-wide wildcard query.
		scoped.UsageMetrics = &scopedUsageMetricsSource{inner: src.UsageMetrics}
	}
	if src.RequestLogs != nil {
		scoped.RequestLogs = &scopedRequestLogSource{
			inner:   src.RequestLogs,
			allowed: cloneAllowedServices(allowedServiceIDs),
		}
	}
	if src.ExposedRequestLogs != nil {
		scoped.ExposedRequestLogs = &scopedRequestLogSource{
			inner:   src.ExposedRequestLogs,
			allowed: cloneAllowedServices(allowedServiceIDs),
		}
	}
	return scoped
}

func cloneAllowedServices(allowed map[string]struct{}) map[string]struct{} {
	if allowed == nil {
		return nil
	}
	cloned := make(map[string]struct{}, len(allowed))
	for serviceID := range allowed {
		cloned[serviceID] = struct{}{}
	}
	return cloned
}

func filterServiceDescriptors(services []ServiceDescriptor, allowed map[string]struct{}) []ServiceDescriptor {
	if allowed == nil {
		return services
	}
	filtered := make([]ServiceDescriptor, 0, len(services))
	for _, service := range services {
		if _, ok := allowed[service.ID]; ok {
			filtered = append(filtered, service)
		}
	}
	return filtered
}

func filterRequestRecords(records []RequestRecord, allowed map[string]struct{}) []RequestRecord {
	if allowed == nil {
		return records
	}
	filtered := make([]RequestRecord, 0, len(records))
	for _, record := range records {
		if _, ok := allowed[record.ServiceID]; ok {
			filtered = append(filtered, record)
		}
	}
	return filtered
}
