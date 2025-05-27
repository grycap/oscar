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
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/grycap/oscar/v3/pkg/types"
	"github.com/minio/madmin-go"
)

const (
	ALL_USERS_GROUP = "all_users_group"
	PRIVATE         = "private"
	RESTRICTED      = "restricted"
	PUBLIC          = "public"
)

var (
	ALL_ACTIONS        = []string{"s3:*"}
	RESTRICTED_ACTIONS = []string{"s3:ListBucket", "s3:GetObject", "s3:PutObject", "s3:DeleteObject"}
)

var minioLogger = log.New(os.Stdout, "[MINIO] ", log.Flags())
var overlappingError = "An object key name filtering rule defined with overlapping prefixes"

// MinIOAdminClient struct to represent a MinIO Admin client to configure webhook notifications
type MinIOAdminClient struct {
	adminClient   *madmin.AdminClient
	oscarEndpoint *url.URL
}

// MinIOBucket definition to create buckets independent of a service
// Note: BucketPath refers to bucket name
type MinIOBucket struct {
	BucketPath   string   `json:"bucket_path"`
	Visibility   string   `json:"visibility"`
	AllowedUsers []string `json:"allowed_users"`
	Owner        string   `json:"owner"`
}

// Define the policy structure using Go structs
type Statement struct {
	Effect   string   `json:"Effect"`
	Action   []string `json:"Action"`
	Resource []string `json:"Resource"`
}

type Policy struct {
	Version   string      `json:"Version"`
	Statement []Statement `json:"Statement"`
}

type ServicePolicy struct {
	Type         string   `json:"Type"`
	UIDS         []string `json:"UIDS"`
	UpdatePolicy bool     `json:"UpdatePolicy"`
}

func getPolicyDefinition(actions []string, resource string) *Policy {
	return &Policy{
		Version: "2012-10-17",
		Statement: []Statement{
			{
				Resource: []string{resource},
				Action:   actions,
				Effect:   "Allow",
			},
		},
	}
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
			// #nosec
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
	err2 := minIOAdminClient.CreateAddGroup(ALL_USERS_GROUP, users, false)
	if err2 != nil {
		return err2
	}
	return nil
}

// CreateS3PathWithWebhook Creates a bucket and its paths and enables the associated webhook
func (minIOAdminClient *MinIOAdminClient) CreateS3PathWithWebhook(s3Client *s3.S3, path []string, arn string, bucketExists bool) error {
	bucketKey := path[0]
	if !bucketExists {
		if err := createBucket(bucketKey, s3Client); err != nil {
			return err
		}
	}
	if len(path) < 2 {
		return fmt.Errorf("error enabling webhook for bucket, missing foler on path \"%s\"", path)
	} else {
		folderKey := fmt.Sprintf("%s/", path[1])
		_, err := s3Client.PutObject(&s3.PutObjectInput{
			Bucket: aws.String(bucketKey),
			Key:    aws.String(folderKey),
		})
		if err != nil {
			return fmt.Errorf("error creating folder \"%s\" in bucket \"%s\": %v", folderKey, bucketKey, err)
		}
		if err := enableInputNotification(s3Client, arn, bucketKey, folderKey); err != nil {
			return err
		}
	}
	return nil
}

// CreateS3Path Creates a bucket and its paths
func (minIOAdminClient *MinIOAdminClient) CreateS3Path(s3Client *s3.S3, path []string, bucketExists bool) error {
	bucketKey := path[0]
	// Only create the bucket itself if is input type to avoid recreation
	if !bucketExists {
		if err := createBucket(bucketKey, s3Client); err != nil {
			return err
		}
	}

	if len(path) >= 2 {
		// Add "/" to the end of the key in order to create a folder
		folderKey := fmt.Sprintf("%s/", path[1])
		_, err := s3Client.PutObject(&s3.PutObjectInput{
			Bucket: aws.String(bucketKey),
			Key:    aws.String(folderKey),
		})
		if err != nil {
			return fmt.Errorf("error creating folder \"%s\" in bucket \"%s\": %v", folderKey, bucketKey, err)
		}
	}
	return nil
}

