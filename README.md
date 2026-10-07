# Echoes

A private family photobook. Invited family members sign in with a magic link sent to their email,
then browse photos organized into albums; a smaller set of admins can upload, delete and rename.

- `backend/` — Go + Fiber API. Verifies Firebase sign-in tokens, enforces the email allowlists and
  stores photos in an S3-compatible bucket (which is also the only datastore).
- `frontend/` — SvelteKit single-page app (Tailwind), served by nginx in production.

## Local setup

Prerequisites: Go (see `backend/go.mod`), Node 22+ with pnpm, a Firebase project with **Email link**
sign-in enabled (and your dev origin in its authorized domains), and an S3-compatible bucket.

```sh
# Backend — http://localhost:8080
cd backend
cp .env.example .env               # fill in Firebase, allowlists and bucket credentials
go run .                           # or `mage dev` for hot reload (needs mage + air)

# Frontend — http://localhost:5173
cd frontend
cp .env.example .env               # fill in the Firebase web app config
pnpm install
pnpm dev
```

## Checks

```sh
cd backend  && mage check          # gofmt, go vet, go test (no credentials needed)
cd frontend && pnpm check && pnpm lint && pnpm build
```

CI runs the same on every pull request. See `CLAUDE.md` for architecture notes.

## Deployment

Each app has its own Dockerfile. The backend reads its configuration from environment variables at
runtime; the frontend's `VITE_*` variables are compiled in, so pass them as `--build-arg`.
