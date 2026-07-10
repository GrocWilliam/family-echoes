package middlewares

import (
	"context"
	"strings"

	"echoes.com/m/config"
	"github.com/gofiber/fiber/v3"
)

// FirebaseAuth vérifie le ID token Firebase envoyé dans l'en-tête Authorization
// et expose l'UID / email de l'utilisateur aux handlers suivants.
func FirebaseAuth(c fiber.Ctx) error {
	header := c.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "missing bearer token"})
	}

	token := strings.TrimPrefix(header, "Bearer ")

	decoded, err := config.AuthClient.VerifyIDToken(context.Background(), token)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid or expired token"})
	}

	email, _ := decoded.Claims["email"].(string)

	// The token being valid only proves Firebase issued it — anyone can create a
	// Firebase account via email-link sign-in with any address. The family
	// allowlist is the actual access control and must be checked here, since this
	// is the one step a client can't bypass.
	if !config.IsEmailAllowed(email) {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "this account is not invited to the family album"})
	}

	c.Locals("user_id", decoded.UID)
	c.Locals("user_email", email)

	return c.Next()
}

// RequireAdmin must run after FirebaseAuth. It restricts a route to the
// smaller ADMIN_EMAILS list (e.g. uploading photos), separate from the
// broader ALLOWED_EMAILS list that only grants viewing access.
func RequireAdmin(c fiber.Ctx) error {
	email, _ := c.Locals("user_email").(string)

	if !config.IsEmailAdmin(email) {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "only album admins can do this"})
	}

	return c.Next()
}
