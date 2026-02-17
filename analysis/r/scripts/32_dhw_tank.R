# ============================================================================
# 32_dhw_tank.R — DHW Tank Standby Loss Analysis
# ============================================================================
# WHAT:    Measures the hot water tank's heat loss rate by analyzing temperature
#          decay during idle periods (no DHW heating). The cooling rate reveals
#          standby loss in watts, insulation quality, and optimal reheat schedule.
#
#          Physics: During idle periods the tank cools approximately as
#            T(t) = T_ambient + (T_start - T_ambient) * exp(-t / tau)
#          where tau = thermal time constant (hours).
#          Heat loss power: Q = C_tank * dT/dt
#          where C_tank = 4186 * volume_liters (J/K) for water.
#
# INPUTS:  load_stats_sensor() for HP_DHW_TEMP, HP_DHW_POWER, HP_OUTSIDE_TEMP,
#          HP_CONSUMPTION, spot_prices
#
# OUTPUTS: output/32_dhw_tank_profile.png  — time series with reheat cycles highlighted
#          output/32_dhw_cooling_rate.png   — histogram of cooling rates with standby W
#          output/32_dhw_reheat_schedule.png — decay curves from different start temps
#
# HOW TO READ:
#   - Tank profile: sawtooth = heat-cool-heat cycles. Wide teeth = good insulation.
#   - Cooling rate: tight distribution = consistent loss. High mean = poor insulation.
#   - Reheat schedule: where each curve crosses the 40C line = time until reheat needed.
#     Higher start temp or lower loss rate = longer intervals between reheats.
# ============================================================================

source("analysis/r/R/load_data.R")

# ============================================================================
# Physical constants
# ============================================================================
TANK_VOLUME_L    <- 200   # Typical Panasonic Aquarea DHW tank (liters)
WATER_HEAT_CAP   <- 4186  # J/(kg*K) — specific heat of water
# Tank thermal capacity: C = 4186 * 200 = 837200 J/K
C_TANK_J_PER_K   <- WATER_HEAT_CAP * TANK_VOLUME_L
MIN_USABLE_TEMP  <- 40    # Minimum usable DHW temperature (deg C)
AMBIENT_TEMP     <- 20    # Assumed indoor ambient near tank (deg C)

# ============================================================================
# Load data
# ============================================================================
dhw_temp   <- load_stats_sensor(HP_DHW_TEMP) |> distinct(hour_bucket, .keep_all = TRUE)
dhw_power  <- load_stats_sensor(HP_DHW_POWER) |> distinct(hour_bucket, .keep_all = TRUE)
hp_cons    <- load_stats_sensor(HP_CONSUMPTION) |> distinct(hour_bucket, .keep_all = TRUE)
outdoor_temp <- load_stats_sensor(HP_OUTSIDE_TEMP) |> distinct(hour_bucket, .keep_all = TRUE)

cat("\n=== DHW Tank Data ===\n")
cat("  DHW temp:      ", nrow(dhw_temp), "hours\n")
cat("  DHW power:     ", nrow(dhw_power), "hours\n")
cat("  HP consumption:", nrow(hp_cons), "hours\n")
cat("  Outdoor temp:  ", nrow(outdoor_temp), "hours\n")

if (nrow(dhw_temp) < 20) {
  cat("Insufficient DHW temperature data. Exiting.\n")
  quit(save = "no")
}

