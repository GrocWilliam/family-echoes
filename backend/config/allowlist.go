package config

import (
	"os"
	"strings"
)

func emailInEnvList(envVar string, email string) bool {
	list := os.Getenv(envVar)
	if list == "" {
		return false
	}

	email = strings.ToLower(strings.TrimSpace(email))
	for _, e := range strings.Split(list, ",") {
		if strings.ToLower(strings.TrimSpace(e)) == email {
			return true
		}
	}

	return false
}

// IsEmailAllowed is the single source of truth for who is invited to the family
// album (sign in + view). It must be enforced wherever identity is actually
// established (the auth middleware), not just at the point where a magic link
// is requested — that request goes through the client-side Firebase SDK, which
// anyone can call directly and bypass any check that only lives in the
// /api/auth/magic-link handler.
func IsEmailAllowed(email string) bool {
	return emailInEnvList("ALLOWED_EMAILS", email)
}

// IsEmailAdmin is the source of truth for who may upload photos. It's a
// separate, smaller list from ALLOWED_EMAILS: every admin must also be an
// allowed viewer, but not every viewer should be able to upload.
func IsEmailAdmin(email string) bool {
	return emailInEnvList("ADMIN_EMAILS", email)
}
