# Feature Ideas

## ~~1. Python + PyTorch: Advanced Neural Networks for Prediction~~ (PARTIALLY DONE)

~~Replace the Go feedforward NNs with proper time-series ML models.~~

**Done:** 6 LightGBM models (PV, PV short-term, consumption, heat pump, DHW, spot price) with feature engineering, rolling smoothing, lag features. R² 0.55–0.90 on test sets.

**Remaining:**
- **LSTM / Temporal Fusion Transformer** — learns sequences, not just point-in-time features. Predict next 24h of grid power given past 7 days of history.
- **Reinforcement learning for battery strategy** — train an agent that learns optimal charge/discharge policy given price forecasts and consumption patterns, instead of hand-coded P33/P67 rules.
- **Probabilistic forecasting** (quantile regression, conformal prediction) — output confidence bands, not just point estimates. "Tomorrow 14:00-16:00 you'll export 2-5 kWh with 90% confidence."

**Learning value:** Deep learning architectures, sequence modeling, RL, uncertainty quantification.

---

## ~~2. Python: Mathematical Optimization for Battery Scheduling~~ (DONE)

**Done:**
- LP optimizer (`optimize.py`) with scipy linprog — provably optimal battery schedules
- P33/P67 heuristic simulator matching Go implementation
- Day-by-day backtest: LP optimal vs heuristic vs no-battery on historical data
- Battery ROI analysis with capacity sweep and monthly breakdown
- Hardware ROI comparison (Dyness vs Pylontech with real Polish market costs)
- MPC controller: continuous loop fetching weather → 5 ML models → LP optimizer → battery action
- Volatility impact analysis: projected price spreads → payback sensitivity

---

## ~~3. Rust + WebAssembly: Client-Side Simulation Engine~~ (DONE)

**Done:**
- Rust WASM battery simulator in `research/wasm-battery/`
- Three strategies: DP optimal (200-bin backward DP), P33/P67 arbitrage, self-consumption
- DP optimizer equivalent to LP but pure Rust, no solver dependency, <1ms in browser
- Chart.js dual-panel visualization (SoC traces + price/net load)
- Calendar date picker, real-time parameter sliders, date range support

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

## 7. Reinforcement Learning Battery Agent (Python + Gymnasium)

Train an RL agent that learns battery control policy from interaction with the simulator, instead of hand-coded rules or LP with perfect foresight.

**What to build:**
- **Gymnasium environment** wrapping the battery simulator: state = (SoC, hour, price_history, load_history, weather), action = charge/discharge/hold power level, reward = -electricity_cost
- **PPO or SAC agent** (stable-baselines3) trained on historical episodes (each day = one episode)
- **Curriculum**: start with single-day episodes, graduate to multi-day with SoC carryover
- Compare RL policy vs LP optimal vs P33/P67 heuristic on held-out test days
- Visualize learned policy: what does the agent do at hour X with SoC Y and price Z?

**Learning value:** RL fundamentals (MDP, policy gradient, value functions), reward shaping, sim-to-real gap, hyperparameter sensitivity.

**Key question:** Can RL match LP (which cheats with perfect foresight) when given only recent history as input? The gap measures the value of forecast accuracy.

**Result:** A battery controller that adapts to patterns without explicit programming. Direct comparison: RL vs LP vs heuristic.

---

## 8. Conformal Prediction for Forecast Uncertainty (Python)

Wrap existing LightGBM models with conformal prediction to get guaranteed coverage intervals — "PV production will be 1.2–3.8 kW with 90% probability" with a statistical guarantee.

**What to build:**
- **Split conformal prediction** on the 6 existing models: calibrate on held-out set, produce prediction intervals at runtime
- **Adaptive conformal**: intervals that widen during volatile weather, narrow during stable periods
- **Stochastic battery scheduling**: feed the prediction intervals into a two-stage stochastic program — optimize a schedule robust to the uncertainty band
- Compare: deterministic LP (point forecast) vs stochastic LP (conformal intervals) — does accounting for uncertainty improve real-world performance?

**Learning value:** Distribution-free uncertainty quantification, stochastic programming, scenario generation, robust optimization.

**Why not Bayesian?** Conformal prediction is model-agnostic (works on top of LightGBM without retraining), provides frequentist coverage guarantees, and is simpler to implement. Different philosophy from Bayesian — learns what to add here.

**Result:** Prediction intervals with guaranteed coverage, battery schedules that are robust to forecast errors.

---

