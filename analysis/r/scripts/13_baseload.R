# ============================================================================
# 13_baseload.R — Always-On Power (Baseload) Detection
# ============================================================================
# WHAT:    Identifies the minimum sustained power draw — the "always-on"
#          devices (fridge, network, standby). Quantifies the annual cost
#          at spot prices.
#
# INPUTS:  hourly (from load_data.R — with min_power), spot_prices
#          load_recent_sensor() for high-res grid data
#
# OUTPUTS: output/13_baseload_hourly.png      — baseload by hour of day
#          output/13_baseload_trend.png        — baseload over time
#          output/13_baseload_cost.png         — annual cost of always-on
#
# HOW TO READ:
#   - Hourly profile: P5 (5th percentile) of min_power shows the floor —
#     the power level you almost never go below. This is your baseload.
#   - Night hours (1-5 AM) give the clearest reading (no cooking, no solar)
#   - Trend: does baseload drift over time? (new devices, forgotten chargers)
#   - Cost: annual PLN spent on always-on devices at spot prices
# ============================================================================

source("analysis/r/R/load_data.R")

# Use hourly data — min_power within each hour is the closest we get to
# instantaneous minimum in the stats data. For hours with positive min_power
# (never exported), this is the baseload floor for that hour.

# Night hours (0-5) are the best proxy — minimal human activity, no PV.
night_data <- hourly |>
  filter(hour >= 0, hour <= 5, !is.na(min_power)) |>
  # Only positive min_power — if min_power < 0, there was export (not baseload)
  filter(min_power > 0)

cat("\n=== Baseload Detection ===\n")
cat("Night hours (0-5 AM) with positive min_power:", nrow(night_data), "\n")
cat("Median night minimum:", round(median(night_data$min_power)), "W\n")
cat("5th percentile:", round(quantile(night_data$min_power, 0.05)), "W\n")
cat("25th percentile:", round(quantile(night_data$min_power, 0.25)), "W\n")

# --- Chart 1: Baseload profile by hour of day --------------------------------
# Shows percentiles of min_power at each hour. The floor (P5) represents
# the true always-on load, while the median includes intermittent loads.
baseload_hourly <- hourly |>
  filter(!is.na(min_power), min_power > 0) |>
  group_by(hour) |>
  summarize(
    p5      = quantile(min_power, 0.05),
    p25     = quantile(min_power, 0.25),
    median  = median(min_power),
    .groups = "drop"
  )

p1 <- ggplot(baseload_hourly, aes(x = hour)) +
  geom_ribbon(aes(ymin = p5, ymax = median), fill = COLORS$charge, alpha = 0.2) +
  geom_ribbon(aes(ymin = p5, ymax = p25), fill = COLORS$charge, alpha = 0.3) +
  geom_line(aes(y = p5, color = "5th percentile"), linewidth = 1) +
  geom_line(aes(y = p25, color = "25th percentile"), linewidth = 0.8,
            linetype = "dashed") +
  geom_line(aes(y = median, color = "Median"), linewidth = 0.8,
            linetype = "dotted") +
  scale_color_manual(values = c(
    "5th percentile"  = COLORS$charge,
    "25th percentile" = COLORS$heat_pump,
    "Median"          = COLORS$import
  )) +
  scale_x_continuous(breaks = 0:23) +
  labs(
    x        = "Hour of Day",
    y        = "Minimum Power Draw (W)",
    title    = "Baseload Profile by Hour of Day",
    subtitle = "The P5 line is your always-on floor \u2014 fridge, network, standby",
    color    = ""
  ) +
  theme_energy()

save_plot(p1, "13_baseload_hourly.png")

# --- Chart 2: Baseload trend over time --------------------------------------
# Weekly P10 of night-time min_power shows if baseload is creeping up.
baseload_weekly <- night_data |>
  mutate(week = floor_date(hour_bucket, "week")) |>
  group_by(week) |>
  summarize(
    p10_min   = quantile(min_power, 0.10),
    median_min = median(min_power),
    .groups   = "drop"
  ) |>
  filter(!is.na(p10_min))

p2 <- ggplot(baseload_weekly, aes(x = week)) +
  geom_line(aes(y = median_min, color = "Median night minimum"),
            linewidth = 0.6, alpha = 0.5) +
  geom_line(aes(y = p10_min, color = "P10 night minimum"),
            linewidth = 1) +
  geom_smooth(aes(y = p10_min), method = "loess", span = 0.3,
              color = COLORS$text, linewidth = 0.8, se = FALSE,
              linetype = "dashed") +
  scale_color_manual(values = c(
    "P10 night minimum"    = COLORS$charge,
    "Median night minimum" = COLORS$muted
  )) +
  labs(
    x        = "",
    y        = "Power (W)",
    title    = "Baseload Trend Over Time",
    subtitle = "Is your always-on power creeping up? (dashed = loess trend)",
    color    = ""
  ) +
  theme_energy()

save_plot(p2, "13_baseload_trend.png")

# --- Chart 3: Annual cost of baseload at spot prices -------------------------
# Estimate: baseload W × hours/year × avg spot price = annual PLN
baseload_w <- round(quantile(night_data$min_power, 0.10))
avg_price  <- mean(spot_prices$price, na.rm = TRUE)

# Use actual hourly data with prices for a more precise estimate
baseload_cost <- hourly |>
  filter(!is.na(price), !is.na(min_power), min_power > 0) |>
  mutate(
    baseload_power = pmin(min_power, baseload_w),  # cap at estimated baseload
    baseload_kwh   = baseload_power / 1000,        # W to kWh (1 hour buckets)
    baseload_cost  = baseload_kwh * price
  )

af <- annualize_factor(baseload_cost$hour_bucket)

# Monthly cost
monthly_cost <- baseload_cost |>
  mutate(month = floor_date(hour_bucket, "month")) |>
  group_by(month) |>
  summarize(
    cost_pln = sum(baseload_cost),
    kwh      = sum(baseload_kwh),
    .groups  = "drop"
  )

annual_cost <- sum(monthly_cost$cost_pln) * af
annual_kwh  <- sum(monthly_cost$kwh) * af

cat("\n=== Baseload Cost ===\n")
cat("Estimated baseload (P10 night):", baseload_w, "W\n")
cat("Annual baseload energy:", round(annual_kwh), "kWh\n")
cat("Annual baseload cost:", round(annual_cost, 2), "PLN\n")
cat("Average spot price:", round(avg_price, 4), "PLN/kWh\n")

p3 <- ggplot(monthly_cost, aes(x = month, y = cost_pln)) +
  geom_col(fill = COLORS$charge, alpha = 0.7) +
  labs(
    x        = "",
    y        = "Baseload Cost (PLN)",
    title    = paste0("Monthly Cost of Always-On Power (~", baseload_w, " W)"),
    subtitle = paste0("Estimated annual: ", round(annual_cost), " PLN / ",
                      round(annual_kwh), " kWh")
  ) +
  theme_energy()

save_plot(p3, "13_baseload_cost.png")
