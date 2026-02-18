# ML Models: Training Lessons

Documentation of knowledge gained from building LightGBM prediction models for home energy data. Written for a presentation about ML training, iterative development, and AI-assisted coding.

## Model Overview

All models use LightGBM (gradient-boosted decision trees) with early stopping. Weather data from Open-Meteo API, sensor data from Home Assistant.

| Model | R² | MAE | Resolution | Key Inputs |
|-------|-----|-----|-----------|------------|
| PV Short-term | 0.904 | 21 W | 1h (2h smoothed) | irradiance, solar position + pv_lag_1h, pv_lag_2h |
| Consumption | 0.862 | 98 W | 1h (6h smoothed) | weather + load_lag_1h |
| Spot Price | 0.860 | 0.04 PLN | 1h | weather + price lags |
| HP Heating | 0.549 | 124 W | 6h resampled | weather only |
| PV Long-term | 0.545 | 46 W | 1h (2h smoothed) | irradiance, cloud, solar position |
| DHW | -0.036 | 28 W | 1h (24h smoothed) | time cyclicals + temperature |

---

## Cross-Cutting Lessons

### 1. Target Leakage in Rolling Features

The single most impactful bug found across all models. Pandas `rolling(N).mean()` includes the current timestep -- when used as a feature, it leaks the target value.

```python
# WRONG (leakage -- includes current value):
df["rolling_avg"] = df["target"].rolling(24).mean()

# CORRECT (past only):
shifted = df["target"].shift(1)
df["rolling_avg"] = shifted.rolling(24, min_periods=1).mean()
```

**Impact**: HP model R² inflated from 0.57 to 0.73. Consumption at 6h resolution showed R²=0.97 (fake). After fixing: HP=0.55, consumption=0.19 (before smoothing improvement).

**Takeaway**: Any feature derived from the target variable must use `shift(1)` before any aggregation. This is easy to miss and hard to detect because the model "works great" -- it just doesn't generalize.

### 2. Rolling Window Smoothing vs Resampling

Two ways to reduce noise in a target variable:
- **Resampling** (`resample("6h").mean()`): reduces sample count by 6x
- **Rolling window** (`rolling(6, center=True).mean()`): same noise reduction, keeps all samples

Rolling window consistently outperformed resampling because the model gets N times more training data with the same smoothed signal.

| Model | Approach | R² |
|-------|----------|-----|
| Consumption | 1h raw | 0.185 |
| Consumption | 6h resampled | 0.011 |
| Consumption | 6h rolling smooth | 0.862 |

**Takeaway**: Prefer rolling window smoothing over resampling. You smooth the target noise while preserving training data volume.

### 3. Lag Features: Only Recent Past Matters

Tested across all models: hourly lags (1h, 6h rolling, 24h rolling), daily lags (yesterday average, 6h blocks from yesterday), weekly lags.

Results were consistent:
- **Previous hour lag (`lag_1h`)** is the dominant feature where lags help at all
- **Daily/weekly lags** add negligible value once `lag_1h` is present
- **For weather-driven models (HP, PV, DHW)**, lags don't help -- weather explains the variance

**Takeaway**: Don't add lag features by default. Test whether they genuinely help. For autoregressive signals (consumption, prices), a single previous-period lag captures most of the information.

### 4. Distribution Shift Defeats Tuning

HP model trained on all seasons (40% summer zeros) performed terribly in winter. No hyperparameter tuning could fix a model that learned a biased average pulled toward summer zeros.

**Solution**: Filter training data to deployment conditions (temp <= 15C for heating). This single change mattered more than all hyperparameter tuning combined.

**Takeaway**: Before tuning anything, make sure train and test distributions match the deployment scenario.

---

## Per-Model Notes

### Heat Pump Heating (R²=0.549)

**The challenge**: HP cycling (thermostat hysteresis, defrost, DHW priority) is invisible to weather data. At 1h resolution, R²=0.46 even with lags. At 6h, cycling noise is smoothed.

