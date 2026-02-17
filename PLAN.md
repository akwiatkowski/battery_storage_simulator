# Future Work

## ~~1. Heat Pump Optimization~~ (DONE)

~~Model COP (Coefficient of Performance) as a function of outdoor temperature.~~

**Implemented:**
- Engine tracks heat pump cost (consumption × spot price) as `heatPumpCostPLN`
- CLI tool `cmd/load-analysis/` provides COP vs temperature curves, hourly cost distribution, shift potential analysis
- Dashboard shows heat pump cost and average price in EnergySummary when spot price data is available

**Also implemented:**
- Pre-heating strategy simulation (thermal mass shadow model, insulation levels, pre-heat during cheap hours / coast during expensive)
- Dashboard shows pre-heating potential savings in HeatingAnalysis

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

## ~~7. PV Configuration & Multi-Orientation Modeling~~ (DONE)

**Implemented:**
- Data-derived PV hourly profiles from actual readings (June/July focus)
- Profile shifting by azimuth and tilt for multi-orientation modeling
- Custom PV configuration UI: East/South/West arrays with peak power, azimuth, tilt, enable toggle
- Engine replaces stored PV with calculated PV when custom mode enabled, adjusts grid accordingly
- Per-array production breakdown in EnergySummary
- Works in both historical replay and prediction modes

## ~~8. Load Shifting Analysis~~ (DONE)

**Implemented:**
- CLI tool `cmd/load-analysis/` with flags: `--input-dir`, `--shift-window`, `--min-power`, `--temp-bucket`
- COP by temperature table for heat pump
- Hourly energy/cost distribution for all appliance sensors
- Shift potential analysis (current vs optimal cost within ±N hour window)
- Auto-discovers appliance sensors (washing, oven, drier, kettle, TV)
- Makefile target: `make load-analysis`

**Also implemented:**
- Dashboard LoadShiftAnalysis component with HP timing efficiency, shift potential, and day-of-week × hour heatmap colored by price
- Engine tracks hourly HP consumption and cost by day-of-week for heatmap generation

## ~~9. Consumption Anomaly Detection~~ (DONE)

**Implemented:**
- CLI tool `cmd/anomaly-detect/` loads CSV data + trained NN models, computes daily actual vs predicted grid import
- Flags statistical outliers using configurable sigma threshold (default 2.0)
- Categorizes anomalies as HIGH (above-normal) or LOW (below-normal) consumption
- Correlates with temperature deviation (unexpected cold → extra heating)
- Dashboard section already existed (collapsed anomaly days list)
- Makefile target: `make anomaly-detect`

## ~~10. Seasonal Heating Cost Analysis~~ (DONE)

**Implemented:**
- HeatingAnalysis dashboard component enhanced with three new sections:
- **Heating season detection**: groups consecutive months with consumption ≥ 5 kWh into seasons, displays date range, duration, consumption, production, COP, cost, avg temp
- **Heating cost fraction**: shows heat pump cost as % of total electricity cost with visual bar
- **Year-over-year comparison**: when ≥2 seasons detected, compares cost, consumption, COP, and avg temp changes with color-coded deltas
- Monthly COP analysis by month already existed in the table view

## ~~11. Voltage & PV Curtailment Analysis~~ (DONE)

**Implemented:**
- `SensorGridVoltage` added to model with HA entity ID mapping (auto-discovered by ha-fetch-history)
- CLI tool `cmd/voltage-analysis/` with flags: `-input-dir`, `-voltage-threshold`, `-min-pv`, `-pv-drop-pct`, `-peak-window`, `-csv-out`, `-daylight-start`, `-daylight-end`
- Export summary: total export kWh, max export power, export revenue
- Voltage summary: avg/max voltage, avg voltage during export
- Curtailment detection: rolling PV peak tracking, flags intervals where voltage > threshold AND PV drops significantly
- Estimates lost energy (kWh) and lost revenue (PLN) from curtailment events
- Optional scatter CSV export (voltage, export_W, pv_W) for external plotting
- Graceful fallback when voltage sensor not yet available
- No phase imbalance analysis (single voltage sensor available)
- Makefile targets: `make voltage-analysis`

## 12. Room Temperature Stability Analysis (R)

Analyze indoor temperature stability and thermal comfort using zone1_temp sensor data. Requires fetching zone1_temp into stats (hourly avg/min/max) for longer historical coverage.

- **Daily min/max range**: how stable is indoor temperature? Plot daily temperature swing over time
- **Thermal lag**: cross-correlate outdoor temp changes with indoor temp response — how many hours until the house "feels" a cold front?
- **Seasonal comfort**: indoor temp distribution by season, identify under-heating/over-heating periods
- **HP cycling**: overlay zone1_temp with HP consumption to visualize heat/coast cycles — how far does temp drop between HP runs?
- **Insulation proxy**: rate of indoor temp decline when HP is off (°C/hour) as a function of outdoor temp — steeper decline = worse insulation

Depends on: zone1_temp data in `input/stats/` (currently only available in legacy CSV with limited resolution)

## 13. Per-Appliance Cost Efficiency

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

## ~~14. Value Explanation Modals~~ (DONE)

**Implemented:**
- HelpTip component with "?" icon buttons next to value labels
- HelpModal overlay with title, description, formula, example, and "why it matters" sections
- 58 help entries defined in `$lib/help-texts.ts` covering all dashboard values
- 78 HelpTip instances across EnergySummary, CostSummary, BatteryStats, BatteryConfig, SimConfig, PredictionComparison, HeatingAnalysis, and more
- Close via X button, click outside, or Escape key
