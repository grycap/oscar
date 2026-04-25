/*
Copyright (C) GRyCAP - I3M - UPV

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/v3/pkg/backends/resources"
	"github.com/grycap/oscar/v3/pkg/types"
	"github.com/grycap/oscar/v3/pkg/utils"
	"github.com/grycap/oscar/v3/pkg/utils/auth"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// MakeListVolumesHandler godoc
// @Summary List volumes
// @Description List managed volumes in the caller namespace.
// @Tags volumes
// @Produce json
// @Success 200 {array} types.ManagedVolume
// @Failure 401 {string} string "Unauthorized"
// @Failure 500 {string} string "Internal Server Error"
// @Security BasicAuth
// @Security BearerAuth
// @Router /system/volumes [get]
func MakeListVolumesHandler(cfg *types.Config, back types.ServerlessBackend) gin.HandlerFunc {
	return func(c *gin.Context) {
		namespace, _, err := resolveVolumeCaller(c, cfg, back)
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}

		volumes, err := resources.ListManagedVolumes(c.Request.Context(), back.GetKubeClientset(), namespace)
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}

		c.JSON(http.StatusOK, volumes)
	}
}

// MakeCreateVolumeHandler godoc
// @Summary Create volume
// @Description Create a managed volume in the caller namespace.
// @Tags volumes
// @Accept json
// @Produce json
// @Param volume body types.ManagedVolumeCreateRequest true "Volume definition"
// @Success 201 {object} types.ManagedVolume
// @Failure 400 {string} string "Bad Request"
// @Failure 401 {string} string "Unauthorized"
// @Failure 409 {string} string "Conflict"
// @Failure 500 {string} string "Internal Server Error"
// @Security BasicAuth
// @Security BearerAuth
// @Router /system/volumes [post]
func MakeCreateVolumeHandler(cfg *types.Config, back types.ServerlessBackend) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req types.ManagedVolumeCreateRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.String(http.StatusBadRequest, fmt.Sprintf("The volume specification is not valid: %v", err))
			return
		}
		if err := utils.ValidateManagedVolumeCreateRequest(&req); err != nil {
			c.String(http.StatusBadRequest, err.Error())
			return
		}

		namespace, owner, err := resolveVolumeCaller(c, cfg, back)
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
		if owner != types.DefaultOwner {
			if err := utils.ValidateManagedVolumeQuota(auth.FormatUID(owner), namespace, req.Size, cfg, back.GetKubeClientset()); err != nil {
				c.String(http.StatusBadRequest, err.Error())
				return
			}
		}

		err = resources.CreateManagedVolume(
			c.Request.Context(),
			cfg,
			back.GetKubeClientset(),
			namespace,
			owner,
			req.Name,
			req.Size,
			types.VolumeCreationModeAPI,
			"",
			"",
		)
		if err != nil {
			switch {
			case apierrors.IsAlreadyExists(err):
				c.String(http.StatusConflict, "volume already exists in caller namespace")
			default:
				c.String(http.StatusInternalServerError, err.Error())
			}
			return
		}

		volume, err := resources.GetManagedVolume(c.Request.Context(), back.GetKubeClientset(), namespace, req.Name)
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
		c.JSON(http.StatusCreated, volume)
	}
}

// MakeReadVolumeHandler godoc
// @Summary Read volume
// @Description Get a managed volume in the caller namespace.
// @Tags volumes
// @Produce json
// @Param volumeName path string true "Volume name"
// @Success 200 {object} types.ManagedVolume
// @Failure 401 {string} string "Unauthorized"
// @Failure 404 {string} string "Not Found"
// @Failure 500 {string} string "Internal Server Error"
// @Security BasicAuth
// @Security BearerAuth
// @Router /system/volumes/{volumeName} [get]
func MakeReadVolumeHandler(cfg *types.Config, back types.ServerlessBackend) gin.HandlerFunc {
	return func(c *gin.Context) {
		namespace, _, err := resolveVolumeCaller(c, cfg, back)
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}

		volume, err := resources.GetManagedVolume(c.Request.Context(), back.GetKubeClientset(), namespace, c.Param("volumeName"))
		if err != nil {
			if apierrors.IsNotFound(err) {
				c.Status(http.StatusNotFound)
				return
			}
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
		c.JSON(http.StatusOK, volume)
	}
}

// MakeDeleteVolumeHandler godoc
// @Summary Delete volume
// @Description Delete a detached managed volume in the caller namespace.
// @Tags volumes
// @Param volumeName path string true "Volume name"
// @Success 204 {string} string "No Content"
// @Failure 400 {string} string "Bad Request"
// @Failure 401 {string} string "Unauthorized"
// @Failure 404 {string} string "Not Found"
// @Failure 500 {string} string "Internal Server Error"
// @Security BasicAuth
// @Security BearerAuth
// @Router /system/volumes/{volumeName} [delete]
func MakeDeleteVolumeHandler(cfg *types.Config, back types.ServerlessBackend) gin.HandlerFunc {
	return func(c *gin.Context) {
		namespace, _, err := resolveVolumeCaller(c, cfg, back)
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}

		err = resources.DeleteManagedVolume(c.Request.Context(), back.GetKubeClientset(), namespace, c.Param("volumeName"), false)
		if err != nil {
			switch {
			case errors.Is(err, resources.ErrManagedVolumeAttached):
				c.String(http.StatusBadRequest, err.Error())
			case apierrors.IsNotFound(err):
				c.Status(http.StatusNotFound)
			default:
				c.String(http.StatusInternalServerError, err.Error())
			}
			return
		}
		c.Status(http.StatusNoContent)
	}
}

func resolveVolumeCaller(c *gin.Context, cfg *types.Config, back types.ServerlessBackend) (string, string, error) {
	authHeader := c.GetHeader("Authorization")
	if len(strings.Split(authHeader, "Bearer")) == 1 {
		return cfg.ServicesNamespace, types.DefaultOwner, nil
	}

	uid, err := auth.GetUIDFromContext(c)
	if err != nil {
		return "", "", err
	}
	namespace, err := utils.EnsureUserNamespace(context.TODO(), back.GetKubeClientset(), cfg, uid)
	if err != nil {
		return "", "", err
	}
	return namespace, uid, nil
}