## 9. Time Series Foundation Models (Python + HuggingFace)

Benchmark pre-trained time series foundation models against your hand-crafted LightGBM pipeline. Can a zero-shot model match months of feature engineering?

**What to build:**
- **Chronos** (Amazon, T5-based) — zero-shot probabilistic forecasting, no training needed
- **TimesFM** (Google) — pre-trained on 100B time points, fine-tunable
- **Lag-Llama** — open-source LLM for time series with lag-based tokenization
- Test on all 6 prediction targets (PV, consumption, HP, DHW, spot price, PV short-term)
- Compare: zero-shot vs fine-tuned vs LightGBM on same test split

**Learning value:** Foundation model paradigm, transfer learning, tokenization of continuous signals, HuggingFace ecosystem, when pre-trained beats hand-crafted.

**Key question:** LightGBM with domain-specific features (clear-sky index, solar position, lag_1h) gets R²=0.90 for PV. Can a foundation model reach that without any feature engineering?

**Result:** Quantified answer to "build vs buy" for energy ML. Likely: foundation models win on easy targets (PV, price), lose on domain-specific ones (HP, DHW).

---

## 10. Real Home Assistant Deployment (Python + MQTT + Modbus)

Deploy the MPC controller as an actual HA add-on that controls a real Deye inverter.

**What to build:**
- **HA add-on** (Docker container) running the MPC controller loop
- **Modbus TCP** to Deye inverter: read actual SoC, write charge/discharge setpoints
- **MQTT integration** for HA dashboard: publish battery schedule, forecast, recommended actions
- **Safety layer**: hardware SoC limits, watchdog timer (fall back to self-consumption if controller dies), rate-of-change limits, manual override
- **Logging**: every decision with context (forecast, price, SoC) for post-hoc analysis
- **A/B testing**: alternate days between MPC and heuristic, measure real savings

**Learning value:** Real-world deployment, Modbus protocol, MQTT, safety engineering, HA add-on development, monitoring, debugging hardware interactions.

**Why this matters:** Everything built so far is simulation. This closes the loop — same ML models, same optimizer, but actually moving electrons.

**Result:** A running battery controller saving real PLN, with measured performance vs simulation predictions.

---

## 11. Causal Inference on Energy Data (Python + DoWhy)

Move beyond correlation to causation. Does pre-heating actually save money, or does it just correlate with cheap hours?

**What to build:**
- **Causal graph** of the home energy system: weather → PV → net_load → grid_cost; weather → HP_demand → consumption; price → battery_action → savings
- **Treatment effect estimation**: "What is the causal effect of shifting HP 2 hours earlier?" using DoWhy/EconML
- **Natural experiments**: exploit weather front arrivals, price spikes, cloud transients as quasi-random treatments
- **Counterfactual queries**: "If I had a 15kWh battery instead of 10kWh, how much would I have saved last winter?" — using structural causal models, not just re-running the simulator

**Learning value:** Causal reasoning, DAGs, do-calculus, propensity scores, instrumental variables, counterfactual analysis. Fundamentally different from predictive ML.

**Why it matters:** Predictive models answer "what will happen?" Causal models answer "what should I do?" — the question that actually drives decisions.

**Result:** Actionable causal insights. "Pre-heating saves 12% ± 3% (causal estimate)" vs "pre-heating correlates with 12% lower cost (maybe confounded)."

---

## 12. Physics-Informed Neural Network for Thermal Model (Python + PyTorch)

Replace the hand-coded thermal shadow model with a PINN that learns thermal parameters from data while respecting heat equation physics.

**What to build:**
- **Neural network** that predicts indoor temperature given: outdoor temp, HP power, solar gain, time
- **Physics loss term**: penalize violations of the heat equation `dT/dt = k₁(T_out - T_in) + k₂·Q_hp + k₃·Q_solar`
- **Learned parameters**: k₁ (insulation), k₂ (HP efficiency), k₃ (solar gain coefficient) — extracted from the trained network
- Compare learned parameters vs hand-coded values in `thermal.go`
- **Transfer learning**: train on one heating season, predict the next

**Learning value:** Physics-informed ML, differential equations in loss functions, hybrid modeling (data + physics), PyTorch autograd for scientific computing.

**Why hybrid?** Pure data: needs huge datasets, can predict physically impossible temperatures. Pure physics: requires knowing all parameters upfront. PINN: learns parameters from data while guaranteeing physical plausibility.

**Result:** A thermal model that's both more accurate and more interpretable than either pure ML or pure physics.

