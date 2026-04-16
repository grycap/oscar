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
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/v3/pkg/backends"
	"github.com/grycap/oscar/v3/pkg/types"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMakeListHandler(t *testing.T) {
	back := backends.MakeFakeBackend()

	r := gin.Default()
	r.GET("/system/services", MakeListHandler(back, back.GetKubeClientset(), &types.Config{}))

	scenarios := []struct {
		name        string
		returnError bool
	}{
		{"valid", false},
		{"invalid", true},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			w := httptest.NewRecorder()

			if s.returnError {
				back.AddError("ListServices", errors.New("test error"))
			}

			req, _ := http.NewRequest("GET", "/system/services", nil)

			r.ServeHTTP(w, req)

			if s.returnError {
				if w.Code != http.StatusInternalServerError {
					t.Errorf("expecting code %d, got %d", http.StatusInternalServerError, w.Code)
				}
			} else {
				if w.Code != http.StatusOK {
					t.Errorf("expecting code %d, got %d", http.StatusOK, w.Code)
				}
			}
		})
	}
}

func TestMakeListHandlerVolumeStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	back := backends.MakeFakeBackend()
	back.Services = []*types.Service{
		{
			Name:      "svc",
			Namespace: "default",
			Volume: &types.ServiceVolumeConfig{
				Size:      "1Gi",
				MountPath: "/data",
			},
		},
	}
	_, _ = back.GetKubeClientset().CoreV1().PersistentVolumeClaims("default").Create(t.Context(), &v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "svc",
			Namespace: "default",
			Labels: map[string]string{
				types.ManagedVolumeLabel:     "true",
				types.ManagedVolumeNameLabel: "svc",
			},
		},
	}, metav1.CreateOptions{})

	r := gin.New()
	r.GET("/system/services", MakeListHandler(back, back.GetKubeClientset(), &types.Config{}))

	req := httptest.NewRequest(http.MethodGet, "/system/services", nil)
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}
	var got []types.Service
	if err := json.Unmarshal(resp.Body.Bytes(), &got); err != nil {
		t.Fatalf("unexpected unmarshal error: %v", err)
	}
	if len(got) != 1 || !got[0].VolumeStatus.Enabled || got[0].VolumeStatus.Name != "svc" {
		t.Fatalf("expected volume status enabled in list response")
	}
}

func TestMakeListHandlerIncludeDeploymentSummary(t *testing.T) {
	gin.SetMode(gin.TestMode)
	back := backends.MakeFakeBackend()
	back.Services = []*types.Service{
		{
			Name:      "svc",
			Namespace: "default",
			Expose: types.Expose{
				APIPort: 8080,
			},
		},
	}

	replicas := int32(2)
	_, _ = back.GetKubeClientset().AppsV1().Deployments("default").Create(t.Context(), &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "svc-dlp",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
		},
		Status: appsv1.DeploymentStatus{
			Replicas:          2,
			AvailableReplicas: 1,
		},
	}, metav1.CreateOptions{})

	r := gin.New()
	r.GET("/system/services", MakeListHandler(back, back.GetKubeClientset(), &types.Config{}))

	req := httptest.NewRequest(http.MethodGet, "/system/services?include=deployment", nil)
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}

	var got []types.Service
	if err := json.Unmarshal(resp.Body.Bytes(), &got); err != nil {
		t.Fatalf("unexpected unmarshal error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected one service, got %d", len(got))
	}
	if got[0].Deployment == nil {
		t.Fatalf("expected deployment summary to be included")
	}
	if got[0].Deployment.State != types.DeploymentStateDegraded {
		t.Fatalf("expected degraded deployment state, got %s", got[0].Deployment.State)
	}
	if got[0].Deployment.ActiveInstances != 2 || got[0].Deployment.AffectedInstances != 1 {
		t.Fatalf("unexpected deployment counters: active=%d affected=%d", got[0].Deployment.ActiveInstances, got[0].Deployment.AffectedInstances)
	}
}

func TestMakeListHandlerDefaultResponseOmitsDeploymentSummary(t *testing.T) {
	gin.SetMode(gin.TestMode)
	back := backends.MakeFakeBackend()
	back.Services = []*types.Service{
		{Name: "svc", Namespace: "default"},
	}

	r := gin.New()
	r.GET("/system/services", MakeListHandler(back, back.GetKubeClientset(), &types.Config{}))

	req := httptest.NewRequest(http.MethodGet, "/system/services", nil)
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}

	var got []map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &got); err != nil {
		t.Fatalf("unexpected unmarshal error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected one service, got %d", len(got))
	}
	if _, exists := got[0]["deployment"]; exists {
		t.Fatalf("expected default list response not to include deployment summary")
	}
}
