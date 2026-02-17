# ============================================================================
# 37_self_sufficiency.R — Electricity Self-Sufficiency Calendar
# ============================================================================
# WHAT:    Visualizes when the house runs entirely on solar power (grid import
#          = 0 or negative). Builds three views: a month x hour heatmap showing
#          the probability of self-sufficiency for each time slot, a monthly
#          summary bar chart, and a GitHub-style calendar heatmap of daily
#          solar coverage during daylight hours.
#
# INPUTS:  hourly grid power (avg_power) from load_data.R
#
# OUTPUTS: output/37_self_sufficiency_heatmap.png  — month x hour probability map
#          output/37_self_sufficiency_monthly.png   — % self-sufficient hours per month
#          output/37_self_sufficiency_calendar.png  — daily calendar heatmap
#
# HOW TO READ:
#   - Month x hour heatmap: green cells = high probability of running on solar
#     alone. Summer midday should be dark green; winter nights should be white.
#   - Monthly bars: taller bars = more self-sufficient months. Summer peaks.
#   - Calendar heatmap: each cell is one day. Green = most daylight hours
#     covered by PV. Grey = low solar coverage. Look for weekday patterns
#     (lower consumption on weekends → higher self-sufficiency).
# ============================================================================

source("analysis/r/R/load_data.R")

# ============================================================================
# Prepare self-sufficiency data
# ============================================================================
# Self-sufficient hour: grid_power <= 0 (house not importing from grid).
# This means PV covers all consumption plus possibly exports surplus.

ss_data <- hourly |>
  filter(!is.na(avg_power)) |>
  mutate(
    self_sufficient = avg_power <= 0,   # TRUE if grid import is zero or exporting
    hour  = hour(hour_bucket),
    month = month(hour_bucket, label = TRUE),
    month_num = month(hour_bucket),
    date  = as.Date(hour_bucket)
  )

cat("\n=== Self-Sufficiency Data ===\n")
cat("  Total hours:              ", nrow(ss_data), "\n")
cat("  Self-sufficient hours:    ", sum(ss_data$self_sufficient), "\n")
cat("  Overall self-sufficiency: ",
    round(mean(ss_data$self_sufficient) * 100, 1), "%\n")

if (nrow(ss_data) < 20) {
  cat("Insufficient data for self-sufficiency analysis.\n")
  quit(save = "no")
}

# ============================================================================
# Chart 1: Month x hour heatmap — probability of self-sufficiency
# ============================================================================
month_hour <- ss_data |>
  group_by(month, hour) |>
  summarize(
    pct_ss = mean(self_sufficient, na.rm = TRUE) * 100,
    n = n(),
    .groups = "drop"
  ) |>
  filter(n >= 5)  # need at least 5 observations per cell

cat("\n=== Month x Hour Summary ===\n")
cat("  Cells with data: ", nrow(month_hour), "\n")
cat("  Max %:           ", round(max(month_hour$pct_ss), 1), "%\n")

p1 <- ggplot(month_hour, aes(x = hour, y = month, fill = pct_ss)) +
  geom_tile(color = "white", linewidth = 0.3) +
  geom_text(aes(label = sprintf("%.0f", pct_ss)),
            color = ifelse(month_hour$pct_ss > 50, "white", COLORS$text),
            size = 2.8) +
  scale_fill_gradient(
    low = COLORS$bg, high = COLORS$export,
    name = "% Self-Sufficient",
    limits = c(0, 100)
  ) +
  scale_x_continuous(breaks = seq(0, 23, 2)) +
  labs(
    x     = "Hour of Day",
    y     = "",
    title = "Self-Sufficiency Probability: Month \u00d7 Hour",
    subtitle = "% of hours where the house ran entirely on PV (grid import = 0). Green = solar powered."
  ) +
  theme_energy() +
  theme(
    panel.grid = element_blank(),
    axis.text.y = element_text(size = 11)
  )

save_plot(p1, "37_self_sufficiency_heatmap.png", width = 12, height = 6)

# ============================================================================
# Chart 2: Monthly self-sufficiency bar chart
# ============================================================================
monthly_ss <- ss_data |>
  mutate(month_date = floor_date(date, "month")) |>
  group_by(month_date) |>
  summarize(
    total_hours = n(),
    ss_hours    = sum(self_sufficient),
    pct_ss      = mean(self_sufficient) * 100,
    .groups = "drop"
  ) |>
  filter(total_hours >= 100)  # only months with reasonable coverage

