# ============================================================================
# 30_auxiliary_heater.R — Auxiliary (Backup) Heater Cost Analysis
# ============================================================================
# WHAT:    Analyzes the backup electric resistor heater (COP=1) that kicks in
#          during extreme cold or DHW boost. Quantifies how much energy and
#          money the heater costs vs the heat pump at its normal COP.
#
#          HP_HEATER_ROOM_HOURS and HP_HEATER_DHW_HOURS are cumulative counters
#          (total operation hours). Positive diffs = heater was running.
#
# INPUTS:  load_stats_sensor() for HP_HEATER_ROOM_HOURS, HP_HEATER_DHW_HOURS,
#          HP_OUTSIDE_TEMP, HP_COP_SENSOR, spot_prices
#
# OUTPUTS: output/30_heater_monthly_energy.png — monthly kWh from room + DHW heater
#          output/30_heater_trigger_temp.png   — outdoor temp vs heater activation
#          output/30_heater_cost_vs_hp.png     — extra cost from heater vs HP COP
#          output/30_heater_time_of_day.png    — hour-of-day heater activation profile
#
# HOW TO READ:
#   - Monthly energy: tall bars in cold months = heater active during peak demand
#   - Trigger temp: cluster below -5C = heater is a cold-weather backup (expected)
#   - Cost comparison: heater at COP=1 vs HP at COP=3 means 3x the electricity
#   - Time of day: DHW heater at night = boost cycle; room heater at any hour = deficiency
# ============================================================================

source("analysis/helpers/load_data.R")

# ============================================================================
# Physical constants
# ============================================================================
HEATER_POWER_W <- 3000  # Typical Panasonic Aquarea backup heater (3 kW)

# ============================================================================
# Load cumulative heater hour counters
# ============================================================================
room_heater_raw <- load_stats_sensor(HP_HEATER_ROOM_HOURS) |>
  distinct(hour_bucket, .keep_all = TRUE)
dhw_heater_raw <- load_stats_sensor(HP_HEATER_DHW_HOURS) |>
  distinct(hour_bucket, .keep_all = TRUE)
outdoor_temp <- load_stats_sensor(HP_OUTSIDE_TEMP) |>
  distinct(hour_bucket, .keep_all = TRUE)
cop_sensor <- load_stats_sensor(HP_COP_SENSOR) |>
  distinct(hour_bucket, .keep_all = TRUE)

cat("\n=== Auxiliary Heater Data ===\n")
cat("  Room heater hours sensor:", nrow(room_heater_raw), "rows\n")
cat("  DHW heater hours sensor: ", nrow(dhw_heater_raw), "rows\n")
cat("  Outdoor temp:            ", nrow(outdoor_temp), "rows\n")
cat("  COP sensor:              ", nrow(cop_sensor), "rows\n")

# ============================================================================
# Compute hourly heater usage from cumulative counters
# ============================================================================
# delta_hours = fraction of the hour the heater was on (0 to 1)
# Energy = delta_hours x HEATER_POWER_W (Wh)

compute_heater_deltas <- function(raw_data, label) {
  if (nrow(raw_data) < 20) {
    cat("  Insufficient", label, "data (", nrow(raw_data), "rows).\n")
    return(tibble(hour_bucket = as.POSIXct(character()),
                  delta_hours = numeric(), energy_wh = numeric()))
  }
  raw_data |>
    arrange(hour_bucket) |>
    mutate(
      delta_hours = avg - lag(avg)
    ) |>
    filter(!is.na(delta_hours), delta_hours >= 0, delta_hours <= 1) |>
    mutate(energy_wh = delta_hours * HEATER_POWER_W) |>
    select(hour_bucket, delta_hours, energy_wh)
}

room_heater <- compute_heater_deltas(room_heater_raw, "room heater")
dhw_heater  <- compute_heater_deltas(dhw_heater_raw, "DHW heater")

cat("\n=== Heater Activity ===\n")
cat("  Room heater active hours:", sum(room_heater$delta_hours > 0), "\n")
cat("  DHW heater active hours: ", sum(dhw_heater$delta_hours > 0), "\n")
cat("  Room heater total energy:", round(sum(room_heater$energy_wh) / 1000, 1), "kWh\n")
cat("  DHW heater total energy: ", round(sum(dhw_heater$energy_wh) / 1000, 1), "kWh\n")

# Check we have enough heater data for at least one chart
total_active <- sum(room_heater$delta_hours > 0) + sum(dhw_heater$delta_hours > 0)
if (total_active < 20) {
  cat("Insufficient heater activation data (", total_active, "active hours). Exiting.\n")
  quit(save = "no")
}

# ============================================================================
# Build combined dataset with outdoor temp, COP, and prices
# ============================================================================
# Tag each heater type and combine
room_tagged <- room_heater |>
  mutate(heater_type = "Room Heating") |>
  filter(delta_hours > 0)

dhw_tagged <- dhw_heater |>
  mutate(heater_type = "DHW Boost") |>
  filter(delta_hours > 0)

