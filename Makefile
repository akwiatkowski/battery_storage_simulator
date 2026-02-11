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

# All-in-one
dev:
	$(MAKE) run-backend & $(MAKE) dev-frontend & wait

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
	rm -rf bin/ frontend/build/ frontend/.svelte-kit/
