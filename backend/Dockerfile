# syntax=docker/dockerfile:1

FROM golang:1.26.4-alpine AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/bin/echoes .

FROM alpine:latest
RUN apk add --no-cache ca-certificates && \
    adduser -D -u 10001 echoes
WORKDIR /app
COPY --from=builder /app/bin/echoes ./echoes
USER echoes

# Railway (and most PaaS) assign the listen port via $PORT at runtime; 8080 is
# also the local-dev fallback baked into main.go.
ENV PORT=8080
EXPOSE 8080
CMD ["./echoes"]