heater_combined <- bind_rows(room_tagged, dhw_tagged) |>
  left_join(outdoor_temp |> select(hour_bucket, outdoor = avg), by = "hour_bucket") |>
  left_join(cop_sensor |> select(hour_bucket, cop = avg), by = "hour_bucket") |>
  left_join(spot_prices, by = "hour_bucket") |>
  mutate(
    hour  = hour(hour_bucket),
    month = month(hour_bucket, label = TRUE),
    date  = as.Date(hour_bucket)
  )

# Full hourly dataset (including zero-activity hours) for monthly aggregation
room_full <- room_heater |>
  mutate(heater_type = "Room Heating")
dhw_full <- dhw_heater |>
  mutate(heater_type = "DHW Boost")

# ============================================================================
# Chart 1: Monthly heater energy — stacked bars (room + DHW)
# ============================================================================
monthly_room <- room_full |>
  mutate(month = month(hour_bucket, label = TRUE)) |>
  group_by(month) |>
  summarize(energy_kwh = sum(energy_wh, na.rm = TRUE) / 1000, .groups = "drop") |>
  mutate(heater_type = "Room Heating")

monthly_dhw <- dhw_full |>
  mutate(month = month(hour_bucket, label = TRUE)) |>
  group_by(month) |>
  summarize(energy_kwh = sum(energy_wh, na.rm = TRUE) / 1000, .groups = "drop") |>
  mutate(heater_type = "DHW Boost")

monthly_energy <- bind_rows(monthly_room, monthly_dhw) |>
  filter(energy_kwh > 0)

if (nrow(monthly_energy) > 0) {
  heater_colors <- c(
    "Room Heating" = COLORS$import,
    "DHW Boost"    = COLORS$heat_pump
  )

  total_kwh <- sum(monthly_energy$energy_kwh)

  p1 <- ggplot(monthly_energy, aes(x = month, y = energy_kwh, fill = heater_type)) +
    geom_col(alpha = 0.8) +
    scale_fill_manual(values = heater_colors) +
    labs(
      x     = "",
      y     = "Backup Heater Energy (kWh)",
      title = "Monthly Auxiliary Heater Energy Consumption",
      subtitle = paste0(
        "Backup resistor heater at ", HEATER_POWER_W / 1000, " kW (COP = 1). ",
        "Total: ", round(total_kwh, 1), " kWh."
      ),
      fill  = ""
    ) +
    theme_energy()

  save_plot(p1, "30_heater_monthly_energy.png")
}

# ============================================================================
# Chart 2: Heater trigger temperature — outdoor temp vs heater activation
# ============================================================================
# Use all heater hours (both types) with outdoor temperature
if (nrow(heater_combined) > 20 && sum(!is.na(heater_combined$outdoor)) > 10) {
  # Also include non-active hours for context: bin by outdoor temp,
  # show activation rate
  all_hours <- bind_rows(room_full, dhw_full) |>
    left_join(outdoor_temp |> select(hour_bucket, outdoor = avg), by = "hour_bucket") |>
    filter(!is.na(outdoor)) |>
    mutate(
      temp_bin = cut(outdoor,
                     breaks = seq(-25, 40, by = 2),
                     include.lowest = TRUE),
      is_active = delta_hours > 0
    )

  trigger_by_temp <- all_hours |>
    group_by(temp_bin, heater_type) |>
    summarize(
      total_hours  = n(),
      active_hours = sum(is_active, na.rm = TRUE),
      activation_pct = mean(is_active, na.rm = TRUE) * 100,
      avg_energy_wh  = mean(energy_wh[is_active], na.rm = TRUE),
      .groups = "drop"
    ) |>
    filter(total_hours >= 5) |>
    mutate(
      temp_mid = as.numeric(sub("\\((-?[0-9.]+),.*", "\\1", as.character(temp_bin))) + 1
    )

  if (nrow(trigger_by_temp) > 5) {
    p2 <- ggplot(trigger_by_temp, aes(x = temp_mid, y = activation_pct,
                                       fill = heater_type)) +
      geom_col(position = "dodge", alpha = 0.8) +
      scale_fill_manual(values = heater_colors) +
      labs(
        x     = "Outdoor Temperature (\u00b0C)",
        y     = "% of Hours Heater Active",
        title = "Auxiliary Heater Trigger Temperature",
        subtitle = "Expect spike below -5\u00b0C for room heater. DHW boost may occur at any temp.",
        fill  = ""
      ) +
      theme_energy()

    save_plot(p2, "30_heater_trigger_temp.png")
  }
} else {
  cat("Insufficient outdoor temp data for trigger temperature analysis.\n")
}

# ============================================================================
# Chart 3: Heater cost vs HP cost — extra cost from COP=1 vs normal HP COP
# ============================================================================
# The heater runs at COP=1. If the HP had handled the same load at its typical
# COP, the electricity cost would be 1/COP of the heater cost.
# Extra cost = heater_cost - heater_cost/COP_typical = heater_cost × (1 - 1/COP)

