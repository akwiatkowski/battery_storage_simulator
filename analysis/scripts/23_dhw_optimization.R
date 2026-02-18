# ============================================================================
# 23_dhw_optimization.R — DHW (Domestic Hot Water) Timing Optimization
# ============================================================================
# WHAT:    Analyzes when DHW heating occurs vs when it would be most efficient.
#          Quantifies DHW tank heat loss rate and potential COP gains from
#          shifting DHW heating to warmer hours.
#
# INPUTS:  load_stats_sensor() for DHW tank temp, DHW power consumption,
#          HP COP, outdoor temp, spot prices
#
# OUTPUTS: output/23_dhw_timing.png       — when DHW heating happens vs COP
#          output/23_dhw_tank_loss.png    — tank temperature decay between reheats
#          output/23_dhw_cop_potential.png — COP gained by shifting to warm hours
#          output/23_dhw_cost_heatmap.png — hour × month cost of DHW heating
#
# HOW TO READ:
#   - Timing: DHW heating at night (cold outdoor) = low COP = waste
#   - Tank loss: steeper decay = worse insulation, more frequent reheats
#   - COP potential: gap between actual and optimal = savings opportunity
#   - Cost heatmap: dark cells = expensive DHW hours to avoid
# ============================================================================

source("analysis/helpers/load_data.R")

# ============================================================================
# Load DHW-related data
# ============================================================================
# Deduplicate all sensors (stats CSVs can have overlapping exports)
dhw_temp    <- load_stats_sensor(HP_DHW_TEMP) |> distinct(hour_bucket, .keep_all = TRUE)
dhw_power   <- load_stats_sensor(HP_DHW_POWER) |> distinct(hour_bucket, .keep_all = TRUE)
cop_sensor  <- load_stats_sensor(HP_COP_SENSOR) |> distinct(hour_bucket, .keep_all = TRUE)
outdoor_temp <- load_stats_sensor(HP_OUTSIDE_TEMP) |> distinct(hour_bucket, .keep_all = TRUE)
hp_cons     <- load_stats_sensor(HP_CONSUMPTION) |> distinct(hour_bucket, .keep_all = TRUE)

cat("\n=== DHW Data ===\n")
cat("  DHW tank temp:    ", nrow(dhw_temp), "hours\n")
cat("  DHW power:        ", nrow(dhw_power), "hours\n")
cat("  COP sensor:       ", nrow(cop_sensor), "hours\n")
cat("  Outdoor temp:     ", nrow(outdoor_temp), "hours\n")

if (nrow(dhw_temp) < 50) {
  cat("Insufficient DHW temperature data.\n")
  quit(save = "no")
}

# ============================================================================
# Build combined DHW dataset
# ============================================================================
dhw_data <- dhw_temp |>
  select(hour_bucket, tank_temp = avg, tank_min = min_val, tank_max = max_val) |>
  left_join(dhw_power |> select(hour_bucket, dhw_power = avg), by = "hour_bucket") |>
  left_join(cop_sensor |> select(hour_bucket, cop = avg), by = "hour_bucket") |>
  left_join(outdoor_temp |> select(hour_bucket, outdoor = avg), by = "hour_bucket") |>
  left_join(spot_prices, by = "hour_bucket") |>
  mutate(
    hour = hour(hour_bucket),
    month = month(hour_bucket, label = TRUE),
    date = as.Date(hour_bucket),
    # Detect DHW heating: DHW power > threshold
    is_dhw_heating = !is.na(dhw_power) & dhw_power > 100,
    # Temperature change from previous hour
    temp_change = tank_temp - lag(tank_temp)
  )

dhw_heating <- dhw_data |> filter(is_dhw_heating)
dhw_idle    <- dhw_data |> filter(!is_dhw_heating, !is.na(tank_temp))

cat("\n=== DHW Heating Profile ===\n")
cat("  Total hours:       ", nrow(dhw_data), "\n")
cat("  DHW heating hours: ", nrow(dhw_heating), "\n")
cat("  Mean tank temp:    ", round(mean(dhw_data$tank_temp, na.rm = TRUE), 1), "°C\n")
if (nrow(dhw_heating) > 0) {
  cat("  Mean DHW power:    ", round(mean(dhw_heating$dhw_power, na.rm = TRUE), 0), "W\n")
  cat("  Mean COP during DHW:", round(mean(dhw_heating$cop, na.rm = TRUE), 2), "\n")
}

