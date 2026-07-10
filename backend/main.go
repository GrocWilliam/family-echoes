package main

import (
	"log"
	"os"
	"strings"

	"echoes.com/m/config"
	"echoes.com/m/routes"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/joho/godotenv"
)

func main() {
	// Only present locally — in deployment, env vars come from the platform
	// directly (e.g. Railway), so a missing .env file here is expected, not fatal.
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using the environment as-is")
	}

	config.InitializeFirebase()
	config.InitializeStorage()

	app := fiber.New(fiber.Config{
		BodyLimit: 25 << 20, // photos can be up to 20MB, leave headroom for multipart overhead
	})

	app.Use(cors.New(cors.Config{
		AllowOrigins: strings.Split(os.Getenv("FRONTEND_URL"), ","),
		AllowHeaders: []string{"Origin", "Content-Type", "Accept", "Authorization"},
		AllowMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
	}))

	routes.Use(app)

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	log.Fatal(app.Listen(":" + port))
}
