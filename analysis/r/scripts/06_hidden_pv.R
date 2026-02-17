# ============================================================================
# 06_hidden_pv.R — Hidden PV Generation in Import Hours
# ============================================================================
# WHAT:    Identifies hours where avg_power > 0 (net import) but min_power < 0
#          (there WAS export within the hour). These hours have PV generation
#          that hourly averaging completely hides.
#
# INPUTS:  hourly (from load_data.R — hourly grid power with avg/max/min)
#
# OUTPUTS: output/06_hidden_pv_scatter.png  — import vs hidden export
#          output/06_hidden_pv_hourly.png   — hour-of-day profile
#          output/06_one_day_range.png      — sample day avg vs actual range
#
# HOW TO READ:
#   - Scatter: each point is an "import" hour that actually had export moments
#   - Hourly profile: bars show how many hidden-PV hours per time of day,
#     red line shows average hidden export power
#   - Sample day: black line = hourly average, red band = actual min-to-max
#     range within each hour — shows how much variation averages conceal
# ============================================================================

source("analysis/r/R/load_data.R")

# Find hours with mixed import/export: avg is positive (looks like import)
# but min_power is negative (there were export moments within the hour).
mixed_hours <- hourly |>
  filter(avg_power > 0, min_power < 0) |>
  mutate(
    hidden_export_w  = abs(min_power),
    apparent_import_w = avg_power
  )

total_import_hours <- hourly |> filter(avg_power > 0) |> nrow()

cat("\n=== Hidden PV Summary ===\n")
cat("Hours that appear as pure import:", total_import_hours, "\n")
cat("Of those, hours with hidden export:", nrow(mixed_hours),
    paste0("(", round(nrow(mixed_hours) / total_import_hours * 100, 1), "%)"), "\n")
cat("Average hidden export peak:", round(mean(mixed_hours$hidden_export_w)), "W\n")
cat("Max hidden export peak:", max(mixed_hours$hidden_export_w), "W\n")

# --- Chart 1: Scatter of apparent import vs hidden export --------------------
p1 <- ggplot(mixed_hours, aes(x = apparent_import_w, y = hidden_export_w)) +
  geom_bin2d(bins = 40) +
  scale_fill_gradient(low = COLORS$bg, high = COLORS$pv) +
  labs(
    x        = "Apparent Hourly Import (W avg)",
    y        = "Hidden Export Peak (W)",
    title    = "Hidden PV Generation in 'Import' Hours",
    subtitle = "These hours look like pure consumption, but had solar export moments"
  ) +
  theme_energy()

save_plot(p1, "06_hidden_pv_scatter.png", width = 10, height = 7)

# --- Chart 2: Hour-of-day profile of hidden PV ------------------------------
# Dual-axis: bars = count of hidden-PV hours, line = avg hidden export power.
# sec_axis creates a second y-axis scaled by factor 5 to align the line.
hourly_hidden <- mixed_hours |>
  mutate(hour = hour(hour_bucket)) |>
  group_by(hour) |>
  summarize(
    count             = n(),
    avg_hidden_export = mean(hidden_export_w),
    .groups           = "drop"
  )

p2 <- ggplot(hourly_hidden, aes(x = hour)) +
  geom_col(aes(y = count), fill = COLORS$pv, alpha = 0.7) +
  geom_line(aes(y = avg_hidden_export / 5), color = COLORS$import, linewidth = 1.2) +
  scale_x_continuous(breaks = 0:23) +
  scale_y_continuous(
    name     = "Number of Hidden-PV Hours",
    sec.axis = sec_axis(~ . * 5, name = "Avg Hidden Export (W)")
  ) +
  labs(
    x        = "Hour of Day",
    title    = "When Does Hidden PV Occur?",
    subtitle = "Bars = frequency, red line = average hidden export power"
  ) +
  theme_energy()

save_plot(p2, "06_hidden_pv_hourly.png", width = 12, height = 6)

# --- Chart 3: Sample day showing average vs actual range ---------------------
# Pick the day with the largest max-min spread in a single hour.
# which.max returns the index of the row with the largest range.
sample_day <- hourly |>
  filter(as.Date(hour_bucket) == as.Date(
    hour_bucket[which.max(max_power - min_power)]
  )) |>
  mutate(hour = hour(hour_bucket))

p3 <- ggplot(sample_day, aes(x = hour)) +
  geom_ribbon(aes(ymin = min_power, ymax = max_power),
              fill = COLORS$import, alpha = 0.3) +
  geom_line(aes(y = avg_power), color = COLORS$text, linewidth = 1.2) +
  geom_hline(yintercept = 0, linetype = "dashed", color = COLORS$muted) +
  scale_x_continuous(breaks = 0:23) +
  annotate("text", x = 1, y = -200, label = "Export", color = COLORS$export) +
  annotate("text", x = 1, y = 200,  label = "Import", color = COLORS$import) +
  labs(
    x        = "Hour of Day",
    y        = "Power (W)",
    title    = paste("One Day's Reality: Average vs Actual Range \u2014",
                     as.Date(sample_day$hour_bucket[1])),
    subtitle = "Black line = hourly average, red band = actual min to max within each hour"
  ) +
  theme_energy()

save_plot(p3, "06_one_day_range.png", width = 12, height = 6)
