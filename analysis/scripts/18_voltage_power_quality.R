# ============================================================================
# 18_voltage_power_quality.R — Voltage & Power Quality Analysis
# ============================================================================
# WHAT:    Analyzes grid voltage patterns, per-circuit voltage comparison,
#          power factor behavior, and correlation with PV export.
#
# INPUTS:  load_stats_sensor() for voltage, power factor, reactive power sensors
#
# OUTPUTS: output/18_voltage_profile.png     — grid voltage by hour
#          output/18_circuit_voltage.png     — cross-circuit voltage comparison
#          output/18_power_factor.png        — power factor by hour and load
#          output/18_voltage_vs_export.png   — voltage during PV export
#
# HOW TO READ:
#   - Voltage profile: higher at night (low load), dips during peak demand
#   - Circuit voltage: differences indicate wiring voltage drop
#   - Power factor: < 0.9 = reactive power penalty, common with HP
#   - Export correlation: rising voltage during PV export = grid saturation
# ============================================================================

source("analysis/helpers/load_data.R")

# ============================================================================
# Load power quality data
# ============================================================================
grid_voltage   <- load_stats_sensor(VOLTAGE_SENSOR)
power_factor   <- load_stats_sensor(POWER_FACTOR_SENSOR)
reactive_power <- load_stats_sensor(REACTIVE_POWER_SENSOR)

# Per-circuit voltages
v_office1 <- load_stats_sensor(VOLTAGE_OFFICE1)
v_office2 <- load_stats_sensor(VOLTAGE_OFFICE2)
v_external <- load_stats_sensor(VOLTAGE_EXTERNAL)
v_living_lamp <- load_stats_sensor(VOLTAGE_LIVING_LAMP)
v_living_media <- load_stats_sensor(VOLTAGE_LIVING_MEDIA)

cat("\n=== Power Quality Data ===\n")
cat("  Grid voltage:   ", nrow(grid_voltage), "hours\n")
cat("  Power factor:   ", nrow(power_factor), "hours\n")
cat("  Reactive power: ", nrow(reactive_power), "hours\n")
cat("  Circuit voltages: office1=", nrow(v_office1), " office2=", nrow(v_office2),
    " external=", nrow(v_external), "\n")

# ============================================================================
# Chart 1: Grid voltage profile by hour of day
# ============================================================================
if (nrow(grid_voltage) > 20) {
  v_hourly <- grid_voltage |>
    mutate(hour = hour(hour_bucket)) |>
    group_by(hour) |>
    summarize(
      avg_v = mean(avg, na.rm = TRUE),
      min_v = mean(min_val, na.rm = TRUE),
      max_v = mean(max_val, na.rm = TRUE),
      p5_v  = quantile(avg, 0.05, na.rm = TRUE),
      p95_v = quantile(avg, 0.95, na.rm = TRUE),
      .groups = "drop"
    )

  p1 <- ggplot(v_hourly, aes(x = hour)) +
    geom_ribbon(aes(ymin = p5_v, ymax = p95_v), fill = COLORS$charge, alpha = 0.2) +
    geom_ribbon(aes(ymin = min_v, ymax = max_v), fill = COLORS$charge, alpha = 0.1) +
    geom_line(aes(y = avg_v), color = COLORS$charge, linewidth = 1.2) +
    geom_hline(yintercept = 230, linetype = "dashed", color = COLORS$muted) +
    geom_hline(yintercept = 253, linetype = "dashed", color = COLORS$import) +
    annotate("text", x = 0.5, y = 254, label = "253V curtailment", color = COLORS$import,
             size = 2.5, hjust = 0) +
    scale_x_continuous(breaks = seq(0, 23, 3)) +
    labs(
      x     = "Hour of Day",
      y     = "Grid Voltage (V)",
      title = "Grid Voltage — Daily Profile",
      subtitle = "Light band = P5–P95, dark band = avg min/max within hour",
    ) +
    theme_energy()

  save_plot(p1, "18_voltage_profile.png")
} else {
  cat("Insufficient grid voltage data.\n")
}

# ============================================================================
# Chart 2: Cross-circuit voltage comparison
# ============================================================================
circuit_data <- bind_rows(
  v_office1      |> mutate(circuit = "Office 1")      |> select(hour_bucket, circuit, voltage = avg),
  v_office2      |> mutate(circuit = "Office 2")      |> select(hour_bucket, circuit, voltage = avg),
  v_external     |> mutate(circuit = "External")      |> select(hour_bucket, circuit, voltage = avg),
  v_living_lamp  |> mutate(circuit = "Living Lamp")   |> select(hour_bucket, circuit, voltage = avg),
  v_living_media |> mutate(circuit = "Living Media")  |> select(hour_bucket, circuit, voltage = avg),
  grid_voltage   |> mutate(circuit = "Grid (meter)")  |> select(hour_bucket, circuit, voltage = avg)
)

