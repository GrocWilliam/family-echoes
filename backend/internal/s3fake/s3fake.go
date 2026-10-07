// Package s3fake points the storage config at an in-memory S3 server for
// tests, so store and handler code runs against a real S3 API without
// credentials or network access.
package s3fake

import (
	"net/http/httptest"
	"testing"

	"echoes.com/m/config"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/johannesboyne/gofakes3"
	"github.com/johannesboyne/gofakes3/backend/s3mem"
)

// Start serves a fresh, empty bucket for the duration of the test and
// re-initializes config.StorageClient against it.
func Start(t *testing.T) {
	t.Helper()

	server := httptest.NewServer(gofakes3.New(s3mem.New()).Server())
	t.Cleanup(server.Close)

	t.Setenv("S3_ENDPOINT", server.URL)
	t.Setenv("S3_REGION", "us-east-1")
	t.Setenv("S3_BUCKET", "echoes-test")
	t.Setenv("S3_ACCESS_KEY_ID", "test")
	t.Setenv("S3_SECRET_ACCESS_KEY", "test")
	config.InitializeStorage()

	if _, err := config.StorageClient.CreateBucket(t.Context(), &s3.CreateBucketInput{
		Bucket: aws.String(config.StorageBucket),
	}); err != nil {
		t.Fatalf("creating fake bucket: %v", err)
	}
}

// Keys lists every object key in the bucket.
func Keys(t *testing.T) []string {
	t.Helper()

	out, err := config.StorageClient.ListObjectsV2(t.Context(), &s3.ListObjectsV2Input{
		Bucket: aws.String(config.StorageBucket),
	})
	if err != nil {
		t.Fatalf("listing bucket: %v", err)
	}

	keys := make([]string, 0, len(out.Contents))
	for _, obj := range out.Contents {
		keys = append(keys, aws.ToString(obj.Key))
	}
	return keys
}
