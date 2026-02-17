# ============================================================================
# 36_noise_occupancy.R — Noise as Occupancy Proxy
# ============================================================================
# WHAT:    Uses Netatmo indoor noise level as an occupancy proxy. Quiet periods
#          during daytime likely indicate nobody home. Analyzes correlation
#          between noise (occupancy) and energy consumption, and estimates
#          heating energy wasted during unoccupied hours.
#
# INPUTS:  load_stats_sensor() for NETATMO_LIVING_NOISE (dB),
#          HP_CONSUMPTION (W), hourly grid power from load_data.R
#
# OUTPUTS: output/36_noise_daily_pattern.png       — noise by hour of day
#          output/36_noise_vs_energy.png            — grid consumption by noise level
#          output/36_unoccupied_heating.png         — heating during likely-empty hours
#
# HOW TO READ:
#   - Daily pattern: quiet at night (sleep), peaks during active hours.
#     Clear day/night signature confirms noise as occupancy proxy.
#   - Noise vs energy: higher noise (more activity) should correlate with
#     higher consumption. The gap between quiet and active shows occupancy-
#     driven load.
#   - Unoccupied heating: energy spent heating when nobody seems home.
#     Setback potential = savings from reducing heating during these hours.
# ============================================================================

source("analysis/r/R/load_data.R")

# ============================================================================
# Load noise, heating, and grid data
# ============================================================================
noise_raw <- load_stats_sensor(NETATMO_LIVING_NOISE) |>
  distinct(hour_bucket, .keep_all = TRUE)
hp_cons   <- load_stats_sensor(HP_CONSUMPTION) |>
  distinct(hour_bucket, .keep_all = TRUE)

cat("\n=== Noise Data ===\n")
cat("  Noise hours:       ", nrow(noise_raw), "\n")
cat("  HP consumption:    ", nrow(hp_cons), "hours\n")
cat("  Grid hourly:       ", nrow(hourly), "hours\n")

if (nrow(noise_raw) < 20) {
  cat("Insufficient noise data (need >= 20, have", nrow(noise_raw), ").\n")
  cat("Netatmo noise sensor may not be available. Skipping noise analysis.\n")
  quit(save = "no")
}

# ============================================================================
# Prepare noise data with time features
# ============================================================================
noise <- noise_raw |>
  select(hour_bucket, noise_db = avg) |>
  filter(!is.na(noise_db), noise_db >= 0, noise_db <= 120) |>
  mutate(
    hour = hour(hour_bucket),
    weekday = wday(hour_bucket, label = TRUE, week_start = 1),
    is_weekend = wday(hour_bucket, week_start = 1) >= 6,
    noise_level = case_when(
      noise_db < 35  ~ "Quiet (<35 dB)",
      noise_db < 45  ~ "Normal (35-45 dB)",
      noise_db >= 45 ~ "Active (>45 dB)"
    ),
    noise_level = factor(noise_level, levels = c(
      "Quiet (<35 dB)", "Normal (35-45 dB)", "Active (>45 dB)"
    ))
  )

cat("  Valid noise rows:  ", nrow(noise), "\n")
cat("  dB range:          ", round(min(noise$noise_db), 1), "to",
    round(max(noise$noise_db), 1), "dB\n")
cat("  Noise level distribution:\n")
print(table(noise$noise_level))

# ============================================================================
# Chart 1: Noise daily pattern — occupancy signature
# ============================================================================
hourly_noise <- noise |>
  group_by(hour, is_weekend) |>
  summarize(
    mean_db   = mean(noise_db, na.rm = TRUE),
    median_db = median(noise_db, na.rm = TRUE),
    p25       = quantile(noise_db, 0.25, na.rm = TRUE),
    p75       = quantile(noise_db, 0.75, na.rm = TRUE),
    .groups = "drop"
  ) |>
  mutate(day_type = if_else(is_weekend, "Weekend", "Weekday"))

