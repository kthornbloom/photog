# =============================================================================
# Photog - Multi-stage Dockerfile
# Produces a single minimal container with embedded frontend
# =============================================================================

# ---- Stage 1: Build frontend ----
FROM node:20-alpine AS frontend-builder
WORKDIR /build/ui

COPY ui/package.json ui/package-lock.json ./
RUN npm ci --production=false

COPY ui/ ./
RUN npm run build

# ---- Stage 2: Build Go backend ----
FROM golang:1.22-alpine AS backend-builder
RUN apk add --no-cache gcc musl-dev

WORKDIR /build
COPY go.mod go.sum* ./
RUN go mod tidy && go mod download

COPY . .
COPY --from=frontend-builder /build/ui/dist ./ui/dist

RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-s -w" -o /photog .

# ---- Stage 3: Final minimal image ----
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=backend-builder /photog /usr/local/bin/photog
COPY --from=frontend-builder /build/ui/dist ./ui/dist

# Default volumes
VOLUME ["/photos", "/cache"]

# Default config
ENV PHOTOG_PHOTO_PATHS="/photos"
ENV PHOTOG_CACHE_DIR="/cache"

EXPOSE 8080

ENTRYPOINT ["photog"]
CMD ["--auto-index=true"]
