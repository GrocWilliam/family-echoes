package handlers

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"echoes.com/m/config"
	"echoes.com/m/models"
	"echoes.com/m/store"
	"github.com/gofiber/fiber/v3"
)

const maxPhotoSize = 20 << 20 // 20 MB

// Image formats every browser can display, keyed by sniffed content type.
// SVG is deliberately absent (it can carry script), as is HEIC (only Safari
// renders it).
var allowedImageTypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/gif":  ".gif",
	"image/webp": ".webp",
	"image/avif": ".avif",
}

var errUnsupportedImage = errors.New("unsupported image type")

// sniffImageType detects the file's real type from its leading bytes instead
// of trusting the client-supplied Content-Type header, then rewinds the file
// so the full contents can be uploaded.
func sniffImageType(file io.ReadSeeker) (contentType, ext string, err error) {
	head := make([]byte, 512)
	n, err := io.ReadFull(file, head)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return "", "", err
	}
	head = head[:n]
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", "", err
	}

	contentType = http.DetectContentType(head)
	// net/http doesn't recognise AVIF: an ISO-BMFF "ftyp" box with an avif brand.
	if len(head) >= 12 && string(head[4:8]) == "ftyp" && bytes.Contains(head[8:min(len(head), 64)], []byte("avi")) {
		contentType = "image/avif"
	}

	ext, ok := allowedImageTypes[contentType]
	if !ok {
		return "", "", errUnsupportedImage
	}
	return contentType, ext, nil
}

// PhotoResponse is the API shape for a photo: unlike models.Photo, its URL is
// a presigned link rather than the bucket key, since a stored URL would
// eventually expire.
type PhotoResponse struct {
	ID         string    `json:"id"`
	FileName   string    `json:"file_name"`
	URL        string    `json:"url"`
	AlbumName  string    `json:"album_name"`
	Uploader   string    `json:"uploader"`
	UploadedAt time.Time `json:"uploaded_at"`
}

func toPhotoResponse(ctx context.Context, photo models.Photo) PhotoResponse {
	url, err := config.PresignPhotoURL(ctx, photo.Key)
	if err != nil {
		url = ""
	}

	return PhotoResponse{
		ID:         photo.ID,
		FileName:   photo.FileName,
		URL:        url,
		AlbumName:  photo.AlbumName,
		Uploader:   photo.Uploader,
		UploadedAt: photo.UploadedAt,
	}
}

func toPhotoResponses(ctx context.Context, photos []models.Photo) []PhotoResponse {
	responses := make([]PhotoResponse, 0, len(photos))
	for _, photo := range photos {
		responses = append(responses, toPhotoResponse(ctx, photo))
	}
	return responses
}

// albumParam returns the decoded :id param. Fiber hands params back still
// percent-encoded (UnescapePath is off, and must stay off so an encoded "/"
// can't change routing), so "Été" would otherwise arrive as "%C3%89t%C3%A9".
func albumParam(c fiber.Ctx) (string, error) {
	name, err := url.PathUnescape(c.Params("id"))
	if err != nil {
		return "", store.ErrInvalidAlbumName
	}
	name = strings.TrimSpace(name)
	if err := store.ValidateAlbumName(name); err != nil {
		return "", err
	}
	return name, nil
}

// GET /api/albums/:id/photos - Récupérer les photos d'un album
func HandlerGetPhotos(c fiber.Ctx) error {
	albumName, err := albumParam(c)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid album name"})
	}

	photos, err := store.ListAlbumPhotos(c.Context(), albumName)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to list photos"})
	}

	return c.JSON(fiber.Map{
		"photos": toPhotoResponses(c.Context(), photos),
		"total":  len(photos),
	})
}

// POST /api/albums/:id/photos - Uploader une photo dans le bucket
func HandlerUploadPhoto(c fiber.Ctx) error {
	albumName, err := albumParam(c)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid album name"})
	}

	fileHeader, err := c.FormFile("photo")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "missing photo file"})
	}

	if fileHeader.Size > maxPhotoSize {
		return c.Status(fiber.StatusRequestEntityTooLarge).JSON(fiber.Map{"error": "photo is too large (max 20MB)"})
	}

	file, err := fileHeader.Open()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to read photo"})
	}
	defer file.Close()

	contentType, ext, err := sniffImageType(file)
	if errors.Is(err, errUnsupportedImage) {
		return c.Status(fiber.StatusUnsupportedMediaType).JSON(fiber.Map{"error": "only JPEG, PNG, GIF, WebP and AVIF images are allowed"})
	} else if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to read photo"})
	}

	uploader, _ := c.Locals("user_email").(string)

	photo, err := store.PutPhoto(c.Context(), albumName, fileHeader.Filename, ext, contentType, uploader, file)
	if err != nil {
		if errors.Is(err, store.ErrInvalidAlbumName) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid album name"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to upload photo"})
	}

	return c.Status(fiber.StatusCreated).JSON(toPhotoResponse(c.Context(), photo))
}

type AlbumSummary struct {
	Name     string `json:"name"`
	Count    int    `json:"count"`
	CoverURL string `json:"cover_url,omitempty"`
}

type albumAccumulator struct {
	Name     string
	Count    int
	CoverKey string
}

// GET /api/albums - Lister les albums avec le nombre de photos et une couverture
func HandlerGetAlbums(c fiber.Ctx) error {
	photos, err := store.ListAllPhotos(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to list albums"})
	}

	// Photos come back oldest first, so each album's cover is its first photo
	// and albums are ordered by when they were started.
	accumulators := make(map[string]*albumAccumulator)
	var order []string
	for _, photo := range photos {
		acc, ok := accumulators[photo.AlbumName]
		if !ok {
			acc = &albumAccumulator{Name: photo.AlbumName, CoverKey: photo.Key}
			accumulators[photo.AlbumName] = acc
			order = append(order, photo.AlbumName)
		}
		acc.Count++
	}

	albums := make([]AlbumSummary, 0, len(order))
	for _, name := range order {
		acc := accumulators[name]
		coverURL, err := config.PresignPhotoURL(c.Context(), acc.CoverKey)
		if err != nil {
			coverURL = ""
		}
		albums = append(albums, AlbumSummary{Name: acc.Name, Count: acc.Count, CoverURL: coverURL})
	}

	return c.JSON(fiber.Map{
		"albums": albums,
		"total":  len(albums),
	})
}
