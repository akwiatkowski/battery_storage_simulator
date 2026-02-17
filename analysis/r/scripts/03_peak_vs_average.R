# ============================================================================
# 03_peak_vs_average.R — Peak vs Average Power: 4 Charts
# ============================================================================
# WHAT:    Demonstrates why hourly averages are misleading for energy system
#          sizing. Within each hour, peak power can be 2-5x the average.
#
# INPUTS:  hourly (from load_data.R — hourly grid power with avg/max/min)
#
# OUTPUTS: output/03_peak_vs_avg_scatter.png  — 2D histogram of peak vs avg
#          output/03_ratio_histogram.png      — distribution of peak/avg ratio
#          output/03_inverter_sizing.png      — coverage curve for peak vs avg
#          output/03_hourly_gap.png           — hour-of-day avg vs peak profile
#
# HOW TO READ:
#   - The scatter shows every hour as a point: x = hourly average, y = peak
#   - Points above the 1:1 diagonal mean peak exceeded average (always true)
#   - The ratio histogram shows most hours have peaks 1.5-3x the average
#   - The sizing curve shows: to cover 95% of actual peaks, you need ~2x the
#     inverter size suggested by averages alone
# ============================================================================

source("analysis/r/R/load_data.R")

# Filter to hours with meaningful consumption (>100W avg)
data <- hourly |>
  filter(avg_power > 0) |>
  mutate(peak_to_avg = max_power / avg_power) |>
  filter(avg_power > 100)

# Print summary statistics to console
cat("\n=== Peak vs Average Summary ===\n")
data |>
  summarize(
    hours          = n(),
    avg_of_averages = mean(avg_power),
    avg_of_peaks    = mean(max_power),
    median_peak_ratio = median(peak_to_avg),
    p90_peak_ratio    = quantile(peak_to_avg, 0.9),
    p95_peak_ratio    = quantile(peak_to_avg, 0.95)
  ) |> print()

# --- Chart 1: Scatter of peak vs average ------------------------------------
# geom_bin2d bins the scatter into a 2D grid and colors by density.
# Diagonal lines show where peak = 1x, 2x, 3x, 5x the average.
p1 <- ggplot(data, aes(x = avg_power, y = max_power)) +
  geom_bin2d(bins = 50) +
  scale_fill_gradient(low = COLORS$bg, high = COLORS$import) +
  geom_abline(slope = 1, linetype = "dashed", color = COLORS$muted) +
  geom_abline(slope = 2, linetype = "dotted", color = COLORS$heat_pump, linewidth = 0.8) +
  geom_abline(slope = 3, linetype = "dotted", color = COLORS$import, linewidth = 0.8) +
  geom_abline(slope = 5, linetype = "dotted", color = "red", linewidth = 0.8) +
  annotate("text", x = 2500, y = 2500, label = "1:1", angle = 30, color = COLORS$muted) +
  annotate("text", x = 1500, y = 3100, label = "2x", angle = 45, color = COLORS$heat_pump) +
  annotate("text", x = 1000, y = 3200, label = "3x", angle = 52, color = COLORS$import) +
  annotate("text", x = 700,  y = 3700, label = "5x", angle = 60, color = "red") +
  labs(
    x        = "Hourly Average Power (W)",
    y        = "Peak Power in That Hour (W)",
    title    = "Peak vs Average Power Per Hour",
    subtitle = "If you size to average, you miss everything above the 1:1 line"
  ) +
  theme_energy()

save_plot(p1, "03_peak_vs_avg_scatter.png", width = 10, height = 8)

# --- Chart 2: Histogram of peak-to-average ratio ----------------------------
# Shows how concentrated the ratio is. Median typically ~1.8x means the
# peak is almost double the hourly average in a typical hour.
p2 <- ggplot(data, aes(x = peak_to_avg)) +
  geom_histogram(bins = 50, fill = COLORS$import, color = "white") +
  geom_vline(xintercept = median(data$peak_to_avg), linetype = "dashed",
             color = COLORS$text) +
  annotate("text",
           x = median(data$peak_to_avg) + 0.3, y = Inf, vjust = 2,
           label = paste0("median: ", round(median(data$peak_to_avg), 1), "x"),
           color = COLORS$text) +
  labs(
    x        = "Peak / Average Ratio",
    y        = "Number of Hours",
    title    = "Distribution of Peak-to-Average Ratio",
    subtitle = "How many times is peak power higher than the hourly average?"
  ) +
  theme_energy()

save_plot(p2, "03_ratio_histogram.png")

# --- Chart 3: Inverter sizing curve ------------------------------------------
# Uses compute_coverage_curve() from helpers.R to compute what % of hours
# fall below each power cap, for both peak and average power.
peak_curve <- compute_coverage_curve(data$max_power) |> mutate(metric = "Peak power")
avg_curve  <- compute_coverage_curve(data$avg_power) |> mutate(metric = "Average power")
coverage   <- bind_rows(peak_curve, avg_curve)

p3 <- ggplot(coverage, aes(x = cap_w, y = pct_covered, color = metric)) +
  geom_line(linewidth = 1.2) +
  scale_color_manual(values = c("Peak power" = COLORS$import,
                                "Average power" = COLORS$export)) +
  geom_hline(yintercept = 95, linetype = "dashed", color = COLORS$muted) +
  annotate("text", x = 500, y = 96, label = "95% coverage", hjust = 0,
           color = COLORS$muted) +
  labs(
    x        = "Inverter Power Rating (W)",
    y        = "% of Hours Fully Covered",
    title    = "Inverter Sizing Curve",
    subtitle = "What power rating covers 95% of actual peaks?",
    color    = ""
  ) +
  theme_energy()

save_plot(p3, "03_inverter_sizing.png")

# --- Chart 4: Hour-of-day gap between average and peak -----------------------
# Shows when during the day the discrepancy is worst (typically midday
# when PV fluctuations cause rapid power swings).
hourly_profile <- data |>
  mutate(hour = hour(hour_bucket)) |>
  group_by(hour) |>
  summarize(
    avg       = mean(avg_power),
    peak_median = median(max_power),
    peak_p90    = quantile(max_power, 0.9),
    .groups     = "drop"
  )

p4 <- ggplot(hourly_profile, aes(x = hour)) +
  geom_ribbon(aes(ymin = avg, ymax = peak_p90), fill = COLORS$import, alpha = 0.2) +
  geom_ribbon(aes(ymin = avg, ymax = peak_median), fill = COLORS$import, alpha = 0.3) +
  geom_line(aes(y = avg, color = "Hourly average"), linewidth = 1.2) +
  geom_line(aes(y = peak_median, color = "Median peak"), linewidth = 1.2) +
  geom_line(aes(y = peak_p90, color = "90th percentile peak"),
            linewidth = 1, linetype = "dashed") +
  scale_color_manual(values = c(
    "Hourly average"         = COLORS$export,
    "Median peak"            = COLORS$import,
    "90th percentile peak"   = "#cc4444"
  )) +
  scale_x_continuous(breaks = 0:23) +
  labs(
    x        = "Hour of Day",
    y        = "Power (W)",
    title    = "Average vs Peak Power by Hour of Day",
    subtitle = "The red gap is what an average-sized inverter would miss",
    color    = ""
  ) +
  theme_energy()

save_plot(p4, "03_hourly_gap.png", width = 12, height = 6)