p1 <- ggplot(hourly_noise, aes(x = hour)) +
  geom_ribbon(aes(ymin = p25, ymax = p75, fill = day_type), alpha = 0.2) +
  geom_line(aes(y = mean_db, color = day_type), linewidth = 1.2) +
  geom_point(aes(y = mean_db, color = day_type), size = 1.5) +
  scale_color_manual(values = c("Weekday" = COLORS$charge, "Weekend" = COLORS$heat_pump)) +
  scale_fill_manual(values = c("Weekday" = COLORS$charge, "Weekend" = COLORS$heat_pump)) +
  # Occupancy threshold lines
  geom_hline(yintercept = 35, linetype = "dashed", color = COLORS$muted, alpha = 0.6) +
  annotate("text", x = 23, y = 35, label = "35 dB (quiet)",
           hjust = 1, vjust = -0.5, color = COLORS$muted, size = 3) +
  geom_hline(yintercept = 45, linetype = "dashed", color = COLORS$muted, alpha = 0.6) +
  annotate("text", x = 23, y = 45, label = "45 dB (active)",
           hjust = 1, vjust = -0.5, color = COLORS$muted, size = 3) +
  scale_x_continuous(breaks = seq(0, 23, 2)) +
  labs(
    x     = "Hour of Day",
    y     = "Noise Level (dB)",
    title = "Indoor Noise Daily Pattern",
    subtitle = "Noise as occupancy proxy. Quiet at night, active during day. IQR ribbon shown.",
    color = "", fill = ""
  ) +
  theme_energy()

save_plot(p1, "36_noise_daily_pattern.png")

# ============================================================================
# Chart 2: Noise vs grid power consumption
# ============================================================================
noise_grid <- noise |>
  inner_join(hourly |> select(hour_bucket, grid_power = avg_power), by = "hour_bucket") |>
  filter(!is.na(grid_power))

if (nrow(noise_grid) >= 20) {
  # Summary by noise level
  consumption_by_noise <- noise_grid |>
    group_by(noise_level) |>
    summarize(
      mean_power   = mean(grid_power, na.rm = TRUE),
      median_power = median(grid_power, na.rm = TRUE),
      n = n(),
      .groups = "drop"
    )

  cat("\n=== Grid Consumption by Noise Level ===\n")
  print(consumption_by_noise)

  p2 <- ggplot(noise_grid, aes(x = noise_level, y = grid_power, fill = noise_level)) +
    geom_boxplot(alpha = 0.7, outlier.alpha = 0.15, outlier.size = 0.5) +
    scale_fill_manual(values = c(
      "Quiet (<35 dB)"   = COLORS$export,
      "Normal (35-45 dB)" = COLORS$charge,
      "Active (>45 dB)"   = COLORS$import
    )) +
    geom_hline(yintercept = 0, linetype = "dashed", color = COLORS$muted) +
    # Count labels
    geom_text(data = consumption_by_noise,
              aes(x = noise_level, y = -Inf,
                  label = paste0("n=", format(n, big.mark = ","))),
              vjust = -0.5, color = COLORS$muted, size = 3.5,
              inherit.aes = FALSE) +
    labs(
      x     = "Noise Level (Occupancy Proxy)",
      y     = "Grid Power (W, positive = import)",
      title = "Energy Consumption by Noise Level",
      subtitle = "Higher noise (more activity) correlates with higher consumption. Below 0 = net export."
    ) +
    theme_energy() +
    theme(legend.position = "none")

  save_plot(p2, "36_noise_vs_energy.png")
} else {
  cat("Insufficient noise + grid data overlap for consumption analysis.\n")
}

# ============================================================================
# Chart 3: Unoccupied heating cost estimate
# ============================================================================
# "Unoccupied" = noise < 30 dB for >= 3 consecutive hours during 8:00-22:00
# (nighttime quiet is normal sleep, not absence)

noise_hp <- noise |>
  filter(hour >= 8, hour < 22) |>   # daytime only
  inner_join(hp_cons |> select(hour_bucket, hp_power = avg), by = "hour_bucket") |>
  filter(!is.na(hp_power)) |>
  arrange(hour_bucket) |>
  mutate(
    very_quiet = noise_db < 30,
    date = as.Date(hour_bucket)
  )

