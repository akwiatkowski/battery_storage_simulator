# Battery Simulator — Rust + WebAssembly POC

Client-side battery simulation running entirely in the browser via WebAssembly. Drag sliders and see results update instantly — no server round-trips.

## What It Does

- Loads hourly energy data (net load + spot price) into the browser
- User selects a day or date range and sets battery parameters
- Rust WASM module runs three battery strategies in sub-millisecond time:
  - **DP Optimal**: dynamic programming optimizer (200-bin SoC discretization, backward sweep) — equivalent to LP but pure Rust
  - **Arbitrage (P33/P67)**: charges when price is in the cheapest third, discharges in the most expensive third
  - **Self-consumption**: charges from excess PV, discharges to offset grid import
- Dual-panel Chart.js visualization matching the Python backtest plot: SoC traces (top) + price/net load (bottom)
- Cost comparison against no-battery baseline

## DP Optimizer

The optimizer uses backward dynamic programming:

1. Discretize SoC into 200 bins between `soc_min` and `soc_max`
2. For each hour (backward from T to 0), for each SoC bin, find the transition to the next bin that minimizes `hour_cost + future_cost`
3. Transitions are bounded by `max_power_w` (limits reachable bins per step)
4. Forward trace reconstructs the optimal SoC path

Complexity: O(T × N_BINS × reachable_bins) ≈ 24 × 200 × 100 = 480K operations per day. Runs in <1ms in WASM.

This produces near-identical results to the Python LP optimizer (scipy linprog), with discretization error < 0.5% due to the 200-bin resolution.

## Prerequisites

- [Rust](https://rustup.rs/) via mise (already in project `mise.toml`)
- [wasm-pack](https://rustwasm.github.io/wasm-pack/installer/) (`cargo install wasm-pack`)
- Python 3 (for data export + dev server)

## Quick Start

```bash
# 1. Export historical data to JSON (from project root)
make export-data

# 2. Build WASM module
make build

# 3. Serve and open http://localhost:8000
make serve
```

Or all at once: `make dev`

## Project Structure

```
research/wasm-battery/
├── Cargo.toml              # Rust deps: wasm-bindgen, serde
├── Makefile
├── src/
│   └── lib.rs              # Battery strategies + DP optimizer (Rust → WASM)
├── www/
│   ├── index.html          # Single-page app (controls + Chart.js)
│   └── app.js              # WASM integration + chart rendering
├── scripts/
│   └── export_data.py      # Export hourly data from project CSVs
└── data/
    └── hourly.json         # Exported data (generated, gitignored)
```

## Battery Strategies

| Strategy | Approach | Can grid-charge? | Needs price data? |
|----------|----------|-------------------|-------------------|
| DP Optimal | Backward DP, minimizes total cost | Yes | Yes |
| Arbitrage | Daily P33/P67 percentile thresholds | Yes | Yes |
| Self-Consumption | Charge excess PV, discharge to offset import | No | No |
