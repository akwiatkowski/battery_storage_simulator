# Energy Simulator

Home energy simulator that replays historical sensor data through a WebSocket-driven dashboard. Visualize grid power, solar generation, heat pump operation, and individual appliance consumption with realistic time-accelerated playback.

![Dashboard screenshot](docs/screenshot.png)

## Quick Start

```bash
make dev              # backend :8080 + frontend :5173
# or
docker compose up     # production build on :8080
```

## Data Setup

The simulator needs CSV data in the `input/` directory. See `input.sample/` for example files in all supported formats.

```bash
cp -r input.sample/ input/      # start with sample data
```

For real data, populate these directories:

| Directory        | Format                  | Description                              |
|------------------|-------------------------|------------------------------------------|
| `input/`         | Legacy per-sensor CSV   | One file per sensor (e.g. `grid_power.csv`) |
| `input/stats/`   | Multi-sensor statistics | Hourly aggregates from Home Assistant    |
| `input/recent/`  | Multi-sensor recent     | Recent readings, spot prices             |

To fetch data directly from Home Assistant:

```bash
cp .env.sample .env          # add your HA_URL and HA_TOKEN
make ha-fetch-history        # fetches all available sensor history
```

The tool writes weekly CSV files (e.g. `2026-W07.csv`) to `input/recent/`. Run it repeatedly — it only fetches new data and backfills older data automatically.

### Automatic Periodic Fetching (macOS)

