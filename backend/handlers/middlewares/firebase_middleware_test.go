package middlewares

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
)

func TestAllowedEmail(t *testing.T) {
	t.Setenv("ALLOWED_EMAILS", "mamie@example.com")

	cases := []struct {
		name   string
		claims map[string]any
		want   bool
	}{
		{"verified and invited", map[string]any{"email": "mamie@example.com", "email_verified": true}, true},
		// e.g. a password account created for an invited address
		{"unverified", map[string]any{"email": "mamie@example.com", "email_verified": false}, false},
		{"no verified claim", map[string]any{"email": "mamie@example.com"}, false},
		{"not invited", map[string]any{"email": "other@example.com", "email_verified": true}, false},
		{"no email", map[string]any{"email_verified": true}, false},
	}
	for _, tc := range cases {
		if _, got := allowedEmail(tc.claims); got != tc.want {
			t.Errorf("%s: allowed = %v, want %v", tc.name, got, tc.want)
		}
	}
}

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
