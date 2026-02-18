# Root Makefile — orchestrates all projects
#
# Each project has its own Makefile:
#   simulator/       — Go backend + Svelte frontend
#   analysis/        — R statistical analysis
#   forecast/        — Python ML models + optimization
#   battery_optimizer_wasm/ — Rust WASM battery optimizer

MISE_PATHS := $(shell mise bin-paths 2>/dev/null | tr '\n' ':')
export PATH := $(MISE_PATHS)$(PATH)

.PHONY: build test lint dev clean \
        docker-build docker-up docker-down \
        ha-fetch-history fetch-prices train compare load-analysis

# ── Build all projects ──────────────────────────────────────────────────────

build:
	$(MAKE) -C simulator build

# ── Test all projects ───────────────────────────────────────────────────────

test:
	$(MAKE) -C simulator test
	$(MAKE) -C forecast test

# ── Lint all projects ───────────────────────────────────────────────────────

lint:
	$(MAKE) -C simulator lint

# ── Development (simulator hot-reload) ──────────────────────────────────────

dev:
	$(MAKE) -C simulator dev

# ── CLI tools (delegated to simulator) ─────────────────────────────────────

ha-fetch-history fetch-prices train compare load-analysis:
	$(MAKE) -C simulator $@

# ── Docker ──────────────────────────────────────────────────────────────────

docker-build:
	docker compose build

docker-up:
	docker compose up

docker-down:
	docker compose down

# ── Clean ───────────────────────────────────────────────────────────────────

clean:
	$(MAKE) -C simulator clean
	$(MAKE) -C forecast clean
	$(MAKE) -C analysis clean
	rm -rf tmp/
