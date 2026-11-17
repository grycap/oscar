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
)

func TestUserAllowedForVisibility(t *testing.T) {
	tests := []struct {
		name         string
		uid          string
		visibility   string
		owner        string
		allowedUsers []string
		want         bool
	}{
		{name: "public allows everyone", uid: "any-user", visibility: types.PUBLIC, owner: "owner", want: true},
		{name: "private allows owner", uid: "owner", visibility: types.PRIVATE, owner: "owner", want: true},
		{name: "private denies non-owner", uid: "other-user", visibility: types.PRIVATE, owner: "owner", want: false},
		{name: "restricted allows owner", uid: "owner", visibility: types.RESTRICTED, owner: "owner", allowedUsers: []string{"allowed-user"}, want: true},
		{name: "restricted allows listed user", uid: "allowed-user", visibility: types.RESTRICTED, owner: "owner", allowedUsers: []string{"allowed-user"}, want: true},
		{name: "restricted denies non-listed user", uid: "other-user", visibility: types.RESTRICTED, owner: "owner", allowedUsers: []string{"allowed-user"}, want: false},
		{name: "empty visibility denies access", uid: "owner", visibility: "", owner: "owner", want: false},
		{name: "unknown visibility denies access", uid: "owner", visibility: "custom", owner: "owner", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := UserAllowedForVisibility(tt.uid, tt.visibility, tt.owner, tt.allowedUsers); got != tt.want {
				t.Errorf("UserAllowedForVisibility() = %v, want %v", got, tt.want)
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
