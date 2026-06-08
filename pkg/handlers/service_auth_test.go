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
	"testing"

	"github.com/gin-gonic/gin"
)

func TestMakeServiceAuthHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Decision graph paths for this handler:
	// 1) Request reaches the handler -> return HTTP 200.
	tests := []struct {
		name       string
		targetPath string
		wantStatus int
	}{
		{
			name:       "returns 200 when request reaches auth handler",
			targetPath: "/system/services/example/auth",
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.GET("/system/services/:serviceName/auth", MakeServiceAuthHandler())

			req, err := http.NewRequest(http.MethodGet, tt.targetPath, nil)
			if err != nil {
				t.Fatalf("failed to create request: %v", err)
			}

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}
