# Energy Autonomy & Battery Metrics

## Implemented

### EnergySummary Metrics
- **Savings/kWh** — battery savings divided by battery capacity
- **Off-Grid %** — `(Self-Consumption + Battery Savings) / Home Demand * 100`, with tooltip explaining formula

### Battery Autonomy Heatmap (`OffGridHeatmap.svelte`)
- GitHub-style calendar grid (7 rows × N weeks)
- Color: red (0h) → blue (48h) via HSL hue interpolation, clipped at 48h
- Metric: hours a fully charged battery could power the house at that day's average consumption rate
- Formula: `batteryCapacityKWh * 24 / dailyDemandKWh` (for finalized days)
- Daily records tracked in `simulation.svelte.ts` via cumulative snapshot deltas
- Day finalization recomputes all deltas from current cumulatives to handle high sim speeds

### Backend
- `Summary.OffGridCoverage(heatPumpPct, appliancePct)` — weighted off-grid calculation
- Extended sensor model with Home Assistant entity ID mapping
- `battery-compare` CLI tool for comparing battery configurations
- `sql-stats` CLI tool for generating Home Assistant DB queries