---

## 13. LLM-Powered Energy Advisor (Python + RAG)

Build a conversational interface to your energy data: "Why was my bill high last month?" → analyzes data, finds the cold snap + high spot prices, generates explanation.

**What to build:**
- **RAG pipeline**: embed historical energy summaries, anomaly reports, cost breakdowns into a vector store
- **Tool-calling agent**: LLM can query DuckDB/CSV data, run the battery simulator, fetch weather forecasts
- **Natural language questions**: "When should I run the dishwasher this week?", "Compare my heating cost to last year", "What battery size would break even in 8 years?"
- **CLI interface** or simple web UI (could extend the Svelte dashboard)

**Learning value:** RAG architecture, embeddings, vector databases, LLM tool use, prompt engineering, agentic workflows.

**Key insight:** You already have all the data and tools — the LLM is just a natural language interface on top. No new ML models needed, just orchestration.

**Result:** A personal energy advisor that knows your house, your data, and your tools.

---

## 14. Edge ML on Raspberry Pi (Rust/Python + GPIO)

Run a lightweight battery controller on a Pi with a display showing real-time decisions.

**What to build:**
- **ONNX runtime** for LightGBM inference (convert .joblib → .onnx, run on ARM)
- **E-ink or small LCD display**: current SoC, next action, forecast, estimated savings today
- **Temperature sensor** (DHT22 or DS18B20) for live outdoor temp reading
- **GPIO relay output**: simulate (or actually trigger) charge/discharge signals
- **SQLite logging**: every decision + context, sync to main server daily
- **Watchdog**: if Pi crashes, inverter falls back to default mode

**Learning value:** Embedded systems, ONNX model conversion, ARM optimization, hardware I/O, resource-constrained ML (4GB RAM, no GPU), reliability engineering.

**Why Rust?** The WASM simulator already exists in Rust. Port the DP optimizer to native ARM Rust — sub-millisecond optimization on a $35 board.

**Result:** A physical device that makes battery decisions autonomously, with a display you can glance at.

---

## 15. Property-Based Testing for Energy Invariants (Python/Rust + Hypothesis)

Test the battery simulator with thousands of random inputs and verify physical invariants always hold.

**What to build:**
- **Hypothesis** (Python) or **proptest** (Rust) strategies generating random: net_load profiles, price sequences, battery parameters
- **Invariants to verify**:
  - SoC always within [soc_min, soc_max] after every hour
  - Energy conservation: import - export = net_load + charge - discharge (per hour)
  - Optimal cost ≤ heuristic cost ≤ no-battery cost (for any valid input)
  - Cost is monotonically non-increasing as battery capacity increases
  - No negative prices produce negative import costs
- **Shrinking**: when a failing case is found, Hypothesis minimizes it to the simplest reproducing input
- Apply to both Python LP optimizer and Rust DP optimizer — do they agree?

**Learning value:** Property-based testing, formal invariants, generative testing, QuickCheck philosophy, finding edge cases humans miss.

**Why this matters:** The DP optimizer discretizes SoC into 200 bins. Are there edge cases where discretization violates constraints? Property testing will find them if they exist.

**Result:** Mathematical confidence that the simulators are correct, plus a suite of regression tests that cover the infinite input space.

---

## Priority

**Tier 1 — High impact, builds directly on existing work:**
- **#8 Conformal Prediction** — wraps existing models, enables stochastic optimization. Low effort, high learning.
- **#7 RL Battery Agent** — natural next step from LP optimizer. Answers: can learning beat optimization?
- **#10 Real HA Deployment** — closes the sim-to-real loop. Actually saves money.

**Tier 2 — New paradigms, broadens skill set:**
- **#9 Foundation Models** — answers build-vs-buy for energy ML. Quick benchmark.
- **#11 Causal Inference** — completely different from predictive ML. High conceptual value.
- **#12 Physics-Informed NN** — bridges physics and ML. Unique to energy domain.
- **#13 LLM Energy Advisor** — practical AI application, trending skill.

**Tier 3 — Infrastructure and engineering:**
- **#4 DuckDB + dbt** — solid data engineering foundation, but less novel learning.
- **#5 Bayesian Modeling** — deep statistical thinking, but niche application.
- **#6 Event Streaming** — production architecture, but overkill for single household.
- **#14 Edge ML** — fun hardware project, teaches deployment constraints.
- **#15 Property-Based Testing** — underrated, quick to implement, catches real bugs.
