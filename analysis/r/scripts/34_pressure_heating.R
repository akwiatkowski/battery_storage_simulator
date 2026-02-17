# ============================================================================
# 34_pressure_heating.R — Pressure Fronts & Heating Demand
# ============================================================================
# WHAT:    Analyzes the relationship between atmospheric pressure changes
#          (weather fronts) and heat pump demand. Falling pressure typically
#          precedes cold fronts and increased heating need. Explores whether
#          pressure rate-of-change can predict heating demand with a lead time.
#
# INPUTS:  load_stats_sensor() for NETATMO_LIVING_PRESSURE (hPa),
#          HP_CONSUMPTION (W), HP_OUTSIDE_TEMP (°C)
#
# OUTPUTS: output/34_pressure_change_vs_heating.png — scatter/binned pressure delta vs HP
#          output/34_pressure_crosscorr.png         — cross-correlation at 0-24h lags
#          output/34_pressure_regime_heating.png     — HP power by pressure regime
#
# HOW TO READ:
#   - Pressure change scatter: negative x = dropping pressure (cold front arriving).
#     If HP power rises on the left side, falling pressure brings colder weather.
#   - Cross-correlation: peaks at positive lags mean pressure drop PRECEDES
#     heating demand increase by that many hours. Useful for pre-heating.
#   - Regime box plot: "Falling" regime should show higher median HP power
#     than "Rising" if pressure fronts drive heating demand.
# ============================================================================

source("analysis/r/R/load_data.R")

# ============================================================================
# Load pressure and heating data
# ============================================================================
pressure_raw <- load_stats_sensor(NETATMO_LIVING_PRESSURE) |>
  distinct(hour_bucket, .keep_all = TRUE)
hp_cons      <- load_stats_sensor(HP_CONSUMPTION) |>
  distinct(hour_bucket, .keep_all = TRUE)
outdoor_temp <- load_stats_sensor(HP_OUTSIDE_TEMP) |>
  distinct(hour_bucket, .keep_all = TRUE)

cat("\n=== Data Availability ===\n")
cat("  Pressure:      ", nrow(pressure_raw), "hours\n")
cat("  HP consumption: ", nrow(hp_cons), "hours\n")
cat("  Outdoor temp:   ", nrow(outdoor_temp), "hours\n")

if (nrow(pressure_raw) < 20) {
  cat("Insufficient pressure data (need >= 20, have", nrow(pressure_raw), ").\n")
  cat("Skipping pressure-heating analysis.\n")
  quit(save = "no")
}

if (nrow(hp_cons) < 20) {
  cat("Insufficient HP consumption data (need >= 20, have", nrow(hp_cons), ").\n")
  cat("Skipping pressure-heating analysis.\n")
  quit(save = "no")
}

# ============================================================================
# Build combined dataset with pressure rate-of-change
# ============================================================================
# Compute 6-hour pressure change: current pressure minus pressure 6 hours ago.
# Negative = dropping (cold front approaching), positive = rising (clearing).

pressure <- pressure_raw |>
  select(hour_bucket, pressure_hpa = avg) |>
  arrange(hour_bucket) |>
  mutate(
    pressure_6h_ago = lag(pressure_hpa, 6),
    pressure_change = pressure_hpa - pressure_6h_ago
  ) |>
  filter(!is.na(pressure_change))

combined <- pressure |>
  inner_join(hp_cons |> select(hour_bucket, hp_power = avg), by = "hour_bucket") |>
  inner_join(outdoor_temp |> select(hour_bucket, outdoor = avg), by = "hour_bucket") |>
  filter(!is.na(hp_power), !is.na(outdoor)) |>
  # Only analyze heating season (outdoor < 12°C)
  filter(outdoor < 12) |>
  mutate(
    hour = hour(hour_bucket),
    month = month(hour_bucket, label = TRUE),
    # Pressure regime classification
    regime = case_when(
      pressure_change > 1   ~ "Rising (>+1 hPa/6h)",
      pressure_change < -1  ~ "Falling (<-1 hPa/6h)",
      TRUE                  ~ "Stable"
    ),
    regime = factor(regime, levels = c(
      "Falling (<-1 hPa/6h)", "Stable", "Rising (>+1 hPa/6h)"
    ))
  )

