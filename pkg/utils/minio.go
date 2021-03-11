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
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/grycap/oscar/v2/pkg/types"
	"github.com/minio/minio/pkg/madmin"
)

// MinIOAdminClient struct to represent a MinIO Admin client to configure webhook notifications
type MinIOAdminClient struct {
	adminClient   *madmin.AdminClient
	oscarEndpoint *url.URL
}

// MakeMinIOAdminClient creates a new MinIO Admin client to configure webhook notifications
func MakeMinIOAdminClient(cfg *types.Config) (*MinIOAdminClient, error) {
	// Parse minIO endpoint
	endpointURL, err := url.Parse(cfg.MinIOProvider.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("The provided MinIO endpoint \"%s\" is not valid", cfg.MinIOProvider.Endpoint)
	}

	// Check URL Scheme for using TLS or not
	var enableTLS bool
	switch endpointURL.Scheme {
	case "http":
		enableTLS = false
	case "https":
		enableTLS = true
	default:
		return nil, fmt.Errorf("Invalid MinIO Endpoint: %s. Must start with \"http://\" or \"https://\"", cfg.MinIOProvider.Endpoint)
	}

	adminClient, err := madmin.New(endpointURL.Host, cfg.MinIOProvider.AccessKey, cfg.MinIOProvider.SecretKey, enableTLS)
	if err != nil {
		return nil, err
	}

	// Disable tls verification in client transport if verify == false
	if !cfg.MinIOProvider.Verify {
		tr := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		adminClient.SetCustomTransport(tr)
	}

	oscarEndpoint, err := url.Parse(fmt.Sprintf("http://%s.%s:%d", cfg.Name, cfg.Namespace, cfg.ServicePort))
	if err != nil {
		return nil, err
	}

	minIOAdminClient := &MinIOAdminClient{
		adminClient:   adminClient,
		oscarEndpoint: oscarEndpoint,
	}

	return minIOAdminClient, nil
}

// RegisterWebhook registers a new webhook in the MinIO configuration
func (minIOAdminClient *MinIOAdminClient) RegisterWebhook(name string) (restarted bool, err error) {
	r, err := minIOAdminClient.adminClient.SetConfigKV(context.TODO(), fmt.Sprintf("notify_webhook:%s endpoint=%s/job/%s", name, minIOAdminClient.oscarEndpoint.String(), name))
	if err != nil {
		return false, err
	}
	return r, nil
}

// RemoveWebhook removes an existent webhook in the MinIO configuration
func (minIOAdminClient *MinIOAdminClient) RemoveWebhook(name string) error {
	err := minIOAdminClient.adminClient.DelConfigKV(context.TODO(), fmt.Sprintf("notify_webhook:%s", name))
	if err != nil {
		return err
	}
	return nil
}

// RestartServer restarts a MinIO server to apply the configuration changes
func (minIOAdminClient *MinIOAdminClient) RestartServer() error {
	err := minIOAdminClient.adminClient.ServiceRestart(context.TODO())
	if err != nil {
		return err
	}

	// Max. time taken by the server to shutdown is 5 seconds.
	// This can happen when there are lot of s3 requests pending when the server
	// receives a restart command.
	// Sleep for 6 seconds and then check if the server is online.
	time.Sleep(6 * time.Second)
	_, err = minIOAdminClient.adminClient.ServerInfo(context.TODO())
	if err != nil {
		return fmt.Errorf("Error restarting the MinIO server: %v", err)
	}

	return nil
}