# ============================================================================
# Chart 1: When does DHW heating happen? Hour profile with COP overlay
# ============================================================================
if (nrow(dhw_heating) > 20) {
  dhw_hourly <- dhw_data |>
    group_by(hour) |>
    summarize(
      dhw_hours = sum(is_dhw_heating, na.rm = TRUE),
      dhw_pct   = mean(is_dhw_heating, na.rm = TRUE) * 100,
      avg_cop_dhw = mean(cop[is_dhw_heating], na.rm = TRUE),
      avg_cop_all = mean(cop, na.rm = TRUE),
      avg_outdoor = mean(outdoor, na.rm = TRUE),
      .groups = "drop"
    )

  # Scale COP for secondary axis
  cop_scale <- max(dhw_hourly$dhw_pct, na.rm = TRUE) / max(dhw_hourly$avg_cop_all, na.rm = TRUE)

  p1 <- ggplot(dhw_hourly, aes(x = hour)) +
    geom_col(aes(y = dhw_pct), fill = COLORS$heat_pump, alpha = 0.7) +
    geom_line(aes(y = avg_cop_all * cop_scale), color = COLORS$export, linewidth = 1.2) +
    geom_point(aes(y = avg_cop_all * cop_scale), color = COLORS$export, size = 2) +
    scale_y_continuous(
      name = "% of Hours with DHW Heating",
      sec.axis = sec_axis(~ . / cop_scale, name = "Average COP")
    ) +
    scale_x_continuous(breaks = seq(0, 23, 3)) +
    labs(
      x     = "Hour of Day",
      title = "DHW Heating Timing vs COP",
      subtitle = "Bars = when DHW happens. Green line = COP. Best: shift DHW to high-COP hours."
    ) +
    theme_energy() +
    theme(
      axis.title.y.right = element_text(color = COLORS$export),
      axis.text.y.right  = element_text(color = COLORS$export)
    )

  save_plot(p1, "23_dhw_timing.png")
}

# ============================================================================
# Chart 2: Tank heat loss rate — temperature decay during idle periods
# ============================================================================
# Find consecutive non-heating hours and measure the cooling rate.
dhw_idle_sorted <- dhw_idle |>
  arrange(hour_bucket) |>
  mutate(
    hours_gap = as.numeric(difftime(hour_bucket, lag(hour_bucket), units = "hours")),
    # Group consecutive idle hours into "idle runs"
    is_new_run = is.na(hours_gap) | hours_gap > 1,
    run_id = cumsum(is_new_run)
  )

idle_runs <- dhw_idle_sorted |>
  group_by(run_id) |>
  summarize(
    start_time   = min(hour_bucket),
    end_time     = max(hour_bucket),
    start_temp   = first(tank_temp),
    end_temp     = last(tank_temp),
    duration_h   = n(),
    temp_drop    = first(tank_temp) - last(tank_temp),
    avg_outdoor  = mean(outdoor, na.rm = TRUE),
    .groups = "drop"
  ) |>
  filter(
    duration_h >= 2,          # at least 2 hours of idle
    duration_h <= 24,         # not unreasonably long
    temp_drop > 0,            # actually cooling
    start_temp > 30           # tank was warm
  ) |>
  mutate(
    loss_rate = temp_drop / duration_h,  # °C per hour
    delta_t   = start_temp - avg_outdoor  # driving force
  )

if (nrow(idle_runs) > 10) {
  cat("\n=== DHW Tank Heat Loss ===\n")
  cat("  Idle runs analyzed:", nrow(idle_runs), "\n")
  cat("  Median loss rate:  ", round(median(idle_runs$loss_rate, na.rm = TRUE), 2), "°C/hour\n")
  cat("  Mean loss rate:    ", round(mean(idle_runs$loss_rate, na.rm = TRUE), 2), "°C/hour\n")
  cat("  Median idle:       ", round(median(idle_runs$duration_h), 1), "hours\n")

  p2 <- ggplot(idle_runs, aes(x = duration_h, y = temp_drop, color = start_temp)) +
    geom_point(alpha = 0.6, size = 2) +
    scale_color_viridis_c(option = "plasma", name = "Start Temp (°C)") +
    geom_smooth(method = "lm", color = COLORS$import, linewidth = 1, se = TRUE) +
    labs(
      x     = "Idle Duration (hours)",
      y     = "Temperature Drop (°C)",
      title = "DHW Tank Heat Loss During Idle Periods",
      subtitle = "Each point = one idle run. Slope = heat loss rate. Color = starting temperature."
    ) +
    theme_energy()

  # Annotate with average loss rate
  model <- lm(temp_drop ~ duration_h, data = idle_runs)
  loss_per_hour <- round(coef(model)[2], 2)
  p2 <- p2 +
    annotate("text",
      x = max(idle_runs$duration_h) * 0.7,
      y = max(idle_runs$temp_drop) * 0.9,
      label = paste0("Loss rate: ~", loss_per_hour, " °C/hour"),
      color = COLORS$muted, size = 3.5
    )

  save_plot(p2, "23_dhw_tank_loss.png")
}

# ============================================================================
# Chart 3: COP gained by shifting DHW to warmest hours
# ============================================================================
# Compare actual DHW COP vs what it would be if DHW only happened at the
# warmest hours (highest COP) of each day.

