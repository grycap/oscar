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
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/grycap/oscar/v2/pkg/types"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const scalePath = "/system/scale-function/"

var scalerLogger = log.New(os.Stdout, "[OF-SCALER] ", log.Flags())

// OpenfaasScaler struct to store the parameters required to scale OpenFaaS functions
type OpenfaasScaler struct {
	kubeClientset           kubernetes.Interface
	openfaasNamespace       string
	namespace               string
	gatewayEndpoint         string
	openfaasBasicAuthSecret string
	prometheusEndpoint      string
	inactivityDuration      string
	reconcileInterval       string
}

// NewOFScaler returns a pointer to a new OpenfaasScaler struct
func NewOFScaler(kubeClientset kubernetes.Interface, cfg *types.Config) *OpenfaasScaler {
	return &OpenfaasScaler{
		kubeClientset:           kubeClientset,
		openfaasNamespace:       cfg.OpenfaasNamespace,
		namespace:               cfg.ServicesNamespace,
		gatewayEndpoint:         fmt.Sprintf("http://gateway.%s:%d", cfg.OpenfaasNamespace, cfg.OpenfaasPort),
		openfaasBasicAuthSecret: cfg.OpenfaasBasicAuthSecret,
		prometheusEndpoint:      fmt.Sprintf("http://prometheus.%s:%d", cfg.OpenfaasNamespace, cfg.OpenfaasPrometheusPort),
		inactivityDuration:      cfg.OpenfaasScalerInactivityDuration,
		reconcileInterval:       cfg.OpenfaasScalerInterval,
	}
}

// Start starts the OpenFaaS scaler
func (ofs *OpenfaasScaler) Start() {
	// Retrieve the basic auth credentials from secret
	var basicAuthUser, basicAuthPass string
	secret, err := ofs.kubeClientset.CoreV1().Secrets(ofs.openfaasNamespace).Get(context.TODO(), ofs.openfaasBasicAuthSecret, metav1.GetOptions{})
	if err != nil {
		scalerLogger.Println("Unable to retrieve the OpenFaaS basic auth secret")
		return
	}
	basicAuthUser = string(secret.Data["basic-auth-user"])
	basicAuthPass = string(secret.Data["basic-auth-password"])

	// Parse the OPENFAAS_SCALER_INTERVAL parameter
	reconcileInterval, err := time.ParseDuration(ofs.reconcileInterval)
	if err != nil {
		scalerLogger.Println("Invalid OPENFAAS_SCALER_INTERVAL value")
		return
	}

	// Make prometheus API client
	prometheusClient, err := api.NewClient(api.Config{
		Address: ofs.prometheusEndpoint,
	})
	if err != nil {
		scalerLogger.Println("Unable to create the prometheus client")
		return
	}

	prometheusAPIClient := v1.NewAPI(prometheusClient)

	// Make HTTP gateway client (with basic auth)
	gatewayClient := &http.Client{
		Timeout: 15 * time.Second,
	}

	// Start the loop
	for {
		// Sleep the reconcileInterval
		time.Sleep(reconcileInterval)

		// Get all scalable functions
		functionNames, err := ofs.getScalableFunctions()
		if err != nil {
			scalerLogger.Println(err.Error())
			continue
		}

		if len(functionNames) == 0 {
			scalerLogger.Println("There are no functions to scale")
			continue
		}

		for _, functionName := range functionNames {
			if isIdle(functionName, ofs.namespace, ofs.inactivityDuration, prometheusAPIClient) {
				// Scale to zero
				err := ofs.scaleToZero(functionName, basicAuthUser, basicAuthPass, gatewayClient)
				if err != nil {
					scalerLogger.Printf("Error scaling function \"%s\": %v\n", functionName, err)
				} else {
					scalerLogger.Printf("Function \"%s\" scaled down to zero\n", functionName)
				}
			}
		}
	}

}

func (ofs *OpenfaasScaler) getScalableFunctions() ([]string, error) {
	// Get the deployment list
	deployments, err := ofs.kubeClientset.AppsV1().Deployments(ofs.namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var functionNames []string
	for _, deploy := range deployments.Items {
		if val, ok := deploy.Spec.Template.Labels["com.openfaas.scale.zero"]; ok {
			if val == "true" && deploy.Status.Replicas != 0 {
				functionNames = append(functionNames, deploy.Name)
			}
		}
	}

	return functionNames, nil
}

func isIdle(functionName string, namespace string, inactivityDuration string, prometheusAPIClient v1.API) bool {
	if len(functionName) == 0 {
		return false
	}

	// Format the query
	query := fmt.Sprintf("rate(gateway_function_invocation_total{function_name=\"%s.%s\"}[%s])", functionName, namespace, inactivityDuration)

	// Make context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Make the query
	result, warnings, err := prometheusAPIClient.Query(ctx, query, time.Now())
	if err != nil {
		scalerLogger.Printf("Error querying prometheus API with function \"%s\"\n", functionName)
		return false
	}
	if len(warnings) > 0 {
		for _, warning := range warnings {
			scalerLogger.Println(warning)
		}
	}

	// Check the result
	var resultString string
	if len(result.String()) == 0 {
		// If the result is an empty string the function has never been invoked, so it can be scaled to zero
		return true
	}
	// Get the invocation rate from the result string
	// Example of a result:
	// {app="gateway", code="200", function_name="figlet.oscar-svc", instance="10.244.1.6:8082", job="kubernetes-pods", kubernetes_namespace="openfaas", kubernetes_pod_name="gateway-69d9bdc47d-l4rf2", pod_template_hash="69d9bdc47d"} => 0 @[1623424600.347]
	split := strings.SplitN(result.String(), "=> ", 2)
	if len(split) == 2 {
		resultString = split[1]
	}
	split = strings.SplitN(resultString, " @", 2)
	if len(split) == 2 {
		resultString = split[0]
	}

	if resultString == "0" {
		// If the result is "0" the function can be scaled to zero
		return true
	}

	return false
}

func (ofs *OpenfaasScaler) scaleToZero(functionName, basicAuthUser, basicAuthPass string, gatewayClient *http.Client) error {
	// JSON body for scaling to zero in the gateway
	jsonBody := []byte("{\"replicas\": 0}")

	url := ofs.gatewayEndpoint + scalePath + functionName

	// Make the request
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return err
	}
	// Set basic auth
	req.SetBasicAuth(basicAuthUser, basicAuthPass)

	// Set the request
	res, err := gatewayClient.Do(req)
	if err != nil {
		return err
	}

	if res.StatusCode != 200 && res.StatusCode != 202 {
		return fmt.Errorf("status code \"%d\"", res.StatusCode)
	}

	return nil
}
