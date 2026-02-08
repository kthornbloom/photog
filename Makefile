.PHONY: dev dev-backend dev-frontend build clean docker

# Development: run backend and frontend separately
dev-backend:
	go run . --config=config.yaml

dev-frontend:
	cd ui && npm run dev

dev:
	@echo "Run these in separate terminals:"
	@echo "  make dev-backend"
	@echo "  make dev-frontend"

# Production build: frontend → embedded in Go binary
build: build-frontend build-backend

build-frontend:
	cd ui && npm ci && npm run build

build-backend:
	CGO_ENABLED=1 go build -ldflags="-s -w" -o photog .

# Docker
docker:
	docker build -t photog .

docker-run:
	docker compose up -d

# Clean
clean:
	rm -f photog photog.exe
	rm -rf ui/dist ui/node_modules