if (nrow(dhw_heating) > 20 && sum(!is.na(dhw_data$cop)) > 50) {
  daily_dhw <- dhw_data |>
    filter(!is.na(cop), cop > 0.5, cop < 10) |>
    group_by(date) |>
    summarize(
      # Actual: COP during DHW heating hours
      actual_cop = weighted.mean(cop[is_dhw_heating],
                                  w = pmax(dhw_power[is_dhw_heating], 1, na.rm = TRUE),
                                  na.rm = TRUE),
      dhw_hours  = sum(is_dhw_heating),
      # Optimal: COP if DHW happened at the N best COP hours of the day
      best_cop   = mean(sort(cop, decreasing = TRUE)[1:max(1, sum(is_dhw_heating))], na.rm = TRUE),
      # Also: COP at warmest hours
      warmest_cop = {
        warm_idx <- order(outdoor, decreasing = TRUE)[1:max(1, sum(is_dhw_heating))]
        mean(cop[warm_idx], na.rm = TRUE)
      },
      avg_outdoor = mean(outdoor, na.rm = TRUE),
      .groups = "drop"
    ) |>
    filter(!is.na(actual_cop), !is.na(best_cop), dhw_hours > 0)

  if (nrow(daily_dhw) > 7) {
    cat("\n=== DHW COP Optimization ===\n")
    cat("  Days analyzed:    ", nrow(daily_dhw), "\n")
    cat("  Mean actual COP:  ", round(mean(daily_dhw$actual_cop, na.rm = TRUE), 2), "\n")
    cat("  Mean best COP:    ", round(mean(daily_dhw$best_cop, na.rm = TRUE), 2), "\n")
    cat("  Mean warmest COP: ", round(mean(daily_dhw$warmest_cop, na.rm = TRUE), 2), "\n")
    cat("  COP improvement:  ",
        round(mean(daily_dhw$best_cop, na.rm = TRUE) - mean(daily_dhw$actual_cop, na.rm = TRUE), 2),
        "\n")

    # Long format for comparison
    cop_compare <- daily_dhw |>
      select(date, avg_outdoor, actual_cop, best_cop) |>
      pivot_longer(
        cols = c(actual_cop, best_cop),
        names_to = "scenario",
        values_to = "cop"
      ) |>
      mutate(scenario = case_when(
        scenario == "actual_cop" ~ "Actual DHW timing",
        scenario == "best_cop"  ~ "Optimal (best COP hours)"
      ))

    p3 <- ggplot(cop_compare, aes(x = avg_outdoor, y = cop, color = scenario)) +
      geom_point(alpha = 0.3, size = 1.5) +
      geom_smooth(method = "loess", linewidth = 1.2, se = TRUE) +
      scale_color_manual(values = c(
        "Actual DHW timing"       = COLORS$import,
        "Optimal (best COP hours)" = COLORS$export
      )) +
      labs(
        x     = "Daily Average Outdoor Temperature (°C)",
        y     = "COP During DHW Heating",
        title = "DHW COP: Actual vs Optimal Timing",
        subtitle = "Green = if DHW ran at best COP hours. Gap = improvement potential.",
        color = ""
      ) +
      theme_energy()

    save_plot(p3, "23_dhw_cop_potential.png")
  }
}

# ============================================================================
# Chart 4: DHW cost heatmap — hour × month
# ============================================================================
# Combine DHW power with spot prices to show when DHW heating is most expensive.

dhw_cost_data <- dhw_data |>
  filter(is_dhw_heating, !is.na(price), !is.na(dhw_power)) |>
  mutate(
    cost_pln = dhw_power / 1000 * price,  # W → kW, times PLN/kWh
    month_num = month(hour_bucket)
  )

if (nrow(dhw_cost_data) > 20) {
  cost_heatmap <- dhw_cost_data |>
    group_by(month, hour) |>
    summarize(
      avg_cost = mean(cost_pln, na.rm = TRUE),
      total_cost = sum(cost_pln, na.rm = TRUE),
      n = n(),
      .groups = "drop"
    ) |>
    filter(n >= 2)

  cat("\n=== DHW Cost by Time ===\n")
  cat("  Total DHW cost hours:", nrow(dhw_cost_data), "\n")
  cat("  Total DHW cost:      ", round(sum(dhw_cost_data$cost_pln, na.rm = TRUE), 2), "PLN\n")

  p4 <- ggplot(cost_heatmap, aes(x = hour, y = month, fill = avg_cost)) +
    geom_tile(color = "white", linewidth = 0.3) +
    scale_fill_viridis_c(option = "inferno", name = "Avg Cost\n(PLN/hour)") +
    scale_x_continuous(breaks = seq(0, 23, 3)) +
    labs(
      x     = "Hour of Day",
      y     = "",
      title = "DHW Heating Cost Heatmap",
      subtitle = "Dark = expensive DHW hours. Shift to lighter cells for savings."
    ) +
    theme_energy()

  save_plot(p4, "23_dhw_cost_heatmap.png")
} else {
  cat("Insufficient DHW + price data for cost heatmap.\n")
}
