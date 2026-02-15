# Frontend

Svelte 5 + SvelteKit dashboard for the energy simulator. Communicates with the backend exclusively via WebSocket.

## Development

```bash
make install-frontend   # npm install
make dev-frontend       # vite dev server on :5173
make test-frontend      # vitest
make lint-frontend      # eslint + prettier
```

## Stack

- **Svelte 5** with SvelteKit (static adapter for production)
- **layerchart** + D3 for time-series charts and heatmaps
- **TypeScript** throughout
- **vitest** + `@testing-library/svelte` for testing

## Structure

```
src/
  lib/
    ws/            WebSocket client + message types
    stores/        Svelte 5 reactive state (simulation, daily records)
    components/    Dashboard UI components
  routes/          SvelteKit pages
```

## Key Components

| Component                  | Description                                                     |
|----------------------------|-----------------------------------------------------------------|
| `HomeSchema.svelte`        | Live power flow diagram (grid, PV, battery, home) with animated dots |
| `EnergySummary.svelte`     | Energy totals, heat pump cost/COP, battery savings, off-grid %  |
| `CostSummary.svelte`       | Energy costs, 5-way strategy comparison, ROI, EV range          |
| `SimControls.svelte`       | Play/pause, speed, data source, seek, NN toggle, price badge    |
| `SimConfig.svelte`         | Export coefficient, tariffs, temp offset, battery cost           |
| `BatteryConfig.svelte`     | Battery capacity, power, SoC limits, degradation cycles         |
| `BatteryStats.svelte`      | Cycle count, degradation %, power/SoC distribution histograms   |
| `SoCHeatmap.svelte`        | Monthly state-of-charge distribution heatmap (teal gradient)    |
| `OffGridHeatmap.svelte`    | Daily battery autonomy heatmap (amber→blue, GitHub calendar)    |
| `TimeSeriesChart.svelte`   | Multi-series chart (grid, PV, battery, HP, price, SoC, temp)   |
| `PredictionComparison.svelte` | NN predicted vs actual power/temperature with MAE            |
| `HeatingAnalysis.svelte`   | Seasonal heating stats with monthly COP breakdown               |
| `AnomalyLog.svelte`        | Consumption anomaly detection log (>20% deviation days)         |
| `ArbitrageLog.svelte`      | Collapsible daily arbitrage log with monthly navigation         |
| `ExportButton.svelte`      | Full HTML report export (energy, costs, arbitrage, daily)       |

## Color Palette (energy-themed)

| Semantic          | Color     | Hex       |
|-------------------|-----------|-----------|
| Import/consuming  | Soft coral | `#e87c6c` |
| Export/savings    | Teal green | `#5bb88a` |
| Electric/charge   | Light blue | `#64b5f6` |
| Discharge/spark   | Warm amber | `#f0a050` |
| PV/solar          | Golden     | `#e8b830` |
| Heat pump         | Warm orange | `#e8884c` |
| Prediction        | Soft violet | `#9b8fd8` |

Cards use 14px radius, `#e8ecf1` borders, `#f7f9fc` page background.

## Conventions

- All WebSocket messages follow `{ type: "namespace:action", payload: {...} }`
- Power values in watts (positive = grid import, negative = export)
- Energy values in kWh
- Imports from `$lib/` must not include `.ts` extension
