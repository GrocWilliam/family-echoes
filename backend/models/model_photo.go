package models

import "time"

// Photo is the storage-layer record, rebuilt from the bucket by package store.
// It's never serialized: the API exposes presigned URLs instead of bucket
// keys — see handlers.PhotoResponse.
type Photo struct {
	ID         string
	FileName   string
	Key        string
	ThumbKey   string
	AlbumName  string
	Uploader   string
	UploadedAt time.Time
}
