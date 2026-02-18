# ============================================================================
# 22_hp_cycling.R — Heat Pump Cycling & Modulation Efficiency
# ============================================================================
# WHAT:    Analyzes HP compressor behavior: short cycling detection,
#          modulation range usage, defrost cycle identification, and
#          the part-load sweet spot where COP is maximized.
#
# INPUTS:  load_stats_sensor() for compressor speed, COP, fan speed,
#          discharge temp, outside pipe temp, outdoor temp, consumption
#
# OUTPUTS: output/22_modulation_histogram.png — compressor speed distribution
#          output/22_cycling_detection.png    — on/off transitions per day
#          output/22_defrost_detection.png    — defrost cycle signatures
#          output/22_partload_sweetspot.png   — COP vs part-load ratio
#
# HOW TO READ:
#   - Modulation: bimodal = HP spending too much time at extremes (bad)
#     vs smooth bell curve = good modulation across the range
#   - Cycling: many on/off transitions per day = short cycling (COP killer)
#   - Defrost: discharge temp spikes + outside pipe temp drops = defrost events
#   - Part-load: peak COP at specific compressor speed = efficiency sweet spot
# ============================================================================

source("analysis/helpers/load_data.R")

# ============================================================================
# Load HP operational data
# ============================================================================
# Deduplicate all sensors (stats CSVs can have overlapping exports)
comp_speed    <- load_stats_sensor(HP_COMPRESSOR_SPEED) |> distinct(hour_bucket, .keep_all = TRUE)
cop_sensor    <- load_stats_sensor(HP_COP_SENSOR) |> distinct(hour_bucket, .keep_all = TRUE)
fan_speed     <- load_stats_sensor(HP_FAN_SPEED) |> distinct(hour_bucket, .keep_all = TRUE)
discharge_temp <- load_stats_sensor(HP_DISCHARGE_TEMP) |> distinct(hour_bucket, .keep_all = TRUE)
outside_pipe  <- load_stats_sensor(HP_OUTSIDE_PIPE_TEMP) |> distinct(hour_bucket, .keep_all = TRUE)
outdoor_temp  <- load_stats_sensor(HP_OUTSIDE_TEMP) |> distinct(hour_bucket, .keep_all = TRUE)
hp_cons       <- load_stats_sensor(HP_CONSUMPTION) |> distinct(hour_bucket, .keep_all = TRUE)

cat("\n=== HP Cycling Data ===\n")
cat("  Compressor speed:", nrow(comp_speed), "hours\n")
cat("  COP sensor:      ", nrow(cop_sensor), "hours\n")
cat("  Fan speed:       ", nrow(fan_speed), "hours\n")
cat("  Discharge temp:  ", nrow(discharge_temp), "hours\n")
cat("  Outside pipe:    ", nrow(outside_pipe), "hours\n")
cat("  HP consumption:  ", nrow(hp_cons), "hours\n")

if (nrow(comp_speed) < 50) {
  cat("Insufficient compressor speed data.\n")
  quit(save = "no")
}

# ============================================================================
# Chart 1: Compressor speed modulation histogram
# ============================================================================
# A healthy variable-speed HP should have a smooth distribution.
# Bimodal (peaks at 0 and max) suggests the HP is undersized or
# the heating curve is too aggressive.

speed_data <- comp_speed |>
  filter(!is.na(avg)) |>
  mutate(
    hour = hour(hour_bucket),
    is_running = avg > 0
  )

# Add outdoor temp for seasonal context
speed_data <- speed_data |>
  left_join(outdoor_temp |> select(hour_bucket, outdoor = avg), by = "hour_bucket")

# Separate heating season (outdoor < 12°C) from mild weather
speed_heating <- speed_data |> filter(!is.na(outdoor), outdoor < 12)
speed_mild    <- speed_data |> filter(!is.na(outdoor), outdoor >= 12)

cat("\n=== Modulation Profile ===\n")
cat("  Heating season hours:", nrow(speed_heating), "\n")
cat("  % running:          ", round(mean(speed_heating$is_running) * 100, 1), "%\n")

running_speeds <- speed_heating |> filter(avg > 0)
if (nrow(running_speeds) > 0) {
  cat("  Speed range:        ", round(min(running_speeds$avg)), "–",
      round(max(running_speeds$avg)), "RPM\n")
  cat("  Median speed:       ", round(median(running_speeds$avg)), "RPM\n")
  cat("  Mean speed:         ", round(mean(running_speeds$avg)), "RPM\n")
}