cat("\n=== Combined Heating-Season Dataset ===\n")
cat("  Rows:              ", nrow(combined), "\n")
cat("  Pressure change:   ", round(min(combined$pressure_change), 1), "to",
    round(max(combined$pressure_change), 1), "hPa/6h\n")
cat("  Outdoor temp:      ", round(min(combined$outdoor), 1), "to",
    round(max(combined$outdoor), 1), "°C\n")
cat("  Regime distribution:\n")
print(table(combined$regime))

if (nrow(combined) < 20) {
  cat("Insufficient combined data for analysis (need >= 20, have", nrow(combined), ").\n")
  quit(save = "no")
}

# ============================================================================
# Chart 1: Pressure change vs heating demand (binned scatter)
# ============================================================================
# Bin pressure change into intervals for cleaner visualization
combined <- combined |>
  mutate(
    pressure_bin = cut(pressure_change,
      breaks = seq(-10, 10, by = 1),
      include.lowest = TRUE
    )
  )

binned_summary <- combined |>
  filter(!is.na(pressure_bin)) |>
  group_by(pressure_bin) |>
  summarize(
    mean_hp = mean(hp_power, na.rm = TRUE),
    median_hp = median(hp_power, na.rm = TRUE),
    mean_outdoor = mean(outdoor, na.rm = TRUE),
    mean_change = mean(pressure_change, na.rm = TRUE),
    n = n(),
    .groups = "drop"
  ) |>
  filter(n >= 5)

cat("\n=== HP Power by Pressure Change Bin ===\n")
print(binned_summary |> select(pressure_bin, mean_hp, mean_outdoor, n))

p1 <- ggplot(combined, aes(x = pressure_change, y = hp_power)) +
  geom_bin2d(bins = 40) +
  scale_fill_viridis_c(option = "magma", trans = "log10", name = "Hours") +
  geom_smooth(method = "loess", color = COLORS$heat_pump, linewidth = 1.3,
              se = TRUE, fill = COLORS$heat_pump, alpha = 0.15) +
  geom_vline(xintercept = 0, linetype = "dashed", color = COLORS$muted) +
  labs(
    x     = "6-Hour Pressure Change (hPa)",
    y     = "HP Electrical Power (W)",
    title = "Pressure Change vs Heat Pump Demand",
    subtitle = "Heating season only (outdoor < 12\u00b0C). Negative = dropping pressure (cold front)."
  ) +
  theme_energy()

save_plot(p1, "34_pressure_change_vs_heating.png")

# ============================================================================
# Chart 2: Cross-correlation — pressure change leads heating demand
# ============================================================================
# For each lag (0-24h), compute the correlation between pressure rate-of-change
# and HP consumption shifted by that lag. Positive lag = pressure change happens
# BEFORE the HP demand change.

# Build a time-aligned series (fill gaps with NA for proper lagging)
ts_pressure <- pressure |>
  select(hour_bucket, pressure_change) |>
  arrange(hour_bucket)

ts_hp <- hp_cons |>
  select(hour_bucket, hp_power = avg) |>
  inner_join(outdoor_temp |> select(hour_bucket, outdoor = avg), by = "hour_bucket") |>
  filter(outdoor < 12) |>  # heating season only
  arrange(hour_bucket)

# Merge into a common timeline
ts_merged <- ts_pressure |>
  inner_join(ts_hp, by = "hour_bucket") |>
  filter(!is.na(pressure_change), !is.na(hp_power)) |>
  arrange(hour_bucket)

