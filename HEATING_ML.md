# Heat Pump ML Model: Training Lessons

Documentation of knowledge gained from building a heat pump power prediction model using Claude Code. Written for a presentation about ML training, iterative development, and AI-assisted coding.

## Problem Statement

Predict heat pump power consumption using weather forecast data. The heat pump heats a house -- its power usage depends on outdoor conditions. The goal is 6-hour block predictions for energy and battery planning.

## Model Choice

LightGBM (gradient-boosted decision trees) via Python's `lightgbm` library. Despite the project's neural network theme, tree-based ensembles are a better fit here: small dataset (~1,200 training samples), tabular features, no spatial/sequential structure that would benefit from deep learning.

## Dataset

- **Source**: Home Assistant sensor data (HP power) + Open-Meteo historical weather
- **Raw size**: ~12,400 hourly HP readings (Sep 2024 -- Feb 2026)
- **After 6h resampling + heating filter (temp <= 15C)**: 1,578 samples
- **Train**: 1,218 samples
- **Test**: 360 samples (Nov 20, 2025 -- Feb 18, 2026)

---

## Key Insights and Lessons Learned

### 1. Distribution Shift Was The Real Enemy

Training data included ~40% warm summer days where HP=0. Test data was 100% deep winter with heavy heating. The model learned a biased average pulled toward summer zeros. No amount of hyperparameter tuning fixed this.

**Solution**: Filter training data to temp <= 15C (heating conditions only). This removed distribution shift and improved results immediately.

**Takeaway**: Before tuning anything, make sure train and test distributions match the deployment scenario. A model trained on "all seasons" will underperform in winter compared to a model trained on "heating season only."

### 2. Resolution Matters -- Hourly HP Power Is Fundamentally Noisy

Heat pumps cycle on/off based on thermostat hysteresis, defrost cycles, and hot water priority -- things invisible to weather data. At 1h resolution, R-squared peaked at 0.46 even with lag features. At 6h resolution, cycling noise is smoothed out, and the model focuses on underlying heating demand.

**6h was the sweet spot** -- enough smoothing to remove noise, enough granularity for energy planning (4 blocks per day).

**Takeaway**: When your target variable is noisy at fine resolution, aggregating to a coarser resolution can dramatically improve model quality. The right resolution depends on both the signal-to-noise ratio and the downstream use case.

### 3. Lag Features: Essential But Must Match Resolution

Three regimes were tested:

- **1h lags (hp_lag_1h, hp_rolling_6h_mean)**: Dominated feature importance, created an autoregressive shortcut. Model learned "if HP was on 1h ago, it's on now" -- good for train, poor for generalization.
- **6h lags (hp_lag_1 block, hp_lag_1d, hp_rolling_1d)**: Capture heating momentum without cycle-level overfitting. "How much heating in the last 6h/24h" is genuinely predictive.
- **No lags at all**: R-squared = -0.23 (worse than predicting the mean). Weather alone cannot predict the exact timing of heating demand.

**Takeaway**: Lag features are critical for time series, but they must match the prediction resolution. Too-fine lags let the model cheat; too-coarse or absent lags leave it blind to system state.

### 4. Feature Engineering Results

**Weather features that helped (in order of importance):**

| Feature | Description |
|---------|-------------|
| `hp_rolling_1d` | 24h rolling average of HP power (heating regime indicator) |
| `temp_lag_12h` | Temperature 12 hours ago (thermal momentum) |
| `hp_lag_1` | Previous 6h block HP power |
| `hp_lag_1d` | Same 6h block yesterday |
| `temperature` | Current outdoor temp (primary heating driver) |
| `wind_chill` | Temperature x wind_speed interaction (convective heat loss) |
| `temp_lag_6h` | Temperature 6h ago |
| `wind_speed` | Direct wind effect on heat loss |
| `humidity` | Affects HP efficiency and perceived cold |
| `temp_min` | Minimum temperature in 6h block (peak demand driver) |
| `heating_degree_hour` | max(0, 18 - temp), standard heating demand proxy |
| `heating_degree_sq` | Squared, captures nonlinear cold extremes |
| `solar_radiation` | Passive solar gains through windows reduce heating |
| `cloud_cover` | Indirect solar effect |

**Features that were useless at 6h resolution:**

| Feature | Why |
|---------|-----|
| `hour_sin`, `hour_cos` | Time-of-day has almost no importance for 6h averages |
| `is_daylight` | Redundant with solar_radiation at 6h |
| `month_sin`, `month_cos` | Redundant with day_of_year cyclicals + temperature |

**Takeaway**: Domain knowledge matters more than feature quantity. Wind chill (a physics-motivated interaction term) and heating degree hours (an HVAC industry standard) both added predictive value. But temporal features that are meaningful at 1h resolution became useless at 6h.

### 5. Hyperparameter Tuning -- Less Is More

| Setting | R-squared | Notes |
|---------|-----------|-------|
| Defaults (31 leaves, lr=0.05) | 0.654 | Good baseline |
| Aggressive regularization (7 leaves, 100 min_child) | -0.55 | Way too constrained |
| Slow learning (lr=0.01, 2000 trees) | 0.670 | 3.5x slower, same result |
| Wide + L1/L2 (31 leaves, reg_alpha=0.1, reg_lambda=1.0) | 0.672 | No improvement |
| **Best: moderate (15 leaves, lr=0.03, 30 min_child)** | **0.679** | Sweet spot |

