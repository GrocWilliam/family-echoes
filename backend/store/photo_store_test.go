package store

import (
	"errors"
	"slices"
	"sort"
	"strings"
	"testing"

	"echoes.com/m/config"
	"echoes.com/m/internal/s3fake"
	"echoes.com/m/models"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func jpeg(t *testing.T) Upload {
	t.Helper()
	return Upload{Body: strings.NewReader("jpeg bytes"), ContentType: "image/jpeg", Ext: ".jpg"}
}

// resetMetaCache simulates a process restart, so metadata must be read back
// from S3 rather than served from memory.
func resetMetaCache() {
	metaCacheMu.Lock()
	metaCache = make(map[string]objectMeta)
	metaCacheMu.Unlock()
}

func put(t *testing.T, album, fileName, uploader string, withThumb bool) models.Photo {
	t.Helper()
	var thumb *Upload
	if withThumb {
		th := jpeg(t)
		thumb = &th
	}
	photo, err := PutPhoto(t.Context(), album, fileName, uploader, jpeg(t), thumb)
	if err != nil {
		t.Fatalf("PutPhoto(%q, %q): %v", album, fileName, err)
	}
	return photo
}

func TestValidateAlbumName(t *testing.T) {
	valid := []string{"Vacances", "Été 2024", "Noël · famille", strings.Repeat("a", 200)}
	invalid := []string{"", "   ", ".", "..", "a/b", `a\b`, strings.Repeat("a", 201)}

	for _, name := range valid {
		if err := ValidateAlbumName(name); err != nil {
			t.Errorf("ValidateAlbumName(%q) = %v, want nil", name, err)
		}
	}
	for _, name := range invalid {
		if err := ValidateAlbumName(name); !errors.Is(err, ErrInvalidAlbumName) {
			t.Errorf("ValidateAlbumName(%q) = %v, want ErrInvalidAlbumName", name, err)
		}
	}
}

func TestPutPhotoRejectsInvalidAlbum(t *testing.T) {
	s3fake.Start(t)

	if _, err := PutPhoto(t.Context(), "a/b", "x.jpg", "u", jpeg(t), nil); !errors.Is(err, ErrInvalidAlbumName) {
		t.Fatalf("got %v, want ErrInvalidAlbumName", err)
	}
	if keys := s3fake.Keys(t); len(keys) != 0 {
		t.Fatalf("nothing should be stored, got %v", keys)
	}
}

func TestListAlbumPhotosRecoversMetadataAfterRestart(t *testing.T) {
	s3fake.Start(t)

	first := put(t, "Été 2024", "plage été.JPG", "mamie@example.com", true)
	second := put(t, "Été 2024", "two.png", "papi@example.com", false)
	put(t, "Noël", "tree.jpg", "mamie@example.com", false)

	// Objects that don't follow the layout must be ignored, not crash listing.
	for _, key := range []string{"root.txt", "Été 2024/nested/deeper.jpg"} {
		if _, err := config.StorageClient.PutObject(t.Context(), &s3.PutObjectInput{
			Bucket: aws.String(config.StorageBucket),
			Key:    aws.String(key),
			Body:   strings.NewReader("x"),
		}); err != nil {
			t.Fatal(err)
		}
	}

	resetMetaCache()

	photos, err := ListAlbumPhotos(t.Context(), "Été 2024")
	if err != nil {
		t.Fatal(err)
	}
	if len(photos) != 2 {
		t.Fatalf("got %d photos, want 2: %+v", len(photos), photos)
	}

	// Both uploads usually land within the same second; ordering must still
	// follow upload order thanks to the stored sub-second timestamp.
	if photos[0].ID != first.ID || photos[1].ID != second.ID {
		t.Errorf("photos out of upload order: %s, %s", photos[0].ID, photos[1].ID)
	}

	got := photos[0]
	if got.FileName != "plage été.JPG" || got.Uploader != "mamie@example.com" || got.AlbumName != "Été 2024" {
		t.Errorf("metadata not recovered from S3: %+v", got)
	}
	if got.ThumbKey != "Été 2024/thumbs/"+first.ID+".jpg" {
		t.Errorf("ThumbKey = %q", got.ThumbKey)
	}
	if photos[1].ThumbKey != "" {
		t.Errorf("photo uploaded without a thumbnail has ThumbKey %q", photos[1].ThumbKey)
	}
	if !got.UploadedAt.Equal(first.UploadedAt) {
		t.Errorf("UploadedAt = %v, want %v", got.UploadedAt, first.UploadedAt)
	}
}

func TestListAllPhotosSkipsThumbnails(t *testing.T) {
	s3fake.Start(t)

	put(t, "Vacances", "a.jpg", "u", true)
	put(t, "Vacances", "b.jpg", "u", true)
	// An album literally named "thumbs" must still work.
	put(t, "thumbs", "c.jpg", "u", true)

	photos, err := ListAllPhotos(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	counts := make(map[string]int)
	for _, p := range photos {
		counts[p.AlbumName]++
		if p.ThumbKey == "" {
			t.Errorf("photo %s lost its thumbnail", p.Key)
		}
	}
	if counts["Vacances"] != 2 || counts["thumbs"] != 1 || len(counts) != 2 {
		t.Errorf("album counts = %v", counts)
	}
}

func TestDeletePhoto(t *testing.T) {
	s3fake.Start(t)

	doomed := put(t, "Vacances", "a.jpg", "u", true)
	kept := put(t, "Vacances", "b.jpg", "u", false)

	if err := DeletePhoto(t.Context(), "Vacances", doomed.ID); err != nil {
		t.Fatal(err)
	}
	if keys := s3fake.Keys(t); !slices.Equal(keys, []string{kept.Key}) {
		t.Errorf("bucket = %v, want only %s (photo and thumbnail both deleted)", keys, kept.Key)
	}

	if err := DeletePhoto(t.Context(), "Vacances", doomed.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("second delete = %v, want ErrNotFound", err)
	}
	if err := DeletePhoto(t.Context(), "Ailleurs", kept.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("delete from wrong album = %v, want ErrNotFound", err)
	}
}

func TestRenameAlbum(t *testing.T) {
	s3fake.Start(t)

	withThumb := put(t, "Vacances", "plage.jpg", "mamie@example.com", true)
	put(t, "Vacances", "dune.jpg", "papi@example.com", false)
	put(t, "Noël", "tree.jpg", "mamie@example.com", false)

	if err := RenameAlbum(t.Context(), "Vacances", "Noël"); !errors.Is(err, ErrAlbumExists) {
		t.Errorf("rename onto existing album = %v, want ErrAlbumExists", err)
	}
	if err := RenameAlbum(t.Context(), "Absent", "Autre"); !errors.Is(err, ErrNotFound) {
		t.Errorf("rename missing album = %v, want ErrNotFound", err)
	}
	if err := RenameAlbum(t.Context(), "Vacances", "a/b"); !errors.Is(err, ErrInvalidAlbumName) {
		t.Errorf("rename to invalid name = %v, want ErrInvalidAlbumName", err)
	}
	if err := RenameAlbum(t.Context(), "Vacances", "Vacances"); err != nil {
		t.Errorf("rename to same name = %v, want nil", err)
	}

	if err := RenameAlbum(t.Context(), "Vacances", "Été 2024"); err != nil {
		t.Fatal(err)
	}

	keys := s3fake.Keys(t)
	sort.Strings(keys)
	for _, key := range keys {
		if strings.HasPrefix(key, "Vacances/") {
			t.Errorf("old object left behind: %s", key)
		}
	}
	if !slices.Contains(keys, "Été 2024/thumbs/"+withThumb.ID+".jpg") {
		t.Errorf("thumbnail not moved: %v", keys)
	}

	// Metadata must travel with the copy in S3, not just in the cache.
	resetMetaCache()
	photos, err := ListAlbumPhotos(t.Context(), "Été 2024")
	if err != nil {
		t.Fatal(err)
	}
	if len(photos) != 2 {
		t.Fatalf("got %d photos after rename, want 2", len(photos))
	}
	if photos[0].ID != withThumb.ID || photos[0].Uploader != "mamie@example.com" ||
		photos[0].FileName != "plage.jpg" || photos[0].ThumbKey == "" {
		t.Errorf("renamed photo = %+v", photos[0])
	}
}
