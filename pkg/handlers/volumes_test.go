package handlers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/v3/pkg/backends"
	"github.com/grycap/oscar/v3/pkg/types"
	"github.com/grycap/oscar/v3/pkg/utils"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestVolumeHandlersCRUD(t *testing.T) {
	gin.SetMode(gin.TestMode)
	back := backends.MakeFakeBackend()
	cfg := &types.Config{ServicesNamespace: "oscar-svc"}
	createBaseRuntimePVC(t, back, cfg)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("uidOrigin", "user@example.org")
		c.Next()
	})
	r.POST("/system/volumes", MakeCreateVolumeHandler(cfg, back))
	r.GET("/system/volumes", MakeListVolumesHandler(cfg, back))
	r.GET("/system/volumes/:volumeName", MakeReadVolumeHandler(cfg, back))
	r.DELETE("/system/volumes/:volumeName", MakeDeleteVolumeHandler(cfg, back))

	postReq := httptest.NewRequest(http.MethodPost, "/system/volumes", strings.NewReader(`{"name":"shared-data","size":"1Gi"}`))
	postReq.Header.Set("Content-Type", "application/json")
	postReq.Header.Set("Authorization", "Bearer token")
	postResp := httptest.NewRecorder()
	r.ServeHTTP(postResp, postReq)
	if postResp.Code != http.StatusCreated {
		t.Fatalf("expected create volume status 201, got %d: %s", postResp.Code, postResp.Body.String())
	}

	listReq := httptest.NewRequest(http.MethodGet, "/system/volumes", nil)
	listReq.Header.Set("Authorization", "Bearer token")
	listResp := httptest.NewRecorder()
	r.ServeHTTP(listResp, listReq)
	if listResp.Code != http.StatusOK {
		fmt.Println(listResp.Body)
		t.Fatalf("expected list volume status 200, got %d", listResp.Code)
	}
	if !strings.Contains(listResp.Body.String(), "shared-data") {
		t.Fatalf("expected list response to include created volume, got %s", listResp.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "/system/volumes/shared-data", nil)
	getReq.Header.Set("Authorization", "Bearer token")
	getResp := httptest.NewRecorder()
	r.ServeHTTP(getResp, getReq)
	if getResp.Code != http.StatusOK {
		t.Fatalf("expected read volume status 200, got %d", getResp.Code)
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/system/volumes/shared-data", nil)
	deleteReq.Header.Set("Authorization", "Bearer token")
	deleteResp := httptest.NewRecorder()
	r.ServeHTTP(deleteResp, deleteReq)
	if deleteResp.Code != http.StatusNoContent {
		t.Fatalf("expected delete volume status 204, got %d: %s", deleteResp.Code, deleteResp.Body.String())
	}
}

func TestCreateVolumeHandlerRejectsQuotaExceeded(t *testing.T) {
	gin.SetMode(gin.TestMode)
	back := backends.MakeFakeBackend()
	cfg := &types.Config{ServicesNamespace: "oscar-svc"}
	createBaseRuntimePVC(t, back, cfg)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("uidOrigin", "user@example.org")
		c.Next()
	})
	r.POST("/system/volumes", MakeCreateVolumeHandler(cfg, back))

	req := httptest.NewRequest(http.MethodPost, "/system/volumes", strings.NewReader(`{"name":"too-large","size":"2Gi"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer token")
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected create volume status 400, got %d: %s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), "not enough volume disk quota") {
		t.Fatalf("expected volume quota error, got %s", resp.Body.String())
	}
}

func TestDeleteVolumeHandlerRejectsAttachedVolume(t *testing.T) {
	gin.SetMode(gin.TestMode)
	back := backends.MakeFakeBackend()
	cfg := &types.Config{ServicesNamespace: "oscar-svc"}
	createBaseRuntimePVC(t, back, cfg)
	namespace, err := utils.EnsureUserNamespace(t.Context(), back.GetKubeClientset(), cfg, "user@example.org")
	if err != nil {
		t.Fatalf("unexpected namespace error: %v", err)
	}
	_, _ = back.GetKubeClientset().CoreV1().PersistentVolumeClaims(namespace).Create(t.Context(), &v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "shared-data",
			Namespace: namespace,
			Labels: map[string]string{
				types.ManagedVolumeLabel:     "true",
				types.ManagedVolumeNameLabel: "shared-data",
			},
		},
	}, metav1.CreateOptions{})
	_, _ = back.GetKubeClientset().CoreV1().ConfigMaps(namespace).Create(t.Context(), &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "consumer",
			Namespace: namespace,
		},
		Data: map[string]string{
			types.FDLFileName: "name: consumer\nvolume:\n  name: shared-data\n  mount_path: /data\n",
		},
	}, metav1.CreateOptions{})

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("uidOrigin", "user@example.org")
		c.Next()
	})
	r.DELETE("/system/volumes/:volumeName", MakeDeleteVolumeHandler(cfg, back))

	req := httptest.NewRequest(http.MethodDelete, "/system/volumes/shared-data", nil)
	req.Header.Set("Authorization", "Bearer token")
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected delete attached volume status 400, got %d: %s", resp.Code, resp.Body.String())
	}
}

func createBaseRuntimePVC(t *testing.T, back *backends.FakeBackend, cfg *types.Config) {
	t.Helper()
	_, _ = back.GetKubeClientset().CoreV1().PersistentVolumes().Create(t.Context(), &v1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: "base-oscar-pv",
		},
	}, metav1.CreateOptions{})
	_, _ = back.GetKubeClientset().CoreV1().PersistentVolumeClaims(cfg.ServicesNamespace).Create(t.Context(), &v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      types.PVCName,
			Namespace: cfg.ServicesNamespace,
		},
		Spec: v1.PersistentVolumeClaimSpec{
			VolumeName: "base-oscar-pv",
		},
		Status: v1.PersistentVolumeClaimStatus{
			Phase: v1.ClaimBound,
		},
	}, metav1.CreateOptions{})
	_, _ = back.GetKubeClientset().CoreV1().ResourceQuotas("oscar-svc-user-example-org-547e41ffe2031bcdc35ffc6687f10d498c46").Create(t.Context(), &v1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "user",
			Namespace: "oscar-svc-user-example-org-547e41ffe2031bcdc35ffc6687f10d498c46",
		},
		Spec: v1.ResourceQuotaSpec{
			Hard: v1.ResourceList{
				v1.ResourceRequestsStorage:        resource.MustParse("1Gi"),
				v1.ResourcePersistentVolumeClaims: resource.MustParse("2"),
			},
		},
	}, metav1.CreateOptions{})
	_, _ = back.GetKubeClientset().CoreV1().LimitRanges("oscar-svc-user-example-org-547e41ffe2031bcdc35ffc6687f10d498c46").Create(t.Context(), &v1.LimitRange{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "user",
			Namespace: "oscar-svc-user-example-org-547e41ffe2031bcdc35ffc6687f10d498c46",
		},
		Spec: v1.LimitRangeSpec{
			Limits: []v1.LimitRangeItem{
				{
					Type: "PersistentVolumeClaim",
					Max: v1.ResourceList{
						v1.ResourceStorage: resource.MustParse("1Gi"),
					},
					Min: v1.ResourceList{
						v1.ResourceStorage: resource.MustParse("200Mi"),
					},
				},
			},
		},
	}, metav1.CreateOptions{})
}
