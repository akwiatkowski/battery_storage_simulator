# ============================================================================
# 15_indoor_temperature.R — Indoor Temperature Stability Analysis
# ============================================================================
# WHAT:    Analyzes indoor temperature across rooms using HA long-term statistics.
#          Shows thermal stability, room-by-room comparison, and correlation
#          with outdoor temperature and HP activity.
#
# INPUTS:  load_stats_sensor() for indoor temp sensors + outdoor temp
#
# OUTPUTS: output/15_room_temp_comparison.png — average temps per room
#          output/15_daily_temp_range.png     — daily min/max swing per room
#          output/15_thermal_response.png     — indoor vs outdoor lag
#
# HOW TO READ:
#   - Room comparison: higher bars = warmer rooms, error bars = typical range
#   - Daily range: wider bands = less stable temperature
#   - Thermal response: steeper slopes = faster response to outdoor changes
# ============================================================================

source("analysis/helpers/load_data.R")

# ============================================================================
# Load indoor temperature data from stats (hourly avg/min/max)
# ============================================================================
room_sensors <- list(
  "Bedroom 1"  = TEMP_BEDROOM1,
  "Bedroom 2"  = TEMP_BEDROOM2,
  "Kitchen"    = TEMP_KITCHEN,
  "Office 1"   = TEMP_OFFICE1,
  "Office 2"   = TEMP_OFFICE2,
  "Bathroom"   = TEMP_BATHROOM,
  "Workshop"   = TEMP_WORKSHOP
)

# Load all room temps from stats data
room_data <- map2(names(room_sensors), room_sensors, function(name, sid) {
  df <- load_stats_sensor(sid)
  if (nrow(df) == 0) return(tibble())
  df |> mutate(room = name)
}) |> bind_rows()

# Load outdoor temp for comparison
outdoor <- load_stats_sensor(HP_OUTSIDE_TEMP)

cat("\n=== Indoor Temperature Data ===\n")
room_data |>
  group_by(room) |>
  summarize(
    hours = n(),
    avg_temp = round(mean(avg, na.rm = TRUE), 1),
    min_temp = round(min(min_val, na.rm = TRUE), 1),
    max_temp = round(max(max_val, na.rm = TRUE), 1),
    .groups = "drop"
  ) |>
  print()

if (nrow(room_data) < 10) {
  cat("Insufficient indoor temperature data for analysis.\n")
  quit(save = "no")
}

# ============================================================================
# Chart 1: Room-by-room average temperature comparison
# ============================================================================
room_summary <- room_data |>
  group_by(room) |>
  summarize(
    mean_temp = mean(avg, na.rm = TRUE),
    sd_temp   = sd(avg, na.rm = TRUE),
    p10       = quantile(avg, 0.1, na.rm = TRUE),
    p90       = quantile(avg, 0.9, na.rm = TRUE),
    .groups   = "drop"
  ) |>
  arrange(desc(mean_temp))

p1 <- ggplot(room_summary, aes(x = reorder(room, mean_temp), y = mean_temp)) +
  geom_col(fill = COLORS$charge, alpha = 0.7, width = 0.6) +
  geom_errorbar(aes(ymin = p10, ymax = p90), width = 0.2, color = COLORS$import) +
  geom_hline(yintercept = 21, linetype = "dashed", color = COLORS$export, linewidth = 0.5) +
  annotate("text", x = 0.7, y = 21.3, label = "21°C target", color = COLORS$export, size = 3, hjust = 0) +
  coord_flip(ylim = c(15, 28)) +
  labs(
    x     = "",
    y     = "Temperature (°C)",
    title = "Average Temperature by Room",
    subtitle = "Bars = mean, whiskers = P10–P90 range"
  ) +
  theme_energy()

save_plot(p1, "15_room_temp_comparison.png")

# ============================================================================
# Chart 2: Daily temperature swing (max - min) per room
# ============================================================================
daily_range <- room_data |>
  mutate(date = as.Date(hour_bucket)) |>
  group_by(room, date) |>
  summarize(
    daily_min = min(min_val, na.rm = TRUE),
    daily_max = max(max_val, na.rm = TRUE),
    swing     = daily_max - daily_min,
    .groups   = "drop"
  )

swing_summary <- daily_range |>
  group_by(room) |>
  summarize(
    median_swing = median(swing, na.rm = TRUE),
    p25_swing    = quantile(swing, 0.25, na.rm = TRUE),
    p75_swing    = quantile(swing, 0.75, na.rm = TRUE),
    .groups      = "drop"
  ) |>
  arrange(desc(median_swing))

p2 <- ggplot(swing_summary, aes(x = reorder(room, median_swing), y = median_swing)) +
  geom_col(fill = COLORS$heat_pump, alpha = 0.7, width = 0.6) +
  geom_errorbar(aes(ymin = p25_swing, ymax = p75_swing), width = 0.2, color = COLORS$import) +
  coord_flip() +
  labs(
    x     = "",
    y     = "Daily Temperature Swing (°C)",
    title = "Temperature Stability by Room",
    subtitle = "Smaller swing = more stable temperature. Whiskers = IQR."
  ) +
  theme_energy()

save_plot(p2, "15_daily_temp_range.png")

# ============================================================================
# Chart 3: Indoor vs outdoor temperature correlation
# ============================================================================
# Pick the room with most data for the correlation analysis
best_room <- room_data |>
  count(room) |>
  slice_max(n, n = 1) |>
  pull(room)

indoor_hourly <- room_data |>
  filter(room == best_room) |>
  select(hour_bucket, indoor_temp = avg)

if (nrow(outdoor) > 10 && nrow(indoor_hourly) > 10) {
  combined <- indoor_hourly |>
    inner_join(outdoor |> select(hour_bucket, outdoor_temp = avg), by = "hour_bucket") |>
    filter(!is.na(indoor_temp), !is.na(outdoor_temp)) |>
    mutate(hour = hour(hour_bucket))

  # Hourly profile: indoor vs outdoor
  hourly_profile <- combined |>
    group_by(hour) |>
    summarize(
      indoor  = mean(indoor_temp, na.rm = TRUE),
      outdoor = mean(outdoor_temp, na.rm = TRUE),
      .groups = "drop"
    ) |>
    pivot_longer(c(indoor, outdoor), names_to = "location", values_to = "temp")

  p3 <- ggplot(hourly_profile, aes(x = hour, y = temp, color = location)) +
    geom_line(linewidth = 1.2) +
    geom_point(size = 1.5) +
    scale_color_manual(
      values = c("indoor" = COLORS$charge, "outdoor" = COLORS$import),
      labels = c("indoor" = paste(best_room, "(indoor)"), "outdoor" = "Outdoor")
    ) +
    scale_x_continuous(breaks = seq(0, 23, 3)) +
    labs(
      x     = "Hour of Day",
      y     = "Temperature (°C)",
      title = "Indoor vs Outdoor Temperature Profile",
      subtitle = paste0("Hourly averages — ", best_room, " tracks outdoor with thermal lag"),
      color = ""
    ) +
    theme_energy()

  save_plot(p3, "15_thermal_response.png")
} else {
  cat("Insufficient outdoor data for thermal response chart.\n")
}
