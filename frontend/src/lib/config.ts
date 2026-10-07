// Must match the Go API's default port (backend/main.go) for local dev.
export const BACKEND_URL = import.meta.env.VITE_BACKEND_URL ?? 'http://localhost:8080';
