# Auto-detect mise tool paths so `make` works without manual activation
MISE_PATHS := $(shell mise bin-paths 2>/dev/null | tr '\n' ':')
export PATH := $(MISE_PATHS)$(PATH)

.PHONY: run-backend build-backend test-backend test-backend-v lint-backend \
       install-frontend dev-frontend build-frontend test-frontend lint-frontend \
       dev test lint build clean \
       docker-build docker-up docker-down

# Backend
run-backend:
	cd backend && go run ./cmd/server/main.go -input-dir ../input -frontend-dir ../frontend/build

build-backend:
	cd backend && go build -o ../bin/server ./cmd/server/main.go

test-backend:
	cd backend && go test ./...

test-backend-v:
	cd backend && go test -v ./...

lint-backend:
	cd backend && golangci-lint run ./...

# Frontend
install-frontend:
	cd frontend && npm install

dev-frontend:
	cd frontend && npm run dev

build-frontend:
	cd frontend && npm run build

test-frontend:
	cd frontend && npm test

lint-frontend:
	cd frontend && npm run lint && npm run check

# Development — full hot-reload
# Backend: air watches .go files, auto-rebuilds and restarts server
# Frontend: vite dev server with HMR
dev:
	$(MAKE) dev-backend & $(MAKE) dev-frontend & wait

dev-backend:
	air

# All-in-one
test: test-backend test-frontend

lint: lint-backend lint-frontend

build: build-backend build-frontend

# Docker
docker-build:
	docker compose build

docker-up:
	docker compose up

docker-down:
	docker compose down

clean:
	rm -rf bin/ tmp/ frontend/build/ frontend/.svelte-kit/
