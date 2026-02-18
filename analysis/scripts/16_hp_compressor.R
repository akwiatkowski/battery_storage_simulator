# ============================================================================
# 16_hp_compressor.R — Heat Pump Compressor Diagnostics
# ============================================================================
# WHAT:    Analyzes heat pump internal diagnostics: compressor speed, discharge
#          temperature, refrigerant pressure, pump flow, and true thermal power.
#          Compares sensor COP vs calculated COP.
#
# INPUTS:  load_stats_sensor() / load_recent_sensor() for HP diagnostic sensors
#
# OUTPUTS: output/16_compressor_vs_cop.png   — compressor speed vs COP
#          output/16_thermal_power.png       — true thermal power vs reported
#          output/16_refrigerant_cycle.png   — discharge temp vs pressure
#
# HOW TO READ:
#   - Compressor vs COP: lower speed = higher COP (part-load efficiency)
#   - Thermal power: deviation between flow×ΔT and reported production
#     indicates sensor accuracy
#   - Refrigerant cycle: higher pressure + higher discharge temp = harder work
# ============================================================================

source("analysis/helpers/load_data.R")

# ============================================================================
# Load HP diagnostic data
# ============================================================================
cop_sensor    <- load_stats_sensor(HP_COP_SENSOR)
comp_speed    <- load_stats_sensor(HP_COMPRESSOR_SPEED)
fan_speed     <- load_stats_sensor(HP_FAN_SPEED)
high_pressure <- load_stats_sensor(HP_HIGH_PRESSURE)
discharge_temp <- load_stats_sensor(HP_DISCHARGE_TEMP)
pump_flow     <- load_stats_sensor(HP_PUMP_FLOW)
inlet_temp_s  <- load_stats_sensor(HP_INLET_TEMP)
outlet_temp_s <- load_stats_sensor(HP_OUTLET_TEMP)
hp_prod_s     <- load_stats_sensor(HP_PRODUCTION)
hp_cons_s     <- load_stats_sensor(HP_CONSUMPTION)

cat("\n=== HP Diagnostic Data ===\n")
cat("  COP sensor:       ", nrow(cop_sensor), "hours\n")
cat("  Compressor speed: ", nrow(comp_speed), "hours\n")
cat("  Discharge temp:   ", nrow(discharge_temp), "hours\n")
cat("  Pump flow:        ", nrow(pump_flow), "hours\n")
cat("  High pressure:    ", nrow(high_pressure), "hours\n")

# ============================================================================
# Chart 1: Compressor speed vs COP
# ============================================================================
# Join compressor speed with COP sensor data by hour
if (nrow(comp_speed) > 50 && nrow(cop_sensor) > 50) {
  speed_cop <- comp_speed |>
    select(hour_bucket, speed = avg) |>
    inner_join(cop_sensor |> select(hour_bucket, cop = avg), by = "hour_bucket") |>
    filter(speed > 0, cop > 0.5, cop < 10)

  cat("\n=== Compressor Speed vs COP ===\n")
  cat("  Joined records:", nrow(speed_cop), "\n")

  if (nrow(speed_cop) > 20) {
    p1 <- ggplot(speed_cop, aes(x = speed, y = cop)) +
      geom_bin2d(bins = 40) +
      scale_fill_viridis_c(option = "plasma", trans = "log10") +
      geom_smooth(method = "loess", color = COLORS$export, linewidth = 1, se = FALSE) +
      labs(
        x     = "Compressor Speed (RPM)",
        y     = "COP",
        title = "Compressor Speed vs COP",
        subtitle = "Lower speed = part-load operation = higher efficiency",
        fill  = "Count"
      ) +
      theme_energy()

    save_plot(p1, "16_compressor_vs_cop.png")
  }
} else {
  cat("Insufficient compressor speed or COP data.\n")
}

