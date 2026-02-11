# Energy Simulator

Home energy simulator webapp that replays historical energy data via WebSocket.

## Quick Start

```bash
make dev          # backend :8080 + frontend :5173
make test         # all tests
make lint         # all linters
docker compose up # production build
```

## Architecture

- **Backend:** Go 1.25, standard library `net/http` + `gorilla/websocket`
- **Frontend:** Svelte 5 + SvelteKit, layerchart for visualizations
- **Communication:** All data flows via WebSocket messages (no REST API)
- **Data:** CSV files in `input/` loaded on startup

## Project Layout

- `backend/cmd/server/main.go` — entry point
- `backend/internal/model/` — domain types (Reading, Sensor, SensorType)
- `backend/internal/ingest/` — CSV parsing (Home Assistant format)
- `backend/internal/store/` — in-memory data store
- `backend/internal/simulator/` — time-based replay engine
- `backend/internal/ws/` — WebSocket hub, handler, message types
- `frontend/src/lib/ws/` — WebSocket client + message types
- `frontend/src/lib/stores/` — Svelte 5 reactive state
- `frontend/src/lib/components/` — dashboard components
- `input/` — CSV data files (committed)
- `testdata/` — test fixture CSVs

## Conventions

- Go tests: co-located `_test.go` files, use `testify` for assertions
- Frontend tests: `vitest` + `@testing-library/svelte`
- All WS messages: `{ type: "namespace:action", payload: {...} }`
- Power values: watts, positive = grid consumption, negative = export
- Energy values: kWh (watt-hours / 1000)

## Running Tests

```bash
make test-backend     # Go tests
make test-backend-v   # Go tests verbose
make test-frontend    # Frontend tests
make test             # All tests
```
