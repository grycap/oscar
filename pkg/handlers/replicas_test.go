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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/v3/pkg/backends"
	"github.com/grycap/oscar/v3/pkg/types"
)

func TestReplicasPostUpdatesServiceWithoutFederation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	back := backends.MakeFakeBackend()
	back.Service = &types.Service{
		Name:     "svc",
		Replicas: types.ReplicaList{},
	}

	r := gin.New()
	r.POST("/system/replicas/:serviceName", MakeReplicasPostHandler(back))

	body := `{"replicas":[{"type":"oscar","cluster_id":"cluster-a","service_name":"svc-a"}]}`
	req := httptest.NewRequest(http.MethodPost, "/system/replicas/svc", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
	if back.UpdatedService == nil {
		t.Fatalf("expected service update via backend")
	}
	if len(back.UpdatedService.Replicas) != 1 {
		t.Fatalf("expected 1 replica, got %d", len(back.UpdatedService.Replicas))
	}
}
