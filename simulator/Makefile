.PHONY: build test lint dev clean \
       build-backend build-frontend \
       test-backend test-frontend \
       run compare train sample-predict load-analysis fetch-prices ha-fetch-history anomaly-detect voltage-analysis sql-stats

# Build
build: build-backend build-frontend

build-backend:
	cd backend && go build -o ../../bin/server ./cmd/server
	cd backend && go build -o ../../bin/battery-compare ./cmd/battery-compare
	cd backend && go build -o ../../bin/load-analysis ./cmd/load-analysis
	cd backend && go build -o ../../bin/fetch-prices ./cmd/fetch-prices
	cd backend && go build -o ../../bin/ha-fetch-history ./cmd/ha-fetch-history
	cd backend && go build -o ../../bin/anomaly-detect ./cmd/anomaly-detect
	cd backend && go build -o ../../bin/voltage-analysis ./cmd/voltage-analysis
	cd backend && go build -o ../../bin/train-predictor ./cmd/train-predictor
	cd backend && go build -o ../../bin/sample-predict ./cmd/sample-predict

build-frontend:
	cd frontend && npm run build

# Test
test: test-backend test-frontend

test-backend:
	cd backend && go test ./...

test-frontend:
	cd frontend && npm test

# Lint
lint:
	cd backend && golangci-lint run ./...
	cd frontend && npm run lint && npm run check

# Development
dev:
	$(MAKE) dev-backend & $(MAKE) dev-frontend & wait

dev-backend:
	cd .. && air

dev-frontend:
	cd frontend && npm run dev

# Run
run: build-backend
	cd .. && ./bin/server -input-dir input -frontend-dir simulator/frontend/build

# CLI tools
compare:
	cd .. && ./bin/battery-compare -input-dir input

train:
	cd .. && ./bin/train-predictor -stats input/stats/export.csv \
		-temp-output simulator/backend/model/temperature.json \
		-power-output simulator/backend/model/grid_power.json \
		-epochs 1000 -lr 0.001

sample-predict:
	cd .. && ./bin/sample-predict \
		-temp-model simulator/backend/model/temperature.json \
		-power-model simulator/backend/model/grid_power.json

load-analysis:
	cd .. && ./bin/load-analysis -input-dir input

fetch-prices:
	cd .. && ./bin/fetch-prices

ha-fetch-history:
	cd .. && ./bin/ha-fetch-history

anomaly-detect:
	cd .. && ./bin/anomaly-detect -input-dir input \
		-temp-model simulator/backend/model/temperature.json \
		-power-model simulator/backend/model/grid_power.json

voltage-analysis:
	cd .. && ./bin/voltage-analysis -input-dir input

sql-stats:
	@cd backend && go run ./cmd/sql-stats

clean:
	rm -rf ../bin/ frontend/build/ frontend/.svelte-kit/