To keep data up to date automatically, set up a launchd job (macOS's native scheduler — works reliably with laptop sleep/wake):

```bash
# 1. Build the binary
make build-ha-fetch-history

# 2. Create a launchd plist
cat > ~/Library/LaunchAgents/com.energy-simulator.ha-fetch.plist << 'EOF'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.energy-simulator.ha-fetch</string>
    <key>ProgramArguments</key>
    <array>
        <string>/Users/olek/projects/llm/energy_simulator/bin/ha-fetch-history</string>
        <string>-output</string>
        <string>/Users/olek/projects/llm/energy_simulator/input/recent</string>
    </array>
    <key>WorkingDirectory</key>
    <string>/Users/olek/projects/llm/energy_simulator</string>
    <key>StartInterval</key>
    <integer>3600</integer>
    <key>StandardOutPath</key>
    <string>/tmp/ha-fetch.log</string>
    <key>StandardErrorPath</key>
    <string>/tmp/ha-fetch.log</string>
    <key>EnvironmentVariables</key>
    <dict>
        <key>HOME</key>
        <string>/Users/olek</string>
    </dict>
</dict>
</plist>
EOF

# 3. Load the job (runs every hour)
launchctl load ~/Library/LaunchAgents/com.energy-simulator.ha-fetch.plist

# Check status
launchctl list | grep ha-fetch

# View logs
tail -f /tmp/ha-fetch.log

# To stop
launchctl unload ~/Library/LaunchAgents/com.energy-simulator.ha-fetch.plist
```

The tool reads `.env` for `HA_URL` and `HA_TOKEN`. Adjust `StartInterval` (seconds) to change frequency — 3600 = hourly, 21600 = every 6 hours.

See [`input/README.md`](input/README.md) for format details and sensor type reference.

## Features

- **Real-time replay** of historical energy data at configurable speed (1s to 1 month per second)
- **Live power flow diagram** showing grid, PV, battery, and home consumption with animated energy dots
- **Battery simulation** with configurable capacity, power, SoC limits, and degradation modeling
- **Dual strategy comparison**: self-consumption vs. price arbitrage (when spot price data available)
- **Cost tracking**: spot pricing, net metering, net billing, battery ROI calculator
- **Heat pump analysis**: COP tracking, consumption cost at spot price, avg HP electricity price
- **Pre-heating simulation**: thermal mass shadow model comparing actual HP cost vs optimal pre-heat/coast strategy
- **Seasonal heating analysis** with monthly COP breakdown and consumption anomaly detection
- **Load shifting dashboard**: day-of-week × hour heatmap, HP timing efficiency, shift potential
- **PV multi-orientation modeling**: data-derived profiles, configurable East/South/West arrays, per-array production
- **Multi-series time-series chart** with togglable grid, PV, battery, heat pump, price, SoC, and temperature layers
- **Neural network prediction** of temperature and grid power for synthetic data generation
- **Heatmaps** for battery state-of-charge (teal gradient) and off-grid autonomy (amber→blue)
- **HTML export** of full simulation report (energy, costs, arbitrage log, daily records)
- **Load analysis CLI** for COP curves, hourly cost distribution, and shift potential
- **HA data fetch** tool for incremental sensor history download with weekly CSV output

## Python ML Prediction System

LightGBM-based models for energy forecasting, battery optimization, and MPC control. Located in `analysis/python/`.

### Setup

```bash
make py-setup             # install Python dependencies (via mise)
make py-fetch-weather     # download Open-Meteo historical weather data
```

### Trained Models

| Model | Test R² | MAE | Key Inputs | Use Case |
|-------|---------|-----|------------|----------|
| PV Short-term | 0.904 | 21 W/kWp | weather + 2h PV history | Real-time battery decisions |
| Consumption | 0.862 | 98 W | weather + 1h load history | Load forecasting |
| Spot Price | 0.860 | 0.04 PLN | weather + 24h price history | Price-aware scheduling |
| HP Heating | 0.549 | 124 W | weather only | Heating demand forecast |
| PV Long-term | 0.545 | 46 W/kWp | weather only | Day-ahead PV planning |
| DHW | -0.036 | 28 W | time + temperature | Hot water (unpredictable) |

See [ML_MODELS.md](ML_MODELS.md) for detailed training lessons and feature engineering decisions.

### Python Tools

All tools run via `mise exec -- python -m analysis.python.src.<tool>`.

#### Train Models

```bash
mise exec -- python -m analysis.python.src.train --model <name>
```

| Arg | Default | Description |
|-----|---------|-------------|
| `--model` | `pv` | Model to train: `pv`, `pv_shortterm`, `consumption`, `heat_pump`, `dhw`, `spot_price` |

Trains the specified model on historical sensor + weather data, saves `.joblib` + `.json` metadata to `analysis/python/models/`.

#### Evaluate Models

```bash
mise exec -- python -m analysis.python.src.evaluate --model <name>
```

| Arg | Default | Description |
|-----|---------|-------------|
| `--model` | `pv` | Model to evaluate |

Generates evaluation plots (actual vs predicted, residuals, feature importance) saved to `analysis/python/output/`.

#### Forecast (Predict)

```bash
mise exec -- python -m analysis.python.src.predict --model <name> --hours 48
```

| Arg | Default | Description |
|-----|---------|-------------|
| `--model` | `pv` | Model: `pv`, `consumption`, `heat_pump`, `dhw`, `spot_price` |
| `--hours` | `48` | Forecast horizon in hours |
| `--csv` | (none) | Optional output CSV path |

Fetches live weather forecast from Open-Meteo, runs the model, prints hourly predictions with daily totals.

#### Battery Backtest

```bash
mise exec -- python -m analysis.python.src.backtest --days 30 --capacity 10 --power 5000
```

| Arg | Default | Description |
|-----|---------|-------------|
| `--days` | `30` | Number of days to backtest |
| `--capacity` | `10.0` | Battery capacity (kWh) |
| `--power` | `5000` | Max charge/discharge power (W) |
| `--soc-min` | `10` | Min SoC (%) |
| `--soc-max` | `90` | Max SoC (%) |
| `--export-coeff` | `0.8` | Export coefficient (0-1) |
| `--plot` | (none) | Plot a specific day (YYYY-MM-DD) |

Compares LP-optimal vs P33/P67 heuristic vs no-battery on historical data. Shows daily cost comparison and total savings.

#### Battery ROI Analysis

```bash
mise exec -- python -m analysis.python.src.battery_roi analyze --capacity 10 --power 5000 --cost-per-kwh 1500
mise exec -- python -m analysis.python.src.battery_roi sweep --cost-per-kwh 1500
```

**`analyze` subcommand** — monthly breakdown for a specific battery:

| Arg | Default | Description |
|-----|---------|-------------|
| `--capacity` | (required) | Battery capacity (kWh) |
| `--power` | (required) | Max charge/discharge power (W) |
| `--cost-per-kwh` | (required) | Battery cost (PLN per kWh capacity) |
| `--start` | (data start) | Start date (YYYY-MM-DD) |
| `--end` | (data end) | End date (YYYY-MM-DD) |
| `--soc-min` | `10` | Min SoC (%) |
| `--soc-max` | `90` | Max SoC (%) |
| `--export-coeff` | `0.8` | Export coefficient |

**`sweep` subcommand** — find optimal battery size for best ROI:

| Arg | Default | Description |
|-----|---------|-------------|
| `--min-capacity` | `5` | Min capacity to test (kWh) |
| `--max-capacity` | `30` | Max capacity to test (kWh) |
| `--step` | `1` | Step size (kWh) |
| `--c-rate` | `0.3` | C-rate for power constraint |
| `--cost-per-kwh` | (required) | Battery cost (PLN per kWh) |
| `--start/--end` | (full range) | Date range |
| `--soc-min/--soc-max` | `10`/`90` | SoC limits (%) |
| `--export-coeff` | `0.8` | Export coefficient |

#### Battery Hardware ROI Comparison

```bash
mise exec -- python -m analysis.python.src.battery_hw_roi
mise exec -- python -m analysis.python.src.battery_hw_roi --start 2024-07-01 --max-towers 2
```

| Arg | Default | Description |
|-----|---------|-------------|
| `--start` | (data start) | Start date (YYYY-MM-DD) |
| `--end` | (data end) | End date (YYYY-MM-DD) |
| `--max-towers` | `2` | Max battery towers per brand |
| `--soc-min` | `10` | Min SoC (%) |
| `--soc-max` | `90` | Max SoC (%) |
| `--export-coeff` | `0.8` | Export coefficient |

Enumerates all valid Dyness and Pylontech battery configurations (including multi-tower setups with real hardware costs: inverter, BMS, modules), runs LP optimization on historical data, and generates ROI comparison plots. Hardware costs are hardcoded based on current Polish market prices.

#### MPC Battery Controller

```bash
mise exec -- python -u -m analysis.python.src.controller --capacity 10 --power 5000
mise exec -- python -u -m analysis.python.src.controller --capacity 10 --power 5000 --interval 1
```

| Arg | Default | Description |
|-----|---------|-------------|
| `--capacity` | `10.0` | Battery capacity (kWh) |
| `--power` | `5000` | Max charge/discharge (W) |
| `--soc-min` | `10` | Min SoC (%) |
| `--soc-max` | `90` | Max SoC (%) |
| `--soc-initial` | `50` | Starting SoC (%) |
| `--export-coeff` | `0.8` | Export coefficient |
| `--interval` | `15` | Replan interval (minutes) |
| `--horizon` | `24` | Optimization horizon (hours) |

Model Predictive Control loop. On startup, prints a full 24h LP-optimal battery schedule. Then runs continuously, re-optimizing every `--interval` minutes and printing the recommended action (CHARGE/DISCHARGE/HOLD with power and SoC). Weather forecasts are cached for 1 hour. Use `-u` flag for unbuffered output. Ctrl+C prints a session summary.

## R Statistical Analysis

R scripts in `analysis/r/` for exploratory data analysis with ggplot2 visualizations.

```bash
make r-analysis           # run all R scripts
make -C analysis/r 03     # run script 03 only
```

| Script | Description |
|--------|-------------|
| `01_cop_analysis.R` | Heat pump COP vs outdoor temperature (3 charts: scatter, boxplot by temp bucket, seasonal) |
| `02_grid_heatmap.R` | Weekday x hour-of-day grid power heatmap (blue=export, red=import) |
| `03_peak_vs_average.R` | Peak vs average power comparison (4 charts: daily peaks, peak/avg ratio, export peaks, import distribution) |
| `04_self_sufficiency.R` | Battery self-sufficiency simulation curve (% off-grid vs battery size) |
| `05_export_clipping.R` | PV export clipping loss at different inverter limits |
| `06_hidden_pv.R` | Hidden PV self-consumption during grid import hours |
| `07_power_duration.R` | Import/export power duration curves (sorted by magnitude) |
| `08_seasonal_inverter.R` | Seasonal inverter sizing analysis |
| `09_cost_of_clipping.R` | Monetary impact of PV curtailment using spot prices |

See [R_TUTORIAL.md](R_TUTORIAL.md) for R learning exercises using this project's data.

## Go CLI Tools

| Tool | Make Target | Description |
|------|-------------|-------------|
| `cmd/server/` | `make dev` | Main web server (WebSocket + static files) |
| `cmd/battery-compare/` | `make compare` | ASCII table comparing battery configurations |
| `cmd/load-analysis/` | `make load-analysis` | COP curves, hourly cost distribution, shift potential |
| `cmd/ha-fetch-history/` | `make ha-fetch-history` | Fetch sensor history from HA REST API to weekly CSVs |
| `cmd/train-predictor/` | `make train` | Train temperature + grid power neural networks |
| `cmd/sample-predict/` | `make sample-predict` | Generate predictions chaining temp NN → power NN |
| `cmd/fetch-prices/` | `make fetch-prices` | Download historic spot prices |
| `cmd/sql-stats/` | `make sql-stats` | Generate SQL for Home Assistant DB queries |
| `cmd/voltage-analysis/` | `make voltage-analysis` | Voltage-based PV curtailment detection |

## Make Targets

```
Development:
  make dev                backend + frontend with hot reload
  make dev-backend        backend only (air file watcher)
  make dev-frontend       frontend only (vite dev server)

Building:
  make build              build backend binary + frontend static assets
  make build-backend      Go binary → bin/server
  make build-frontend     static site → frontend/build/

Testing:
  make test               all tests (Go + frontend + Python)
  make test-backend       Go tests
  make test-backend-v     Go tests (verbose)
  make test-frontend      vitest
  make py-test            Python ML tests

Linting:
  make lint               all linters
  make lint-backend       go vet
  make lint-frontend      eslint + prettier

Neural Networks (Go):
  make train              train temperature + grid power NN models
  make sample-predict     generate predictions (temp NN → power NN)

ML Models (Python):
  make py-setup           install Python dependencies
  make py-fetch-weather   download Open-Meteo historical weather
  make py-train-pv        train PV production model
  make py-evaluate-pv     generate evaluation plots
  make py-predict         48h PV production forecast

Data & Analysis:
  make ha-fetch-history   fetch sensor history from Home Assistant REST API
  make fetch-prices       download historic spot prices to input/recent/
  make load-analysis      COP curves, hourly cost distribution, shift potential
  make compare            battery configuration comparison
  make sql-stats          print SQL for Home Assistant DB queries
  make r-analysis         run all R analysis scripts

Docker:
  docker compose up       production build on :8080
  make clean              remove build artifacts
```

## Tech Stack

- **Backend:** Go 1.25, `net/http` + `gorilla/websocket`
- **Frontend:** Svelte 5, SvelteKit, layerchart, D3
- **ML:** Python 3, LightGBM, scikit-learn, scipy (LP optimizer)
- **Statistics:** R, tidyverse, ggplot2
- **Communication:** WebSocket (no REST API)
- **Testing:** Go `testify`, vitest + `@testing-library/svelte`, pytest

## License

[GPLv3](LICENSE)
