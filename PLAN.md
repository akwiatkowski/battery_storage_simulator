# Future Work

## 1. Heat Pump Optimization

Model COP (Coefficient of Performance) as a function of outdoor temperature. Simulate pre-heating the house during cheap electricity hours or peak solar production, reducing grid import during expensive periods.

- COP curve: typical air-source heat pump COP ranges from ~2 at -15C to ~5 at +15C
- Strategy: pre-heat when electricity is cheap or PV is exporting, coast during expensive hours
- Compare thermal storage (overheating the house slightly) vs battery storage

## 2. Time-Series Charts

Add interactive line charts below the power flow diagram using layerchart, showing power, price, and temperature over a sliding replay window.

- X-axis: simulation time (configurable window: 1h, 6h, 24h, 7d)
- Multiple Y-axes: power (W), price (PLN/kWh), temperature (C)
- Overlay: battery SoC, predicted vs actual values
- Zoom/pan with scroll and drag

## 3. Home Assistant WebSocket Connection

Connect directly to a Home Assistant instance via `ws://<host>:8123/api/websocket` for live data instead of replaying CSV files.

- Auth: long-lived access tokens (`Authorization: Bearer <token>`)
- Subscribe to state changes: `subscribe_events` with `event_type: state_changed`
- History queries: `recorder/statistics_during_period` for backfill
- Entities to subscribe: grid power, PV, heat pump, outdoor temp, spot price
- Dual mode: live monitoring + historical replay from HA's recorder database

## 4. Net Metering & Net Billing Simulation

Simulate Poland's two prosumer billing systems alongside the current spot-price model. Both use a fixed tariff for import.

### Net Metering (system opustów — pre-2022 prosumers)

Energy exported to grid is stored as credits at a ratio (1:0.8 for installations ≤10kWp, 1:0.7 for >10kWp). Credits are used to offset future import within a 12-month rolling window.

- **Credit bank**: track exported kWh × ratio as credits, deduct from import kWh before billing
- **12-month expiry**: credits older than 12 months are lost (track per-month buckets)
- **No cash for surplus**: excess credits at expiry = lost energy, no payment
- **Fixed tariff only**: import price is the contracted fixed rate (e.g. 0.65 PLN/kWh)
- **Distribution fee**: credited energy still pays ~0.20 PLN/kWh distribution component

### Net Billing (system net-billing — post April 2022)

Exported energy is valued at a market reference price (RCEm — monthly average spot price). This value is credited to the prosumer's account in PLN, then used to offset import costs.

- **Export valuation**: exported kWh × RCEm (monthly avg spot price) → PLN credit
- **Import cost**: imported kWh × fixed tariff rate
- **PLN deposit account**: export revenue accumulates as PLN balance, deducted from import bills
- **Surplus payout**: unused PLN balance after 12 months paid out at ~20% of value (effectively lost)
- **No 1:1 energy offset**: unlike net metering, it's purely financial — export at wholesale, import at retail

### Implementation

- Engine tracks three parallel cost models: spot (current), net metering credits, net billing PLN deposit
- New section in CostSummary showing 3-way comparison
- SimConfig: fixed tariff rate input (PLN/kWh), installation size for credit ratio selection
- Credit/deposit state resets on seek

## 5. Battery Degradation & ROI

### Degradation Model

Track cumulative cycle count and model capacity fade over battery lifetime.

- **Capacity fade curve**: linear approximation — 80% remaining capacity at ~4000 full equivalent cycles (configurable)
- **Effective capacity**: `originalCapacity × (1 - cycleCount / degradationCycles × 0.2)` clamped to [0.8, 1.0]
- **Apply during simulation**: reduce usable capacity as cycles accumulate
- **Display**: show current effective capacity %, projected months to 80% at current cycling rate

### ROI Calculator

Calculate return on investment for battery storage.

- **Input**: battery cost per kWh of storage (default: 1000 PLN/kWh), so 10kWh = 10,000 PLN
- **Compute from simulation**: cumulative battery savings (PLN) from self-consumption strategy
- **Display in BatteryStats or new section**:
  - Total investment: capacity × cost/kWh
  - Cumulative savings to date (PLN)
  - Simple payback: investment / (savings per simulated year)
  - Projected payback including degradation (savings decrease as capacity fades)
  - Savings/cycle (PLN) — efficiency metric
- **Annualize**: extrapolate from simulation period to yearly savings rate

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

## 8. Load Shifting Analysis

CLI tool + collapsed dashboard section. Analyze when high-power appliances run vs when PV/cheap electricity is available.

### CLI Tool (`cmd/load-analysis/`)

- Read appliance sensor data (kettle, oven, washing, drier) + PV + spot price
- For each appliance activation (power > threshold for > N minutes):
  - Record start time, duration, energy consumed
  - Calculate cost at actual time vs hypothetical cost if shifted to cheapest/sunniest window
  - Flag activations during expensive hours or when PV was exporting
- **Output**:
  - Per-appliance: activations count, total kWh, avg cost/kWh, potential savings if shifted
  - Best windows: recommend time-of-day ranges with lowest avg cost (combining PV + spot)
  - Weekly pattern: which days/hours appliances typically run

### Dashboard Section (collapsed)

- Summary cards: "You could save X PLN/month by running the washing machine at 10am instead of 8pm"
- Appliance timing heatmap: hour-of-day vs day-of-week, colored by cost efficiency

Lower priority — the east-oriented PV peak around 10am is already known. More useful once west panels are added (creating an afternoon sweet spot too).

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
