// Package store keeps photo records in the bucket itself instead of a separate
// database: each object's key is "<album>/<id><ext>", its LastModified is the
// upload time, and the uploader / original filename ride along as S3 user
// metadata. Listing the bucket is therefore enough to rebuild every album, and
// nothing is lost when the container restarts.
//
// A photo's optional thumbnail lives at "<album>/thumbs/<id><ext>". Keys with
// more than one "/" are never parsed as photos, so thumbnails don't show up as
// photos (or as an album of their own).
package store

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
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
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
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

const thumbDir = "thumbs"

// Bounds the HeadObject / CopyObject fan-out.
const s3Concurrency = 8

var (
	ErrInvalidAlbumName = errors.New("invalid album name")
	ErrNotFound         = errors.New("not found")
	ErrAlbumExists      = errors.New("album already exists")
)

// Upload is one file to store, already validated by the caller. Ext is the
// key's file extension (".jpg"), derived from the verified content type
// rather than trusted from the client's filename.
type Upload struct {
	Body        io.Reader
	ContentType string
	Ext         string
}

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

// PutPhoto uploads a photo, plus its thumbnail when one is given, and returns
// the record for it. A failed thumbnail upload isn't fatal: the photo is
// already stored, and clients fall back to the full image.
func PutPhoto(ctx context.Context, album, fileName, uploader string, photo Upload, thumb *Upload) (models.Photo, error) {
	if err := ValidateAlbumName(album); err != nil {
		return models.Photo{}, err
	}

	id := uuid.NewString()
	uploadedAt := time.Now().UTC()
	key := fmt.Sprintf("%s/%s%s", album, id, photo.Ext)

	_, err := config.StorageClient.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(config.StorageBucket),
		Key:         aws.String(key),
		Body:        photo.Body,
		ContentType: aws.String(photo.ContentType),
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

	var thumbKey string
	if thumb != nil {
		candidate := fmt.Sprintf("%s/%s/%s%s", album, thumbDir, id, thumb.Ext)
		_, err := config.StorageClient.PutObject(ctx, &s3.PutObjectInput{
			Bucket:      aws.String(config.StorageBucket),
			Key:         aws.String(candidate),
			Body:        thumb.Body,
			ContentType: aws.String(thumb.ContentType),
		})
		if err != nil {
			log.Printf("thumbnail upload failed for %s: %v", key, err)
		} else {
			thumbKey = candidate
		}
	}

	return models.Photo{
		ID:         id,
		FileName:   fileName,
		Key:        key,
		ThumbKey:   thumbKey,
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
	g.SetLimit(s3Concurrency)
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

// DeletePhoto removes a photo and its thumbnail.
func DeletePhoto(ctx context.Context, album, id string) error {
	if err := ValidateAlbumName(album); err != nil {
		return err
	}

	photos, err := listPhotos(ctx, album+"/")
	if err != nil {
		return err
	}

	for _, photo := range photos {
		if photo.ID != id {
			continue
		}

		keys := []string{photo.Key}
		if photo.ThumbKey != "" {
			keys = append(keys, photo.ThumbKey)
		}
		for _, key := range keys {
			if err := deleteObject(ctx, key); err != nil {
				return err
			}
		}
		forgetMeta(photo.Key)
		return nil
	}

	return ErrNotFound
}

// RenameAlbum moves every object under "<from>/" to "<to>/". S3 has no
// rename, so each object is copied (metadata included) and then the originals
// are deleted. It isn't atomic: if it fails midway, the album is left split
// across both names, and retrying the same rename finishes the move.
func RenameAlbum(ctx context.Context, from, to string) error {
	if err := ValidateAlbumName(from); err != nil {
		return err
	}
	if err := ValidateAlbumName(to); err != nil {
		return err
	}
	if from == to {
		return nil
	}

	existing, err := listObjects(ctx, to+"/")
	if err != nil {
		return err
	}
	if len(existing) > 0 {
		return ErrAlbumExists
	}

	objects, err := listObjects(ctx, from+"/")
	if err != nil {
		return err
	}
	if len(objects) == 0 {
		return ErrNotFound
	}

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(s3Concurrency)
	for _, obj := range objects {
		oldKey := aws.ToString(obj.Key)
		newKey := to + strings.TrimPrefix(oldKey, from)
		g.Go(func() error {
			_, err := config.StorageClient.CopyObject(gctx, &s3.CopyObjectInput{
				Bucket:     aws.String(config.StorageBucket),
				CopySource: aws.String(copySource(oldKey)),
				Key:        aws.String(newKey),
			})
			return err
		})
	}
	if err := g.Wait(); err != nil {
		return err
	}

	// Only delete once every copy has landed, so a failure above never loses
	// a photo.
	for _, obj := range objects {
		oldKey := aws.ToString(obj.Key)
		if err := deleteObject(ctx, oldKey); err != nil {
			return err
		}
		newKey := to + strings.TrimPrefix(oldKey, from)
		metaCacheMu.Lock()
		if meta, ok := metaCache[oldKey]; ok {
			metaCache[newKey] = meta
			delete(metaCache, oldKey)
		}
		metaCacheMu.Unlock()
	}

	return nil
}

func listObjects(ctx context.Context, prefix string) ([]types.Object, error) {
	var objects []types.Object

	paginator := s3.NewListObjectsV2Paginator(config.StorageClient, &s3.ListObjectsV2Input{
		Bucket: aws.String(config.StorageBucket),
		Prefix: aws.String(prefix),
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		objects = append(objects, page.Contents...)
	}

	return objects, nil
}

func listPhotos(ctx context.Context, prefix string) ([]models.Photo, error) {
	objects, err := listObjects(ctx, prefix)
	if err != nil {
		return nil, err
	}

	var photos []models.Photo
	// "<album>/<id>" -> thumbnail key
	thumbs := make(map[string]string)
	for _, obj := range objects {
		key := aws.ToString(obj.Key)
		if photo, ok := photoFromKey(key); ok {
			photo.UploadedAt = aws.ToTime(obj.LastModified)
			photos = append(photos, photo)
		} else if album, id, ok := thumbFromKey(key); ok {
			thumbs[album+"/"+id] = key
		}
	}

	for i := range photos {
		photos[i].ThumbKey = thumbs[photos[i].AlbumName+"/"+photos[i].ID]
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

// thumbFromKey parses "<album>/thumbs/<id><ext>".
func thumbFromKey(key string) (album, id string, ok bool) {
	parts := strings.Split(key, "/")
	if len(parts) != 3 || parts[0] == "" || parts[1] != thumbDir || parts[2] == "" {
		return "", "", false
	}
	return parts[0], strings.TrimSuffix(parts[2], path.Ext(parts[2])), true
}

// copySource is CopyObject's "bucket/key", which S3 expects URL-encoded.
func copySource(key string) string {
	segments := strings.Split(key, "/")
	for i, segment := range segments {
		segments[i] = url.PathEscape(segment)
	}
	return config.StorageBucket + "/" + strings.Join(segments, "/")
}

func deleteObject(ctx context.Context, key string) error {
	_, err := config.StorageClient.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(config.StorageBucket),
		Key:    aws.String(key),
	})
	return err
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
	if meta.fileName == "" {
		meta.fileName = fallback.fileName
	}
	if t, err := time.Parse(time.RFC3339Nano, out.Metadata[metaUploadedAt]); err == nil {
		meta.uploadedAt = t
	}

	metaCacheMu.Lock()
	metaCache[key] = meta
	metaCacheMu.Unlock()

	return meta
}

func forgetMeta(key string) {
	metaCacheMu.Lock()
	delete(metaCache, key)
	metaCacheMu.Unlock()
}

func unescapeMeta(value string) string {
	if decoded, err := url.QueryUnescape(value); err == nil {
		return decoded
	}
	return value
}
