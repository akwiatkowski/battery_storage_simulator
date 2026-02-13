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

| Component              | Description                                              |
|------------------------|----------------------------------------------------------|
| `HomeSchema.svelte`    | Live power flow diagram (grid, PV, battery, home)        |
| `EnergySummary.svelte` | Energy totals, battery savings, off-grid %               |
| `CostSummary.svelte`   | Energy costs, 3-way battery strategy comparison          |
| `BatteryConfig.svelte` | Battery parameter controls (capacity, power, efficiency) |
| `BatteryStats.svelte`  | Cycle count and power distribution stats                 |
| `SoCHeatmap.svelte`    | Monthly state-of-charge distribution heatmap             |
| `OffGridHeatmap.svelte` | Daily battery autonomy heatmap (GitHub calendar style)  |

## Conventions

- All WebSocket messages follow `{ type: "namespace:action", payload: {...} }`
- Power values in watts (positive = grid import, negative = export)
- Energy values in kWh
- Imports from `$lib/` must not include `.ts` extension