cat("\n=== Monthly Self-Sufficiency ===\n")
print(monthly_ss |> mutate(across(c(pct_ss), round, 1)))

p2 <- ggplot(monthly_ss, aes(x = month_date, y = pct_ss)) +
  geom_col(fill = COLORS$export, alpha = 0.75) +
  geom_text(aes(label = paste0(round(pct_ss, 0), "%")),
            vjust = -0.3, color = COLORS$text, size = 3.5) +
  geom_hline(yintercept = c(25, 50), linetype = "dashed",
             color = COLORS$muted, alpha = 0.5) +
  scale_x_date(date_labels = "%b\n%Y", date_breaks = "1 month") +
  scale_y_continuous(limits = c(0, max(monthly_ss$pct_ss * 1.15, 10))) +
  labs(
    x     = "",
    y     = "% of Hours Self-Sufficient",
    title = "Monthly Self-Sufficiency Rate",
    subtitle = "% of all hours where PV covered 100% of demand (grid import = 0)."
  ) +
  theme_energy()

save_plot(p2, "37_self_sufficiency_monthly.png")

# ============================================================================
# Chart 3: GitHub-style calendar heatmap — daily self-sufficiency
# ============================================================================
# Compute daily self-sufficiency during daylight hours (6:00-20:00)
daily_ss <- ss_data |>
  filter(hour >= 6, hour < 20) |>   # daylight hours only
  group_by(date) |>
  summarize(
    daylight_hours = n(),
    ss_hours       = sum(self_sufficient),
    pct_ss         = mean(self_sufficient) * 100,
    .groups = "drop"
  ) |>
  filter(daylight_hours >= 8) |>   # need most daylight hours present
  mutate(
    weekday  = wday(date, label = TRUE, week_start = 1),
    # ISO week number for x-axis positioning
    week_num = as.numeric(format(date, "%V")),
    year     = year(date),
    month    = month(date, label = TRUE),
    # For multi-year data, create a continuous week counter
    year_week = paste0(year, "-W", sprintf("%02d", week_num))
  )

cat("\n=== Daily Self-Sufficiency (6-20h) ===\n")
cat("  Days with data: ", nrow(daily_ss), "\n")
cat("  Median %:       ", round(median(daily_ss$pct_ss), 1), "%\n")
cat("  Days >= 50%:    ", sum(daily_ss$pct_ss >= 50), "\n")
cat("  Days >= 75%:    ", sum(daily_ss$pct_ss >= 75), "\n")

if (nrow(daily_ss) >= 14) {
  # Create a continuous week index for proper x-axis layout
  week_order <- daily_ss |>
    distinct(year_week, date) |>
    arrange(date) |>
    mutate(week_idx = as.numeric(factor(year_week, levels = unique(year_week))))

  daily_ss <- daily_ss |>
    left_join(week_order |> select(year_week, week_idx) |> distinct(),
              by = "year_week")

  # Month labels: position at the middle week of each month
  month_labels <- daily_ss |>
    mutate(year_month = floor_date(date, "month")) |>
    group_by(year_month) |>
    summarize(
      label_x = median(week_idx, na.rm = TRUE),
      label = format(first(date), "%b %Y"),
      .groups = "drop"
    )

  p3 <- ggplot(daily_ss, aes(x = week_idx, y = fct_rev(weekday), fill = pct_ss)) +
    geom_tile(color = "white", linewidth = 0.4) +
    scale_fill_gradient(
      low = COLORS$border, high = COLORS$export,
      name = "% Self-Sufficient\n(daylight hours)",
      limits = c(0, 100)
    ) +
    scale_x_continuous(
      breaks = month_labels$label_x,
      labels = month_labels$label,
      expand = expansion(mult = c(0.01, 0.01))
    ) +
    labs(
      x     = "",
      y     = "",
      title = "Daily Self-Sufficiency Calendar",
      subtitle = "Each cell = one day, colored by % of daylight hours (6-20h) running on PV alone."
    ) +
    theme_energy() +
    theme(
      panel.grid = element_blank(),
      axis.text.x = element_text(angle = 45, hjust = 1, size = 9),
      axis.text.y = element_text(size = 10)
    )

  save_plot(p3, "37_self_sufficiency_calendar.png", width = 14, height = 4)
} else {
  cat("Insufficient daily data for calendar heatmap (need >= 14 days).\n")
}

cat("\n=== Self-Sufficiency Analysis Complete ===\n")
