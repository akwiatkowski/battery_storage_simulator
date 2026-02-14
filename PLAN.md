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

## 4. Multiple Tariff Models

Compare different electricity pricing structures against each other:

- **Fixed import + dynamic export** (current setup): fixed import rate, spot price for export
- **Fully dynamic**: both import and export at spot price (with margins)
- **Time-of-use (TOU)**: predefined cheap/expensive hour bands (e.g. night tariff)
- **Flat rate**: single fixed price for both import and export

Display as a comparison table showing net cost under each tariff model for the same replay period.

## 5. Voltage & PV Curtailment Analysis

Ingest additional sensor data — per-phase voltage (L1/L2/L3) and grid voltage — and build a CLI tool that correlates PV power output, spot price, and voltage levels to identify curtailment patterns.

- **New sensors**: phase voltages (V), grid voltage (V) from HA or smart meter
- **CLI tool** (`cmd/voltage-analysis/`): reads CSV/store data, outputs pattern report
- **Analysis**:
  - Detect PV curtailment: periods where PV output drops while irradiance should be high (midday), correlated with high grid voltage (>253V)
  - Revenue loss: estimate lost export revenue from curtailment using spot price at those timestamps
  - Phase imbalance: identify asymmetric load/export across phases
  - Voltage vs export scatter plot data: export power (W) vs grid voltage (V) to find the inverter's voltage trip point
- **Output**: summary stats (curtailment hours, lost kWh, lost PLN) + CSV export for external plotting