if (nrow(circuit_data) > 20) {
  circuit_summary <- circuit_data |>
    group_by(circuit) |>
    summarize(
      mean_v = mean(voltage, na.rm = TRUE),
      sd_v   = sd(voltage, na.rm = TRUE),
      min_v  = min(voltage, na.rm = TRUE),
      max_v  = max(voltage, na.rm = TRUE),
      .groups = "drop"
    ) |>
    arrange(desc(mean_v))

  cat("\n=== Circuit Voltage Summary ===\n")
  print(circuit_summary)

  p2 <- ggplot(circuit_summary, aes(x = reorder(circuit, mean_v), y = mean_v)) +
    geom_col(fill = COLORS$charge, alpha = 0.7, width = 0.6) +
    geom_errorbar(aes(ymin = mean_v - sd_v, ymax = mean_v + sd_v),
                  width = 0.2, color = COLORS$import) +
    coord_flip(ylim = c(min(circuit_summary$mean_v) - 5,
                         max(circuit_summary$mean_v) + 5)) +
    labs(
      x     = "",
      y     = "Average Voltage (V)",
      title = "Voltage by Circuit",
      subtitle = "Differences indicate wiring voltage drop from meter to outlet",
    ) +
    theme_energy()

  save_plot(p2, "18_circuit_voltage.png")
} else {
  cat("Insufficient circuit voltage data.\n")
}

# ============================================================================
# Chart 3: Power factor by hour
# ============================================================================
if (nrow(power_factor) > 20) {
  pf_hourly <- power_factor |>
    mutate(hour = hour(hour_bucket)) |>
    group_by(hour) |>
    summarize(
      avg_pf  = mean(avg, na.rm = TRUE),
      p25_pf  = quantile(avg, 0.25, na.rm = TRUE),
      p75_pf  = quantile(avg, 0.75, na.rm = TRUE),
      .groups = "drop"
    )

  p3 <- ggplot(pf_hourly, aes(x = hour)) +
    geom_ribbon(aes(ymin = p25_pf, ymax = p75_pf), fill = COLORS$pv, alpha = 0.2) +
    geom_line(aes(y = avg_pf), color = COLORS$pv, linewidth = 1.2) +
    geom_hline(yintercept = 90, linetype = "dashed", color = COLORS$export) +
    annotate("text", x = 0.5, y = 91, label = "90% target", color = COLORS$export,
             size = 3, hjust = 0) +
    scale_x_continuous(breaks = seq(0, 23, 3)) +
    labs(
      x     = "Hour of Day",
      y     = "Power Factor (%)",
      title = "Power Factor — Daily Profile",
      subtitle = "Below 90% = significant reactive power. Band = IQR.",
    ) +
    theme_energy()

  save_plot(p3, "18_power_factor.png")
} else {
  cat("Insufficient power factor data.\n")
}

# ============================================================================
# Chart 4: Voltage vs PV export (grid power < 0 = export)
# ============================================================================
if (nrow(grid_voltage) > 20 && nrow(hourly) > 20) {
  # Join voltage with grid power
  v_export <- grid_voltage |>
    select(hour_bucket, voltage = avg) |>
    inner_join(hourly |> select(hour_bucket, grid_power = avg_power), by = "hour_bucket") |>
    filter(!is.na(voltage), !is.na(grid_power))

  if (nrow(v_export) > 20) {
    p4 <- ggplot(v_export, aes(x = grid_power, y = voltage)) +
      geom_bin2d(bins = 50) +
      scale_fill_viridis_c(option = "plasma", trans = "log10") +
      geom_smooth(method = "loess", color = COLORS$import, linewidth = 1, se = FALSE) +
      geom_vline(xintercept = 0, linetype = "dashed", color = COLORS$muted) +
      annotate("text", x = -100, y = max(v_export$voltage) - 1,
               label = "← Export", color = COLORS$export, size = 3) +
      annotate("text", x = 100, y = max(v_export$voltage) - 1,
               label = "Import →", color = COLORS$import, size = 3) +
      labs(
        x     = "Grid Power (W, negative = export)",
        y     = "Grid Voltage (V)",
        title = "Voltage vs Grid Power",
        subtitle = "Voltage rises during export — grid saturation indicator",
        fill  = "Count"
      ) +
      theme_energy()

    save_plot(p4, "18_voltage_vs_export.png")
  }
} else {
  cat("Insufficient data for voltage vs export analysis.\n")
}
