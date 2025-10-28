package buckets

import (
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/grycap/oscar/v3/pkg/handlers"
	"github.com/grycap/oscar/v3/pkg/types"
	"github.com/grycap/oscar/v3/pkg/utils"
)

type bucketAdminClient interface {
	CreateS3Path(s3Client *s3.S3, path []string, bucketExists bool) error
	SetTags(bucket string, tags map[string]string) error
	SetPolicies(bucket utils.MinIOBucket) error
	UnsetPolicies(bucket utils.MinIOBucket) error
	DeleteBuckets(s3Client *s3.S3, bucket utils.MinIOBucket) error
	UpdateServiceGroup(groupName string, users []string) error
	GetTaggedMetadata(bucket string) (map[string]string, error)
	GetCurrentResourceVisibility(bucket utils.MinIOBucket) string
	ResourceInPolicy(policyName string, resource string) bool
	GetBucketMembers(bucket string) ([]string, error)
}

type defaultBucketAdminClient struct {
	delegate *utils.MinIOAdminClient
}

func (c *defaultBucketAdminClient) CreateS3Path(s3Client *s3.S3, path []string, bucketExists bool) error {
	return c.delegate.CreateS3Path(s3Client, path, bucketExists)
}

func (c *defaultBucketAdminClient) SetTags(bucket string, tags map[string]string) error {
	return c.delegate.SetTags(bucket, tags)
}

func (c *defaultBucketAdminClient) SetPolicies(bucket utils.MinIOBucket) error {
	return c.delegate.SetPolicies(bucket)
}

func (c *defaultBucketAdminClient) UnsetPolicies(bucket utils.MinIOBucket) error {
	return c.delegate.UnsetPolicies(bucket)
}

func (c *defaultBucketAdminClient) DeleteBuckets(s3Client *s3.S3, bucket utils.MinIOBucket) error {
	return handlers.DeleteMinIOBuckets(s3Client, c.delegate, bucket)
}

func (c *defaultBucketAdminClient) UpdateServiceGroup(groupName string, users []string) error {
	return c.delegate.UpdateServiceGroup(groupName, users)
}

func (c *defaultBucketAdminClient) GetTaggedMetadata(bucket string) (map[string]string, error) {
	return c.delegate.GetTaggedMetadata(bucket)
}

func (c *defaultBucketAdminClient) GetCurrentResourceVisibility(bucket utils.MinIOBucket) string {
	return c.delegate.GetCurrentResourceVisibility(bucket)
}

func (c *defaultBucketAdminClient) ResourceInPolicy(policyName string, resource string) bool {
	return c.delegate.ResourceInPolicy(policyName, resource)
}

func (c *defaultBucketAdminClient) GetBucketMembers(bucket string) ([]string, error) {
	return c.delegate.GetBucketMembers(bucket)
}

var makeBucketAdminClient = func(cfg *types.Config) (bucketAdminClient, error) {
	client, err := utils.MakeMinIOAdminClient(cfg)
	if err != nil {
		return nil, err
	}
	return &defaultBucketAdminClient{delegate: client}, nil
}
