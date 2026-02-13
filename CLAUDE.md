# Energy Simulator

Home energy simulator webapp that replays historical energy data via WebSocket.

## Quick Start

```bash
make dev          # backend :8080 + frontend :5173
make test         # all tests
make lint         # all linters
make train        # train temperature + grid power neural networks
make sample-predict # generate predictions (temp NN → power NN)
docker compose up # production build
```

## Architecture

- **Backend:** Go 1.25, standard library `net/http` + `gorilla/websocket`
- **Frontend:** Svelte 5 + SvelteKit, layerchart for visualizations
- **Communication:** All data flows via WebSocket messages (no REST API)
- **Data:** CSV files in `input/` loaded on startup

## Project Layout

- `backend/cmd/server/main.go` — entry point
- `backend/cmd/battery-compare/` — CLI tool for battery config comparison
- `backend/cmd/train-predictor/` — trains temperature + grid power neural networks
- `backend/cmd/sample-predict/` — generates predictions chaining temp NN → power NN
- `backend/cmd/sql-stats/` — generates SQL for Home Assistant DB queries
- `backend/internal/model/` — domain types (Reading, Sensor, SensorType)
- `backend/internal/ingest/` — CSV parsing (Home Assistant format)
- `backend/internal/store/` — in-memory data store
- `backend/internal/simulator/` — time-based replay engine
- `backend/internal/predictor/` — neural network engine, temperature + grid power predictors
- `backend/internal/ws/` — WebSocket hub, handler, message types
- `frontend/src/lib/ws/` — WebSocket client + message types
- `frontend/src/lib/stores/` — Svelte 5 reactive state (includes daily record tracking)
- `frontend/src/lib/components/` — dashboard components
- `input/` — CSV data files (committed)
- `input/stats/` — Home Assistant long-term statistics CSV export (training data)
- `model/` — trained neural network models (temperature.json, grid_power.json)
- `testdata/` — test fixture CSVs

### Key Frontend Components

- `HomeSchema.svelte` — live power flow diagram
- `EnergySummary.svelte` — energy totals, battery savings, savings/kWh, off-grid %
- `CostSummary.svelte` — energy costs, battery strategy comparison (self-consumption vs arbitrage)
- `BatteryConfig.svelte` — battery parameter controls
- `BatteryStats.svelte` — battery cycle and power distribution stats
- `SoCHeatmap.svelte` — monthly SoC distribution heatmap
- `OffGridHeatmap.svelte` — daily battery autonomy heatmap (GitHub calendar style, red→blue)

## Neural Network Predictors

Two chained neural networks generate realistic energy data:

1. **Temperature NN** (`model/temperature.json`) — predicts outdoor temperature from day-of-year (cyclical), hour (cyclical), and anomaly input. Anomaly=0 is normal; anomaly=+1 shifts output ~0.1-3°C warmer. Training augments data with random anomalies.
2. **Grid Power NN** (`model/grid_power.json`) — predicts grid power from month (cyclical), hour (cyclical), and temperature. Uses temperature output from the first network.

Both use `[5, 32, 16, 1]` architecture (ReLU hidden, linear output), Adam optimizer, per-hour noise profiles.

Temperature sequences use AR(1) correlated noise and rate-of-change constraints (max 5°C/1h, 10°C/4h, 15°C/10h, 20°C/14h).

## Battery Strategies

When battery is enabled, the engine runs two independent Battery instances on the same data:

1. **Self-consumption** (primary): charges from excess PV, discharges to offset grid import. Affects the main simulation.
2. **Arbitrage** (shadow): charges at max power when spot price is cheap, discharges at max power when expensive. Runs silently for cost comparison only.

Price thresholds use daily P33/P67 percentiles of spot prices (cached per calendar day). The 3-way comparison appears automatically in CostSummary when battery + price data are both available.

- `Battery.Process()` — self-consumption strategy (backward-looking demand)
- `Battery.ProcessArbitrage()` — price arbitrage strategy
- Both share a common `battery.process()` core (energy constraints, SoC, stats)
- Engine tracks arb costs separately via `updateArbGridEnergy()`

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
