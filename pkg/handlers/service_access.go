package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/v4/pkg/types"
	"github.com/grycap/oscar/v4/pkg/utils/auth"
	"k8s.io/apimachinery/pkg/api/errors"
)

func isBearerRequest(c *gin.Context) bool {
	return strings.HasPrefix(c.GetHeader("Authorization"), "Bearer ")
}

func isServiceOwnedByUser(service *types.Service, uid string) bool {
	return service != nil && service.Owner == uid
}

func listAuthorizedServicesForMetrics(c *gin.Context, back types.ServerlessBackend) ([]*types.Service, bool) {
	services, err := back.ListServices()
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return nil, false
	}
	if !isBearerRequest(c) {
		return services, true
	}

	uid, err := auth.GetUIDFromContext(c)
	if err != nil {
		c.String(http.StatusUnauthorized, err.Error())
		return nil, false
	}

	filtered := make([]*types.Service, 0, len(services))
	for _, service := range services {
		if isServiceOwnedByUser(service, uid) {
			filtered = append(filtered, service)
		}
	}
	return filtered, true
}

// authorizeServiceMetricsQuery permits an OIDC user to query a service that no
// longer exists. The metrics sources still constrain that query to the user's
// namespace, so a missing live service cannot expose another user's data.
func authorizeServiceMetricsQuery(c *gin.Context, back types.ServerlessBackend, serviceName string) bool {
	service, err := back.ReadService("", serviceName)
	if err != nil {
		if errors.IsNotFound(err) || errors.IsGone(err) {
			return true
		}
		c.String(http.StatusInternalServerError, err.Error())
		return false
	}

	uid, err := auth.GetUIDFromContext(c)
	if err != nil {
		c.String(http.StatusUnauthorized, err.Error())
		return false
	}
	if !isServiceOwnedByUser(service, uid) {
		c.Status(http.StatusForbidden)
		return false
	}
	return true
}
