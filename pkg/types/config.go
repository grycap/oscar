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
	defaultMinioTLSVerify                   = true
	defaultMinIOEndpoint                    = "https://minio-service.minio:9000"
	defaultMinIORegion                      = "us-east-1"
	defaultTimeout                          = time.Duration(300) * time.Second
	defaultServiceName                      = "oscar"
	defaultServicePort                      = 8080
	defaultNamespace                        = "oscar"
	defaultServicesNamespace                = "oscar-svc"
	defaultOpenfaasNamespace                = "openfaas"
	defaultOpenfaasPort                     = 8080
	defaultOpenfaasBasicAuthSecret          = "basic-auth"
	defaultOpenfaasPrometheusPort           = 9090
	defaultOpenfaasScalerEnable             = false
	defaultOpenfaasScalerInterval           = "2m"
	defaultOpenfaasScalerInactivityDuration = "10m"
	defaultWatchdogMaxInflight              = 1
	defaultWatchdogWriteDebug               = true
	defaultWatchdogExecTimeout              = 0
	defaultWatchdogReadTimeout              = 300
	defaultWatchdogWriteTimeout             = 300
	defaultWatchdogHealthCheckInterval      = 5
	defaultYunikornEnable                   = false
	defaultYunikornNamespace                = "yunikorn"
	defaultYunikornConfigMap                = "yunikorn-configs"
	defaultYunikornConfigFileName           = "queues.yaml"
)

// Config stores the configuration for the OSCAR server
type Config struct {
	// MinIOProvider access info
	MinIOProvider *MinIOProvider `json:"minio_provider"`

	// Basic auth username
	Username string `json:"-"`

	// Basic auth password
	Password string `json:"-"`

	// Kubernetes name for the deployment and service (default: oscar)
	Name string `json:"name"`

	// Kubernetes namespace for the deployment and service (default: oscar)
	Namespace string `json:"namespace"`

	// Kubernetes namespace for services and jobs (default: oscar-svc)
	ServicesNamespace string `json:"services_namespace"`

	// Port used for the ClusterIP k8s service (default: 8080)
	ServicePort int `json:"-"`

	// Serverless framework used to deploy services (Openfaas | Knative)
	// If not defined only async invocations allowed (Using KubeBackend)
	ServerlessBackend string `json:"serverless_backend,omitempty"`

	// OpenfaasNamespace namespace where the OpenFaaS gateway is deployed
	OpenfaasNamespace string `json:"-"`

	// OpenfaasPort service port where the OpenFaaS gateway is exposed
	OpenfaasPort int `json:"-"`

	// OpenfaasBasicAuthSecret name of the secret used to store the OpenFaaS credentials
	OpenfaasBasicAuthSecret string `json:"-"`

	// OpenfaasPrometheusPort service port where the OpenFaaS' Prometheus is exposed
	OpenfaasPrometheusPort int `json:"-"`

	// OpenfaasScalerEnable option to enable the Openfaas scaler
	OpenfaasScalerEnable bool `json:"-"`

	// OpenfaasScalerInterval time interval to check if any function could be scaled
	OpenfaasScalerInterval string `json:"-"`

	// OpenfaasScalerInactivityDuration
	OpenfaasScalerInactivityDuration string `json:"-"`

	// WatchdogMaxInflight
	WatchdogMaxInflight int `json:"-"`

	// WatchdogWriteDebug
	WatchdogWriteDebug bool `json:"-"`

	// WatchdogExecTimeout
	WatchdogExecTimeout int `json:"-"`

	// WatchdogReadTimeout
	WatchdogReadTimeout int `json:"-"`

	// WatchdogWriteTimeout
	WatchdogWriteTimeout int `json:"-"`

	// WatchdogHealthCheckInterval
	WatchdogHealthCheckInterval int `json:"-"`

	// HTTP timeout for reading the payload (default: 300)
	ReadTimeout time.Duration `json:"-"`

	// HTTP timeout for writing the response (default: 300)
	WriteTimeout time.Duration `json:"-"`

	// YunikornEnable option to configure Apache Yunikorn
	YunikornEnable bool `json:"yunikorn_enable"`

	// YunikornNamespace
	YunikornNamespace string `json:"-"`

	// YunikornConfigMap
	YunikornConfigMap string `json:"-"`

	// YunikornConfigFileName
	YunikornConfigFileName string `json:"-"`
}

func parseSeconds(s string) (time.Duration, error) {
	if len(s) > 0 {
		parsed, err := strconv.Atoi(s)
		if err == nil && parsed > 0 {
			return time.Duration(parsed) * time.Second, nil
		}
	}
	return time.Duration(0), fmt.Errorf("the value must be a positive integer")
}

