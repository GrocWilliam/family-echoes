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
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
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

	log.Fatal(app.Listen(":3000"))
}