# ============================================================================
# Build combined dataset
# ============================================================================
dhw_data <- dhw_temp |>
  select(hour_bucket, tank_temp = avg, tank_min = min_val, tank_max = max_val) |>
  left_join(dhw_power |> select(hour_bucket, dhw_power = avg), by = "hour_bucket") |>
  left_join(hp_cons |> select(hour_bucket, consumption = avg), by = "hour_bucket") |>
  left_join(outdoor_temp |> select(hour_bucket, outdoor = avg), by = "hour_bucket") |>
  arrange(hour_bucket) |>
  mutate(
    hour  = hour(hour_bucket),
    month = month(hour_bucket, label = TRUE),
    date  = as.Date(hour_bucket),
    # Detect DHW heating: DHW power > threshold OR consumption spike with temp rising
    is_heating = !is.na(dhw_power) & dhw_power > 100,
    # Temperature change from previous hour
    temp_change = tank_temp - lag(tank_temp),
    hours_gap = as.numeric(difftime(hour_bucket, lag(hour_bucket), units = "hours"))
  )

n_heating <- sum(dhw_data$is_heating, na.rm = TRUE)
n_idle    <- sum(!dhw_data$is_heating, na.rm = TRUE)

cat("\n=== DHW Heating Profile ===\n")
cat("  Total hours:    ", nrow(dhw_data), "\n")
cat("  Heating hours:  ", n_heating, "\n")
cat("  Idle hours:     ", n_idle, "\n")
cat("  Mean tank temp: ", round(mean(dhw_data$tank_temp, na.rm = TRUE), 1), "\u00b0C\n")
cat("  Min tank temp:  ", round(min(dhw_data$tank_temp, na.rm = TRUE), 1), "\u00b0C\n")
cat("  Max tank temp:  ", round(max(dhw_data$tank_temp, na.rm = TRUE), 1), "\u00b0C\n")

# ============================================================================
# Identify idle cooling runs
# ============================================================================
# Find consecutive non-heating hours where tank is cooling (temp decreasing)
dhw_idle <- dhw_data |>
  filter(!is_heating, !is.na(tank_temp)) |>
  arrange(hour_bucket) |>
  mutate(
    hours_gap = as.numeric(difftime(hour_bucket, lag(hour_bucket), units = "hours")),
    is_new_run = is.na(hours_gap) | hours_gap > 1.5,
    run_id = cumsum(is_new_run)
  )

idle_runs <- dhw_idle |>
  group_by(run_id) |>
  summarize(
    start_time  = min(hour_bucket),
    end_time    = max(hour_bucket),
    start_temp  = first(tank_temp),
    end_temp    = last(tank_temp),
    duration_h  = n(),
    temp_drop   = first(tank_temp) - last(tank_temp),
    avg_outdoor = mean(outdoor, na.rm = TRUE),
    .groups = "drop"
  ) |>
  filter(
    duration_h >= 3,          # at least 3 hours of idle for reliable measurement
    duration_h <= 48,         # not unreasonably long
    temp_drop > 0,            # actually cooling (not measurement noise)
    start_temp > 35           # tank was warm enough to be meaningful
  ) |>
  mutate(
    loss_rate_c_per_h = temp_drop / duration_h,  # deg C per hour
    delta_t_start     = start_temp - AMBIENT_TEMP,
    # Standby heat loss in W:
    # Q = C_tank * loss_rate / 3600 (convert J/K * K/h to W)
    standby_loss_w    = C_TANK_J_PER_K * loss_rate_c_per_h / 3600,
    # Estimate thermal time constant tau from exponential decay
    # T(t) = T_amb + (T0 - T_amb)*exp(-t/tau)
    # end_temp = T_amb + (start_temp - T_amb)*exp(-duration/tau)
    # tau = -duration / ln((end_temp - T_amb) / (start_temp - T_amb))
    tau_h = ifelse(
      end_temp > AMBIENT_TEMP + 1 & start_temp > AMBIENT_TEMP + 1,
      -duration_h / log((end_temp - AMBIENT_TEMP) / (start_temp - AMBIENT_TEMP)),
      NA_real_
    )
  )

cat("\n=== Idle Cooling Runs ===\n")
cat("  Valid runs:        ", nrow(idle_runs), "\n")

if (nrow(idle_runs) < 20) {
  cat("Insufficient idle cooling runs (", nrow(idle_runs), "). Need at least 20.\n")
  cat("Proceeding with available data for charts.\n")
}

