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
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMakeListHandler(t *testing.T) {
	back := backends.MakeFakeBackend()

	r := gin.Default()
	r.GET("/system/services", MakeListHandler(back))

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
	r.GET("/system/services", MakeListHandler(back))

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