**Takeaway**: With ~1,200 training samples, `num_leaves=15` is the right complexity. Early stopping (patience=50) naturally prevents overfitting better than explicit regularization. Tuning hyperparameters gave only +0.025 R-squared over defaults -- the real gains came from data preparation (distribution matching, resolution, feature selection).

### 6. The Overfitting Gap

Every configuration showed a large train/test gap:

- Train R-squared: 0.96--0.98 regardless of settings
- Test R-squared: 0.46--0.68 depending on approach

This is not a bug -- it is the nature of the problem. The model can memorize training patterns perfectly, but test performance is limited by:

1. **No indoor temperature data** -- cannot see thermostat state
2. **Weather forecast uncertainty** -- real forecasts differ from actuals
3. **HP control logic** -- defrost cycles, DHW priority are invisible
4. **Building occupancy** -- human behavior affects heating setpoints

**Takeaway**: A persistent train/test gap does not always mean your model is broken. Sometimes it reflects irreducible uncertainty -- information the model simply does not have access to. Recognizing this prevents wasting time on diminishing-returns tuning.

### 7. Test Window Matters Enormously

| Test period | R-squared | Why |
|-------------|-----------|-----|
| Last 30 days (Jan--Feb, deep winter) | 0.45 | Hardest period, extreme cold |
| Last 90 days (Nov--Feb, full heating season) | 0.68 | More representative |

An earlier evaluation with a different test window showed R-squared = 0.78, likely because it included milder autumn weather.

**Takeaway**: Always compare models on the same test period. A "better" R-squared might just mean an easier test set. Fix the test window before comparing approaches.

---

## Final Model Architecture

```
Resolution: 6h (4 blocks per day)
Training filter: temperature <= 15C
Test window: 90 days (Nov 20, 2025 -- Feb 18, 2026)

Algorithm: LightGBM
  n_estimators: 500 (early stopped at ~370)
  learning_rate: 0.03
  num_leaves: 15
  min_child_samples: 30
  early_stopping: 50 rounds

Features (18 total):
  Time:     day_of_year_sin, day_of_year_cos
  Weather:  temperature, wind_speed, cloud_cover, humidity,
            wind_chill, heating_degree_hour, heating_degree_sq,
            temp_derivative, temp_lag_6h, temp_lag_12h,
            solar_radiation, temp_min
  Lags:     hp_lag_1, hp_lag_1d, hp_rolling_1d

Performance:
  Test R-squared: 0.679
  Test MAE:       101 W (~0.6 kWh per 6h block)
  Test RMSE:      156 W
```

---

## What Would Improve It Further

1. **More winter training data** -- only ~1,200 heating samples across 1.5 winters. A third winter would help significantly.
2. **Indoor temperature sensor** -- would dramatically improve prediction by revealing thermostat state.
3. **HP operating mode** -- distinguishing heat vs defrost vs DHW priority would remove a major source of noise.
4. **Building thermal model** -- physics-based heat loss coefficient calibrated to actual data (hybrid approach).
5. **Ensemble with physics model** -- combine ML predictions with degree-day calculations for robustness.

---

## The Claude Code Development Process

The model was developed iteratively over multiple sessions using Claude Code (Opus). Here is what that process looked like:

### Iteration Cycle

Each improvement followed the same pattern:

1. **Hypothesis** -- "Distribution shift is hurting us" or "6h resolution should smooth cycling noise"
2. **Implementation** -- Claude Code modifies the training pipeline, adds features, changes parameters
3. **Evaluation** -- Run training, examine R-squared / MAE / feature importances / plots
4. **Diagnosis** -- Analyze what changed, form next hypothesis

This cycle repeated roughly 10--15 times across the development of this model.

### What Worked Well With Claude Code

- **Rapid iteration**: Changing resolution from 1h to 6h, adding/removing features, adjusting hyperparameters -- each change took seconds to implement and minutes to evaluate.
- **Domain knowledge application**: Claude could suggest HVAC-relevant features (heating degree hours, wind chill) and explain why certain features would or would not help at a given resolution.
- **Systematic exploration**: Rather than random guessing, each change was motivated by analysis of the previous result -- feature importances, error distributions, train/test gaps.

### What Required Human Judgment

- **Problem framing**: The decision to predict 6h blocks (not 1h, not daily) came from understanding the downstream use case (battery planning).
- **Data filtering**: Recognizing that summer zeros were poisoning the model required domain understanding of heat pump behavior.
- **Knowing when to stop**: R-squared of 0.68 with the available features is a reasonable ceiling. The missing information (indoor temp, HP mode) cannot be recovered by better modeling.

---

## Tools Used

- **Claude Code** (Opus) -- iterative model development, feature engineering, hyperparameter search
- **LightGBM** -- gradient-boosted decision trees
- **Python** with pandas, scikit-learn, joblib
- **Open-Meteo API** -- historical weather data
- **Home Assistant** -- sensor data collection