# Get monthly average HP COP for comparison
monthly_cop <- cop_sensor |>
  filter(avg > 0.5, avg < 10) |>
  mutate(month = month(hour_bucket, label = TRUE)) |>
  group_by(month) |>
  summarize(avg_cop = mean(avg, na.rm = TRUE), .groups = "drop")

# Monthly heater cost: use spot prices where available, fallback to 0.80 PLN/kWh
heater_with_price <- bind_rows(room_full, dhw_full) |>
  left_join(spot_prices, by = "hour_bucket") |>
  mutate(
    month = month(hour_bucket, label = TRUE),
    price_effective = ifelse(!is.na(price), price, 0.80),
    cost_pln = energy_wh / 1000 * price_effective
  )

monthly_cost <- heater_with_price |>
  group_by(month, heater_type) |>
  summarize(
    heater_kwh  = sum(energy_wh, na.rm = TRUE) / 1000,
    heater_cost = sum(cost_pln, na.rm = TRUE),
    .groups = "drop"
  ) |>
  left_join(monthly_cop, by = "month") |>
  mutate(
    # If HP had done this work, cost = heater_cost / COP
    # Use a safe default COP of 2.5 if COP data is missing
    avg_cop = ifelse(is.na(avg_cop), 2.5, avg_cop),
    hp_would_cost = heater_cost / avg_cop,
    extra_cost    = heater_cost - hp_would_cost
  ) |>
  filter(heater_cost > 0)

if (nrow(monthly_cost) > 0) {
  cost_long <- monthly_cost |>
    select(month, heater_type, heater_cost, hp_would_cost) |>
    pivot_longer(
      cols = c(heater_cost, hp_would_cost),
      names_to = "cost_type",
      values_to = "pln"
    ) |>
    mutate(cost_type = case_when(
      cost_type == "heater_cost"    ~ "Actual (heater, COP=1)",
      cost_type == "hp_would_cost"  ~ "If HP handled it (at avg COP)"
    ))

  total_extra <- sum(monthly_cost$extra_cost, na.rm = TRUE)

  p3 <- ggplot(cost_long, aes(x = month, y = pln, fill = cost_type)) +
    geom_col(position = "dodge", alpha = 0.8) +
    facet_wrap(~ heater_type, scales = "free_y") +
    scale_fill_manual(values = c(
      "Actual (heater, COP=1)"       = COLORS$import,
      "If HP handled it (at avg COP)" = COLORS$export
    )) +
    labs(
      x     = "",
      y     = "Cost (PLN)",
      title = "Auxiliary Heater Cost vs Heat Pump Cost",
      subtitle = paste0(
        "Red = actual heater cost at COP=1. Green = what HP would cost at its typical COP. ",
        "Extra cost: ", round(total_extra, 1), " PLN."
      ),
      fill  = ""
    ) +
    theme_energy()

  save_plot(p3, "30_heater_cost_vs_hp.png", width = 12, height = 6)
}

# ============================================================================
# Chart 4: Heater time of day — hour-of-day activation profile
# ============================================================================
if (nrow(heater_combined) > 20) {
  hourly_profile <- heater_combined |>
    group_by(hour, heater_type) |>
    summarize(
      total_activation_h = sum(delta_hours, na.rm = TRUE),
      count_active = n(),
      avg_delta    = mean(delta_hours, na.rm = TRUE),
      .groups = "drop"
    )

  p4 <- ggplot(hourly_profile, aes(x = hour, y = total_activation_h,
                                    fill = heater_type)) +
    geom_col(position = "dodge", alpha = 0.8) +
    scale_fill_manual(values = heater_colors) +
    scale_x_continuous(breaks = seq(0, 23, 3)) +
    labs(
      x     = "Hour of Day",
      y     = "Total Heater Runtime (hours)",
      title = "Auxiliary Heater Activation by Time of Day",
      subtitle = "DHW heater often peaks at night (boost cycle). Room heater follows cold spells.",
      fill  = ""
    ) +
    theme_energy()

  save_plot(p4, "30_heater_time_of_day.png")
}

# ============================================================================
# Summary
# ============================================================================
cat("\n=== AUXILIARY HEATER SUMMARY ===\n")
cat("  Backup heater rated power:", HEATER_POWER_W, "W\n")
cat("  Room heater energy:       ", round(sum(room_heater$energy_wh) / 1000, 1), "kWh\n")
cat("  DHW heater energy:        ", round(sum(dhw_heater$energy_wh) / 1000, 1), "kWh\n")
cat("  Combined heater energy:   ",
    round((sum(room_heater$energy_wh) + sum(dhw_heater$energy_wh)) / 1000, 1), "kWh\n")
if (nrow(monthly_cost) > 0) {
  cat("  Total extra cost vs HP:   ", round(sum(monthly_cost$extra_cost, na.rm = TRUE), 1), "PLN\n")
}
