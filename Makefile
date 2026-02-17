# Auto-detect mise tool paths so `make` works without manual activation
MISE_PATHS := $(shell mise bin-paths 2>/dev/null | tr '\n' ':')
export PATH := $(MISE_PATHS)$(PATH)

.PHONY: run-backend build-backend build-battery-compare build-train-predictor build-sample-predict build-fetch-prices build-load-analysis build-ha-fetch-history build-anomaly-detect build-voltage-analysis test-backend test-backend-v lint-backend \
       install-frontend dev-frontend build-frontend test-frontend lint-frontend \
       dev test lint build clean compare train sample-predict fetch-prices load-analysis ha-fetch-history anomaly-detect voltage-analysis \
       docker-build docker-up docker-down \
       sql-stats \
       r-analysis r-clean

# Backend
run-backend:
	cd backend && go run ./cmd/server/main.go -input-dir ../input -frontend-dir ../frontend/build

build-backend:
	cd backend && go build -o ../bin/server ./cmd/server

build-battery-compare:
	cd backend && go build -o ../bin/battery-compare ./cmd/battery-compare

compare: build-battery-compare
	./bin/battery-compare -input-dir input

build-train-predictor:
	cd backend && go build -o ../bin/train-predictor ./cmd/train-predictor

train: build-train-predictor
	./bin/train-predictor -stats input/stats/export.csv \
		-temp-output model/temperature.json \
		-power-output model/grid_power.json \
		-epochs 1000 -lr 0.001

build-sample-predict:
	cd backend && go build -o ../bin/sample-predict ./cmd/sample-predict

sample-predict: build-sample-predict
	./bin/sample-predict -temp-model model/temperature.json -power-model model/grid_power.json

build-load-analysis:
	cd backend && go build -o ../bin/load-analysis ./cmd/load-analysis

load-analysis: build-load-analysis
	./bin/load-analysis -input-dir input

build-fetch-prices:
	cd backend && go build -o ../bin/fetch-prices ./cmd/fetch-prices

fetch-prices: build-fetch-prices
	./bin/fetch-prices

build-ha-fetch-history:
	cd backend && go build -o ../bin/ha-fetch-history ./cmd/ha-fetch-history

ha-fetch-history: build-ha-fetch-history
	./bin/ha-fetch-history

build-anomaly-detect:
	cd backend && go build -o ../bin/anomaly-detect ./cmd/anomaly-detect

anomaly-detect: build-anomaly-detect
	./bin/anomaly-detect -input-dir input -temp-model model/temperature.json -power-model model/grid_power.json

build-voltage-analysis:
	cd backend && go build -o ../bin/voltage-analysis ./cmd/voltage-analysis

voltage-analysis: build-voltage-analysis
	./bin/voltage-analysis -input-dir input

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

# R analysis
r-analysis:
	$(MAKE) -C analysis/r

r-clean:
	$(MAKE) -C analysis/r clean

# Print SQL query for fetching sensor statistics from Home Assistant DB
sql-stats:
	@cd backend && go run ./cmd/sql-stats
