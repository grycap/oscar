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
	"github.com/grycap/oscar/v3/pkg/backends"
)

func TestMakeInfoHandler(t *testing.T) {
	back := backends.MakeFakeBackend()

	r := gin.Default()
	r.GET("/system/info", MakeInfoHandler(back.GetKubeClientset(), back))

	w := httptest.NewRecorder()

	req, _ := http.NewRequest("GET", "/system/info", nil)

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expecting code %d, got %d", http.StatusOK, w.Code)
	}
}