**Key decisions**:
- 6h resampled resolution (not rolling window -- 3h rolling was tested, performed worse at R²=0.475)
- Weather-only features (lags added only +0.02 R² after leakage fix -- not worth the inference complexity)
- Asymmetric MSE loss (`under_weight: 3.0`) to penalize underestimation 3x (safer for battery planning)
- Sample weights proportional to heating severity: `1 + (HDH/10)²`
- Heating filter: temp <= 15C training data only

**Features (16)**: day_of_year cyclicals, temperature, wind_speed, cloud_cover, humidity, wind_chill, heating_degree_hour, heating_degree_sq, wind_heat_loss, temp_derivative, temp_lag_6h, temp_lag_12h, temp_change_24h, precipitation, solar_radiation, temp_min.

**Ceiling**: ~0.55 R² with available data. Missing indoor temperature, thermostat state, and HP operating mode limit further improvement. More winter training data (third heating season) would help most.

### PV Production

Two models planned: long-term (weather-only) and short-term (with lag features).

**The challenge**: Train R²=0.93 but test drops to R²=0.54. Test period is deep winter (mean 89 W/kWp vs train mean 205 W/kWp). The model also shows systematic bias: underestimates morning production (-21W at 8-10h) and overestimates afternoon (+17W at 12-15h), consistent with east-facing panels whose seasonal peak shift isn't fully captured.

**Useless features removed**: `is_daylight` (zero importance, redundant with solar_elevation filter), `month_sin/cos` (34 splits, redundant with day_of_year), `hour_cos` (29 splits).

**Features (10)**: hour_sin, day_of_year_sin, day_of_year_cos, direct_radiation, diffuse_radiation, cloud_cover, temperature, wind_speed, solar_elevation, clear_sky_index.

**Smoothing x lag matrix** (2h smoothing, no lags = current model):

| Smoothing | No lags | lag_1h | lag_1h + roll_3h |
|-----------|---------|--------|------------------|
| 1h (raw)  | 0.501 / 49W | 0.788 / 29W | 0.790 / 30W |
| 2h        | 0.545 / 46W | 0.867 / 23W | 0.883 / 22W |
| 4h        | 0.547 / 44W | 0.927 / 15W | 0.935 / 14W |
| 6h        | 0.493 / 43W | 0.953 / 11W | 0.960 / 9W  |

**Key findings**:
- Without lags, smoothing barely helps (0.50 → 0.55). Weather alone hits a ceiling around R²=0.55.
- With lags, smoothing has massive impact: lag_1h at 4h smooth gives R²=0.93. Cloud cover persists hour-to-hour, making the previous hour's PV highly predictive.
- At 6h smoothing without lags, R² actually drops (0.49) -- over-smoothing removes signal without lag compensation.
- `roll_3h` adds a small but consistent bump over `lag_1h` alone (+0.01-0.02 R²).

**Final models**:
- **Long-term** (`pv`): 2h smoothing, no lags, 10 features. R²=0.545, MAE=46W. For day-ahead planning from weather forecast.
- **Short-term** (`pv_shortterm`): 2h smoothing + pv_lag_1h + pv_lag_2h, 12 features. R²=0.904, MAE=21W. For real-time battery decisions when recent PV data is available. The 2h lag gives the model a slope estimate (production trending up or down), adding +0.038 R² over lag_1h alone.

### Consumption (R²=0.862)

**The breakthrough**: Combining 6h rolling window smoothing with a single lag feature (`load_lag_1h`) produced the biggest improvement across all models.

**Key finding**: Without lags, 6h smoothing gives R²=0.001. Without smoothing, lags give R²=0.185. Together: R²=0.878. The lag captures autocorrelation, smoothing makes the signal learnable.

**Lag ablation**: tested 4 lags → 3 → 2 → 1 → 0. The cliff is between "has lag_1h" (R²=0.86) and "doesn't" (R²=0.00). Rolling averages and yesterday's lag are redundant.

