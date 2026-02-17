# R Tutorial — Energy Simulator Data Analysis

Learning R by exploring real home energy data from this project.

## Setup

```bash
brew install r                    # install R
R                                 # open R console
install.packages("renv")          # install package manager
```

Then from the project root in R:
```r
renv::init()                      # initialize renv (creates renv.lock)
renv::install("tidyverse")        # install tidyverse bundle
renv::snapshot()                  # save to lockfile
```

R scripts live in `analysis/r/scripts/`, with shared libraries in `analysis/r/R/`:
```
analysis/r/
  R/                    # Shared library code
    load_data.R         # Comprehensive data loader for all sensors
    theme.R             # Color palette, ggplot theme, save_plot() helper
    helpers.R           # Reusable computation functions
  scripts/              # Numbered analysis scripts (01-09)
    01_cop_analysis.R   # COP vs temperature (3 charts)
    02_grid_heatmap.R   # Weekday x hour heatmap
    03_peak_vs_average.R # Peak vs avg — 4 charts
    04_self_sufficiency.R # Battery self-sufficiency curve
    05_export_clipping.R  # PV export clipping loss
    06_hidden_pv.R      # Hidden PV in import hours
    07_power_duration.R # Import/export duration curves
    08_seasonal_inverter.R # Seasonal sizing
    09_cost_of_clipping.R  # Monetary impact with spot prices
  output/               # Generated PNGs (gitignored)
  report.md             # Presentation document with all charts
  Makefile              # Run all analyses
```

Run all scripts: `make r-analysis` (from project root)
Run one script: `make -C analysis/r 03` (by number)
Run directly: `Rscript analysis/r/scripts/01_cop_analysis.R`

### Tidyverse cheat sheet

```r
library(tidyverse)    # loads dplyr, ggplot2, readr, tidyr, lubridate, etc.

# Read data
df <- read_csv("file.csv")           # readr (fast, guesses types)
df <- read.csv("file.csv")           # base R (simpler)

# Inspect
glimpse(df)                           # compact column overview
head(df)                              # first 6 rows
nrow(df)                              # row count

# Transform (dplyr) — pipe with |> or %>%
df |>
  filter(value > 0) |>               # keep rows matching condition
  select(timestamp, value) |>        # pick columns
  mutate(doubled = value * 2) |>     # add/modify columns
  group_by(hour) |>                  # group rows
  summarize(avg = mean(value))       # aggregate per group

# Join
inner_join(df1, df2, by = "timestamp")  # keep only matching rows

# Plot (ggplot2)
ggplot(df, aes(x = temp, y = cop)) +    # set up axes
  geom_point(alpha = 0.3) +             # scatter plot
  geom_smooth(method = "loess") +       # trend line
  labs(title = "COP vs Temperature")    # labels

# Save plot
ggsave("output.png", width = 10, height = 6)
```

## Data Available

| File | Rows | Description |
|------|------|-------------|
| `input/grid_power.csv` | 25k | Grid power (W), positive=import, negative=export |
| `input/pv_power.csv` | 8k | PV solar generation (W) |
| `input/pump_ext_temp.csv` | 23k | Outdoor temperature (°C) |
| `input/pump_heat_power_consumed.csv` | 14k | Heat pump heating consumption (W) |
| `input/pump_total_consumption.csv` | 15k | Heat pump total electrical consumption (W) |
| `input/pump_total_production.csv` | 35k | Heat pump total heat output (W) |
| `input/pump_cwu_power_consumed.csv` | 3k | Hot water (DHW) consumption (W) |
| `input/pump_inlet_temp.csv` | 24k | Heat pump water inlet temp (°C) |
| `input/pump_outlet_temp.csv` | 29k | Heat pump water outlet temp (°C) |
| `input/pump_zone1_temp.csv` | 18k | Zone 1 room temperature (°C) |
| `input/recent/historic_spot_prices.csv` | 81k | Spot electricity prices (PLN/kWh) since 2018 |

CSV format: `entity_id,state,last_changed` (hourly readings, ISO timestamps).
Spot prices: `sensor_id,value,updated_ts` (Unix timestamps).

## Tasks

### 1. Heat Pump COP Analysis (beginner)

**Goal:** Compute actual COP (Coefficient of Performance) at each outdoor temperature and plot the COP curve.

