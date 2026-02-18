# Feature Ideas

## 1. Python + PyTorch: Advanced Neural Networks for Prediction

Replace the Go feedforward NNs with proper time-series ML models.

**What to build:**
- **LSTM / Temporal Fusion Transformer** — learns sequences, not just point-in-time features. Predict next 24h of grid power given past 7 days of history.
- **Reinforcement learning for battery strategy** — train an agent that learns optimal charge/discharge policy given price forecasts and consumption patterns, instead of hand-coded P33/P67 rules.
- **Probabilistic forecasting** (quantile regression, conformal prediction) — output confidence bands, not just point estimates. "Tomorrow 14:00-16:00 you'll export 2-5 kWh with 90% confidence."
- Run as a sidecar Python service with HTTP API that the Go backend calls.

**Learning value:** Deep learning architectures, sequence modeling, RL, uncertainty quantification.

**Result:** Better predictions (lower MAE), richer outputs (confidence intervals), and a battery strategy that adapts to patterns the heuristic can't see.

---

## 2. Python: Mathematical Optimization for Battery Scheduling

Formulate battery scheduling as a linear program instead of using heuristic strategies.

**What to build:**
- Given known (or forecasted) prices and consumption for the next 24h, solve for the mathematically optimal charge/discharge schedule using `scipy.optimize` or `PuLP`.
- Compare optimal-with-perfect-foresight vs the P33/P67 heuristic — quantify how much money the heuristic leaves on the table.
- Rolling-horizon optimization with forecast uncertainty from idea #1.
- Chain with #1: use NN forecasts as input to the optimizer.

**Learning value:** Operations research, LP/MIP formulation, optimization under uncertainty.

**Result:** Provably optimal battery schedules, quantified gap between heuristic and optimal, actionable daily charge plans.

---

## 3. Rust + WebAssembly: Client-Side Simulation Engine

Port the simulator core to Rust, compile to WASM, run in the browser.

**What to build:**
- Instant "what-if" scenarios without WebSocket round-trips.
- Drag a slider for battery capacity and see results update in real-time at native speed.
- All simulation logic runs client-side.

**Learning value:** Rust ownership/borrowing, wasm-bindgen, systems programming, browser performance.

**Result:** Sub-millisecond simulation response, offline-capable, no server dependency for scenarios.

---

## 4. DuckDB + dbt: Analytics Pipeline

Build a proper analytics layer over the 71+ HA sensors.

**What to build:**
- **DuckDB** as embedded analytical database (reads CSV/Parquet natively, SQL).
- **dbt** for transformation pipeline (staging -> intermediate -> mart models).
- Replace some R scripts with testable, composable SQL models.
- Parquet output for 10-100x faster reads than CSV.

**Learning value:** Modern data engineering, analytical SQL, data modeling, columnar formats.

**Result:** Fast queries over all historical data, reproducible transformations, foundation for dashboards.

---

## 5. Bayesian Modeling in R (Stan/brms)

Level up the existing R analyses with probabilistic models.

**What to build:**
- **Bayesian heating curve** — posterior distribution for thermal loss coefficient with credible intervals.
- **Hierarchical room temperature model** — rooms share building-level insulation but have individual characteristics.
- **Change-point detection** — detect structural breaks in energy patterns (renovation effects, behavior changes).

**Learning value:** Bayesian statistics, Stan/MCMC, probabilistic thinking, uncertainty propagation.

**Result:** Uncertainty-aware insights instead of point estimates, principled model comparison.

---

## 6. Event Streaming Architecture (Go + NATS)

Replace direct WebSocket with an event bus.

**What to build:**
- Sensor readings -> NATS JetStream -> multiple consumers (simulator, anomaly detector, price alerter).
- Telegram/ntfy notifications: "Price dropping below P10 in 2 hours — charge battery."
- Replay historical streams at arbitrary speed.

**Learning value:** Event-driven architecture, message brokers, stream processing patterns.

**Result:** Decoupled services, real-time alerts, scalable data pipeline.

---

## Priority

Ideas #1 and #2 are the most interesting. They chain together naturally:
- #1 produces forecasts (consumption, price, PV production for next 24h)
- #2 takes those forecasts and computes the optimal battery schedule
- Together they form a complete predict-then-optimize pipeline