func createBucket(bucketKey string, s3Client *s3.S3) error {
	_, err := s3Client.CreateBucket(&s3.CreateBucketInput{
		Bucket: aws.String(bucketKey),
	})
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			// Check if the error is caused because the bucket already exists
			if aerr.Code() == s3.ErrCodeBucketAlreadyExists || aerr.Code() == s3.ErrCodeBucketAlreadyOwnedByYou {
				return fmt.Errorf("The bucket \"%s\" already exists\n", bucketKey)
			} else {
				return fmt.Errorf("error creating bucket %s: %v", bucketKey, err)
			}
		} else {
			return fmt.Errorf("error creating bucket %s: %v", bucketKey, err)
		}
	}
	return nil
}

func enableInputNotification(s3Client *s3.S3, arnStr string, bucket string, path string) error {
	gbncRequest := &s3.GetBucketNotificationConfigurationRequest{
		Bucket: aws.String(bucket),
	}
	nCfg, err := s3Client.GetBucketNotificationConfiguration(gbncRequest)
	if err != nil {
		return fmt.Errorf("error getting bucket \"%s\" notifications: %v", bucket, err)
	}
	queueConfiguration := s3.QueueConfiguration{
		QueueArn: aws.String(arnStr),
		Events:   []*string{aws.String(s3.EventS3ObjectCreated)},
	}

	// Add folder filter if required
	if path != "" {
		queueConfiguration.Filter = &s3.NotificationConfigurationFilter{
			Key: &s3.KeyFilter{
				FilterRules: []*s3.FilterRule{
					{
						Name:  aws.String(s3.FilterRuleNamePrefix),
						Value: aws.String(path),
					},
				},
			},
		}
	}

	// Append the new queueConfiguration
	nCfg.QueueConfigurations = append(nCfg.QueueConfigurations, &queueConfiguration)
	pbncInput := &s3.PutBucketNotificationConfigurationInput{
		Bucket:                    aws.String(bucket),
		NotificationConfiguration: nCfg,
	}

	// Enable the notification
	_, err = s3Client.PutBucketNotificationConfiguration(pbncInput)

	if err != nil && !strings.Contains(err.Error(), overlappingError) {
		return fmt.Errorf("error enabling bucket notification: %v", err)
	}

	return nil
}

