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

	"github.com/grycap/oscar/v3/pkg/types"
	"github.com/minio/madmin-go"
)

const ALL_USERS_GROUP = "all_users_group"

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
		return nil, fmt.Errorf("the provided MinIO endpoint \"%s\" is not valid", cfg.MinIOProvider.Endpoint)
	}

	// Check URL Scheme for using TLS or not
	var enableTLS bool
	switch endpointURL.Scheme {
	case "http":
		enableTLS = false
	case "https":
		enableTLS = true
	default:
		return nil, fmt.Errorf("invalid MinIO Endpoint: %s. Must start with \"http://\" or \"https://\"", cfg.MinIOProvider.Endpoint)
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

// CreateMinIOUser creates a new user for multitenancy
func (minIOAdminClient *MinIOAdminClient) CreateMinIOUser(ak string, sk string) error {
	var users []string
	err := minIOAdminClient.adminClient.AddUser(context.TODO(), ak, sk)
	if err != nil {
		return fmt.Errorf("error creating MinIO user: %v", err)
	}

	users = append(users, ak)
	err2 := minIOAdminClient.AddUserToGroup(users, ALL_USERS_GROUP)
	if err2 != nil {
		return err2
	}
	return nil
}

// CreateAllUsersGroup creates a group used for public services
func (minIOAdminClient *MinIOAdminClient) CreateAllUsersGroup() error {
	err := createGroup(minIOAdminClient.adminClient, ALL_USERS_GROUP)
	if err != nil {
		return err
	}
	return nil
}

// CreateServiceGroup creates a MinIO group and its associated policy for a service
func (minIOAdminClient *MinIOAdminClient) CreateServiceGroup(bucketName string) error {
	err := createGroup(minIOAdminClient.adminClient, bucketName)
	if err != nil {
		return err
	}

	err = createPolicy(minIOAdminClient.adminClient, bucketName, false)
	if err != nil {
		return err
	}

	return nil
}

// AddServiceToAllUsersGroup associates policy of all users to a service
func (minIOAdminClient *MinIOAdminClient) AddServiceToAllUsersGroup(bucketName string) error {
	err := createPolicy(minIOAdminClient.adminClient, bucketName, true)
	if err != nil {
		return err
	}

	return nil
}

// AddUserToGroup adds  user/users to a group
func (minIOAdminClient *MinIOAdminClient) AddUserToGroup(users []string, groupName string) error {
	group := madmin.GroupAddRemove{
		Group:    groupName,
		Members:  users,
		Status:   "enable",
		IsRemove: false,
	}

	err := minIOAdminClient.adminClient.UpdateGroupMembers(context.TODO(), group)
	if err != nil {
		return fmt.Errorf("error adding users to group: %v", err)
	}

	return nil
}

// DeleteServiceGroup empty the service group and policy
func (minIOAdminClient *MinIOAdminClient) DeleteServiceGroup(groupName string) error {
	description, err := minIOAdminClient.adminClient.GetGroupDescription(context.Background(), groupName)
	if err != nil {
		return err
	}
	group := madmin.GroupAddRemove{
		Group:    groupName,
		Members:  description.Members,
		Status:   "enable",
		IsRemove: true,
	}

	err = minIOAdminClient.adminClient.UpdateGroupMembers(context.Background(), group)
	if err != nil {
		return fmt.Errorf("Error emptying group: %v", err)
	}

	err = minIOAdminClient.adminClient.RemoveCannedPolicy(context.TODO(), groupName)
	if err != nil {
		return fmt.Errorf("Error removing group's policy: %v", err)
	}
	return nil
}

// RegisterWebhook registers a new webhook in the MinIO configuration
func (minIOAdminClient *MinIOAdminClient) RegisterWebhook(name string, token string) error {
	_, err := minIOAdminClient.adminClient.SetConfigKV(context.TODO(), fmt.Sprintf("notify_webhook:%s endpoint=%s/job/%s auth_token=%s", name, minIOAdminClient.oscarEndpoint.String(), name, token))
	if err != nil {
		return err
	}
	return nil
}

// RemoveWebhook removes an existent webhook in the MinIO configuration
func (minIOAdminClient *MinIOAdminClient) RemoveWebhook(name string) error {
	_, err := minIOAdminClient.adminClient.DelConfigKV(context.TODO(), fmt.Sprintf("notify_webhook:%s", name))
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
		return fmt.Errorf("error restarting the MinIO server: %v", err)
	}

	return nil
}

func createPolicy(adminClient *madmin.AdminClient, bucketName string, allUsers bool) error {

	var groupName string
	if allUsers {
		groupName = ALL_USERS_GROUP
	} else {
		groupName = bucketName
	}

	policy := `{
		"Version": "2012-10-17",
		"Statement": [
			{
				"Effect": "Allow",
				"Action": [
					"s3:*"
				],
				"Resource": [
					"arn:aws:s3:::` + bucketName + `*"
				]
			}
		]
	}`

	err := adminClient.AddCannedPolicy(context.TODO(), groupName, []byte(policy))
	if err != nil {
		return fmt.Errorf("error creating MinIO policy for group %s: %v", groupName, err)
	}

	err = adminClient.SetPolicy(context.TODO(), groupName, groupName, true)
	if err != nil {
		return fmt.Errorf("error setting MinIO policy for group %s: %v", groupName, err)
	}
	return nil
}

func createGroup(adminClient *madmin.AdminClient, groupName string) error {
	group := madmin.GroupAddRemove{
		Group:    groupName,
		Members:  []string{},
		Status:   "enable",
		IsRemove: false,
	}
	err := adminClient.UpdateGroupMembers(context.TODO(), group)
	if err != nil {
		return fmt.Errorf("error creating MinIO group %s: %v", groupName, err)
	}

	return nil
}