**Features (16)**: hour/month/day_of_year/day_of_week cyclicals, is_weekend, is_holiday, temperature, wind_speed, cloud_cover, humidity, solar_radiation, load_lag_1h.

### DHW -- Hot Water (R²=-0.036)

**Fundamentally unpredictable**. DHW is thermostat-triggered -- timing is random. 24h rolling window smoothing helps slightly vs raw (-0.04 vs -0.13). Feb 2026 has anomalous 4x DHW usage that destroys test metrics.

**Features (9)**: hour/month/day_of_week cyclicals, is_weekend, is_holiday, temperature.

**Not worth more effort** -- the signal-to-noise ratio is too low for useful prediction.

### Spot Price (R²=0.860)

**Lag-driven model**. Price lags (1h, 24h, 24h rolling mean) are the dominant features. Weather (temperature, wind) adds context for demand/supply dynamics.

**Leakage fix**: `price_rolling_24h_mean` originally included current price. Fixed with `shift(1)`. R² barely changed (0.860 vs 0.860) -- the 24h lag already captured most of the rolling average's information.

**Features (13)**: hour/month/day_of_week cyclicals, is_weekend, is_holiday, temperature, wind_speed, price_lag_1h, price_lag_24h, price_rolling_24h_mean.

---

## The Overfitting Gap

Every model shows a large train/test gap (train R²=0.90-0.96, test R²=0.54-0.86). This reflects irreducible uncertainty from:

1. **No indoor state** -- thermostat settings, occupancy, appliance usage are invisible
2. **Weather forecast vs actuals** -- real deployment uses forecasts, not perfect weather
3. **Device control logic** -- HP defrost, battery management, load switching are opaque
4. **Human behavior** -- unpredictable and varies day-to-day

Recognizing this ceiling prevents wasting time on diminishing-returns tuning.

## Hyperparameter Tuning -- Less Is More

With ~1,200--6,200 training samples, gains from tuning are marginal (+0.01--0.03 R²). The real improvements came from:

1. **Data preparation** -- distribution matching, resolution choice, filtering
2. **Smoothing strategy** -- rolling window vs resampling (biggest single improvement)
3. **Feature selection** -- removing leaked/redundant features
4. **Domain knowledge** -- heating degree hours, wind chill, clear sky index

Early stopping (patience=50) naturally prevents overfitting better than explicit regularization.

---

## The Claude Code Development Process

The models were developed iteratively over multiple sessions using Claude Code (Opus).

### Iteration Cycle

Each improvement followed the same pattern:

1. **Hypothesis** -- "Distribution shift is hurting us" or "rolling smoothing should preserve more data"
2. **Implementation** -- Claude Code modifies the training pipeline, adds features, changes parameters
3. **Evaluation** -- Run training, examine R²/MAE/feature importances
4. **Diagnosis** -- Analyze what changed, form next hypothesis

### What Worked Well With Claude Code

- **Rapid A/B testing**: Running parallel experiments (with/without lags, different smoothing windows) in a single script to compare approaches side by side.
- **Bug detection**: The target leakage bug was caught by noticing suspiciously high R² at coarse resolution -- Claude flagged that `rolling(1).mean()` at 6h resolution equals the current value.
- **Systematic ablation**: Testing feature importance by removing features one at a time, measuring impact, and making data-driven keep/drop decisions.

### What Required Human Judgment

- **Problem framing**: Choosing 6h blocks for HP (battery planning use case) came from domain knowledge.
- **Smoothing vs resampling insight**: The user suggested "averaging window to keep all samples" -- a key idea that led to the biggest model improvement.
- **Knowing when to stop**: Recognizing DHW is unpredictable by nature, and that HP R²=0.55 reflects a data ceiling rather than a modeling failure.

---

## Tools Used

- **Claude Code** (Opus) -- iterative model development, feature engineering, experiment design
- **LightGBM** -- gradient-boosted decision trees
- **Python** with pandas, scikit-learn, joblib
- **Open-Meteo API** -- historical weather data
- **Home Assistant** -- sensor data collection