if (nrow(noise_hp) >= 20) {
  # Identify runs of consecutive very-quiet hours using rle
  # We work per-day to avoid spanning midnight
  unoccupied_hours <- noise_hp |>
    group_by(date) |>
    arrange(hour_bucket) |>
    mutate(
      # Detect transitions in quiet status
      run_id = cumsum(c(1, diff(very_quiet) != 0 | diff(as.numeric(hour_bucket)) > 3700)),
      .groups = "drop"
    ) |>
    ungroup()

  # Find runs of quiet >= 3 consecutive hours
  quiet_runs <- unoccupied_hours |>
    filter(very_quiet) |>
    group_by(date, run_id) |>
    summarize(
      run_length = n(),
      total_hp_wh = sum(hp_power, na.rm = TRUE),  # W * 1h = Wh
      .groups = "drop"
    ) |>
    filter(run_length >= 3)

  # Mark individual hours as "likely unoccupied"
  unoccupied_runs <- unoccupied_hours |>
    filter(very_quiet) |>
    semi_join(quiet_runs, by = c("date", "run_id"))

  total_daytime_hp_kwh <- sum(noise_hp$hp_power, na.rm = TRUE) / 1000
  unoccupied_hp_kwh    <- sum(unoccupied_runs$hp_power, na.rm = TRUE) / 1000
  total_daytime_hours  <- nrow(noise_hp)
  unoccupied_hours_n   <- nrow(unoccupied_runs)

  cat("\n=== Unoccupied Heating Analysis (8:00-22:00) ===\n")
  cat("  Total daytime hours:   ", total_daytime_hours, "\n")
  cat("  Likely unoccupied:     ", unoccupied_hours_n, "hours (",
      round(unoccupied_hours_n / total_daytime_hours * 100, 1), "%)\n")
  cat("  HP during unoccupied:  ", round(unoccupied_hp_kwh, 1), "kWh\n")
  cat("  HP during total day:   ", round(total_daytime_hp_kwh, 1), "kWh\n")
  cat("  Unoccupied fraction:   ",
      round(unoccupied_hp_kwh / max(total_daytime_hp_kwh, 0.01) * 100, 1), "%\n")

  # Setback savings estimate: assume 30% reduction with a 2°C setback
  setback_savings_kwh <- unoccupied_hp_kwh * 0.30
  cat("  Estimated setback savings (30% reduction):", round(setback_savings_kwh, 1), "kWh\n")

  # Build summary for bar chart
  heating_breakdown <- tibble(
    category = c("Occupied", "Unoccupied\n(nobody home)", "Potential\nSetback Savings"),
    kwh = c(
      total_daytime_hp_kwh - unoccupied_hp_kwh,
      unoccupied_hp_kwh,
      setback_savings_kwh
    ),
    type = c("normal", "waste", "savings")
  ) |>
    mutate(category = factor(category, levels = category))

  p3 <- ggplot(heating_breakdown, aes(x = category, y = kwh, fill = type)) +
    geom_col(alpha = 0.75, width = 0.6) +
    geom_text(aes(label = paste0(round(kwh, 1), " kWh")),
              vjust = -0.3, color = COLORS$text, size = 4) +
    scale_fill_manual(values = c(
      "normal"  = COLORS$heat_pump,
      "waste"   = COLORS$warning,
      "savings" = COLORS$export
    )) +
    labs(
      x     = "",
      y     = "Heat Pump Energy (kWh)",
      title = "Daytime Heating: Occupied vs Unoccupied",
      subtitle = paste0(
        "Unoccupied = noise < 30 dB for 3+ consecutive hours (8-22h). ",
        "Setback = 30% savings estimate with 2\u00b0C reduction."
      )
    ) +
    theme_energy() +
    theme(legend.position = "none")

  save_plot(p3, "36_unoccupied_heating.png")
} else {
  cat("Insufficient noise + HP overlap for unoccupied heating analysis.\n")
}

cat("\n=== Noise & Occupancy Analysis Complete ===\n")