if (nrow(idle_runs) > 0) {
  cat("  Median loss rate:  ", round(median(idle_runs$loss_rate_c_per_h, na.rm = TRUE), 2), "\u00b0C/h\n")
  cat("  Mean loss rate:    ", round(mean(idle_runs$loss_rate_c_per_h, na.rm = TRUE), 2), "\u00b0C/h\n")
  cat("  Median standby W:  ", round(median(idle_runs$standby_loss_w, na.rm = TRUE), 0), "W\n")
  cat("  Mean standby W:    ", round(mean(idle_runs$standby_loss_w, na.rm = TRUE), 0), "W\n")
  if (sum(!is.na(idle_runs$tau_h)) > 3) {
    cat("  Median tau:        ", round(median(idle_runs$tau_h, na.rm = TRUE), 0), "hours\n")
  }
}

# ============================================================================
# Chart 1: Tank temperature profile with reheat cycles highlighted
# ============================================================================
# Show a representative 7-day window with the sawtooth pattern

# Pick the week with the most data points for a nice visualization
dhw_weekly <- dhw_data |>
  mutate(week = floor_date(hour_bucket, "week")) |>
  group_by(week) |>
  summarize(n = n(), .groups = "drop") |>
  arrange(desc(n))

if (nrow(dhw_weekly) > 0) {
  best_week <- dhw_weekly$week[1]
  profile_data <- dhw_data |>
    filter(hour_bucket >= best_week, hour_bucket < best_week + days(7))

  if (nrow(profile_data) > 20) {
    # Mark heating periods
    heating_bands <- profile_data |>
      filter(is_heating) |>
      mutate(
        xmin = hour_bucket - minutes(30),
        xmax = hour_bucket + minutes(30)
      )

    p1 <- ggplot(profile_data, aes(x = hour_bucket, y = tank_temp)) +
      {if (nrow(heating_bands) > 0)
        geom_rect(data = heating_bands,
                  aes(xmin = xmin, xmax = xmax, ymin = -Inf, ymax = Inf),
                  inherit.aes = FALSE,
                  fill = COLORS$heat_pump, alpha = 0.15)
      } +
      geom_line(color = COLORS$import, linewidth = 0.8) +
      geom_hline(yintercept = MIN_USABLE_TEMP, linetype = "dashed",
                 color = COLORS$warning, linewidth = 0.6) +
      annotate("text",
        x = min(profile_data$hour_bucket) + hours(2),
        y = MIN_USABLE_TEMP + 1,
        label = paste0("Min usable: ", MIN_USABLE_TEMP, "\u00b0C"),
        color = COLORS$warning, size = 3.5, hjust = 0
      ) +
      labs(
        x     = "",
        y     = "Tank Temperature (\u00b0C)",
        title = "DHW Tank Temperature Profile (1 Week Sample)",
        subtitle = paste0(
          "Orange bands = DHW heating active. Sawtooth pattern: heat to setpoint, then cool. ",
          "Week of ", format(best_week, "%Y-%m-%d"), "."
        )
      ) +
      theme_energy()

    save_plot(p1, "32_dhw_tank_profile.png", width = 12, height = 6)
  }
}

