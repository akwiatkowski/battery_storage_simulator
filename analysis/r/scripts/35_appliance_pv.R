# ============================================================================
# 35_appliance_pv.R — Appliance vs PV Overlap
# ============================================================================
# WHAT:    Analyzes how well appliance usage (washing machine, drier, oven)
#          aligns with PV generation. Identifies how much appliance energy is
#          "covered" by solar and estimates savings from shifting usage to
#          peak PV hours.
#
# INPUTS:  load_stats_sensor() for WASHING_SENSOR, DRIER_SENSOR, OVEN_SENSOR
#          hourly grid power (avg_power) from load_data.R for PV proxy
#          spot_prices for cost analysis
#
# OUTPUTS: output/35_appliance_timing_vs_pv.png   — hour-of-day usage vs PV
#          output/35_appliance_solar_coverage.png  — % energy covered by PV
#          output/35_appliance_shift_savings.png   — savings from shifting to PV peak
#
# HOW TO READ:
#   - Timing chart: appliance bars should overlap with the PV curve for
#     maximum self-consumption. Gaps = missed solar opportunity.
#   - Solar coverage: higher % = more appliance energy came from PV.
#     Drier/washing during midday = high coverage.
#   - Shift savings: compares actual price paid vs price during 10-14h peak PV.
#     Large bars = significant savings possible by shifting to solar hours.
# ============================================================================

source("analysis/r/R/load_data.R")

# ============================================================================
# Load appliance data
# ============================================================================
appliances <- list(
  "Washing Machine" = list(sensor = WASHING_SENSOR, threshold = 10),
  "Drier"           = list(sensor = DRIER_SENSOR,   threshold = 10),
  "Oven"            = list(sensor = OVEN_SENSOR,    threshold = 100)
)

appliance_data <- map2(names(appliances), appliances, function(name, cfg) {
  df <- load_stats_sensor(cfg$sensor) |>
    distinct(hour_bucket, .keep_all = TRUE) |>
    select(hour_bucket, power = avg)

  cat("  ", name, ":", nrow(df), "hours\n")

  if (nrow(df) < 20) {
    cat("    Skipping — insufficient data.\n")
    return(tibble())
  }

  df |>
    mutate(
      appliance = name,
      active = power > cfg$threshold,
      hour = hour(hour_bucket)
    )
}) |> bind_rows()

cat("\n=== Appliance Data ===\n")
cat("  Total rows:    ", nrow(appliance_data), "\n")
cat("  Appliances:    ", paste(unique(appliance_data$appliance), collapse = ", "), "\n")

if (nrow(appliance_data) < 20) {
  cat("Insufficient appliance data. Skipping analysis.\n")
  quit(save = "no")
}

# ============================================================================
# PV generation profile (from grid power)
# ============================================================================
# PV proxy: negative grid power = house is exporting surplus PV
pv_profile <- hourly |>
  filter(!is.na(avg_power)) |>
  mutate(
    hour = hour(hour_bucket),
    pv_export_w = pmax(-avg_power, 0)
  ) |>
  group_by(hour) |>
  summarize(
    avg_pv_w = mean(pv_export_w, na.rm = TRUE),
    .groups = "drop"
  )

# Grid power by hour for identifying net-exporting hours
hourly_grid <- hourly |>
  filter(!is.na(avg_power)) |>
  select(hour_bucket, grid_power = avg_power)

# ============================================================================
# Chart 1: Appliance timing vs PV generation
# ============================================================================
# Compute hourly activity rate and average power for each appliance
hourly_appliance <- appliance_data |>
  group_by(appliance, hour) |>
  summarize(
    activity_pct = mean(active, na.rm = TRUE) * 100,
    avg_power_w  = mean(power[active], na.rm = TRUE),
    .groups = "drop"
  ) |>
  # Replace NaN from mean of empty vector (no active hours)
  mutate(avg_power_w = if_else(is.na(avg_power_w), 0, avg_power_w))

# Scale PV to match the activity % axis for overlay
pv_scale <- max(hourly_appliance$activity_pct, na.rm = TRUE) /
            max(pv_profile$avg_pv_w, na.rm = TRUE)

p1 <- ggplot() +
  # PV generation ribbon (background)
  geom_area(data = pv_profile,
            aes(x = hour, y = avg_pv_w * pv_scale),
            fill = COLORS$pv, alpha = 0.2) +
  geom_line(data = pv_profile,
            aes(x = hour, y = avg_pv_w * pv_scale),
            color = COLORS$pv, linewidth = 1.2, linetype = "solid") +
  # Appliance activity bars
  geom_col(data = hourly_appliance,
           aes(x = hour, y = activity_pct, fill = appliance),
           position = "dodge", alpha = 0.7, width = 0.7) +
  scale_fill_manual(values = c(
    "Washing Machine" = COLORS$charge,
    "Drier"           = COLORS$heat_pump,
    "Oven"            = COLORS$import
  )) +
  scale_x_continuous(breaks = seq(0, 23, 2)) +
  scale_y_continuous(
    name = "Appliance Activity (%)",
    sec.axis = sec_axis(~ . / pv_scale, name = "Avg PV Export (W)")
  ) +
  labs(
    x     = "Hour of Day",
    title = "Appliance Usage Timing vs PV Generation",
    subtitle = "Bars = % of hours each appliance was active. Gold area/line = average PV export.",
    fill  = "Appliance"
  ) +
  theme_energy()

