# Future Work

## ~~1. Heat Pump Optimization~~ (DONE)

~~Model COP (Coefficient of Performance) as a function of outdoor temperature.~~

**Implemented:**
- Engine tracks heat pump cost (consumption × spot price) as `heatPumpCostPLN`
- CLI tool `cmd/load-analysis/` provides COP vs temperature curves, hourly cost distribution, shift potential analysis
- Dashboard shows heat pump cost and average price in EnergySummary when spot price data is available

**Remaining:**
- Pre-heating strategy simulation (heat during cheap hours, coast during expensive)
- Compare thermal storage (overheating the house) vs battery storage

## 2. Time-Series Charts

Add interactive line charts below the power flow diagram using layerchart, showing power, price, and temperature over a sliding replay window.

- X-axis: simulation time (configurable window: 1h, 6h, 24h, 7d)
- Multiple Y-axes: power (W), price (PLN/kWh), temperature (C)
- Overlay: battery SoC, predicted vs actual values
- Zoom/pan with scroll and drag

## ~~3. Home Assistant Data Fetch~~ (PARTIALLY DONE)

**Implemented:**
- CLI tool `cmd/ha-fetch-history/` fetches sensor history via HA REST API
- Reads credentials from `.env` file (gitignored) or `-url`/`-token` flags
- Bidirectional: backfills older data (up to 2 years) + appends new data
- Output split into weekly CSV files (e.g. `2026-W07.csv`) in `input/recent/`
- Incremental: only rewrites week files with new data, old weeks untouched
- Handles `minimal_response` format, skips non-numeric states, deduplicates
- Makefile target: `make ha-fetch-history`

**Remaining:**
- Live WebSocket connection (`ws://<host>:8123/api/websocket`) for real-time monitoring
- Subscribe to `state_changed` events for live dashboard updates
- Dual mode: live monitoring + historical replay

## ~~4. Net Metering & Net Billing Simulation~~ (DONE)

**Implemented:**
- Net metering: credit bank (kWh) with configurable ratio and distribution fee
- Net billing: PLN deposit from export at spot price, import at fixed tariff
- CostSummary shows up to 5-way comparison (no battery, self-consumption, arbitrage, net metering, net billing)
- SimConfig: fixed tariff, distribution fee, net metering ratio inputs

## ~~5. Battery Degradation & ROI~~ (DONE)

**Implemented:**
- Degradation model: linear capacity fade based on configurable cycle-to-80% parameter
- Effective capacity displayed in BatteryStats when degradation > 0.01%
- ROI calculator in CostSummary: investment, annual savings, payback years, savings/cycle
- Battery cost per kWh configurable in SimConfig

## 6. Export Limiting

Simulate inverter export power caps. In Germany the "70% rule" limits feed-in to 70% of installed peak power. Poland doesn't enforce this currently, but some grid operators or future regulations may impose limits, and it's relevant for sizing decisions.

- **Config**: max export power (W), default unlimited (0 = no limit)
- **Engine**: in `updateEnergy()`, clamp negative grid power to `-maxExportW`; excess PV beyond this cap is curtailed (wasted)
- **Track**: curtailed energy (kWh) and lost revenue (PLN) as separate accumulators
- **Use case**: "what if my grid operator limits me to 5kW export — how much revenue do I lose?"
- **Display**: curtailment stats in EnergySummary (kWh wasted, PLN lost)

Lower priority for Poland but useful for what-if analysis when considering larger PV installations.

## 7. PV Configuration & Multi-Orientation Modeling

Model PV production from multiple roof orientations to analyze adding panels or changing layout. Current setup: 6.5 kWp on east roof at ~40° tilt.

### Solar Position Model

- Compute sun azimuth and elevation for each hour using latitude/longitude (Wrocław area ~51.1°N, 17.0°E)
- For each roof orientation (azimuth + tilt), calculate incident irradiance factor: `cos(angle_of_incidence)` clamped to [0, 1]
- Apply atmospheric losses (air mass model) and panel efficiency curve

### PV Array Configuration

UI section similar to BatteryConfig:
- **East array**: peak power (kWp), azimuth (°), tilt (°) — default: 6.5 kWp, 90° (east), 40°
- **South array**: default: 0 kWp, 180°, 40°
- **West array**: default: 0 kWp, 270°, 40°
- Enable/disable each array independently

### Analysis

- During historical replay: scale actual PV readings by ratio of new config's theoretical output vs current config's theoretical output at each timestamp
- During prediction mode: generate PV curve from solar model directly
- Show per-array production breakdown
- **Key question**: "Adding 3 kWp on west roof — how does afternoon production change net cost and self-consumption?"

### Simplifications

- Use clearsky irradiance model (no cloud modeling — actual cloud cover comes from real PV data scaling)
- Ignore shading, snow, soiling
- Temperature derating: -0.4%/°C above 25°C (use outdoor temp sensor)

## ~~8. Load Shifting Analysis~~ (DONE)

**Implemented:**
- CLI tool `cmd/load-analysis/` with flags: `--input-dir`, `--shift-window`, `--min-power`, `--temp-bucket`
- COP by temperature table for heat pump
- Hourly energy/cost distribution for all appliance sensors
- Shift potential analysis (current vs optimal cost within ±N hour window)
- Auto-discovers appliance sensors (washing, oven, drier, kettle, TV)
- Makefile target: `make load-analysis`

