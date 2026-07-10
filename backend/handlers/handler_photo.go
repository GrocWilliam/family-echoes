package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"echoes.com/m/config"
	"echoes.com/m/models"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

const metadataPath = "/tmp/photos-metadata.json" // Fichier local temporaire

const maxPhotoSize = 20 << 20 // 20 MB

func loadMetadata() (*models.Metadata, error) {
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		return &models.Metadata{}, nil // Retourne une liste vide si pas de fichier
	}

	var metadata models.Metadata
	json.Unmarshal(data, &metadata)
	return &metadata, nil
}

func saveMetadata(metadata *models.Metadata) error {
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(metadataPath, data, 0644)
}

// PhotoResponse is the API shape for a photo: unlike models.Photo, its URL is
// a presigned link generated fresh for this request rather than a value
// persisted to disk, since a stored URL would eventually expire.
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

// GET /api/albums/:id/photos - Récupérer les photos d'un album
func HandlerGetPhotos(c fiber.Ctx) error {
	albumName := c.Params("id")

	metadata, _ := loadMetadata()

	var filtered []models.Photo
	for _, p := range metadata.Photos {
		if p.AlbumName == albumName {
			filtered = append(filtered, p)
		}
	}

	return c.JSON(fiber.Map{
		"photos": toPhotoResponses(context.Background(), filtered),
		"total":  len(filtered),
	})
}

// POST /api/albums/:id/photos - Uploader une photo dans le bucket et l'enregistrer
func HandlerUploadPhoto(c fiber.Ctx) error {
	albumName := c.Params("id")
	if albumName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "album name is required"})
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

	key := fmt.Sprintf("%s/%s%s", albumName, uuid.NewString(), filepath.Ext(fileHeader.Filename))

	_, err = config.StorageClient.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:      aws.String(config.StorageBucket),
		Key:         aws.String(key),
		Body:        file,
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to upload photo"})
	}

	uploader, _ := c.Locals("user_email").(string)

	newPhoto := models.Photo{
		ID:         uuid.NewString(),
		FileName:   fileHeader.Filename,
		Key:        key,
		AlbumName:  albumName,
		Uploader:   uploader,
		UploadedAt: time.Now(),
	}

	metadata, _ := loadMetadata()
	metadata.Photos = append(metadata.Photos, newPhoto)

	if err := saveMetadata(metadata); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to save photo metadata"})
	}

	return c.Status(fiber.StatusCreated).JSON(toPhotoResponse(context.Background(), newPhoto))
}

// DELETE /api/photos/:id - Supprimer une photo
func HandlerDeletePhoto(c fiber.Ctx) error {
	id := c.Params("id")

	metadata, _ := loadMetadata()

	// Filtrer les photos (supprimer celle qui match l'ID)
	var filtered []models.Photo
	for _, p := range metadata.Photos {
		if p.ID != id {
			filtered = append(filtered, p)
		}
	}

	metadata.Photos = filtered

	// Sauvegarder les métadonnées mises à jour
	saveMetadata(metadata)

	return c.Status(fiber.StatusOK).JSON(map[string]string{"message": "Supprimé"})
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
	metadata, _ := loadMetadata()

	accumulators := make(map[string]*albumAccumulator)
	var order []string
	for _, photo := range metadata.Photos {
		acc, ok := accumulators[photo.AlbumName]
		if !ok {
			acc = &albumAccumulator{Name: photo.AlbumName, CoverKey: photo.Key}
			accumulators[photo.AlbumName] = acc
			order = append(order, photo.AlbumName)
		}
		acc.Count++
	}

	ctx := context.Background()
	albums := make([]AlbumSummary, 0, len(order))
	for _, name := range order {
		acc := accumulators[name]
		coverURL, err := config.PresignPhotoURL(ctx, acc.CoverKey)
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

// GET /api/albums/:id - Filtrer les photos par album
func HandlerGetAlbum(c fiber.Ctx) error {
	albumName := c.Params("id")

	metadata, _ := loadMetadata()

	// Filtrer les photos par album
	var filtered []models.Photo
	for _, p := range metadata.Photos {
		if p.AlbumName == albumName {
			filtered = append(filtered, p)
		}
	}

	return c.JSON(fiber.Map{
		"photos": toPhotoResponses(context.Background(), filtered),
		"total":  len(filtered),
	})
}
