package config

import (
	"context"
	"log"
	"os"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
	"google.golang.org/api/option"
)

var AuthClient *auth.Client

func InitializeFirebase() {
	ctx := context.Background()

	opts := []option.ClientOption{}
	switch {
	case os.Getenv("GOOGLE_APPLICATION_CREDENTIALS_JSON") != "":
		// Deployment: paste the service account key's JSON content directly as
		// an env var. There's no file to upload on most hosts (e.g. Railway),
		// and it must never be committed to git.
		json := []byte(os.Getenv("GOOGLE_APPLICATION_CREDENTIALS_JSON"))
		opts = append(opts, option.WithAuthCredentialsJSON(option.ServiceAccount, json))
	case os.Getenv("GOOGLE_APPLICATION_CREDENTIALS") != "":
		// Local dev: path to the gitignored service account file on disk.
		credsPath := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
		opts = append(opts, option.WithAuthCredentialsFile(option.ServiceAccount, credsPath))
	}

	app, err := firebase.NewApp(ctx, &firebase.Config{ProjectID: os.Getenv("FIREBASE_PROJECT_ID")}, opts...)
	if err != nil {
		log.Fatalf("error initializing firebase app: %v", err)
	}

	client, err := app.Auth(ctx)
	if err != nil {
		log.Fatalf("error initializing firebase auth client: %v", err)
	}

	AuthClient = client
}
