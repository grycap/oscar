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
package utils

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/grycap/oscar/v3/pkg/testsupport"
	"time"

	"github.com/grycap/oscar/v3/pkg/types"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestNewOFScaler(t *testing.T) {
	kubeClientset := fake.NewSimpleClientset()
	cfg := &types.Config{
		OpenfaasNamespace:                "openfaas",
		ServicesNamespace:                "default",
		OpenfaasPort:                     8080,
		OpenfaasBasicAuthSecret:          "basic-auth",
		OpenfaasPrometheusPort:           9090,
		OpenfaasScalerInactivityDuration: "5m",
		OpenfaasScalerInterval:           "1m",
	}

	scaler := NewOFScaler(kubeClientset, cfg)

	if scaler.openfaasNamespace != "openfaas" {
		t.Errorf("Expected openfaasNamespace to be 'openfaas', got %s", scaler.openfaasNamespace)
	}
	if scaler.namespace != "default" {
		t.Errorf("Expected namespace to be 'default', got %s", scaler.namespace)
	}
	if scaler.gatewayEndpoint != "http://gateway.openfaas:8080" {
		t.Errorf("Expected gatewayEndpoint to be 'http://gateway.openfaas:8080', got %s", scaler.gatewayEndpoint)
	}
	if scaler.prometheusEndpoint != "http://prometheus.openfaas:9090" {
		t.Errorf("Expected prometheusEndpoint to be 'http://prometheus.openfaas:9090', got %s", scaler.prometheusEndpoint)
	}
}

func TestGetScalableFunctions(t *testing.T) {
	// Create a deployment with the label "com.openfaas.scale.zero" set to "true"
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-function",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"com.openfaas.scale.zero": "true",
					},
				},
			},
		},
		Status: appsv1.DeploymentStatus{
			Replicas: 1,
		},
	}

	kubeClientset := fake.NewSimpleClientset(deployment)
	scaler := &OpenfaasScaler{
		kubeClientset: kubeClientset,
		namespace:     "default",
	}

	functions, err := scaler.getScalableFunctions()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if len(functions) != 1 {
		t.Errorf("Expected 1 function, got %d", len(functions))
	}
	if functions[0] != "test-function" {
		t.Errorf("Expected function name to be 'test-function', got %s", functions[0])
	}
}

func TestScaleToZero(t *testing.T) {
	testsupport.SkipIfCannotListen(t)

	kubeClientset := fake.NewSimpleClientset()
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, hreq *http.Request) {
	}))

	scaler := &OpenfaasScaler{
		kubeClientset:   kubeClientset,
		gatewayEndpoint: server.URL,
	}

	err := scaler.scaleToZero("test-function", "user", "pass", server.Client())
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestIsIdle(t *testing.T) {
	testsupport.SkipIfCannotListen(t)

	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, hreq *http.Request) {
		if hreq.URL.Path == "/api/v1/query" {
			rw.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[{"metric":{},"value":[1620810000,"0"]}]},"error":null}`))
		}
	}))

	prometheusClient, _ := api.NewClient(api.Config{
		Address: server.URL,
	})
	prometheusAPIClient := v1.NewAPI(prometheusClient)

	idle := isIdle("test-function", "default", "5m", prometheusAPIClient)
	if !idle {
		t.Errorf("Expected function to be idle")
	}
}

func TestStart(t *testing.T) {
	testsupport.SkipIfCannotListen(t)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "basic-auth",
			Namespace: "openfaas",
		},
		Data: map[string][]byte{
			"basic-auth-user":     []byte("user"),
			"basic-auth-password": []byte("pass"),
		},
	}
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-function",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"com.openfaas.scale.zero": "true",
					},
				},
			},
		},
		Status: appsv1.DeploymentStatus{
			Replicas: 1,
		},
	}
	kubeClientset := fake.NewSimpleClientset(secret, deployment)
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, hreq *http.Request) {
		if hreq.URL.Path == "/api/v1/query" {
			rw.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[{"metric":{},"value":[1620810000,"1"]}]},"error":null}`))
		}
	}))

	cfg := &types.Config{
		OpenfaasNamespace:                "openfaas",
		ServicesNamespace:                "default",
		OpenfaasPort:                     8080,
		OpenfaasBasicAuthSecret:          "basic-auth",
		OpenfaasPrometheusPort:           9090,
		OpenfaasScalerInactivityDuration: "5m",
		OpenfaasScalerInterval:           "0.5s",
	}

	scaler := NewOFScaler(kubeClientset, cfg)
	scaler.gatewayEndpoint = server.URL
	scaler.prometheusEndpoint = server.URL

	var buf bytes.Buffer
	scalerLogger = log.New(&buf, "[OF-SCALER] ", log.Flags())

	go scaler.Start()
	time.Sleep(1 * time.Second)

	if buf.String() != "" {
		t.Errorf("Unexpected log output: %s", buf.String())
	}
}
