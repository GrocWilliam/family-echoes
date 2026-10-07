package handlers

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"time"

	"echoes.com/m/config"
	"echoes.com/m/models"
	"echoes.com/m/store"
	"github.com/gofiber/fiber/v3"
)

const maxPhotoSize = 20 << 20 // 20 MB

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

// GET /api/albums/:id - Alias de GET /api/albums/:id/photos
func HandlerGetAlbum(c fiber.Ctx) error {
	return HandlerGetPhotos(c)
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

	contentType := fileHeader.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "image/") {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "only image uploads are allowed"})
	}

	file, err := fileHeader.Open()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to read photo"})
	}
	defer file.Close()

	uploader, _ := c.Locals("user_email").(string)

	photo, err := store.PutPhoto(c.Context(), albumName, fileHeader.Filename, contentType, uploader, file)
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
