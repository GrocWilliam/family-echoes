package config

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

var (
	StorageClient *s3.Client
	PresignClient *s3.PresignClient
	StorageBucket string
)

const (
	photoURLExpiry = 12 * time.Hour
	// Keep serving the same cached presigned URL for an hour. A photo's URL
	// changing on every request would defeat the browser's HTTP cache
	// entirely — same bytes, "new" URL, re-downloaded every time an album is
	// revisited. Object keys are UUID-based and never overwritten, so the
	// underlying bytes for a given key never change, making this safe to cache
	// aggressively. The gap to photoURLExpiry guarantees every URL handed out
	// stays valid for 11+ hours, so a page left open doesn't break.
	photoURLCacheTTL = time.Hour
)

type cachedPhotoURL struct {
	url       string
	expiresAt time.Time
}

var (
	photoURLCacheMu sync.Mutex
	photoURLCache   = make(map[string]cachedPhotoURL)
)

// InitializeStorage sets up an S3-compatible client for the photo bucket.
// Railway's bucket offering (and alternatives like R2/B2/MinIO) all speak the
// S3 API, so a generic client with a configurable endpoint works for any of them.
func InitializeStorage() {
	ctx := context.Background()

	StorageBucket = os.Getenv("S3_BUCKET")

	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(os.Getenv("S3_REGION")),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			os.Getenv("S3_ACCESS_KEY_ID"),
			os.Getenv("S3_SECRET_ACCESS_KEY"),
			"",
		)),
	)
	if err != nil {
		log.Fatalf("error loading storage config: %v", err)
	}

	StorageClient = s3.NewFromConfig(cfg, func(o *s3.Options) {
		if endpoint := os.Getenv("S3_ENDPOINT"); endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
		}
		// Required for most non-AWS S3-compatible providers (MinIO, R2, B2, Railway bucket).
		o.UsePathStyle = true
	})

	PresignClient = s3.NewPresignClient(StorageClient)
}

// PresignPhotoURL generates a short-lived signed GET URL for a photo. The
// bucket doesn't need to be publicly readable this way — every read goes
// through the backend deciding, at request time, that the caller is allowed
// to see it.
//
// The URL (and the underlying image response) are cached so repeated calls
// for the same key return an identical, browser-cacheable URL instead of a
// fresh one every time.
func PresignPhotoURL(ctx context.Context, key string) (string, error) {
	if key == "" {
		return "", nil
	}

	photoURLCacheMu.Lock()
	cached, ok := photoURLCache[key]
	photoURLCacheMu.Unlock()
	if ok && time.Now().Before(cached.expiresAt) {
		return cached.url, nil
	}

	request, err := PresignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(StorageBucket),
		Key:    aws.String(key),
		// Tell the browser it can reuse the image without re-checking, for as
		// long as we intend to keep serving this same signed URL.
		ResponseCacheControl: aws.String(fmt.Sprintf("private, max-age=%d, immutable", int(photoURLCacheTTL.Seconds()))),
	}, s3.WithPresignExpires(photoURLExpiry))
	if err != nil {
		return "", err
	}

	photoURLCacheMu.Lock()
	photoURLCache[key] = cachedPhotoURL{
		url:       request.URL,
		expiresAt: time.Now().Add(photoURLCacheTTL),
	}
	photoURLCacheMu.Unlock()

	return request.URL, nil
}

// ForgetPhotoURL drops a deleted or moved key's cached URL, so the cache
// doesn't keep every key ever served.
func ForgetPhotoURL(key string) {
	photoURLCacheMu.Lock()
	delete(photoURLCache, key)
	photoURLCacheMu.Unlock()
}
