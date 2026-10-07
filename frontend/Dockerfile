# syntax=docker/dockerfile:1

FROM node:22-alpine AS builder
WORKDIR /app

RUN corepack enable

COPY package.json pnpm-lock.yaml pnpm-workspace.yaml .npmrc ./
RUN pnpm install --frozen-lockfile

COPY . .

# This is a static SPA (ssr = false): these are baked into the JS bundle at
# build time, so they must be passed as --build-arg, not just set at runtime.
ARG VITE_FIREBASE_API_KEY
ARG VITE_FIREBASE_AUTH_DOMAIN
ARG VITE_FIREBASE_PROJECT_ID
ARG VITE_FIREBASE_APP_ID
ARG VITE_BACKEND_URL
ENV VITE_FIREBASE_API_KEY=$VITE_FIREBASE_API_KEY \
    VITE_FIREBASE_AUTH_DOMAIN=$VITE_FIREBASE_AUTH_DOMAIN \
    VITE_FIREBASE_PROJECT_ID=$VITE_FIREBASE_PROJECT_ID \
    VITE_FIREBASE_APP_ID=$VITE_FIREBASE_APP_ID \
    VITE_BACKEND_URL=$VITE_BACKEND_URL

RUN pnpm run build

FROM nginx:1.30-alpine
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=builder /app/build /usr/share/nginx/html

# Fixed port rather than substituting Railway's dynamic $PORT at container
# start: one less moving part to debug. In the Railway service's Networking
# settings, make sure the target port is set to 80 (should auto-detect from
# EXPOSE below, since Dockerfile builds are picked up from this exact name now).
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
