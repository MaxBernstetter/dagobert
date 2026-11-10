// Package floci connects system tests to the shared Floci stack (task up) and gives each test its own S3 bucket.
package floci

import (
	"context"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/google/uuid"
)

// Endpoint returns the shared Floci endpoint from FLOCI_ENDPOINT. The test fails when it is unset or Floci
// does not answer its health check.
func Endpoint(t *testing.T) string {
	t.Helper()
	endpoint := os.Getenv("FLOCI_ENDPOINT")
	if endpoint == "" {
		t.Fatal("FLOCI_ENDPOINT is unset: run `task up` and export FLOCI_ENDPOINT=http://127.0.0.1:11566, or use `task test:system`")
	}
	resp, err := http.Get(endpoint + "/_floci/health")
	if err != nil {
		t.Fatalf("floci at %s is not reachable (run `task up`): %v", endpoint, err)
	}
	resp.Body.Close()
	return endpoint
}

// BucketName returns a bucket name no other test or Terraform-owned bucket uses: the prefix plus a random
// suffix, lowercase and S3-valid.
func BucketName(prefix string) string {
	return "dagobert-test-" + strings.ToLower(prefix) + "-" + uuid.NewString()[:8]
}

// Start returns the shared Floci endpoint and an S3 client (path style). It creates bucket, which must come
// from BucketName, and registers a cleanup that empties and deletes it.
func Start(t *testing.T, bucket string) (endpoint string, client *s3.Client) {
	t.Helper()
	endpoint = Endpoint(t)
	client = s3.New(s3.Options{
		Region:       "us-east-1",
		BaseEndpoint: aws.String(endpoint),
		UsePathStyle: true,
		Credentials:  credentials.NewStaticCredentialsProvider("test", "test", ""),
	})
	if _, err := client.CreateBucket(context.Background(), &s3.CreateBucketInput{Bucket: aws.String(bucket)}); err != nil {
		t.Fatalf("create bucket %s: %v", bucket, err)
	}
	t.Cleanup(func() { deleteBucket(t, client, bucket) })
	return endpoint, client
}

// deleteBucket removes every object (paginated, batches of up to 1000) and then the bucket.
func deleteBucket(t *testing.T, client *s3.Client, bucket string) {
	t.Helper()
	ctx := context.Background()
	p := s3.NewListObjectsV2Paginator(client, &s3.ListObjectsV2Input{Bucket: aws.String(bucket)})
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			t.Errorf("list bucket %s for cleanup: %v", bucket, err)
			return
		}
		if len(page.Contents) == 0 {
			continue
		}
		objs := make([]types.ObjectIdentifier, 0, len(page.Contents))
		for _, o := range page.Contents {
			objs = append(objs, types.ObjectIdentifier{Key: o.Key})
		}
		if _, err := client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
			Bucket: aws.String(bucket), Delete: &types.Delete{Objects: objs, Quiet: aws.Bool(true)},
		}); err != nil {
			t.Errorf("empty bucket %s: %v", bucket, err)
			return
		}
	}
	if _, err := client.DeleteBucket(ctx, &s3.DeleteBucketInput{Bucket: aws.String(bucket)}); err != nil {
		t.Errorf("delete bucket %s: %v", bucket, err)
	}
}