if (nrow(ts_merged) >= 50) {
  lags <- 0:24

  cross_corr <- map_dfr(lags, function(lag_h) {
    n <- nrow(ts_merged)
    if (lag_h >= n) return(tibble(lag = lag_h, correlation = NA_real_))

    # Pressure change at time t, HP power at time t + lag_h
    p_change <- ts_merged$pressure_change[1:(n - lag_h)]
    hp_demand <- ts_merged$hp_power[(1 + lag_h):n]

    valid <- !is.na(p_change) & !is.na(hp_demand)
    if (sum(valid) < 20) return(tibble(lag = lag_h, correlation = NA_real_))

    tibble(
      lag = lag_h,
      correlation = cor(p_change[valid], hp_demand[valid])
    )
  })

  cross_corr <- cross_corr |> filter(!is.na(correlation))

  cat("\n=== Cross-Correlation: Pressure Change → HP Demand ===\n")
  cat("  Peak lag:", cross_corr |> filter(abs(correlation) == max(abs(correlation))) |>
      pull(lag), "hours\n")
  cat("  Peak correlation:", round(min(cross_corr$correlation), 3), "\n")

  p2 <- ggplot(cross_corr, aes(x = lag, y = correlation)) +
    geom_col(aes(fill = correlation < 0), alpha = 0.7, width = 0.8) +
    scale_fill_manual(values = c("TRUE" = COLORS$import, "FALSE" = COLORS$export),
                      guide = "none") +
    geom_hline(yintercept = 0, color = COLORS$muted) +
    # Significance threshold (approximate for large N)
    geom_hline(yintercept = c(-2 / sqrt(nrow(ts_merged)), 2 / sqrt(nrow(ts_merged))),
               linetype = "dashed", color = COLORS$muted, alpha = 0.6) +
    scale_x_continuous(breaks = seq(0, 24, 3)) +
    labs(
      x     = "Lag (hours, pressure change leads HP demand)",
      y     = "Correlation",
      title = "Cross-Correlation: Pressure Rate-of-Change vs HP Demand",
      subtitle = "Negative correlation at positive lag = pressure drop predicts heating increase. Dashed = 95% CI."
    ) +
    theme_energy()

  save_plot(p2, "34_pressure_crosscorr.png")
} else {
  cat("Insufficient time-aligned data for cross-correlation (need >= 50, have",
      nrow(ts_merged), ").\n")
}

# ============================================================================
# Chart 3: Pressure regime classification — HP demand by regime
# ============================================================================
regime_summary <- combined |>
  group_by(regime) |>
  summarize(
    mean_hp      = mean(hp_power, na.rm = TRUE),
    median_hp    = median(hp_power, na.rm = TRUE),
    mean_outdoor = mean(outdoor, na.rm = TRUE),
    n = n(),
    .groups = "drop"
  )

cat("\n=== HP Power by Pressure Regime ===\n")
print(regime_summary)

# Check that at least 2 regimes have data
if (sum(regime_summary$n >= 5) >= 2) {
  combined_plot <- combined |>
    filter(!is.na(hp_power), hp_power >= 0)

  p3 <- ggplot(combined_plot, aes(x = regime, y = hp_power, fill = regime)) +
    geom_boxplot(alpha = 0.7, outlier.alpha = 0.2, outlier.size = 0.8) +
    scale_fill_manual(values = c(
      "Falling (<-1 hPa/6h)" = COLORS$import,
      "Stable"                = COLORS$muted,
      "Rising (>+1 hPa/6h)"  = COLORS$export
    )) +
    # Add count labels below
    geom_text(data = regime_summary |> filter(n >= 5),
              aes(x = regime, y = -Inf, label = paste0("n=", n)),
              vjust = -0.5, color = COLORS$muted, size = 3.5,
              inherit.aes = FALSE) +
    labs(
      x     = "Pressure Regime (6-hour change)",
      y     = "HP Electrical Power (W)",
      title = "Heat Pump Demand by Pressure Regime",
      subtitle = "Heating season (outdoor < 12\u00b0C). Falling pressure = incoming cold front."
    ) +
    theme_energy() +
    theme(legend.position = "none")

  save_plot(p3, "34_pressure_regime_heating.png")
} else {
  cat("Insufficient data in pressure regimes for box plot.\n")
}

cat("\n=== Pressure & Heating Analysis Complete ===\n")