# Build histogram data with both seasons
hist_data <- bind_rows(
  speed_heating |> filter(avg > 0) |> mutate(period = "Heating (outdoor < 12°C)"),
  speed_mild |> filter(avg > 0) |> mutate(period = "Mild (outdoor ≥ 12°C)")
) |>
  filter(nrow(speed_heating |> filter(avg > 0)) > 0)

if (nrow(hist_data) > 20) {
  p1 <- ggplot(hist_data, aes(x = avg, fill = period)) +
    geom_histogram(bins = 40, alpha = 0.7, position = "identity") +
    scale_fill_manual(values = c(
      "Heating (outdoor < 12°C)" = COLORS$import,
      "Mild (outdoor ≥ 12°C)" = COLORS$export
    )) +
    labs(
      x     = "Compressor Speed (RPM)",
      y     = "Hours",
      title = "Compressor Speed Distribution",
      subtitle = "Smooth curve = good modulation. Peaks at extremes = undersized or bad curve.",
      fill  = ""
    ) +
    theme_energy()

  save_plot(p1, "22_modulation_histogram.png")
}

# ============================================================================
# Chart 2: Cycling detection — on/off transitions per day
# ============================================================================
# Count how many times the compressor turns on/off per day.
# More than 6-8 cycles/day typically indicates short cycling.

cycling_data <- speed_data |>
  arrange(hour_bucket) |>
  mutate(
    was_running = lag(is_running, default = FALSE),
    # A "start" is when compressor goes from off to on
    is_start = is_running & !was_running,
    date = as.Date(hour_bucket)
  )

daily_cycles <- cycling_data |>
  group_by(date) |>
  summarize(
    starts_per_day = sum(is_start, na.rm = TRUE),
    hours_running  = sum(is_running, na.rm = TRUE),
    hours_total    = n(),
    duty_cycle_pct = hours_running / hours_total * 100,
    .groups = "drop"
  ) |>
  left_join(
    outdoor_temp |>
      mutate(date = as.Date(hour_bucket)) |>
      group_by(date) |>
      summarize(avg_outdoor = mean(avg, na.rm = TRUE), .groups = "drop"),
    by = "date"
  )

if (nrow(daily_cycles) > 7) {
  cat("\n=== Cycling Analysis ===\n")
  cat("  Mean starts/day:", round(mean(daily_cycles$starts_per_day, na.rm = TRUE), 1), "\n")
  cat("  Max starts/day: ", max(daily_cycles$starts_per_day, na.rm = TRUE), "\n")
  cat("  Mean duty cycle:", round(mean(daily_cycles$duty_cycle_pct, na.rm = TRUE), 1), "%\n")
  cat("  Days with >6 starts:", sum(daily_cycles$starts_per_day > 6, na.rm = TRUE), "\n")

  p2 <- ggplot(daily_cycles |> filter(!is.na(avg_outdoor)),
               aes(x = avg_outdoor, y = starts_per_day)) +
    geom_point(aes(color = duty_cycle_pct), alpha = 0.6, size = 2) +
    scale_color_viridis_c(option = "plasma", name = "Duty Cycle %") +
    geom_smooth(method = "loess", color = COLORS$import, linewidth = 1, se = TRUE) +
    geom_hline(yintercept = 6, linetype = "dashed", color = COLORS$warning) +
    annotate("text", x = max(daily_cycles$avg_outdoor, na.rm = TRUE) - 1, y = 6.5,
             label = "6 starts/day threshold", color = COLORS$warning, size = 3, hjust = 1) +
    labs(
      x     = "Average Outdoor Temperature (°C)",
      y     = "Compressor Starts per Day",
      title = "Short Cycling Detection",
      subtitle = "More starts at mild temps = HP oversized for mild conditions. >6 = short cycling."
    ) +
    theme_energy()

  save_plot(p2, "22_cycling_detection.png")
}

# ============================================================================
# Chart 3: Defrost cycle detection
# ============================================================================
# Defrost signatures: outside pipe temp drops sharply (ice forming on coil),
# discharge temp spikes (reversed refrigerant cycle), fan may stop.
# We look for hours where outside pipe temp is unusually low.