**Data:** `pump_heat_power_consumed.csv` + `pump_total_production.csv` + `pump_ext_temp.csv`

**Steps:**
1. Load the three CSVs with `read.csv()`
2. Parse timestamps with `lubridate::ymd_hms()`
3. Join by timestamp using `dplyr::inner_join()`
4. Compute COP = heat_output / electrical_input (filter out zeros)
5. Plot COP vs outdoor temperature with `ggplot2` scatter + `geom_smooth()`
6. Add hourly/seasonal facets to see how COP varies

**R skills:** read.csv, dplyr (select, filter, mutate, inner_join), ggplot2 (geom_point, geom_smooth), lubridate

---

### 2. Spot Price Pattern Explorer (beginner → intermediate)

**Goal:** Discover hourly, weekly, and seasonal electricity price patterns.

**Data:** `input/recent/historic_spot_prices.csv`

**Steps:**
1. Load CSV, convert Unix timestamps to POSIXct
2. Extract hour, weekday, month, year
3. Plot average price by hour of day (line chart)
4. Compare weekday vs weekend profiles
5. Boxplots of price distribution by month
6. Year-over-year trend comparison
7. Identify cheapest/most expensive hours per season

**R skills:** lubridate, group_by/summarize, ggplot2 (geom_line, geom_boxplot, facet_wrap)

---

### 3. Grid Power Demand Heatmap (intermediate)

**Goal:** Build a day-of-week x hour-of-day heatmap of average grid consumption, split by season.

**Data:** `input/grid_power.csv`

**Steps:**
1. Load and parse timestamps
2. Extract hour, weekday, season
3. Compute average power per (weekday, hour, season) group
4. Pivot to wide format with `tidyr::pivot_wider()`
5. Plot with `geom_tile()` and a diverging color scale (blue=export, red=import)

**R skills:** tidyr, pivot_wider, geom_tile, scale_fill_gradient2

---

### 4. PV Generation vs Temperature Regression (intermediate)

**Goal:** Fit a linear model predicting PV output from temperature, hour, and month.

**Data:** `input/pv_power.csv` + `input/pump_ext_temp.csv`

**Steps:**
1. Join PV power with outdoor temperature by timestamp
2. Add hour and month as features
3. Fit `lm(pv_power ~ temperature * hour + month)`
4. Examine `summary()`, R-squared, residuals
5. Plot diagnostic charts and predicted vs actual
6. Try polynomial or interaction terms

**R skills:** lm(), formula syntax, summary(), broom::tidy(), diagnostic plots

---

### 5. Arbitrage Profit Backtester (intermediate → advanced)

**Goal:** Simulate a battery arbitrage strategy on historic spot prices, compare threshold strategies.

**Data:** `input/recent/historic_spot_prices.csv`

**Steps:**
1. Load prices, compute daily P33/P67 percentiles
2. Simulate battery: charge when price <= P33, discharge when >= P67
3. Track SoC, revenue, cycles using `dplyr::accumulate()` or a loop
4. Compare strategies: P33/P67 vs P25/P75 vs fixed thresholds vs rolling window
5. Plot cumulative profit curves and SoC over time

**R skills:** cumulative state management, purrr::accumulate, complex dplyr pipelines, multi-series ggplot2

---

### 6. Temperature Anomaly Detection (advanced)

**Goal:** Detect unusual temperature readings using R's time series tools and compare with the project's sigma-based approach.

**Data:** `input/pump_ext_temp.csv`

**Steps:**
1. Convert to a `ts` object at hourly frequency
2. Apply STL decomposition (`stl()`) to separate trend + seasonal + remainder
3. Flag anomalies where remainder exceeds 2-3 sigma
4. Compare with `forecast::tsoutliers()`
5. Visualize detected anomalies on the original series

**R skills:** ts(), stl(), forecast package, time series decomposition

---

## Progress

- [ ] Task 1 — Heat Pump COP Analysis
- [ ] Task 2 — Spot Price Pattern Explorer
- [ ] Task 3 — Grid Power Demand Heatmap
- [ ] Task 4 — PV Generation vs Temperature Regression
- [ ] Task 5 — Arbitrage Profit Backtester
- [ ] Task 6 — Temperature Anomaly Detection
