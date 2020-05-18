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
	defaultMinioTLSVerify    = true
	defaultMinIOEndpoint     = "https://minio-service.minio:9000"
	defaultMinIORegion       = "us-east-1"
	defaultTimeout           = time.Duration(300) * time.Second
	defaultServiceName       = "oscar"
	defaultServicePort       = 8080
	defaultNamespace         = "oscar"
	defaultServicesNamespace = "oscar-svc"
)

// Config stores the configuration for the OSCAR server
type Config struct {
	// MinIOProvider access info
	MinIOProvider *MinIOProvider

	// Basic auth username
	Username string

	// Basic auth password
	Password string

	// Kubernetes name for the deployment and service (default: oscar)
	Name string

	// Kubernetes namespace for the deployment and service (default: oscar)
	Namespace string

	// Kubernetes namespace for services and jobs (default: oscar-svc)
	ServicesNamespace string

	// Port used for the ClusterIP k8s service (default: 8080)
	ServicePort int

	// Serverless framework used to deploy services (Openfaas | Knative)
	// If not defined only async invokations allowed (Using KubeBackend)
	ServerlessBackend string

	// HTTP timeout for reading the payload (default: 300)
	ReadTimeout time.Duration

	// HTTP timeout for writing the response (default: 300)
	WriteTimeout time.Duration
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
	config := &Config{}
	config.MinIOProvider = &MinIOProvider{}
	var err error

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

	if len(os.Getenv("MINIO_ACCESS_KEY")) > 0 {
		config.MinIOProvider.AccessKey = os.Getenv("MINIO_ACCESS_KEY")
	} else {
		return nil, fmt.Errorf("A MINIO_ACCESS_KEY must be provided")
	}

	if len(os.Getenv("MINIO_SECRET_KEY")) > 0 {
		config.MinIOProvider.SecretKey = os.Getenv("MINIO_SECRET_KEY")
	} else {
		return nil, fmt.Errorf("A MINIO_SECRET_KEY must be provided")
	}

	if len(os.Getenv("MINIO_REGION")) > 0 {
		config.MinIOProvider.Region = os.Getenv("MINIO_REGION")
	} else {
		config.MinIOProvider.Region = defaultMinIORegion
	}

	if len(os.Getenv("MINIO_TLS_VERIFY")) > 0 {
		config.MinIOProvider.Verify, err = strconv.ParseBool(os.Getenv("MINIO_TLS_VERIFY"))
		if err != nil {
			return nil, fmt.Errorf("The MINIO_TLS_VERIFY value must be a boolean")
		}
	} else {
		config.MinIOProvider.Verify = defaultMinioTLSVerify
	}

	if len(os.Getenv("MINIO_ENDPOINT")) > 0 {
		config.MinIOProvider.Endpoint = os.Getenv("MINIO_ENDPOINT")
		if _, err = url.Parse(config.MinIOProvider.Endpoint); err != nil {
			return nil, fmt.Errorf("The MINIO_ENDPOINT value is not valid. Error: %s", err)
		}
	} else {
		config.MinIOProvider.Endpoint = defaultMinIOEndpoint
	}

	if len(os.Getenv("OSCAR_NAME")) > 0 {
		config.Name = os.Getenv("OSCAR_NAME")
	} else {
		config.Name = defaultServiceName
	}

	if len(os.Getenv("OSCAR_NAMESPACE")) > 0 {
		config.Namespace = os.Getenv("OSCAR_NAMESPACE")
	} else {
		config.Namespace = defaultNamespace
	}

	if len(os.Getenv("OSCAR_SERVICES_NAMESPACE")) > 0 {
		config.ServicesNamespace = os.Getenv("OSCAR_SERVICES_NAMESPACE")
	} else {
		config.ServicesNamespace = defaultServicesNamespace
	}

	if len(os.Getenv("SERVERLESS_BACKEND")) > 0 {
		config.ServerlessBackend = strings.ToLower(os.Getenv("SERVERLESS_BACKEND"))
		if config.ServerlessBackend != "openfaas" && config.ServerlessBackend != "knative" {
			return nil, fmt.Errorf("The SERVERLESS_BACKEND is not valid. Must be \"Openfaas\" or \"Knative\"")
		}
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

	if len(os.Getenv("OSCAR_PORT")) > 0 {
		config.ServicePort, err = strconv.Atoi(os.Getenv("OSCAR_PORT"))
		if err != nil {
			return nil, fmt.Errorf("The OSCAR_PORT value is not valid. Error: %s", err)
		}
	} else {
		config.ServicePort = defaultServicePort
	}

	return config, nil
}
