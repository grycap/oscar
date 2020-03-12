// Copyright (C) GRyCAP - I3M - UPV
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package types

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultMinioTLSVerify          = true
	defaultMinIOEndpoint           = "https://minio-service.minio:9000"
	defaultMinIORegion             = "us-east-1"
	defaultOpenfaasGatewayEndpoint = "http://gateway.openfaas:8080"
	defaultTimeout                 = time.Duration(300) * time.Second
	defaultServiceName             = "oscar"
	defaultServicePort             = 8080
	defaultNamespace               = "oscar"
)

// Config stores the configuration for the OSCAR server
type Config struct {
	// MinIO access key
	MinIOAccessKey string

	// MinIO secret key
	MinIOSecretKey string

	// Enable TLS verification for MinIO server (default: true)
	MinIOTLSVerify bool

	// MinIO server endpoint (default: https://minio-service.minio:9000)
	MinIOEndpoint *url.URL

	// MinIO region
	MinIORegion string

	// OpenFaaS gateway basic auth user
	OpenfaasGatewayUsername string

	// OpenFaaS gateway basic auth password
	OpenfaasGatewayPassword string

	// OpenFaaS gateway endpoint (default: http://gateway.openfaas:8080)
	OpenfaasGatewayEndpoint *url.URL

	// Basic auth username
	Username string

	// Basic auth password
	Password string

	// Kubernetes name for the deployment and service (default: oscar)
	Name string

	// Kubernetes namespace for services and jobs (default: oscar)
	Namespace string

	// Port used for the ClusterIP k8s service
	ServicePort int

	// Use a Serverless framework to support sync invocations (default: false)
	EnableServerlessBackend bool

	// Serverless framework used to deploy services (Openfaas | Knative)
	ServerlessBackend string

	// HTTP timeout for reading the payload (default: 300)
	ReadTimeout time.Duration

	// HTTP timeout for writing the response (default: 300)
	WriteTimeout time.Duration
}

func parseBool(s string) (bool, error) {
	if strings.ToLower(s) == "true" {
		return true, nil
	} else if strings.ToLower(s) == "false" {
		return false, nil
	}
	return false, fmt.Errorf("The value must be a boolean")
}

func parseSeconds(s string) (time.Duration, error) {
	if len(s) > 0 {
		parsed, err := strconv.Atoi(s)
		if err == nil && parsed > 0 {
			return time.Duration(parsed) * time.Second, nil
		}
	}
	return time.Duration(0), fmt.Errorf("The value must be a positive integer")
}

// ReadConfig reads environment variables to create the OSCAR server configuration
func ReadConfig() (*Config, error) {
	// TODO: check if serverless backend is enabled.. if it is check ServerlessBackend names
	var config Config
	var err error

	if len(os.Getenv("MINIO_ACCESS_KEY")) > 0 {
		config.MinIOAccessKey = os.Getenv("MINIO_ACCESS_KEY")
	} else {
		return nil, fmt.Errorf("A MINIO_ACCESS_KEY must be provided")
	}

	if len(os.Getenv("MINIO_SECRET_KEY")) > 0 {
		config.MinIOSecretKey = os.Getenv("MINIO_SECRET_KEY")
	} else {
		return nil, fmt.Errorf("A MINIO_SECRET_KEY must be provided")
	}

	if len(os.Getenv("MINIO_TLS_VERIFY")) > 0 {
		config.MinIOTLSVerify, err = parseBool(os.Getenv("MINIO_SECRET_KEY"))
		if err != nil {
			return nil, fmt.Errorf("The MINIO_TLS_VERIFY value must be a boolean")
		}
	} else {
		config.MinIOTLSVerify = defaultMinioTLSVerify
	}

	if len(os.Getenv("MINIO_ENDPOINT")) > 0 {
		config.MinIOEndpoint, err = url.Parse(os.Getenv("MINIO_ENDPOINT"))
		if err != nil {
			return nil, fmt.Errorf("The MINIO_ENDPOINT \"%s\" is not valid. Error: %s", os.Getenv("MINIO_ENDPOINT"), err)
		}
	} else {
		config.MinIOEndpoint, _ = url.Parse(defaultMinIOEndpoint)
	}

	if len(os.Getenv("OPENFAAS_GATEWAY_ENDPOINT")) > 0 {
		config.OpenfaasGatewayEndpoint, err = url.Parse(os.Getenv("OPENFAAS_GATEWAY_ENDPOINT"))
		if err != nil {
			return nil, fmt.Errorf("The OPENFAAS_GATEWAY_ENDPOINT \"%s\" is not valid. Error: %s", os.Getenv("OPENFAAS_GATEWAY_ENDPOINT"), err)
		}
	} else {
		config.OpenfaasGatewayEndpoint, _ = url.Parse(defaultOpenfaasGatewayEndpoint)
	}

	if len(os.Getenv("OSCAR_USERNAME")) > 0 {
		config.Username = os.Getenv("OSCAR_USERNAME")
	} else {
		return nil, fmt.Errorf("An OSCAR_USERNAME must be provided")
	}

	if len(os.Getenv("OSCAR_PASSWORD")) > 0 {
		config.Password = os.Getenv("OSCAR_PASSWORD")
	} else {
		return nil, fmt.Errorf("An OSCAR_PASSWORD must be provided")
	}

	if len(os.Getenv("OSCAR_NAMESPACE")) > 0 {
		config.Namespace = os.Getenv("OSCAR_NAMESPACE")
	} else {
		return nil, fmt.Errorf("An OSCAR_NAMESPACE must be provided")
	}

	if len(os.Getenv("SERVERLESS_BACKEND")) > 0 {
		config.ServerlessBackend = strings.ToLower(os.Getenv("SERVERLESS_BACKEND"))
		if config.ServerlessBackend != "openfaas" && config.ServerlessBackend != "knative" {
			return nil, fmt.Errorf("The SERVERLESS_BACKEND is not valid. Must be \"Openfaas\" or \"Knative\"")
		}
	} else {
		return nil, fmt.Errorf("A SERVERLESS_BACKEND (Openfaas or Knative) must be provided")
	}

	if len(os.Getenv("READ_TIMEOUT")) > 0 {
		config.ReadTimeout, err = parseSeconds(os.Getenv("READ_TIMEOUT"))
		if err != nil {
			return nil, fmt.Errorf("The READ_TIMEOUT value is not valid. Error: %s", err)
		}
	} else {
		config.ReadTimeout = defaultTimeout
	}

	if len(os.Getenv("WRITE_TIMEOUT")) > 0 {
		config.WriteTimeout, err = parseSeconds(os.Getenv("WRITE_TIMEOUT"))
		if err != nil {
			return nil, fmt.Errorf("The WRITE_TIMEOUT value is not valid. Error: %s", err)
		}
	} else {
		config.WriteTimeout = defaultTimeout
	}

	return &config, nil
}
