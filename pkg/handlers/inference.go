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
	"fmt"
	"net/http"
	"net/http/httputil"

	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/v3/pkg/types"
)

func MakeInferenceHandler(cfg *types.Config, back types.SyncBackend) gin.HandlerFunc {
	return func(c *gin.Context) {
		fmt.Println("making call")
		serviceName := c.Param("serviceName")
		proxy := &httputil.ReverseProxy{
			Director: func(req *http.Request) {
				// Set the request Host parameter to avoid issues in the redirection
				// related issue: https://github.com/golang/go/issues/7682
				host := fmt.Sprintf("%s.%s", serviceName, "kserve-test.svc.cluster.local")
				req.Host = host

				req.URL.Scheme = "http"
				req.URL.Host = host
				req.URL.Path = c.Param("path")
			},
		}
		proxy.ServeHTTP(c.Writer, c.Request)
	}
}
