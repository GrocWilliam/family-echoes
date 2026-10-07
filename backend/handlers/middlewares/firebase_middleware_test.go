package middlewares

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
)

func TestRequireAdmin(t *testing.T) {
	t.Setenv("ALLOWED_EMAILS", "mamie@example.com,papi@example.com")
	t.Setenv("ADMIN_EMAILS", "mamie@example.com")

	cases := map[string]int{
		"mamie@example.com": fiber.StatusOK,
		"papi@example.com":  fiber.StatusForbidden,
		"":                  fiber.StatusForbidden,
	}
	for email, want := range cases {
		app := fiber.New()
		// Stands in for FirebaseAuth, which needs a real Firebase project.
		app.Use(func(c fiber.Ctx) error {
			c.Locals("user_email", email)
			return c.Next()
		})
		app.Post("/upload", RequireAdmin, func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

		res, err := app.Test(httptest.NewRequest("POST", "/upload", nil))
		if err != nil {
			t.Fatal(err)
		}
		if res.StatusCode != want {
			t.Errorf("RequireAdmin for %q = %d, want %d", email, res.StatusCode, want)
		}
	}
}
