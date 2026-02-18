# Energy Simulator

Home energy simulator webapp that replays historical energy data via WebSocket.

## Quick Start

```bash
make dev          # backend :8080 + frontend :5173
make test         # all tests (Go + Svelte + Python)
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

```
├── simulator/                 # Go+Svelte energy simulator webapp
│   ├── backend/               # Go server + CLI tools
│   └── frontend/              # Svelte dashboard
├── analysis/                  # R statistical analysis & graphs
│   ├── helpers/               # shared R code (load_data.R, theme.R, helpers.R)
│   └── scripts/               # numbered analysis scripts (01–38)
├── forecast/                  # Python ML models + optimization + MPC
│   ├── src/                   # source code
│   ├── models/                # trained .joblib files
│   └── tests/                 # pytest tests
├── battery_optimizer_wasm/    # Rust WASM battery strategy comparison
│   ├── src/                   # Rust source
│   └── www/                   # web frontend
├── input/                     # shared sensor/price CSV data
│   └── weather/               # Open-Meteo historical cache (tracked in git)
├── docs/                      # all rendered output (plots, graphs)
│   ├── analysis/              # R-generated plots
│   ├── forecast/              # ML evaluation plots, backtests
│   └── battery_optimizer_wasm/ # WASM comparison charts
```

### Simulator Backend

- `simulator/backend/cmd/server/main.go` — entry point
- `simulator/backend/cmd/battery-compare/` — CLI tool for battery config comparison
- `simulator/backend/cmd/load-analysis/` — CLI tool for load shifting analysis
- `simulator/backend/cmd/ha-fetch-history/` — fetches sensor history from Home Assistant REST API
- `simulator/backend/cmd/train-predictor/` — trains temperature + grid power neural networks
- `simulator/backend/cmd/sample-predict/` — generates predictions chaining temp NN → power NN
- `simulator/backend/cmd/fetch-prices/` — downloads historic spot prices
- `simulator/backend/cmd/sql-stats/` — generates SQL for Home Assistant DB queries
- `simulator/backend/internal/model/` — domain types (Reading, Sensor, SensorType)
- `simulator/backend/internal/ingest/` — CSV parsing (Home Assistant format)
- `simulator/backend/internal/store/` — in-memory data store
- `simulator/backend/internal/simulator/` — time-based replay engine, thermal model, battery
- `simulator/backend/internal/solar/` — PV profile engine (data-derived hourly profiles, orientation shifting)
- `simulator/backend/internal/predictor/` — neural network engine, temperature + grid power predictors
- `simulator/backend/internal/ws/` — WebSocket hub, handler, message types
- `simulator/backend/model/` — trained neural network models (temperature.json, grid_power.json)
- `simulator/backend/testdata/` — test fixture CSVs

### Simulator Frontend

- `simulator/frontend/src/lib/ws/` — WebSocket client + message types
- `simulator/frontend/src/lib/stores/` — Svelte 5 reactive state (includes daily record tracking)
- `simulator/frontend/src/lib/components/` — dashboard components

### Key Frontend Components

- `HomeSchema.svelte` — live power flow diagram (grid, PV, battery, home)
- `EnergySummary.svelte` — energy totals, heat pump cost, battery savings, off-grid %
- `CostSummary.svelte` — energy costs, battery strategy comparison (self-consumption vs arbitrage vs net metering vs net billing), ROI
- `BatteryConfig.svelte` — battery parameter controls (capacity, power, SoC limits, degradation)
- `BatteryStats.svelte` — battery cycle count, degradation %, power distribution histograms
- `SimConfig.svelte` — simulation parameters (export coefficient, tariffs, temp offset, battery cost, insulation level)
- `SimControls.svelte` — play/pause, speed, data source, seek, NN prediction toggle, price badge
- `SoCHeatmap.svelte` — monthly SoC distribution heatmap (teal gradient)
- `OffGridHeatmap.svelte` — daily battery autonomy heatmap (GitHub calendar style, amber→blue)
- `PredictionComparison.svelte` — NN predicted vs actual power/temperature with MAE
- `HeatingAnalysis.svelte` — monthly COP table, heating seasons, cost fraction, YoY comparison, pre-heating potential
- `LoadShiftAnalysis.svelte` — HP timing efficiency, shift potential, day-of-week × hour price heatmap
- `PVConfig.svelte` — custom PV array configuration (East/South/West, peak power, azimuth, tilt)
- `AnomalyLog.svelte` — consumption anomaly detection log
- `ArbitrageLog.svelte` — collapsible daily arbitrage log with monthly navigation
- `ExportButton.svelte` — exports full HTML report (energy summary, costs, arbitrage log, daily records)

### Frontend Color Palette (energy-themed)

- Import/consuming: `#e87c6c` (soft coral)
- Export/savings: `#5bb88a` (teal green)
- Electric/charge: `#64b5f6` (light electric blue)
- Discharge/spark: `#f0a050` (warm amber)
- PV/solar: `#e8b830` (golden)
- Heat pump: `#e8884c` (warm orange)
- Prediction: `#9b8fd8` (soft violet)
- Cards: 14px radius, `#e8ecf1` borders, subtle shadows