if (nrow(outside_pipe) > 50 && nrow(discharge_temp) > 50) {
  defrost_data <- outside_pipe |>
    select(hour_bucket, pipe_temp = avg) |>
    inner_join(discharge_temp |> select(hour_bucket, discharge = avg), by = "hour_bucket") |>
    inner_join(outdoor_temp |> select(hour_bucket, outdoor = avg), by = "hour_bucket") |>
    left_join(fan_speed |> select(hour_bucket, fan = avg), by = "hour_bucket") |>
    left_join(cop_sensor |> select(hour_bucket, cop = avg), by = "hour_bucket") |>
    filter(!is.na(pipe_temp), !is.na(discharge), !is.na(outdoor)) |>
    mutate(
      hour = hour(hour_bucket),
      date = as.Date(hour_bucket),
      # Defrost indicator: pipe temp much colder than outdoor = icing
      pipe_depression = outdoor - pipe_temp,
      # Flag potential defrost hours
      is_defrost = pipe_depression > 5 & outdoor < 7
    )

  n_defrost <- sum(defrost_data$is_defrost, na.rm = TRUE)
  cat("\n=== Defrost Detection ===\n")
  cat("  Total hours analyzed:", nrow(defrost_data), "\n")
  cat("  Potential defrost hours:", n_defrost, "\n")
  cat("  Defrost rate:", round(n_defrost / nrow(defrost_data) * 100, 1), "%\n")

  if (nrow(defrost_data) > 50) {
    # Scatter: outdoor temp vs pipe depression, colored by defrost flag
    p3 <- ggplot(defrost_data |> filter(outdoor < 10),
                 aes(x = outdoor, y = pipe_depression)) +
      geom_bin2d(bins = 35) +
      scale_fill_viridis_c(option = "plasma", trans = "log10") +
      geom_hline(yintercept = 5, linetype = "dashed", color = COLORS$warning) +
      annotate("text", x = min(defrost_data$outdoor[defrost_data$outdoor < 10], na.rm = TRUE) + 0.5,
               y = 5.5, label = "Defrost threshold (5°C depression)",
               color = COLORS$warning, size = 3, hjust = 0) +
      labs(
        x     = "Outdoor Temperature (°C)",
        y     = "Pipe Depression (°C below outdoor)",
        title = "Defrost Cycle Detection",
        subtitle = "Pipe temp much lower than outdoor = ice on evaporator coil. Points above line = defrost likely.",
        fill  = "Hours"
      ) +
      theme_energy()

    save_plot(p3, "22_defrost_detection.png")
  }
} else {
  cat("Insufficient outside pipe / discharge temp data for defrost analysis.\n")
}

# ============================================================================
# Chart 4: Part-load sweet spot — COP vs compressor speed with outdoor bins
# ============================================================================
# Find the compressor speed range where COP is highest at each outdoor temp.

if (nrow(comp_speed) > 50 && nrow(cop_sensor) > 50) {
  partload <- comp_speed |>
    select(hour_bucket, speed = avg) |>
    inner_join(cop_sensor |> select(hour_bucket, cop = avg), by = "hour_bucket") |>
    inner_join(outdoor_temp |> select(hour_bucket, outdoor = avg), by = "hour_bucket") |>
    filter(speed > 0, cop > 0.5, cop < 10) |>
    mutate(
      outdoor_bin = cut(outdoor, breaks = c(-20, -5, 0, 5, 10, 20),
                        labels = c("< -5°C", "-5 to 0°C", "0 to 5°C", "5 to 10°C", "> 10°C"),
                        include.lowest = TRUE)
    ) |>
    filter(!is.na(outdoor_bin))

  cat("\n=== Part-Load Sweet Spot ===\n")
  partload |>
    group_by(outdoor_bin) |>
    summarize(
      n = n(),
      speed_at_max_cop = speed[which.max(cop)],
      max_cop = max(cop),
      median_speed = median(speed),
      .groups = "drop"
    ) |>
    print()

  if (nrow(partload) > 30) {
    p4 <- ggplot(partload, aes(x = speed, y = cop, color = outdoor_bin)) +
      geom_point(alpha = 0.15, size = 0.8) +
      geom_smooth(method = "loess", se = FALSE, linewidth = 1.2) +
      scale_color_brewer(palette = "RdYlBu", direction = 1) +
      labs(
        x     = "Compressor Speed (RPM)",
        y     = "COP",
        title = "Part-Load Efficiency: COP vs Compressor Speed",
        subtitle = "Each line = outdoor temp range. Peak of each curve = sweet spot.",
        color = "Outdoor Temp"
      ) +
      theme_energy()

    save_plot(p4, "22_partload_sweetspot.png")
  }
}
