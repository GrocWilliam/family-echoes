package models

import "time"

// Photo is the storage-layer record persisted to the metadata file. Key is
// the object's path in the bucket; it's kept private (not the public API
// shape) because the API always exposes a freshly generated presigned URL
// instead — see handlers.PhotoResponse.
type Photo struct {
	ID         string    `json:"id"`
	FileName   string    `json:"file_name"`
	Key        string    `json:"key"`
	AlbumName  string    `json:"album_name"`
	Uploader   string    `json:"uploader"`
	UploadedAt time.Time `json:"uploaded_at"`
}

type Metadata struct {
	Photos []Photo `json:"photos"`
}
