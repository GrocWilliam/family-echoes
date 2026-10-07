// Package store keeps photo records in the bucket itself instead of a separate
// database: each object's key is "<album>/<id><ext>", its LastModified is the
// upload time, and the uploader / original filename ride along as S3 user
// metadata. Listing the bucket is therefore enough to rebuild every album, and
// nothing is lost when the container restarts.
package store

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	"echoes.com/m/config"
	"echoes.com/m/models"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"
)

// S3 user metadata travels as HTTP headers, so values are query-escaped to
// survive non-ASCII filenames (e.g. "été.jpg").
const (
	metaUploader = "uploader"
	metaFileName = "filename"
	// LastModified only has one-second precision, which leaves photos from a
	// quick batch upload in arbitrary order, so the exact time is kept too.
	metaUploadedAt = "uploaded-at"
)

// Bounds the HeadObject fan-out when an album's metadata isn't cached yet.
const headConcurrency = 8

var ErrInvalidAlbumName = errors.New("invalid album name")

// objectMeta is what HeadObject tells us beyond what a listing already does.
// Objects are never overwritten (keys are UUID-based), so once fetched it can
// be cached for the life of the process.
type objectMeta struct {
	uploader   string
	fileName   string
	uploadedAt time.Time
}

var (
	metaCacheMu sync.Mutex
	metaCache   = make(map[string]objectMeta)
)

// ValidateAlbumName rejects names that would break the "<album>/<id>" key
// layout or escape it.
func ValidateAlbumName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 200 || strings.ContainsAny(name, "/\\") || name == "." || name == ".." {
		return ErrInvalidAlbumName
	}
	return nil
}

// PutPhoto uploads a photo and returns the record for it.
func PutPhoto(ctx context.Context, album, fileName, contentType, uploader string, body io.Reader) (models.Photo, error) {
	if err := ValidateAlbumName(album); err != nil {
		return models.Photo{}, err
	}

	id := uuid.NewString()
	uploadedAt := time.Now().UTC()
	key := fmt.Sprintf("%s/%s%s", album, id, strings.ToLower(path.Ext(fileName)))

	_, err := config.StorageClient.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(config.StorageBucket),
		Key:         aws.String(key),
		Body:        body,
		ContentType: aws.String(contentType),
		Metadata: map[string]string{
			metaUploader:   url.QueryEscape(uploader),
			metaFileName:   url.QueryEscape(fileName),
			metaUploadedAt: uploadedAt.Format(time.RFC3339Nano),
		},
	})
	if err != nil {
		return models.Photo{}, err
	}

	meta := objectMeta{uploader: uploader, fileName: fileName, uploadedAt: uploadedAt}
	metaCacheMu.Lock()
	metaCache[key] = meta
	metaCacheMu.Unlock()

	return models.Photo{
		ID:         id,
		FileName:   fileName,
		Key:        key,
		AlbumName:  album,
		Uploader:   uploader,
		UploadedAt: uploadedAt,
	}, nil
}

// ListAlbumPhotos returns an album's photos, oldest first.
func ListAlbumPhotos(ctx context.Context, album string) ([]models.Photo, error) {
	if err := ValidateAlbumName(album); err != nil {
		return nil, err
	}

	photos, err := listPhotos(ctx, album+"/")
	if err != nil {
		return nil, err
	}

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(headConcurrency)
	for i := range photos {
		g.Go(func() error {
			meta := headMeta(gctx, photos[i].Key)
			photos[i].Uploader = meta.uploader
			photos[i].FileName = meta.fileName
			if !meta.uploadedAt.IsZero() {
				photos[i].UploadedAt = meta.uploadedAt
			}
			return nil
		})
	}
	_ = g.Wait()
	sortOldestFirst(photos)

	return photos, nil
}

// ListAllPhotos returns every photo in the bucket, oldest first, without
// uploader/filename (a listing doesn't include user metadata, and the album
// overview doesn't need it).
func ListAllPhotos(ctx context.Context) ([]models.Photo, error) {
	return listPhotos(ctx, "")
}

func listPhotos(ctx context.Context, prefix string) ([]models.Photo, error) {
	var photos []models.Photo

	paginator := s3.NewListObjectsV2Paginator(config.StorageClient, &s3.ListObjectsV2Input{
		Bucket: aws.String(config.StorageBucket),
		Prefix: aws.String(prefix),
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, obj := range page.Contents {
			if photo, ok := photoFromKey(aws.ToString(obj.Key)); ok {
				photo.UploadedAt = aws.ToTime(obj.LastModified)
				photos = append(photos, photo)
			}
		}
	}

	sortOldestFirst(photos)

	return photos, nil
}

func sortOldestFirst(photos []models.Photo) {
	sort.SliceStable(photos, func(i, j int) bool {
		return photos[i].UploadedAt.Before(photos[j].UploadedAt)
	})
}

// photoFromKey parses "<album>/<id><ext>"; anything else in the bucket is ignored.
func photoFromKey(key string) (models.Photo, bool) {
	album, name, ok := strings.Cut(key, "/")
	if !ok || album == "" || name == "" || strings.Contains(name, "/") {
		return models.Photo{}, false
	}

	return models.Photo{
		ID:        strings.TrimSuffix(name, path.Ext(name)),
		FileName:  name,
		Key:       key,
		AlbumName: album,
	}, true
}

// headMeta fetches (and caches) an object's uploader/filename. On failure it
// falls back to the key's basename so one bad object doesn't break the album.
func headMeta(ctx context.Context, key string) objectMeta {
	metaCacheMu.Lock()
	cached, ok := metaCache[key]
	metaCacheMu.Unlock()
	if ok {
		return cached
	}

	fallback := objectMeta{fileName: path.Base(key)}

	out, err := config.StorageClient.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(config.StorageBucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fallback
	}

	meta := objectMeta{
		uploader: unescapeMeta(out.Metadata[metaUploader]),
		fileName: unescapeMeta(out.Metadata[metaFileName]),
	}
	if t, err := time.Parse(time.RFC3339Nano, out.Metadata[metaUploadedAt]); err == nil {
		meta.uploadedAt = t
	}
	if meta.fileName == "" {
		meta.fileName = fallback.fileName
	}

	metaCacheMu.Lock()
	metaCache[key] = meta
	metaCacheMu.Unlock()

	return meta
}

func unescapeMeta(value string) string {
	if decoded, err := url.QueryUnescape(value); err == nil {
		return decoded
	}
	return value
}
