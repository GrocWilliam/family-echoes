package handlers

import (
	"strings"

	"echoes.com/m/config"
	"github.com/gofiber/fiber/v3"
)

type SendMagicLinkRequest struct {
	Email string `json:"email"`
}

// POST /api/auth/magic-link - Vérifie que l'email fait partie de la famille invitée.
// L'envoi effectif du lien est fait côté client via le SDK Firebase, qui gère
// l'email de connexion sans infrastructure SMTP dédiée.
func HandlerSendMagicLink(c fiber.Ctx) error {
	req := new(SendMagicLinkRequest)
	if err := c.Bind().Body(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	email := strings.ToLower(strings.TrimSpace(req.Email))
	if email == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "email is required"})
	}

	if !config.IsEmailAllowed(email) {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "this email is not invited to the family album"})
	}

	return c.JSON(fiber.Map{"allowed": true})
}

// GET /api/me - Renvoie l'identité et le rôle de l'utilisateur courant.
// Sert uniquement à adapter l'UI (afficher ou non le bouton d'upload) ; le
// contrôle d'accès réel est fait par le middleware RequireAdmin côté serveur.
func HandlerMe(c fiber.Ctx) error {
	email, _ := c.Locals("user_email").(string)

	return c.JSON(fiber.Map{
		"email":    email,
		"is_admin": config.IsEmailAdmin(email),
	})
}