# ============================================================================
# Chart 2: True thermal power vs reported production
# ============================================================================
# True thermal power = flow (L/min) × ΔT (°C) × 69.77 W/(L/min·°C)
# Compare with reported HP production sensor
if (nrow(pump_flow) > 50 && nrow(inlet_temp_s) > 50 && nrow(outlet_temp_s) > 50) {
  thermal <- pump_flow |>
    select(hour_bucket, flow = avg) |>
    inner_join(inlet_temp_s |> select(hour_bucket, inlet = avg), by = "hour_bucket") |>
    inner_join(outlet_temp_s |> select(hour_bucket, outlet = avg), by = "hour_bucket") |>
    mutate(
      delta_t = outlet - inlet,
      thermal_power_w = flow * delta_t * 69.77
    ) |>
    filter(flow > 0, delta_t > 0, thermal_power_w > 0, thermal_power_w < 15000)

  # Join with reported production if available
  if (nrow(hp_prod_s) > 50) {
    thermal <- thermal |>
      inner_join(hp_prod_s |> select(hour_bucket, reported_w = avg), by = "hour_bucket") |>
      filter(reported_w > 0)
  }

  cat("\n=== True Thermal Power ===\n")
  cat("  Records:", nrow(thermal), "\n")
  if (nrow(thermal) > 0) {
    cat("  Avg thermal power:", round(mean(thermal$thermal_power_w)), "W\n")
    cat("  Avg flow:", round(mean(thermal$flow), 1), "L/min\n")
    cat("  Avg ΔT:", round(mean(thermal$delta_t), 1), "°C\n")
  }

  if (nrow(thermal) > 20 && "reported_w" %in% names(thermal)) {
    p2 <- ggplot(thermal, aes(x = reported_w, y = thermal_power_w)) +
      geom_bin2d(bins = 40) +
      scale_fill_viridis_c(option = "plasma", trans = "log10") +
      geom_abline(slope = 1, intercept = 0, linetype = "dashed", color = COLORS$import) +
      geom_smooth(method = "lm", color = COLORS$export, linewidth = 1, se = FALSE) +
      labs(
        x     = "Reported Production (W)",
        y     = "True Thermal Power (W) — Flow × ΔT × 69.77",
        title = "True Thermal Power vs Reported HP Production",
        subtitle = "Dashed = 1:1 line. Deviation indicates sensor measurement gap.",
        fill  = "Count"
      ) +
      theme_energy()

    save_plot(p2, "16_thermal_power.png")
  } else if (nrow(thermal) > 20) {
    # No reported production — just show thermal power distribution by hour
    thermal_hourly <- thermal |>
      mutate(hour = hour(hour_bucket)) |>
      group_by(hour) |>
      summarize(
        avg_power = mean(thermal_power_w),
        avg_flow  = mean(flow),
        avg_dt    = mean(delta_t),
        .groups   = "drop"
      )

    p2 <- ggplot(thermal_hourly, aes(x = hour, y = avg_power)) +
      geom_col(fill = COLORS$heat_pump, alpha = 0.7) +
      scale_x_continuous(breaks = seq(0, 23, 3)) +
      labs(
        x     = "Hour of Day",
        y     = "Avg True Thermal Power (W)",
        title = "True Thermal Power by Hour",
        subtitle = "Calculated from flow × ΔT × 69.77",
      ) +
      theme_energy()

    save_plot(p2, "16_thermal_power.png")
  }
} else {
  cat("Insufficient flow/temperature data for thermal power analysis.\n")
}

# ============================================================================
# Chart 3: Refrigerant cycle — discharge temp vs high pressure
# ============================================================================
if (nrow(discharge_temp) > 50 && nrow(high_pressure) > 50) {
  refrigerant <- discharge_temp |>
    select(hour_bucket, discharge = avg) |>
    inner_join(high_pressure |> select(hour_bucket, pressure = avg), by = "hour_bucket") |>
    filter(discharge > 0, pressure > 0)

  # Add COP coloring if available
  if (nrow(cop_sensor) > 50) {
    refrigerant <- refrigerant |>
      inner_join(cop_sensor |> select(hour_bucket, cop = avg), by = "hour_bucket") |>
      filter(cop > 0.5, cop < 10)
  }

  cat("\n=== Refrigerant Cycle ===\n")
  cat("  Records:", nrow(refrigerant), "\n")

  if (nrow(refrigerant) > 20 && "cop" %in% names(refrigerant)) {
    p3 <- ggplot(refrigerant, aes(x = pressure, y = discharge, color = cop)) +
      geom_point(alpha = 0.3, size = 0.8) +
      scale_color_viridis_c(option = "plasma", limits = c(1, 6)) +
      labs(
        x     = "High Pressure (Kgf/cm²)",
        y     = "Discharge Temperature (°C)",
        title = "Refrigerant Cycle: Discharge Temp vs High Pressure",
        subtitle = "Color = COP. Upper-right = harder work, lower efficiency.",
        color = "COP"
      ) +
      theme_energy()

    save_plot(p3, "16_refrigerant_cycle.png")
  } else if (nrow(refrigerant) > 20) {
    p3 <- ggplot(refrigerant, aes(x = pressure, y = discharge)) +
      geom_bin2d(bins = 40) +
      scale_fill_viridis_c(option = "plasma", trans = "log10") +
      labs(
        x     = "High Pressure (Kgf/cm²)",
        y     = "Discharge Temperature (°C)",
        title = "Refrigerant Cycle: Discharge Temp vs High Pressure",
        subtitle = "Upper-right = higher load, more compressor work",
        fill  = "Count"
      ) +
      theme_energy()

    save_plot(p3, "16_refrigerant_cycle.png")
  }
} else {
  cat("Insufficient discharge temp / pressure data.\n")
}