**Remaining:**
- Dashboard section with load shifting recommendations
- Appliance timing heatmap (hour vs day-of-week colored by cost)

## 9. Consumption Anomaly Detection

CLI tool + collapsed dashboard section. Use the NN prediction as a baseline and flag days where actual consumption deviates significantly.

### CLI Tool (`cmd/anomaly-detect/`)

- Compare actual grid power readings against NN-predicted values (already available via PredictedPowerAt)
- Compute daily residuals: `actual_daily_kWh - predicted_daily_kWh`
- Flag days where residual exceeds 2× standard deviation
- Categorize: high consumption anomaly (guests? broken appliance?) vs low (away from home?)
- Correlate with temperature anomalies (unexpected heating demand?)

### Dashboard Section (collapsed)

- List of anomaly days with date, actual vs predicted, deviation %
- Possible cause hints based on which sensors showed unusual patterns

## 10. Seasonal Heating Cost Analysis

Analyze heating costs and season duration from real heat pump data.

- **Heating season detection**: find contiguous periods where daily heat pump consumption exceeds a threshold (e.g. >5 kWh/day)
- **Per-season stats**: start/end dates, duration (days), total heat pump consumption (kWh), total heating cost (PLN), avg outdoor temp
- **Cost breakdown**: electricity cost of heating vs total electricity cost — what fraction of the bill is heating?
- **COP analysis by month**: actual COP from production/consumption ratio, correlated with outdoor temp
- **Year-over-year comparison**: if multiple seasons in data, compare heating costs and degree-days
- Display in a dedicated section or as part of EnergySummary when heat pump data exists

## 11. Voltage & PV Curtailment Analysis

Ingest additional sensor data — per-phase voltage (L1/L2/L3) and grid voltage — and build a CLI tool that correlates PV power output, spot price, and voltage levels to identify curtailment patterns.

- **New sensors**: phase voltages (V), grid voltage (V) from HA or smart meter
- **CLI tool** (`cmd/voltage-analysis/`): reads CSV/store data, outputs pattern report
- **Analysis**:
  - Detect PV curtailment: periods where PV output drops while irradiance should be high (midday), correlated with high grid voltage (>253V)
  - Revenue loss: estimate lost export revenue from curtailment using spot price at those timestamps
  - Phase imbalance: identify asymmetric load/export across phases
  - Voltage vs export scatter plot data: export power (W) vs grid voltage (V) to find the inverter's voltage trip point
- **Output**: summary stats (curtailment hours, lost kWh, lost PLN) + CSV export for external plotting

## 12. Per-Appliance Cost Efficiency

Analyze how well each appliance's usage aligns with cheap/PV electricity.

- For each appliance sensor: compute weighted average electricity cost during operation
- Compare against overall average cost — "the drier runs at 0.72 PLN/kWh avg vs household avg 0.55 PLN/kWh"
- Rank appliances by cost efficiency (best to worst timing)
- Show % of each appliance's energy that came from PV self-consumption vs grid
- Lower priority — Home Assistant already provides per-device energy dashboards, but the cost-timing analysis adds value HA doesn't have

## 13. Multiple Tariff Models

Compare different electricity pricing structures against each other:

- **Fixed import + dynamic export** (current setup): fixed import rate, spot price for export
- **Fully dynamic**: both import and export at spot price (with margins)
- **Time-of-use (TOU)**: predefined cheap/expensive hour bands (e.g. night tariff)
- **Flat rate**: single fixed price for both import and export

Display as a comparison table showing net cost under each tariff model for the same replay period.

Partially superseded by #4 (Net Metering & Net Billing) which covers the most important Polish tariff variants.

## 14. Value Explanation Modals

Every numeric value displayed on the dashboard should have a small "?" button next to it. Clicking it opens a modal with a plain-language explanation of what the value means and how it's calculated. The goal is to make the dashboard understandable to someone with zero energy/technical background.

### Requirements

- **"?" icon button** next to each value label (small, unobtrusive, consistent with current help-icon style)
- **Modal overlay** appears on click with:
  - **Title**: the value name (e.g. "Self-Consumption")
  - **What it means**: 1-2 sentence plain-language explanation
  - **How it's calculated**: simple formula or description (e.g. "PV production that was used directly by the home instead of exported to the grid")
  - **Example**: concrete numeric example if helpful (e.g. "If your PV produced 10 kWh and you used 7 kWh directly, self-consumption = 7 kWh (70%)")
  - **Why it matters**: brief practical insight (e.g. "Higher self-consumption means less reliance on expensive grid electricity")
- **Close** via X button, clicking outside, or Escape key
- **Reusable component**: `HelpModal.svelte` with props for title, description, formula, example
- **Content**: define all explanations in a single data file (`$lib/help-texts.ts`) for easy editing

### Values to cover

All values in EnergySummary, CostSummary, BatteryStats, BatteryConfig fields, SimConfig fields, and PredictionComparison. Approximately 30-40 distinct explanations.