# ============================================================================
# Chart 2: Cooling rate distribution with standby loss in W
# ============================================================================
if (nrow(idle_runs) >= 5) {
  median_loss_rate <- median(idle_runs$loss_rate_c_per_h, na.rm = TRUE)
  median_standby_w <- median(idle_runs$standby_loss_w, na.rm = TRUE)
  daily_kwh        <- median_standby_w * 24 / 1000

  p2 <- ggplot(idle_runs, aes(x = loss_rate_c_per_h)) +
    geom_histogram(bins = 30, fill = COLORS$charge, alpha = 0.8, color = "white") +
    geom_vline(xintercept = median_loss_rate, linetype = "dashed",
               color = COLORS$import, linewidth = 0.8) +
    annotate("text",
      x = median_loss_rate,
      y = Inf,
      vjust = 2,
      label = paste0(
        "Median: ", round(median_loss_rate, 2), " \u00b0C/h\n",
        "\u2248 ", round(median_standby_w, 0), " W standby\n",
        "\u2248 ", round(daily_kwh, 2), " kWh/day"
      ),
      color = COLORS$import, size = 3.5, hjust = -0.1
    ) +
    # Add secondary x-axis showing watts
    scale_x_continuous(
      sec.axis = sec_axis(
        ~ . * C_TANK_J_PER_K / 3600,
        name = "Standby Heat Loss (W)"
      )
    ) +
    labs(
      x     = "Cooling Rate (\u00b0C/hour)",
      y     = "Number of Idle Runs",
      title = "DHW Tank Cooling Rate Distribution",
      subtitle = paste0(
        "Measured from ", nrow(idle_runs), " idle cooling periods (", TANK_VOLUME_L,
        "L tank). Lower = better insulated."
      )
    ) +
    theme_energy()

  save_plot(p2, "32_dhw_cooling_rate.png")
} else {
  cat("Not enough idle runs for cooling rate histogram.\n")
}

# ============================================================================
# Chart 3: Optimal reheat schedule — decay curves from different start temps
# ============================================================================
# Using the measured time constant tau, show how long the tank can go without
# reheating from various starting temperatures.

valid_taus <- idle_runs |> filter(!is.na(tau_h), tau_h > 10, tau_h < 500)