save_plot(p1, "35_appliance_timing_vs_pv.png")

# ============================================================================
# Chart 2: Solar coverage per appliance
# ============================================================================
# For each appliance-hour, check if grid was net-exporting (grid_power <= 0).
# If so, the appliance energy was effectively "covered" by PV.

solar_coverage <- appliance_data |>
  filter(active) |>
  inner_join(hourly_grid, by = "hour_bucket") |>
  mutate(covered_by_pv = grid_power <= 0) |>
  group_by(appliance) |>
  summarize(
    total_hours   = n(),
    solar_hours   = sum(covered_by_pv, na.rm = TRUE),
    total_kwh     = sum(power, na.rm = TRUE) / 1000,
    solar_kwh     = sum(power[covered_by_pv], na.rm = TRUE) / 1000,
    coverage_pct  = solar_hours / total_hours * 100,
    energy_pct    = solar_kwh / total_kwh * 100,
    .groups = "drop"
  )

cat("\n=== Solar Coverage per Appliance ===\n")
print(solar_coverage)

if (nrow(solar_coverage) > 0) {
  p2 <- ggplot(solar_coverage, aes(x = reorder(appliance, coverage_pct),
                                    y = coverage_pct)) +
    geom_col(fill = COLORS$export, alpha = 0.75, width = 0.6) +
    geom_text(aes(label = paste0(round(coverage_pct, 1), "%")),
              hjust = -0.15, color = COLORS$text, size = 4) +
    geom_col(aes(y = energy_pct), fill = COLORS$pv, alpha = 0.5, width = 0.4) +
    coord_flip() +
    scale_y_continuous(limits = c(0, max(solar_coverage$coverage_pct * 1.2, 10)),
                       expand = expansion(mult = c(0, 0.1))) +
    labs(
      x     = "",
      y     = "% Covered by PV",
      title = "Appliance Energy Covered by Solar",
      subtitle = "Green = % of active hours during net-export. Gold = % of energy during net-export."
    ) +
    theme_energy()

  save_plot(p2, "35_appliance_solar_coverage.png")
}

# ============================================================================
# Chart 3: Optimal shift savings — current price vs peak PV price
# ============================================================================
# Compare average spot price during actual appliance usage vs price during
# peak PV hours (10:00-14:00), showing potential savings from load shifting.

if (nrow(spot_prices) >= 20) {
  # Average spot price during peak PV hours
  pv_peak_price <- spot_prices |>
    mutate(hour = hour(hour_bucket)) |>
    filter(hour >= 10, hour <= 14) |>
    summarize(avg_pv_price = mean(price, na.rm = TRUE)) |>
    pull(avg_pv_price)

  appliance_costs <- appliance_data |>
    filter(active) |>
    left_join(spot_prices, by = "hour_bucket") |>
    filter(!is.na(price)) |>
    group_by(appliance) |>
    summarize(
      avg_actual_price = mean(price, na.rm = TRUE),
      total_kwh        = sum(power, na.rm = TRUE) / 1000,
      n_hours          = n(),
      .groups = "drop"
    ) |>
    mutate(
      avg_pv_price    = pv_peak_price,
      price_diff      = avg_actual_price - avg_pv_price,
      annual_savings  = price_diff * total_kwh *
                        annualize_factor(appliance_data$hour_bucket)
    )

  cat("\n=== Shift Savings Potential ===\n")
  cat("  Avg price during PV peak (10-14h):", round(pv_peak_price, 4), "PLN/kWh\n")
  print(appliance_costs |> select(appliance, avg_actual_price, avg_pv_price,
                                   price_diff, total_kwh, annual_savings))

  if (nrow(appliance_costs) > 0) {
    costs_long <- appliance_costs |>
      select(appliance, `Current Usage` = avg_actual_price,
             `PV Peak (10-14h)` = avg_pv_price) |>
      pivot_longer(cols = -appliance, names_to = "scenario", values_to = "price")

    p3 <- ggplot(costs_long, aes(x = appliance, y = price, fill = scenario)) +
      geom_col(position = "dodge", alpha = 0.75, width = 0.6) +
      scale_fill_manual(values = c(
        "Current Usage"    = COLORS$import,
        "PV Peak (10-14h)" = COLORS$export
      )) +
      labs(
        x     = "",
        y     = "Avg Spot Price (PLN/kWh)",
        title = "Appliance Cost: Current Timing vs PV Peak Hours",
        subtitle = "Shifting to 10-14h exploits cheap solar. Green bar lower = savings from shift.",
        fill  = ""
      ) +
      theme_energy()

    # Add annual savings annotation
    for (i in seq_len(nrow(appliance_costs))) {
      row <- appliance_costs[i, ]
      if (row$annual_savings > 0) {
        p3 <- p3 + annotate(
          "text", x = row$appliance,
          y = max(costs_long$price, na.rm = TRUE) * 1.02,
          label = paste0("+", round(row$annual_savings, 1), " PLN/yr"),
          color = COLORS$export, size = 3.5, fontface = "bold"
        )
      }
    }

    save_plot(p3, "35_appliance_shift_savings.png")
  }
} else {
  cat("Insufficient spot price data for shift savings analysis.\n")
}

cat("\n=== Appliance vs PV Analysis Complete ===\n")