func disableInputNotifications(s3Client *s3.S3, arnStr string, bucket string) error {
	parsedARN, _ := arn.Parse(arnStr)

	updatedQueueConfigurations := []*s3.QueueConfiguration{}
	// Get bucket notification
	nCfg, err := s3Client.GetBucketNotificationConfiguration(&s3.GetBucketNotificationConfigurationRequest{Bucket: aws.String(bucket)})
	if err != nil {
		return fmt.Errorf("error getting bucket \"%s\" notifications: %v", bucket, err)
	}

	// Filter elements that doesn't match with service's ARN
	for _, q := range nCfg.QueueConfigurations {
		queueARN, _ := arn.Parse(*q.QueueArn)
		if queueARN.Resource == parsedARN.Resource &&
			queueARN.AccountID != parsedARN.AccountID {
			updatedQueueConfigurations = append(updatedQueueConfigurations, q)
		}
	}

	// Put the updated bucket configuration
	nCfg.QueueConfigurations = updatedQueueConfigurations
	pbncInput := &s3.PutBucketNotificationConfigurationInput{
		Bucket:                    aws.String(bucket),
		NotificationConfiguration: nCfg,
	}
	_, err = s3Client.PutBucketNotificationConfiguration(pbncInput)
	if err != nil {
		return fmt.Errorf("error disabling bucket notification: %v", err)
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

func (minIOAdminClient *MinIOAdminClient) SetPolicies(bucket MinIOBucket) error {
	if bucket.Visibility == RESTRICTED || bucket.Visibility == PRIVATE {
		// Both types of visibility require config of the user policy
		if err := minIOAdminClient.CreateAddPolicy(bucket.BucketPath, bucket.Owner, ALL_ACTIONS, false); err != nil {
			return err
		}
		if bucket.Visibility == RESTRICTED {
			if err := minIOAdminClient.CreateAddGroup(bucket.BucketPath, bucket.AllowedUsers, false); err != nil {
				return fmt.Errorf("error creating bucket group: %v", err)
			}
			if err := minIOAdminClient.CreateAddPolicy(bucket.BucketPath, bucket.BucketPath, RESTRICTED_ACTIONS, true); err != nil {
				return fmt.Errorf("error creating policy: %v", err)
			}
		}
	} else {
		// Config public visibility
		if err := minIOAdminClient.CreateAddPolicy(bucket.BucketPath, ALL_USERS_GROUP, ALL_ACTIONS, true); err != nil {
			return fmt.Errorf("error creating policy: %v", err)
		}
	}
	return nil
}

func (minIOAdminClient *MinIOAdminClient) UnsetPolicies(bucket MinIOBucket) error {
	var policyName string
	var isGroup bool
	if strings.ToLower(bucket.Visibility) == PUBLIC {
		policyName = ALL_USERS_GROUP
		isGroup = true
	} else {
		policyName = bucket.Owner
	}

	err := minIOAdminClient.RemoveResource(bucket.BucketPath, policyName, isGroup)
	if err != nil {
		return fmt.Errorf("error removing resource")
	}

	if strings.ToLower(bucket.Visibility) == RESTRICTED {
		err := minIOAdminClient.RemoveGroupPolicy(bucket.BucketPath)
		if err != nil {
			return fmt.Errorf("error removing policy for group")
		}
	}
	return nil
}

func (minIOAdminClient *MinIOAdminClient) CreateAddGroup(groupName string, users []string, remove bool) error {
	group := madmin.GroupAddRemove{
		Group:    groupName,
		Members:  users,
		Status:   "enable",
		IsRemove: remove,
	}
	err := minIOAdminClient.adminClient.UpdateGroupMembers(context.TODO(), group)
	if err != nil {
		return fmt.Errorf("error creating/updating MinIO group %s: %v", groupName, err)
	}

	return nil
}

func (minIOAdminClient *MinIOAdminClient) GetCurrentResourceVisibility(bucket MinIOBucket) string {
	if minIOAdminClient.ResourceInPolicy(bucket.Owner, bucket.BucketPath) {
		if minIOAdminClient.ResourceInPolicy(bucket.BucketPath, bucket.BucketPath) {
			return RESTRICTED
		}
		return PRIVATE
	} else {
		if minIOAdminClient.ResourceInPolicy(ALL_USERS_GROUP, bucket.BucketPath) {
			return PUBLIC
		}
	}
	// If not found return empty string
	return ""
}

func (minIOAdminClient *MinIOAdminClient) UpdateServiceGroup(groupName string, users []string) error {
	groupDescription, err := minIOAdminClient.adminClient.GetGroupDescription(context.TODO(), groupName)
	if err != nil {
		return fmt.Errorf("error getting group description for %s: %v", groupName, err)
	}

	membersMap := make(map[string]bool)
	for _, member := range groupDescription.Members {
		membersMap[member] = true

	}
	for _, user := range users {
		if found := membersMap[user]; found {
			membersMap[user] = false
		}
	}
	fmt.Println(membersMap)

	seq := func(yield func(string) bool) {
		for member := range membersMap {
			if !membersMap[member] {
				continue
			}
			if !yield(member) {
				return
			}
		}
	}

	isRemove := slices.Collect(seq)

	// Remove users not present on the new users list
	if len(isRemove) > 0 {
		err := minIOAdminClient.CreateAddGroup(groupName, isRemove, true)
		if err != nil {
			return fmt.Errorf("error removing users for %s: %v", groupName, err)
		}
	}

	err = minIOAdminClient.CreateAddGroup(groupName, users, false)
	if err != nil {
		return fmt.Errorf("error updating users for %s: %v", groupName, err)
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

// UserInPolicy asserts if a user policy has a given resource (bucketPath)
func (minIOAdminClient *MinIOAdminClient) ResourceInPolicy(policyName string, resource string) bool {
	rs := "arn:aws:s3:::" + resource + "/*"
	getPolicy, err := minIOAdminClient.adminClient.InfoCannedPolicyV2(context.TODO(), policyName)
	if err != nil {
		fmt.Printf("error reading policy for user %s", policyName)
		return false
	}

	// Search resource on user policy
	actualPolicy := &Policy{}

	jsonErr := json.Unmarshal(getPolicy.Policy, actualPolicy)
	if jsonErr != nil {
		fmt.Printf("error parsing policy for user %s", policyName)
		return false
	}

	for _, r := range actualPolicy.Statement[0].Resource {
		if r == rs {
			return true
		}
	}
	return false
}

// CreateAddPolicy creates a policy asociated to a bucket to set its visibility
func (minIOAdminClient *MinIOAdminClient) CreateAddPolicy(bucket string, policyName string, policyActions []string, isGroup bool) error {
	var jsonErr error
	var policy []byte

	rs := "arn:aws:s3:::" + bucket + "/*"

	getPolicy, errInfo := minIOAdminClient.adminClient.InfoCannedPolicyV2(context.TODO(), policyName)
	if errInfo != nil {
		// If the policy does not exist create it
		newPolicy := getPolicyDefinition(policyActions, rs)
		policy, jsonErr = json.Marshal(newPolicy)
		if jsonErr != nil {
			return jsonErr
		}
	} else {
		actualPolicy := &Policy{}

		jsonErr = json.Unmarshal(getPolicy.Policy, actualPolicy)
		if jsonErr != nil {
			return jsonErr
		}
		// Add new resource and apply policy
		actualPolicy.Statement[0].Resource = append(actualPolicy.Statement[0].Resource, rs)

		policy, jsonErr = json.Marshal(actualPolicy)
		if jsonErr != nil {
			return jsonErr
		}
	}

	err := minIOAdminClient.adminClient.AddCannedPolicy(context.TODO(), policyName, []byte(policy))
	if err != nil {
		return fmt.Errorf("error creating/adding MinIO policy for user/group %s: %v", policyName, err)
	}

	err = minIOAdminClient.adminClient.SetPolicy(context.TODO(), policyName, policyName, isGroup)
	if err != nil {
		return fmt.Errorf("error setting MinIO policy for user/group %s: %v", policyName, err)
	}

	return nil
}

func (minIOAdminClient *MinIOAdminClient) RemoveFromPolicy(bucketName string, policyName string, isGroup bool) error {

	rs := "arn:aws:s3:::" + bucketName + "/*"
	policyInfo, errInfo := minIOAdminClient.adminClient.InfoCannedPolicyV2(context.TODO(), policyName)
	if errInfo != nil {
		return fmt.Errorf("policy '%s' does not exist: %v", policyName, errInfo)
	}
	actualPolicy := &Policy{}
	jsonErr := json.Unmarshal(policyInfo.Policy, actualPolicy)
	if jsonErr != nil {
		return jsonErr
	}
	if len(actualPolicy.Statement[0].Resource) == 1 {

	} else {
		for i, r := range actualPolicy.Statement[0].Resource {
			if r == rs {
				actualPolicy.Statement[0].Resource = append(actualPolicy.Statement[0].Resource[:i], actualPolicy.Statement[0].Resource[i+1:]...)
				break
			}
		}
	}

	policy, jsonErr := json.Marshal(actualPolicy)
	if jsonErr != nil {
		return jsonErr
	}

	err := minIOAdminClient.adminClient.AddCannedPolicy(context.TODO(), policyName, []byte(policy))
	if err != nil {
		return fmt.Errorf("error creating MinIO policy for user %s: %v", policyName, err)
	}

	err = minIOAdminClient.adminClient.SetPolicy(context.TODO(), policyName, policyName, isGroup)
	if err != nil {
		return fmt.Errorf("error setting MinIO policy for user %s: %v", policyName, err)
	}
	return nil
}

// DeleteBucket deletes a MinIO bucket and its contents
func (minIOAdminClient *MinIOAdminClient) DeleteBucket(s3Client *s3.S3, bucketName string) error {

	iter := s3manager.NewDeleteListIterator(s3Client, &s3.ListObjectsInput{
		Bucket: aws.String(bucketName),
	})

	if err := s3manager.NewBatchDeleteWithClient(s3Client).Delete(aws.BackgroundContext(), iter); err != nil {
		return fmt.Errorf("unable to delete objects from bucket %q, %v", bucketName, err)
	}

	_, err := s3Client.DeleteBucket(&s3.DeleteBucketInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		return fmt.Errorf("unable to delete bucket %q, %v", bucketName, err)
	}

	return nil
}

// RemoveResource deletes a resource from a given policy
func (minIOAdminClient *MinIOAdminClient) RemoveResource(bucketName string, policyName string, isGroup bool) error {
	var policy []byte
	var jsonErr error

	resource := "arn:aws:s3:::" + bucketName + "/*"
	policyInfo, errInfo := minIOAdminClient.adminClient.InfoCannedPolicyV2(context.TODO(), policyName)
	if errInfo != nil {
		return fmt.Errorf("policy '%s' does not exist: %v", policyName, errInfo)
	}
	actualPolicy := &Policy{}
	json.Unmarshal(policyInfo.Policy, actualPolicy)
	for i, rs := range actualPolicy.Statement[0].Resource {
		if rs == resource {
			actualPolicy.Statement[0].Resource = append(actualPolicy.Statement[0].Resource[:i], actualPolicy.Statement[0].Resource[i+1:]...)
			break
		}
	}
	policy, jsonErr = json.Marshal(actualPolicy)
	if jsonErr != nil {
		return jsonErr
	}

	err := minIOAdminClient.adminClient.AddCannedPolicy(context.TODO(), policyName, []byte(policy))
	if err != nil {
		return fmt.Errorf("error creating MinIO policy %s: %v", policyName, err)
	}

	err = minIOAdminClient.adminClient.SetPolicy(context.TODO(), policyName, policyName, isGroup)
	if err != nil {
		return fmt.Errorf("error setting MinIO policy for user %s: %v", policyName, err)
	}

	return nil
}

// RemoveGroupPolicy a group and its associated policy
func (minIOAdminClient *MinIOAdminClient) RemoveGroupPolicy(policyName string) error {
	fmt.Printf("Removing policy: %s", policyName)
	// Empty group
	groupDescription, err := minIOAdminClient.adminClient.GetGroupDescription(context.TODO(), policyName)
	if err != nil {
		return fmt.Errorf("error getting group description for %s: %v", policyName, err)
	}
	err = minIOAdminClient.CreateAddGroup(policyName, groupDescription.Members, true)
	if err != nil {
		return fmt.Errorf("error emptying group %s: %v", policyName, err)
	}
	err = minIOAdminClient.CreateAddGroup(policyName, []string{}, true)
	if err != nil {
		return fmt.Errorf("error removing group %s: %v", policyName, err)
	}
	if err := minIOAdminClient.adminClient.RemoveCannedPolicy(context.TODO(), policyName); err != nil {
		return fmt.Errorf("error removing policy: %v", err)
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
