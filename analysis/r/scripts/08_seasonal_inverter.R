# ============================================================================
# 08_seasonal_inverter.R — Seasonal Inverter Sizing
# ============================================================================
# WHAT:    Breaks down inverter sizing curves by season to show which time
#          of year drives the requirements.
#
# INPUTS:  hourly (from load_data.R — hourly grid power with season column)
#
# OUTPUTS: output/08_seasonal_peak_sizing.png — sizing curve per season
#          output/08_seasonal_gap.png         — avg vs peak bar chart
#          output/08_seasonal_profiles.png    — hourly profiles per season
#
# HOW TO READ:
#   - Sizing by season: the leftmost (lowest) curve sets the binding constraint
#   - If Winter reaches 95% coverage much later than Summer, winter drives
#     your inverter size
#   - Gap chart: the distance between green and red bars is what averages hide
#   - Seasonal profiles: solid = average, dashed = median peak; the gap
#     between them is worst at certain hours
# ============================================================================

source("analysis/r/R/load_data.R")

data <- hourly |> filter(avg_power > 100)

power_levels <- seq(500, 8000, by = 100)

# Compute coverage curves per season using compute_coverage_curve().
# group_map splits by season and applies our helper to each group.
coverage <- data |>
  group_by(season) |>
  group_map(~ {
    s <- .y$season
    compute_coverage_curve(.x$max_power, power_levels) |>
      mutate(season = s)
  }) |>
  bind_rows()

# --- Chart 1: Peak coverage by season ----------------------------------------
p1 <- ggplot(coverage, aes(x = cap_w, y = pct_covered, color = season)) +
  geom_line(linewidth = 1.2) +
  geom_hline(yintercept = 95, linetype = "dashed", color = COLORS$muted) +
  annotate("text", x = 500, y = 96, label = "95% coverage", hjust = 0,
           color = COLORS$muted) +
  scale_color_manual(values = SEASON_COLORS) +
  labs(
    x        = "Inverter Power Rating (W)",
    y        = "% of Peak Hours Covered",
    title    = "Inverter Sizing by Season \u2014 Peak Power",
    subtitle = "Which season drives your inverter size?",
    color    = ""
  ) +
  theme_energy()

save_plot(p1, "08_seasonal_peak_sizing.png")

# --- Chart 2: Average vs peak gap by season ----------------------------------
gap_by_season <- data |>
  group_by(season) |>
  summarize(
    avg_of_avg  = mean(avg_power),
    avg_of_peak = mean(max_power),
    p95_peak    = quantile(max_power, 0.95),
    median_ratio = median(max_power / avg_power),
    .groups     = "drop"
  )

cat("\n=== Seasonal Peak vs Average ===\n")
gap_by_season |> print()

# pivot_longer reshapes from wide to long format so ggplot can map
# each metric to a different bar color.
gap_long <- gap_by_season |>
  select(season,
         "Hourly Average" = avg_of_avg,
         "Average Peak" = avg_of_peak,
         "95th Percentile Peak" = p95_peak) |>
  pivot_longer(-season, names_to = "metric", values_to = "power")

p2 <- ggplot(gap_long, aes(x = season, y = power, fill = metric)) +
  geom_col(position = "dodge") +
  scale_fill_manual(values = c(
    "Hourly Average"         = COLORS$export,
    "Average Peak"           = COLORS$heat_pump,
    "95th Percentile Peak"   = COLORS$import
  )) +
  labs(
    x        = "",
    y        = "Power (W)",
    title    = "Seasonal Power Levels \u2014 Average vs Peak",
    subtitle = "The gap between green and red is what averages hide",
    fill     = ""
  ) +
  theme_energy()

save_plot(p2, "08_seasonal_gap.png")

# --- Chart 3: Hourly profile by season (faceted) -----------------------------
hourly_seasonal <- data |>
  group_by(season, hour) |>
  summarize(
    avg        = mean(avg_power),
    peak_median = median(max_power),
    .groups    = "drop"
  )

p3 <- ggplot(hourly_seasonal, aes(x = hour)) +
  geom_ribbon(aes(ymin = avg, ymax = peak_median, fill = season), alpha = 0.2) +
  geom_line(aes(y = avg, color = season), linewidth = 0.8) +
  geom_line(aes(y = peak_median, color = season), linewidth = 0.8, linetype = "dashed") +
  facet_wrap(~season) +
  scale_color_manual(values = SEASON_COLORS) +
  scale_fill_manual(values = SEASON_COLORS) +
  scale_x_continuous(breaks = seq(0, 23, 3)) +
  labs(
    x        = "Hour of Day",
    y        = "Power (W)",
    title    = "Daily Power Profile by Season",
    subtitle = "Solid = hourly average, dashed = median peak"
  ) +
  theme_energy() +
  theme(legend.position = "none")

save_plot(p3, "08_seasonal_profiles.png", width = 12, height = 8)
