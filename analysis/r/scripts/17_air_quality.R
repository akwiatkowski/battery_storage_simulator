# ============================================================================
# 17_air_quality.R — Indoor Air Quality Analysis
# ============================================================================
# WHAT:    Analyzes CO2 levels, noise, and atmospheric pressure from Netatmo
#          sensors. Identifies occupancy patterns and ventilation needs.
#
# INPUTS:  load_stats_sensor() for Netatmo CO2, noise, pressure, humidity
#
# OUTPUTS: output/17_co2_daily_pattern.png   — CO2 by hour (bedroom vs living)
#          output/17_noise_pattern.png       — noise level by hour
#          output/17_pressure_trend.png      — atmospheric pressure over time
#
# HOW TO READ:
#   - CO2 pattern: peaks indicate occupancy, >1000 ppm = ventilation needed
#   - Noise: daily activity pattern visible as higher daytime noise
#   - Pressure: weather system correlation (low pressure = storms)
# ============================================================================

source("analysis/r/R/load_data.R")

# ============================================================================
# Load air quality data
# ============================================================================
co2_bedroom <- load_stats_sensor(NETATMO_BEDROOM_CO2)
co2_living  <- load_stats_sensor(NETATMO_LIVING_CO2)
noise       <- load_stats_sensor(NETATMO_LIVING_NOISE)
pressure    <- load_stats_sensor(NETATMO_LIVING_PRESSURE)

cat("\n=== Air Quality Data ===\n")
cat("  CO2 bedroom:", nrow(co2_bedroom), "hours\n")
cat("  CO2 living: ", nrow(co2_living), "hours\n")
cat("  Noise:      ", nrow(noise), "hours\n")
cat("  Pressure:   ", nrow(pressure), "hours\n")

# ============================================================================
# Chart 1: CO2 daily pattern — bedroom vs living room
# ============================================================================
co2_data <- bind_rows(
  co2_bedroom |> mutate(room = "Bedroom") |> select(hour_bucket, room, co2 = avg),
  co2_living  |> mutate(room = "Living Room") |> select(hour_bucket, room, co2 = avg)
)

if (nrow(co2_data) > 20) {
  co2_hourly <- co2_data |>
    mutate(hour = hour(hour_bucket)) |>
    group_by(room, hour) |>
    summarize(
      avg_co2 = mean(co2, na.rm = TRUE),
      p25     = quantile(co2, 0.25, na.rm = TRUE),
      p75     = quantile(co2, 0.75, na.rm = TRUE),
      .groups = "drop"
    )

  p1 <- ggplot(co2_hourly, aes(x = hour, y = avg_co2, color = room, fill = room)) +
    geom_ribbon(aes(ymin = p25, ymax = p75), alpha = 0.15, color = NA) +
    geom_line(linewidth = 1.2) +
    geom_hline(yintercept = 1000, linetype = "dashed", color = COLORS$warning) +
    annotate("text", x = 0.5, y = 1050, label = "1000 ppm threshold",
             color = COLORS$warning, size = 3, hjust = 0) +
    scale_color_manual(values = c("Bedroom" = COLORS$prediction, "Living Room" = COLORS$charge)) +
    scale_fill_manual(values = c("Bedroom" = COLORS$prediction, "Living Room" = COLORS$charge)) +
    scale_x_continuous(breaks = seq(0, 23, 3)) +
    labs(
      x     = "Hour of Day",
      y     = "CO2 (ppm)",
      title = "CO2 Levels by Room — Daily Pattern",
      subtitle = "Bedroom peaks at night (occupancy), living room peaks in evening. Band = IQR.",
      color = "", fill = ""
    ) +
    theme_energy()

  save_plot(p1, "17_co2_daily_pattern.png")
} else {
  cat("Insufficient CO2 data.\n")
}

# ============================================================================
# Chart 2: Noise level daily pattern
# ============================================================================
if (nrow(noise) > 20) {
  noise_hourly <- noise |>
    mutate(hour = hour(hour_bucket)) |>
    group_by(hour) |>
    summarize(
      avg_noise = mean(avg, na.rm = TRUE),
      max_noise = mean(max_val, na.rm = TRUE),
      min_noise = mean(min_val, na.rm = TRUE),
      .groups   = "drop"
    )

  p2 <- ggplot(noise_hourly, aes(x = hour)) +
    geom_ribbon(aes(ymin = min_noise, ymax = max_noise), fill = COLORS$charge, alpha = 0.2) +
    geom_line(aes(y = avg_noise), color = COLORS$charge, linewidth = 1.2) +
    geom_point(aes(y = avg_noise), color = COLORS$charge, size = 1.5) +
    scale_x_continuous(breaks = seq(0, 23, 3)) +
    labs(
      x     = "Hour of Day",
      y     = "Noise Level (dB)",
      title = "Indoor Noise Level — Daily Pattern",
      subtitle = "Living room Netatmo sensor. Band = avg min/max range.",
    ) +
    theme_energy()

  save_plot(p2, "17_noise_pattern.png")
} else {
  cat("Insufficient noise data.\n")
}

# ============================================================================
# Chart 3: Atmospheric pressure trend
# ============================================================================
if (nrow(pressure) > 20) {
  p3 <- ggplot(pressure, aes(x = hour_bucket, y = avg)) +
    geom_line(color = COLORS$muted, alpha = 0.5, linewidth = 0.3) +
    geom_smooth(method = "loess", span = 0.1, color = COLORS$prediction, linewidth = 1, se = FALSE) +
    labs(
      x     = "",
      y     = "Atmospheric Pressure (mbar)",
      title = "Atmospheric Pressure Trend",
      subtitle = "Drops correlate with weather fronts. Smooth = loess trend."
    ) +
    theme_energy()

  save_plot(p3, "17_pressure_trend.png")
} else {
  cat("Insufficient pressure data.\n")
}