if (nrow(valid_taus) >= 3) {
  tau_median <- median(valid_taus$tau_h, na.rm = TRUE)

  cat("\n=== Reheat Schedule Model ===\n")
  cat("  Median tau:     ", round(tau_median, 0), "hours\n")
  cat("  At 55\u00b0C start:  tank reaches 40\u00b0C in",
      round(-tau_median * log((MIN_USABLE_TEMP - AMBIENT_TEMP) /
                                (55 - AMBIENT_TEMP)), 0), "hours\n")
  cat("  At 50\u00b0C start:  tank reaches 40\u00b0C in",
      round(-tau_median * log((MIN_USABLE_TEMP - AMBIENT_TEMP) /
                                (50 - AMBIENT_TEMP)), 0), "hours\n")
  cat("  At 45\u00b0C start:  tank reaches 40\u00b0C in",
      round(-tau_median * log((MIN_USABLE_TEMP - AMBIENT_TEMP) /
                                (45 - AMBIENT_TEMP)), 0), "hours\n")

  # Generate decay curves
  start_temps <- c(60, 55, 50, 45)
  hours_seq   <- seq(0, 72, by = 0.5)

  decay_curves <- expand_grid(
    start_temp = start_temps,
    t_hours    = hours_seq
  ) |>
    mutate(
      tank_temp = AMBIENT_TEMP + (start_temp - AMBIENT_TEMP) * exp(-t_hours / tau_median),
      start_label = paste0(start_temp, "\u00b0C start")
    )

  # Calculate time-to-minimum for each start temp
  time_to_min <- tibble(start_temp = start_temps) |>
    mutate(
      hours_to_min = ifelse(
        start_temp > MIN_USABLE_TEMP,
        -tau_median * log((MIN_USABLE_TEMP - AMBIENT_TEMP) / (start_temp - AMBIENT_TEMP)),
        0
      ),
      start_label = paste0(start_temp, "\u00b0C start")
    )

  # Color gradient: hotter start = more orange, cooler = more blue
  start_colors <- c(
    "60\u00b0C start" = COLORS$import,
    "55\u00b0C start" = COLORS$heat_pump,
    "50\u00b0C start" = COLORS$pv,
    "45\u00b0C start" = COLORS$charge
  )

  p3 <- ggplot(decay_curves, aes(x = t_hours, y = tank_temp, color = start_label)) +
    geom_line(linewidth = 1.0) +
    # Minimum usable temperature line
    geom_hline(yintercept = MIN_USABLE_TEMP, linetype = "dashed",
               color = COLORS$warning, linewidth = 0.6) +
    annotate("text",
      x = 2, y = MIN_USABLE_TEMP - 1.5,
      label = paste0("Min usable: ", MIN_USABLE_TEMP, "\u00b0C"),
      color = COLORS$warning, size = 3.5, hjust = 0
    ) +
    # Ambient line
    geom_hline(yintercept = AMBIENT_TEMP, linetype = "dotted",
               color = COLORS$muted, linewidth = 0.5) +
    annotate("text",
      x = max(hours_seq) - 2, y = AMBIENT_TEMP + 1.5,
      label = paste0("Ambient: ", AMBIENT_TEMP, "\u00b0C"),
      color = COLORS$muted, size = 3, hjust = 1
    ) +
    # Mark crossing points (time to reach min usable)
    {if (nrow(time_to_min |> filter(hours_to_min > 0 & hours_to_min <= max(hours_seq))) > 0)
      geom_point(
        data = time_to_min |> filter(hours_to_min > 0, hours_to_min <= max(hours_seq)),
        aes(x = hours_to_min, y = MIN_USABLE_TEMP, color = start_label),
        size = 3, shape = 4, stroke = 1.5
      )
    } +
    {if (nrow(time_to_min |> filter(hours_to_min > 0 & hours_to_min <= max(hours_seq))) > 0)
      geom_text(
        data = time_to_min |> filter(hours_to_min > 0, hours_to_min <= max(hours_seq)),
        aes(x = hours_to_min, y = MIN_USABLE_TEMP + 2,
            label = paste0(round(hours_to_min, 0), "h"),
            color = start_label),
        size = 3, fontface = "bold", show.legend = FALSE
      )
    } +
    scale_color_manual(values = start_colors) +
    labs(
      x     = "Hours Since Last Reheat",
      y     = "Tank Temperature (\u00b0C)",
      title = "DHW Tank Decay: Optimal Reheat Schedule",
      subtitle = paste0(
        "Exponential decay with measured \u03c4 = ", round(tau_median, 0),
        " hours (", TANK_VOLUME_L, "L tank). ",
        "X marks = when tank reaches ", MIN_USABLE_TEMP, "\u00b0C."
      ),
      color = "Starting Temperature"
    ) +
    coord_cartesian(ylim = c(AMBIENT_TEMP - 2, max(start_temps) + 2)) +
    theme_energy()

  save_plot(p3, "32_dhw_reheat_schedule.png", width = 11, height = 7)

} else if (nrow(idle_runs) >= 5) {
  # Fallback: use linear loss rate instead of exponential tau
  median_rate <- median(idle_runs$loss_rate_c_per_h, na.rm = TRUE)

  cat("\n=== Reheat Schedule (linear approximation) ===\n")
  cat("  Median loss rate: ", round(median_rate, 2), "\u00b0C/h\n")

  start_temps <- c(60, 55, 50, 45)
  hours_seq   <- seq(0, 72, by = 0.5)

  decay_curves <- expand_grid(
    start_temp = start_temps,
    t_hours    = hours_seq
  ) |>
    mutate(
      tank_temp = pmax(start_temp - median_rate * t_hours, AMBIENT_TEMP),
      start_label = paste0(start_temp, "\u00b0C start")
    )

  time_to_min <- tibble(start_temp = start_temps) |>
    mutate(
      hours_to_min = ifelse(start_temp > MIN_USABLE_TEMP,
                            (start_temp - MIN_USABLE_TEMP) / median_rate, 0),
      start_label = paste0(start_temp, "\u00b0C start")
    )

  start_colors <- c(
    "60\u00b0C start" = COLORS$import,
    "55\u00b0C start" = COLORS$heat_pump,
    "50\u00b0C start" = COLORS$pv,
    "45\u00b0C start" = COLORS$charge
  )

  p3 <- ggplot(decay_curves, aes(x = t_hours, y = tank_temp, color = start_label)) +
    geom_line(linewidth = 1.0) +
    geom_hline(yintercept = MIN_USABLE_TEMP, linetype = "dashed",
               color = COLORS$warning, linewidth = 0.6) +
    annotate("text",
      x = 2, y = MIN_USABLE_TEMP - 1.5,
      label = paste0("Min usable: ", MIN_USABLE_TEMP, "\u00b0C"),
      color = COLORS$warning, size = 3.5, hjust = 0
    ) +
    geom_point(
      data = time_to_min |> filter(hours_to_min > 0, hours_to_min <= max(hours_seq)),
      aes(x = hours_to_min, y = MIN_USABLE_TEMP, color = start_label),
      size = 3, shape = 4, stroke = 1.5
    ) +
    geom_text(
      data = time_to_min |> filter(hours_to_min > 0, hours_to_min <= max(hours_seq)),
      aes(x = hours_to_min, y = MIN_USABLE_TEMP + 2,
          label = paste0(round(hours_to_min, 0), "h"),
          color = start_label),
      size = 3, fontface = "bold", show.legend = FALSE
    ) +
    scale_color_manual(values = start_colors) +
    labs(
      x     = "Hours Since Last Reheat",
      y     = "Tank Temperature (\u00b0C)",
      title = "DHW Tank Decay: Optimal Reheat Schedule (Linear Approximation)",
      subtitle = paste0(
        "Linear decay at ", round(median_rate, 2), " \u00b0C/h (",
        TANK_VOLUME_L, "L tank). X marks = time to reach ", MIN_USABLE_TEMP, "\u00b0C."
      ),
      color = "Starting Temperature"
    ) +
    coord_cartesian(ylim = c(AMBIENT_TEMP - 2, max(start_temps) + 2)) +
    theme_energy()

  save_plot(p3, "32_dhw_reheat_schedule.png", width = 11, height = 7)
} else {
  cat("Insufficient idle runs for reheat schedule modeling.\n")
}

