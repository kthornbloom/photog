.PHONY: dev dev-backend dev-frontend build clean docker

# Development: run mock API + frontend concurrently with one command
dev:
	@echo Starting mock API (port 3001) and Vite frontend (port 5173)...
	$(MAKE) dev-mock & $(MAKE) dev-frontend & wait

dev-mock:
	cd ui && node mock-server.js

dev-frontend:
	cd ui && npm run dev

# Run the real Go backend (requires CGO / C compiler)
dev-backend:
	go run . --config=config.dev.yaml

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
