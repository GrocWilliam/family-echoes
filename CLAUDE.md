# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project overview

Echoes is a private family photobook app: family members sign in via a Firebase magic link
(passwordless email link), then view/upload photos organized into albums. Photos are stored in a
Railway bucket (S3-compatible object storage), which is also the only datastore.

The repo has two independent apps, each with its own dependency tree and Dockerfile:

- `backend/` — Go + Fiber v3 API
- `frontend/` — SvelteKit 5 (runes mode) + Tailwind v4

There is no root-level build; work in each subdirectory independently.

## Commands

### Backend (`backend/`)

- Run in dev mode with hot reload: `mage dev` (wraps `air main.go`; requires the `air` binary and
  the `mage` CLI installed)
- Build: `mage build` (runs `go build -o Echoes .`) or plain `go build ./...`
- Install/update deps: `go mod tidy`
- Vet: `go vet ./...`
- No test suite exists yet.

The backend loads `backend/.env` on startup via `godotenv` and calls `log.Fatal` if it's missing —
a `.env` file must exist (see below) even if empty of secrets.

### Frontend (`frontend/`)

Package manager is pnpm (`pnpm-lock.yaml`, `pnpm-workspace.yaml`), though npm scripts also work.

- Dev server: `npm run dev` (or `pnpm dev`)
- Typecheck: `npm run check` (runs `svelte-kit sync && svelte-check`)
- Lint/format check: `npm run lint` (prettier --check + eslint)
- Format: `npm run format`
- Build: `npm run build`

## Environment variables

Both apps read from a local `.env` file (never commit real secrets into these):

**`backend/.env`**
- `FIREBASE_PROJECT_ID`
- `GOOGLE_APPLICATION_CREDENTIALS` — path to a Firebase service-account JSON file
- `ALLOWED_EMAILS` — comma-separated allowlist of family emails permitted to request a magic link

**`frontend/.env`**
- `VITE_FIREBASE_API_KEY`, `VITE_FIREBASE_AUTH_DOMAIN`, `VITE_FIREBASE_PROJECT_ID`, `VITE_FIREBASE_APP_ID`
- `VITE_BACKEND_URL` — base URL of the Go API (defaults to `http://localhost:8080`, `frontend/src/lib/config.ts`)

In the Firebase console, Email Link sign-in must be enabled and both dev/prod origins added to
authorized domains, or `sendSignInLinkToEmail` will fail.

## Architecture

### Auth flow (magic link)

Email delivery is handled entirely by the **Firebase client SDK** (`sendSignInLinkToEmail`), not
by the Go backend — there is no SMTP/SendGrid integration. The backend's role is gatekeeping and
verification only:

1. Frontend calls `POST /api/auth/magic-link` (`backend/handlers/handler_auth.go`) with an email.
   The handler checks it against the `ALLOWED_EMAILS` allowlist and returns 403 if not invited.
2. Only if allowed, the frontend (`frontend/src/lib/authentication.ts`) calls Firebase's
   `sendSignInLinkToEmail`, storing the email in `localStorage` for later completion.
3. The emailed link lands on `/auth/callback` (`frontend/src/routes/auth/callback/+page.svelte`),
   which calls `completeSignInFromLink()` to finish sign-in and redirects to `/album`.
4. For subsequent API calls, the frontend attaches the Firebase ID token as
   `Authorization: Bearer <token>`. The backend's `middlewares.FirebaseAuth`
   (`backend/handlers/middlewares/firebase_middleware.go`) verifies it via the Admin SDK
   (`config.AuthClient`, initialized in `backend/config/firebase.go`) and sets
   `c.Locals("user_id")` / `c.Locals("user_email")` for handlers.

`frontend/src/lib/authentication.ts` also exposes a Svelte `authStore` kept in sync via
`onAuthStateChanged`, and `getIdToken()` for attaching auth to fetches.

### Backend routing/data

- `backend/routes/routes.go` wires all routes under `/api`. `/api/auth/*` is public;
  `/api/albums/*` is gated behind `middlewares.FirebaseAuth`.
- There is no database: `backend/store/photo_store.go` rebuilds albums by listing the bucket.
  Object keys are `<album name>/<uuid><ext>` (the album exists only as a key prefix); the
  uploader, original filename and exact upload time are stored as S3 user metadata
  (query-escaped) and read back with `HeadObject`, cached in memory since objects are never
  overwritten.
- Album names arrive percent-encoded from Fiber params (`UnescapePath` is off on purpose);
  `albumParam` in `backend/handlers/handler_photo.go` decodes and validates them.
- The API never exposes bucket keys — photos are served via presigned URLs
  (`config.PresignPhotoURL`).

### Frontend structure

- SvelteKit with `ssr = false` (`src/routes/+layout.ts`) — this is a client-rendered SPA, not
  server-rendered.
- Runes mode is forced project-wide via `vite.config.ts` (except inside `node_modules`).
- Routes: `/auth/login`, `/auth/callback`, `/album`. `frontend/src/lib/firebase.ts` initializes the
  Firebase app/client used by `authentication.ts`.