# ============================================================================
# Summary
# ============================================================================
cat("\n=== DHW TANK STANDBY LOSS SUMMARY ===\n")
cat("  Tank volume:         ", TANK_VOLUME_L, "L\n")
cat("  Thermal capacity:    ", round(C_TANK_J_PER_K / 1000, 0), "kJ/K\n")
if (nrow(idle_runs) > 0) {
  med_rate <- median(idle_runs$loss_rate_c_per_h, na.rm = TRUE)
  med_w    <- median(idle_runs$standby_loss_w, na.rm = TRUE)
  cat("  Median cooling rate: ", round(med_rate, 2), "\u00b0C/h\n")
  cat("  Median standby loss: ", round(med_w, 0), "W\n")
  cat("  Daily standby energy:", round(med_w * 24 / 1000, 2), "kWh\n")
  cat("  Annual standby cost: ~",
      round(med_w * 24 * 365.25 / 1000 * 0.80, 0), "PLN (at 0.80 PLN/kWh)\n")
  if (sum(!is.na(idle_runs$tau_h) & idle_runs$tau_h > 10 & idle_runs$tau_h < 500) >= 3) {
    tau_med <- median(idle_runs$tau_h[idle_runs$tau_h > 10 & idle_runs$tau_h < 500], na.rm = TRUE)
    cat("  Thermal time const:  ", round(tau_med, 0), "hours\n")
    cat("  55\u00b0C -> 40\u00b0C:       ~",
        round(-tau_med * log((MIN_USABLE_TEMP - AMBIENT_TEMP) / (55 - AMBIENT_TEMP)), 0),
        "hours\n")
  }
}
