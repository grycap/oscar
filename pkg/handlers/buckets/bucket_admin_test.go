package buckets

import (
	"testing"

	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/grycap/oscar/v3/pkg/types"
	"github.com/grycap/oscar/v3/pkg/utils"
)

type stubBucketAdmin struct {
	Metadata       map[string]string
	Visibility     string
	Members        []string
	ResourceAccess bool

	CreateS3PathFn                 func(*s3.S3, []string, bool) error
	SetTagsFn                      func(string, map[string]string) error
	SetPoliciesFn                  func(utils.MinIOBucket) error
	UnsetPoliciesFn                func(utils.MinIOBucket) error
	DeleteBucketsFn                func(*s3.S3, utils.MinIOBucket) error
	UpdateServiceGroupFn           func(string, []string) error
	GetTaggedMetadataFn            func(string) (map[string]string, error)
	ResourceInPolicyFn             func(string, string) bool
	GetCurrentResourceVisibilityFn func(utils.MinIOBucket) string
	GetBucketMembersFn             func(string) ([]string, error)
}

func (s *stubBucketAdmin) CreateS3Path(client *s3.S3, path []string, bucketExists bool) error {
	if s.CreateS3PathFn != nil {
		return s.CreateS3PathFn(client, path, bucketExists)
	}
	return nil
}

func (s *stubBucketAdmin) SetTags(bucket string, tags map[string]string) error {
	if s.SetTagsFn != nil {
		return s.SetTagsFn(bucket, tags)
	}
	return nil
}

func (s *stubBucketAdmin) SetPolicies(bucket utils.MinIOBucket) error {
	if s.SetPoliciesFn != nil {
		return s.SetPoliciesFn(bucket)
	}
	return nil
}

func (s *stubBucketAdmin) UnsetPolicies(bucket utils.MinIOBucket) error {
	if s.UnsetPoliciesFn != nil {
		return s.UnsetPoliciesFn(bucket)
	}
	return nil
}

func (s *stubBucketAdmin) DeleteBuckets(client *s3.S3, bucket utils.MinIOBucket) error {
	if s.DeleteBucketsFn != nil {
		return s.DeleteBucketsFn(client, bucket)
	}
	return nil
}

func (s *stubBucketAdmin) UpdateServiceGroup(group string, users []string) error {
	if s.UpdateServiceGroupFn != nil {
		return s.UpdateServiceGroupFn(group, users)
	}
	return nil
}

func (s *stubBucketAdmin) GetTaggedMetadata(bucket string) (map[string]string, error) {
	if s.GetTaggedMetadataFn != nil {
		return s.GetTaggedMetadataFn(bucket)
	}
	return s.Metadata, nil
}

func (s *stubBucketAdmin) GetCurrentResourceVisibility(bucket utils.MinIOBucket) string {
	if s.GetCurrentResourceVisibilityFn != nil {
		return s.GetCurrentResourceVisibilityFn(bucket)
	}
	return s.Visibility
}

func (s *stubBucketAdmin) ResourceInPolicy(policyName string, resource string) bool {
	if s.ResourceInPolicyFn != nil {
		return s.ResourceInPolicyFn(policyName, resource)
	}
	return s.ResourceAccess
}

func (s *stubBucketAdmin) GetBucketMembers(bucket string) ([]string, error) {
	if s.GetBucketMembersFn != nil {
		return s.GetBucketMembersFn(bucket)
	}
	return s.Members, nil
}

func overrideBucketAdminClient(t *testing.T, admin bucketAdminClient) {
	t.Helper()
	original := makeBucketAdminClient
	makeBucketAdminClient = func(cfg *types.Config) (bucketAdminClient, error) {
		return admin, nil
	}
	t.Cleanup(func() {
		makeBucketAdminClient = original
	})
}
