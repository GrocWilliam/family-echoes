package routes

import (
	"echoes.com/m/handlers"
	"echoes.com/m/handlers/middlewares"
	"github.com/gofiber/fiber/v3"
)

func Use(app *fiber.App) {
	api := app.Group("/api")

	auth := api.Group("/auth")
	auth.Post("/magic-link", handlers.HandlerSendMagicLink)

	api.Get("/me", middlewares.FirebaseAuth, handlers.HandlerMe)

	albums := api.Group("/albums", middlewares.FirebaseAuth)
	albums.Get("/", handlers.HandlerGetAlbums)
	albums.Get("/:id", handlers.HandlerGetAlbum)
	albums.Get("/:id/photos", handlers.HandlerGetPhotos)
	albums.Post("/:id/photos", middlewares.RequireAdmin, handlers.HandlerUploadPhoto)
}