## Neural Network Predictors

Two chained neural networks generate realistic energy data:

1. **Temperature NN** (`simulator/backend/model/temperature.json`) — predicts outdoor temperature from day-of-year (cyclical), hour (cyclical), and anomaly input. Anomaly=0 is normal; anomaly=+1 shifts output ~0.1-3°C warmer. Training augments data with random anomalies.
2. **Grid Power NN** (`simulator/backend/model/grid_power.json`) — predicts grid power from month (cyclical), hour (cyclical), and temperature. Uses temperature output from the first network.

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
- Battery degradation: configurable cycle-to-80% parameter, linear capacity fade

## Cost Tracking

- **Spot pricing**: grid import cost and export revenue at spot price per reading
- **Heat pump cost**: heat pump consumption × spot price, tracked separately
- **Net metering**: credit bank (kWh) with configurable ratio, distribution fee
- **Net billing**: PLN deposit from export at spot, import at fixed tariff
- **Pre-heating**: shadow thermal model compares actual HP cost vs optimal pre-heat/coast strategy
- **Battery savings**: difference between no-battery and with-battery net cost (both self-consumption and arbitrage)
- **ROI**: investment = capacity × cost/kWh, annual savings extrapolated, simple payback years

## Python ML Prediction System

LightGBM-based models in `forecast/` for accurate energy forecasting.

```bash
make py-setup         # install Python dependencies
make py-fetch-weather # download Open-Meteo historical weather
make py-train-pv      # train PV production model
make py-evaluate-pv   # generate evaluation plots
make py-predict       # 48h PV production forecast
make py-test          # run Python tests
```

### Models

| Model | Status | Inputs | Predicts |
|-------|--------|--------|----------|
| PV Production | Done | irradiance, cloud, temp, hour, month | W per kWp |
| Base Consumption | Done | hour, day_of_week, month, temp, wind, cloud, weekend, holiday | W household |
| Heat Pump Heating | Done | hour, month, temp, wind, cloud, temp_derivative, is_daylight | W HP |
| DHW (hot water) | Done | hour, month, day_of_week, weekend, holiday | W DHW |
| Spot Price | Done | hour, month, day_of_week, temp, wind, weekend, holiday, price lags | PLN/kWh |

### Forecast Layout

- `forecast/config.yaml` — location, PV system, model hyperparams
- `forecast/src/config.py` — config loader
- `forecast/src/data_loading.py` — load HA sensor CSVs (legacy + recent + stats)
- `forecast/src/weather.py` — Open-Meteo API with monthly CSV caching
- `forecast/src/holidays.py` — Polish bank holidays (pure computation)
- `forecast/src/features.py` — feature engineering (cyclical encoding, solar position, clear-sky index)
- `forecast/src/models/base.py` — abstract model base class
- `forecast/src/models/lightgbm_model.py` — LightGBM implementation
- `forecast/src/train.py` — unified training CLI
- `forecast/src/evaluate.py` — evaluation plots CLI
- `forecast/src/predict.py` — forecast CLI
- `forecast/src/optimize.py` — LP battery scheduler (scipy linprog)
- `forecast/src/backtest.py` — day-by-day strategy comparison CLI
- `forecast/src/battery_roi.py` — monthly ROI breakdown + capacity sweep CLI
- `forecast/src/battery_hw_roi.py` — Dyness vs Pylontech hardware ROI comparison
- `forecast/src/controller.py` — MPC battery controller (continuous optimization loop)
- `input/weather/` — cached Open-Meteo CSVs (monthly, tracked in git)
- `forecast/models/` — trained model files (.joblib + .json)
- `docs/forecast/` — evaluation plots (PNG)

## Conventions

- Go tests: co-located `_test.go` files, use `testify` for assertions
- Frontend tests: `vitest` + `@testing-library/svelte`
- Python tests: `pytest` in `forecast/tests/`
- All WS messages: `{ type: "namespace:action", payload: {...} }`
- Power values: watts, positive = grid consumption, negative = export
- Energy values: kWh (watt-hours / 1000)

## Running Tests

```bash
make test-backend     # Go tests
make test-backend-v   # Go tests verbose
make test-frontend    # Frontend tests
make py-test          # Python ML tests
make test             # All tests (Go + Svelte + Python)
```