// ReadConfig reads environment variables to create the OSCAR server configuration
func ReadConfig() (*Config, error) {
	config := &Config{}
	config.MinIOProvider = &MinIOProvider{}
	var err error

	if len(os.Getenv("OSCAR_USERNAME")) > 0 {
		config.Username = os.Getenv("OSCAR_USERNAME")
	} else {
		return nil, fmt.Errorf("an OSCAR_USERNAME must be provided")
	}

	if len(os.Getenv("OSCAR_PASSWORD")) > 0 {
		config.Password = os.Getenv("OSCAR_PASSWORD")
	} else {
		return nil, fmt.Errorf("an OSCAR_PASSWORD must be provided")
	}

	if len(os.Getenv("MINIO_ACCESS_KEY")) > 0 {
		config.MinIOProvider.AccessKey = os.Getenv("MINIO_ACCESS_KEY")
	} else {
		return nil, fmt.Errorf("a MINIO_ACCESS_KEY must be provided")
	}

	if len(os.Getenv("MINIO_SECRET_KEY")) > 0 {
		config.MinIOProvider.SecretKey = os.Getenv("MINIO_SECRET_KEY")
	} else {
		return nil, fmt.Errorf("a MINIO_SECRET_KEY must be provided")
	}

	if len(os.Getenv("MINIO_REGION")) > 0 {
		config.MinIOProvider.Region = os.Getenv("MINIO_REGION")
	} else {
		config.MinIOProvider.Region = defaultMinIORegion
	}

	if len(os.Getenv("MINIO_TLS_VERIFY")) > 0 {
		config.MinIOProvider.Verify, err = strconv.ParseBool(os.Getenv("MINIO_TLS_VERIFY"))
		if err != nil {
			return nil, fmt.Errorf("the MINIO_TLS_VERIFY value must be a boolean")
		}
	} else {
		config.MinIOProvider.Verify = defaultMinioTLSVerify
	}

	if len(os.Getenv("MINIO_ENDPOINT")) > 0 {
		config.MinIOProvider.Endpoint = os.Getenv("MINIO_ENDPOINT")
		if _, err = url.Parse(config.MinIOProvider.Endpoint); err != nil {
			return nil, fmt.Errorf("the MINIO_ENDPOINT value is not valid. Error: %v", err)
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
			return nil, fmt.Errorf("the SERVERLESS_BACKEND is not valid. Must be \"Openfaas\" or \"Knative\"")
		}
	}

	if config.ServerlessBackend == "openfaas" {
		if len(os.Getenv("OPENFAAS_NAMESPACE")) > 0 {
			config.OpenfaasNamespace = strings.ToLower(os.Getenv("OPENFAAS_NAMESPACE"))
		} else {
			config.OpenfaasNamespace = defaultOpenfaasNamespace
		}

		if len(os.Getenv("OPENFAAS_PORT")) > 0 {
			config.OpenfaasPort, err = strconv.Atoi(os.Getenv("OPENFAAS_PORT"))
			if err != nil {
				return nil, fmt.Errorf("the OPENFAAS_PORT value is not valid. Error: %v", err)
			}
		} else {
			config.OpenfaasPort = defaultOpenfaasPort
		}

		if len(os.Getenv("OPENFAAS_BASIC_AUTH_SECRET")) > 0 {
			config.OpenfaasBasicAuthSecret = os.Getenv("OPENFAAS_BASIC_AUTH_SECRET")
		} else {
			config.OpenfaasBasicAuthSecret = defaultOpenfaasBasicAuthSecret
		}

		if len(os.Getenv("OPENFAAS_PROMETHEUS_PORT")) > 0 {
			config.OpenfaasPrometheusPort, err = strconv.Atoi(os.Getenv("OPENFAAS_PROMETHEUS_PORT"))
			if err != nil {
				return nil, fmt.Errorf("the OPENFAAS_PORT value is not valid. Error: %v", err)
			}
		} else {
			config.OpenfaasPrometheusPort = defaultOpenfaasPrometheusPort
		}

		if len(os.Getenv("OPENFAAS_SCALER_ENABLE")) > 0 {
			config.OpenfaasScalerEnable, err = strconv.ParseBool(os.Getenv("OPENFAAS_SCALER_ENABLE"))
			if err != nil {
				return nil, fmt.Errorf("the OPENFAAS_SCALER_ENABLE value must be a boolean")
			}
		} else {
			config.OpenfaasScalerEnable = defaultOpenfaasScalerEnable
		}

		if len(os.Getenv("OPENFAAS_SCALER_INTERVAL")) > 0 {
			config.OpenfaasScalerInterval = os.Getenv("OPENFAAS_SCALER_INTERVAL")
		} else {
			config.OpenfaasScalerInterval = defaultOpenfaasScalerInterval
		}

		if len(os.Getenv("OPENFAAS_SCALER_INACTIVITY_DURATION")) > 0 {
			config.OpenfaasScalerInactivityDuration = os.Getenv("OPENFAAS_SCALER_INACTIVITY_DURATION")
		} else {
			config.OpenfaasScalerInactivityDuration = defaultOpenfaasScalerInactivityDuration
		}
	}

	if len(os.Getenv("WATCHDOG_MAX_INFLIGHT")) > 0 {
		config.WatchdogMaxInflight, err = strconv.Atoi(os.Getenv("WATCHDOG_MAX_INFLIGHT"))
		if err != nil {
			return nil, fmt.Errorf("the WATCHDOG_MAX_INFLIGHT value is not valid. Error: %v", err)
		}
	} else {
		config.WatchdogMaxInflight = defaultWatchdogMaxInflight
	}

	if len(os.Getenv("WATCHDOG_WRITE_DEBUG")) > 0 {
		config.WatchdogWriteDebug, err = strconv.ParseBool(os.Getenv("WATCHDOG_WRITE_DEBUG"))
		if err != nil {
			return nil, fmt.Errorf("the WATCHDOG_WRITE_DEBUG value must be a boolean")
		}
	} else {
		config.WatchdogWriteDebug = defaultWatchdogWriteDebug
	}

	if len(os.Getenv("WATCHDOG_EXEC_TIMEOUT")) > 0 {
		config.WatchdogExecTimeout, err = strconv.Atoi(os.Getenv("WATCHDOG_EXEC_TIMEOUT"))
		if err != nil {
			return nil, fmt.Errorf("the WATCHDOG_EXEC_TIMEOUT value is not valid. Error: %v", err)
		}
	} else {
		config.WatchdogExecTimeout = defaultWatchdogExecTimeout
	}

	if len(os.Getenv("WATCHDOG_READ_TIMEOUT")) > 0 {
		config.WatchdogReadTimeout, err = strconv.Atoi(os.Getenv("WATCHDOG_READ_TIMEOUT"))
		if err != nil {
			return nil, fmt.Errorf("the WATCHDOG_READ_TIMEOUT value is not valid. Error: %v", err)
		}
	} else {
		config.WatchdogReadTimeout = defaultWatchdogReadTimeout
	}

	if len(os.Getenv("WATCHDOG_WRITE_TIMEOUT")) > 0 {
		config.WatchdogWriteTimeout, err = strconv.Atoi(os.Getenv("WATCHDOG_WRITE_TIMEOUT"))
		if err != nil {
			return nil, fmt.Errorf("the WATCHDOG_WRITE_TIMEOUT value is not valid. Error: %v", err)
		}
	} else {
		config.WatchdogWriteTimeout = defaultWatchdogWriteTimeout
	}

	if len(os.Getenv("WATCHDOG_HEALTHCHECK_INTERVAL")) > 0 {
		config.WatchdogHealthCheckInterval, err = strconv.Atoi(os.Getenv("WATCHDOG_HEALTHCHECK_INTERVAL"))
		if err != nil {
			return nil, fmt.Errorf("the WATCHDOG_HEALTHCHECK_INTERVAL value is not valid. Error: %v", err)
		}
	} else {
		config.WatchdogHealthCheckInterval = defaultWatchdogHealthCheckInterval
	}

	if len(os.Getenv("READ_TIMEOUT")) > 0 {
		config.ReadTimeout, err = parseSeconds(os.Getenv("READ_TIMEOUT"))
		if err != nil {
			return nil, fmt.Errorf("the READ_TIMEOUT value is not valid. Error: %v", err)
		}
	} else {
		config.ReadTimeout = defaultTimeout
	}

	if len(os.Getenv("WRITE_TIMEOUT")) > 0 {
		config.WriteTimeout, err = parseSeconds(os.Getenv("WRITE_TIMEOUT"))
		if err != nil {
			return nil, fmt.Errorf("the WRITE_TIMEOUT value is not valid. Error: %v", err)
		}
	} else {
		config.WriteTimeout = defaultTimeout
	}

	if len(os.Getenv("OSCAR_SERVICE_PORT")) > 0 {
		config.ServicePort, err = strconv.Atoi(os.Getenv("OSCAR_SERVICE_PORT"))
		if err != nil {
			return nil, fmt.Errorf("the OSCAR_SERVICE_PORT value is not valid. Error: %v", err)
		}
	} else {
		config.ServicePort = defaultServicePort
	}

	if len(os.Getenv("YUNIKORN_ENABLE")) > 0 {
		config.YunikornEnable, err = strconv.ParseBool(os.Getenv("YUNIKORN_ENABLE"))
		if err != nil {
			return nil, fmt.Errorf("the YUNIKORN_ENABLE value must be a boolean")
		}
	} else {
		config.YunikornEnable = defaultYunikornEnable
	}

	if len(os.Getenv("YUNIKORN_NAMESPACE")) > 0 {
		config.YunikornNamespace = os.Getenv("YUNIKORN_NAMESPACE")
	} else {
		config.YunikornNamespace = defaultYunikornNamespace
	}

	if len(os.Getenv("YUNIKORN_CONFIGMAP")) > 0 {
		config.YunikornConfigMap = os.Getenv("YUNIKORN_CONFIGMAP")
	} else {
		config.YunikornConfigMap = defaultYunikornConfigMap
	}

	if len(os.Getenv("YUNIKORN_CONFIG_FILENAME")) > 0 {
		config.YunikornConfigFileName = os.Getenv("YUNIKORN_CONFIG_FILENAME")
	} else {
		config.YunikornConfigFileName = defaultYunikornConfigFileName
	}

	return config, nil
}
