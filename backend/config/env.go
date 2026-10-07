package config

import (
	"log"
	"os"
	"strings"
)

// requiredEnv lists settings the API can't do anything useful without. A
// missing one otherwise surfaces much later and far from its cause (e.g. an
// empty FRONTEND_URL makes the CORS middleware panic on an "invalid origin").
var requiredEnv = []string{
	"FIREBASE_PROJECT_ID",
	"ALLOWED_EMAILS",
	"FRONTEND_URL",
	"S3_REGION",
	"S3_BUCKET",
	"S3_ACCESS_KEY_ID",
	"S3_SECRET_ACCESS_KEY",
}

// CheckEnv exits with every missing required variable named at once, rather
// than one per restart.
func CheckEnv() {
	var missing []string
	for _, name := range requiredEnv {
		if strings.TrimSpace(os.Getenv(name)) == "" {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		log.Fatalf("missing required environment variables: %s (see backend/.env.example)", strings.Join(missing, ", "))
	}
}

// SplitList parses a comma-separated env value, dropping blanks so a
// trailing comma or "a, b" spacing doesn't produce empty entries.
func SplitList(value string) []string {
	var items []string
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			items = append(items, item)
		}
	}
	return items
}
