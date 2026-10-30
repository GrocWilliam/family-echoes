# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project overview

Echoes is a private family photobook app: family members sign in via a Firebase magic link
(passwordless email link), then view/upload photos organized into albums. Photos are stored in a
Railway bucket (S3-compatible object storage), which is also the only datastore.

The repo has two independent apps, each with its own dependency tree and Dockerfile:

- `backend/` — Go + Fiber v3 API
- `frontend/` — SvelteKit 5 (runes mode) + Tailwind v4

There is no root-level build; work in each subdirectory independently. CI
(`.github/workflows/ci.yml`) runs the backend and frontend checks below on every push to `main`
and on pull requests.

## Commands

### Backend (`backend/`)

- Run in dev mode with hot reload: `mage dev` (wraps `air main.go`; requires the `air` binary and
  the `mage` CLI installed)
- Build: `mage build` (runs `go build -o Echoes .`) or plain `go build ./...`
- Test: `mage test` or `go test ./...`; single test: `go test ./store -run TestRenameAlbum`
- Everything CI runs (gofmt, vet, tests): `mage check`
- Install/update deps: `go mod tidy`

Store and handler tests run against an in-memory S3 server (`backend/internal/s3fake`, built on
gofakes3), so they need no credentials or network. `FirebaseAuth` itself is not unit-tested (it
needs a real Firebase project); handler tests mount routes without it and set `user_email` directly.

The backend loads `backend/.env` via `godotenv` if present (in deployment, variables come from the
platform), then `config.CheckEnv` exits listing any missing required variable.

### Frontend (`frontend/`)

Package manager is pnpm (`pnpm-lock.yaml`, `pnpm-workspace.yaml`), though npm scripts also work.

- Dev server: `npm run dev` (or `pnpm dev`)
- Typecheck: `npm run check` (runs `svelte-kit sync && svelte-check`)
- Lint/format check: `npm run lint` (prettier --check + eslint)
- Format: `npm run format`
- Build: `npm run build`

## Environment variables

Both apps read from a local, gitignored `.env`; `backend/.env.example` and
`frontend/.env.example` document every variable. When adding one, update the example file (and
`requiredEnv` in `backend/config/env.go` if the API can't run without it).

**Backend**
- `FIREBASE_PROJECT_ID`, plus `GOOGLE_APPLICATION_CREDENTIALS` (path to a service-account JSON,
  local dev) or `GOOGLE_APPLICATION_CREDENTIALS_JSON` (its contents, deployment)
- `ALLOWED_EMAILS` — who may sign in and view; `ADMIN_EMAILS` — who may upload/delete/rename
- `FRONTEND_URL` — comma-separated CORS origins
- `S3_ENDPOINT`, `S3_REGION`, `S3_BUCKET`, `S3_ACCESS_KEY_ID`, `S3_SECRET_ACCESS_KEY`
- `PORT` — defaults to 8080

**Frontend** (baked in at build time — Docker needs them as `--build-arg`)
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
   which calls `completeSignInFromLink()` to finish sign-in and redirects to `/`.
4. For subsequent API calls, the frontend attaches the Firebase ID token as
   `Authorization: Bearer <token>`. The backend's `middlewares.FirebaseAuth`
   (`backend/handlers/middlewares/firebase_middleware.go`) verifies it via the Admin SDK
   (`config.AuthClient`, initialized in `backend/config/firebase.go`) and sets
   `c.Locals("user_id")` / `c.Locals("user_email")` for handlers. It re-checks `ALLOWED_EMAILS`:
   step 1 is only a courtesy, since anyone can call Firebase's client SDK directly.
5. `middlewares.RequireAdmin` gates writes on `ADMIN_EMAILS`. The frontend learns the role from
   `GET /api/me` purely to show or hide admin UI.

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
- Thumbnails (~480px JPEG) are generated in the browser (`frontend/src/lib/images.ts`, which also
  converts HEIC to JPEG where the browser can decode it) and uploaded alongside the photo; they're
  stored at `<album>/thumbs/<uuid>.jpg` and exposed as `thumb_url`. Photos without one fall back to
  the full `url`.
- Renaming an album (`PATCH /api/albums/:id`) copies every object to the new prefix, then deletes
  the old ones — not atomic, but retrying finishes a partial rename.
- Album names arrive percent-encoded from Fiber params (`UnescapePath` is off on purpose);
  `albumParam` in `backend/handlers/handler_photo.go` decodes and validates them.
- The API never exposes bucket keys — photos are served via presigned URLs
  (`config.PresignPhotoURL`).

### Frontend structure

- SvelteKit with `ssr = false` (`src/routes/+layout.ts`) — this is a client-rendered SPA, not
  server-rendered.
- Runes mode is forced project-wide via `vite.config.ts` (except inside `node_modules`).
- Routes: `/` (album list), `/album/[id]` (photos; `id` is the URL-encoded album name),
  `/bienvenue` (public welcome + FAQ, also the signed-in "Aide" page), `/auth/login`,
  `/auth/callback`. The root `+layout.svelte` redirects signed-out users to `/bienvenue`
  everywhere except `/bienvenue` and `/auth/*`.
- Users are mostly non-technical: UI text is French, plain, and says what to do next.
  Per-feature help uses `lib/components/Tip.svelte` (dismissible, remembered in
  `localStorage`; `/bienvenue` can show them all again).
- `frontend/src/lib/firebase.ts` initializes the Firebase app/client used by `authentication.ts`;
  `frontend/src/lib/api.ts` (`apiFetch`) attaches the ID token to backend calls.
