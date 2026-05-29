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

package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/v4/pkg/types"
	"github.com/grycap/oscar/v4/pkg/utils"
)

func TestHasPermission(t *testing.T) {
	// Decision graph paths for hasPermission:
	// 1) public -> allow
	// 2) private + owner -> allow
	// 3) private + non-owner -> deny
	// 4) restricted + owner -> allow
	// 5) restricted + allowed user -> allow
	// 6) restricted + non-owner + not allowed -> deny
	// 7) unknown visibility -> deny
	tests := []struct {
		name    string
		service *types.Service
		uid     string
		want    bool
	}{
		{
			name: "public service allows everyone",
			service: &types.Service{
				Visibility: utils.PUBLIC,
				Owner:      "owner",
			},
			uid:  "any-user",
			want: true,
		},
		{
			name: "private service allows owner",
			service: &types.Service{
				Visibility: utils.PRIVATE,
				Owner:      "owner",
			},
			uid:  "owner",
			want: true,
		},
		{
			name: "private service denies non-owner",
			service: &types.Service{
				Visibility: utils.PRIVATE,
				Owner:      "owner",
			},
			uid:  "other-user",
			want: false,
		},
		{
			name: "restricted service allows owner",
			service: &types.Service{
				Visibility:   utils.RESTRICTED,
				Owner:        "owner",
				AllowedUsers: []string{"allowed-user"},
			},
			uid:  "owner",
			want: true,
		},
		{
			name: "restricted service allows listed user",
			service: &types.Service{
				Visibility:   utils.RESTRICTED,
				Owner:        "owner",
				AllowedUsers: []string{"allowed-user"},
			},
			uid:  "allowed-user",
			want: true,
		},
		{
			name: "restricted service denies non-listed user",
			service: &types.Service{
				Visibility:   utils.RESTRICTED,
				Owner:        "owner",
				AllowedUsers: []string{"allowed-user"},
			},
			uid:  "other-user",
			want: false,
		},
		{
			name: "unknown visibility denies access",
			service: &types.Service{
				Visibility: "custom",
				Owner:      "owner",
			},
			uid:  "owner",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasPermission(tt.service, tt.uid)
			if got != tt.want {
				t.Errorf("hasPermission() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsAuthBearer(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   bool
	}{
		{name: "valid bearer header", header: "Bearer token-123", want: true},
		{name: "missing authorization header", header: "", want: false},
		{name: "non bearer scheme", header: "Basic dXNlcjpwYXNz", want: false},
		{name: "incomplete bearer prefix", header: "Bearer", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			req, _ := http.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("Authorization", tt.header)
			c.Request = req

			_, got := isAuthBearer(c)
			if got != tt.want {
				t.Errorf("isAuthBearer() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsBasicAuth(t *testing.T) {
	tests := []struct {
		name       string
		setAuth    bool
		username   string
		password   string
		wantResult bool
	}{
		{name: "request with basic auth", setAuth: true, username: "user", password: "pass", wantResult: true},
		{name: "request without basic auth", setAuth: false, wantResult: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			req, _ := http.NewRequest(http.MethodGet, "/", nil)
			if tt.setAuth {
				req.SetBasicAuth(tt.username, tt.password)
			}
			c.Request = req

			got := isBasicAuth(c)
			if got != tt.wantResult {
				t.Errorf("isBasicAuth() = %v, want %v", got, tt.wantResult)
			}
		})
	}
}
