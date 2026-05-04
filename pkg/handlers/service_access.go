package handlers

import (
	"net/http"
	"slices"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/v4/pkg/types"
	"github.com/grycap/oscar/v4/pkg/utils"
	"github.com/grycap/oscar/v4/pkg/utils/auth"
	"k8s.io/apimachinery/pkg/api/errors"
)

func isBearerRequest(c *gin.Context) bool {
	return strings.HasPrefix(c.GetHeader("Authorization"), "Bearer ")
}

func isServiceAccessibleByUser(service *types.Service, uid string) bool {
	if service == nil {
		return false
	}
	if service.Visibility == utils.PUBLIC {
		return true
	}
	if uid == service.Owner {
		return true
	}
	return service.Visibility == utils.RESTRICTED && slices.Contains(service.AllowedUsers, uid)
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
		if isServiceAccessibleByUser(service, uid) {
			filtered = append(filtered, service)
		}
	}
	return filtered, true
}

func getAuthorizedServiceForMetrics(c *gin.Context, back types.ServerlessBackend, serviceName string) (*types.Service, bool) {
	service, err := back.ReadService("", serviceName)
	if err != nil {
		if errors.IsNotFound(err) || errors.IsGone(err) {
			c.Status(http.StatusNotFound)
		} else {
			c.String(http.StatusInternalServerError, err.Error())
		}
		return nil, false
	}
	if !isBearerRequest(c) {
		return service, true
	}

	uid, err := auth.GetUIDFromContext(c)
	if err != nil {
		c.String(http.StatusUnauthorized, err.Error())
		return nil, false
	}
	if !isServiceAccessibleByUser(service, uid) {
		c.Status(http.StatusForbidden)
		return nil, false
	}
	return service, true
}
