# ============================================================================
# 39_floor_temperature_diff.R — Ground Floor vs First Floor Temperature Analysis
# ============================================================================
# WHAT:    Compares temperatures between ground floor rooms (living room,
#          kitchen, olek, beata) and first floor/attic rooms (bathroom,
#          bedrooms). Shows monthly and hourly difference patterns.
#
# INPUTS:  load_stats_sensor() for indoor temp sensors
#
# OUTPUTS: docs/analysis/39_monthly_floor_diff.png  — monthly boxplot of ΔT
#          docs/analysis/39_hourly_floor_diff.png   — hourly profile of ΔT
#          docs/analysis/39_floor_temp_profiles.png — hourly temp by floor
#          docs/analysis/39_room_by_floor.png       — room-level comparison
# ============================================================================

source("analysis/helpers/load_data.R")

# ============================================================================
# Define floor groupings
# ============================================================================
ground_floor <- list(
  "Living Room" = NETATMO_LIVING_TEMP,
  "Kitchen"     = TEMP_KITCHEN,
  "Olek"        = TEMP_OFFICE1,
  "Beata"       = TEMP_OFFICE2
)

first_floor <- list(
  "Bathroom"  = TEMP_BATHROOM,
  "Bedroom 1" = TEMP_BEDROOM1,
  "Bedroom 2" = TEMP_BEDROOM2
)

# ============================================================================
# Load all room data with floor assignment
# ============================================================================
load_floor_data <- function(sensors, floor_name) {
  map2(names(sensors), sensors, function(name, sid) {
    df <- load_stats_sensor(sid)
    if (nrow(df) == 0) return(tibble())
    df |> mutate(room = name, floor = floor_name)
  }) |> bind_rows()
}

room_data <- bind_rows(
  load_floor_data(ground_floor, "Ground Floor"),
  load_floor_data(first_floor, "First Floor")
)

# Load outdoor temp for context
outdoor <- load_stats_sensor(HP_OUTSIDE_TEMP)

cat("\n=== Floor Temperature Data ===\n")
room_data |>
  group_by(floor, room) |>
  summarize(
    hours = n(),
    avg_temp = round(mean(avg, na.rm = TRUE), 1),
    .groups = "drop"
  ) |>
  print()

if (nrow(room_data) < 100) {
  cat("Insufficient temperature data for floor analysis.\n")
  quit(save = "no")
}

# ============================================================================
# Compute floor averages per hour
# ============================================================================
floor_hourly <- room_data |>
  group_by(floor, hour_bucket) |>
  summarize(temp = mean(avg, na.rm = TRUE), .groups = "drop")

# Pivot to compute difference
floor_wide <- floor_hourly |>
  pivot_wider(names_from = floor, values_from = temp) |>
  rename(ground = `Ground Floor`, first = `First Floor`) |>
  filter(!is.na(ground), !is.na(first)) |>
  mutate(
    diff = first - ground,
    hour = hour(hour_bucket),
    month = month(hour_bucket, label = TRUE),
    date = as.Date(hour_bucket)
  )

# Join outdoor temp
if (nrow(outdoor) > 0) {
  floor_wide <- floor_wide |>
    left_join(outdoor |> select(hour_bucket, outdoor_temp = avg), by = "hour_bucket")
}

cat("\n=== Floor Temperature Summary ===\n")
cat("Ground floor mean:", round(mean(floor_wide$ground, na.rm = TRUE), 1), "°C\n")
cat("First floor mean: ", round(mean(floor_wide$first, na.rm = TRUE), 1), "°C\n")
cat("Mean difference (1st - ground):", round(mean(floor_wide$diff, na.rm = TRUE), 1), "°C\n")

# ============================================================================
# Chart 1: Monthly boxplot of floor temperature difference
# ============================================================================
p1 <- ggplot(floor_wide, aes(x = month, y = diff)) +
  geom_boxplot(fill = COLORS$charge, alpha = 0.5, outlier.size = 0.5) +
  geom_hline(yintercept = 0, linetype = "dashed", color = COLORS$muted) +
  labs(
    x     = "",
    y     = "ΔT: First Floor − Ground Floor (°C)",
    title = "Monthly Temperature Difference Between Floors",
    subtitle = "Positive = first floor warmer (heat rises). Boxes = IQR, whiskers = 1.5×IQR."
  ) +
  theme_energy()

save_plot(p1, "39_monthly_floor_diff.png")

# ============================================================================
# Chart 2: Hourly profile of floor temperature difference
# ============================================================================
hourly_diff <- floor_wide |>
  group_by(hour) |>
  summarize(
    mean_diff = mean(diff, na.rm = TRUE),
    p25 = quantile(diff, 0.25, na.rm = TRUE),
    p75 = quantile(diff, 0.75, na.rm = TRUE),
    .groups = "drop"
  )

p2 <- ggplot(hourly_diff, aes(x = hour, y = mean_diff)) +
  geom_ribbon(aes(ymin = p25, ymax = p75), fill = COLORS$charge, alpha = 0.3) +
  geom_line(color = COLORS$charge, linewidth = 1.2) +
  geom_point(color = COLORS$charge, size = 2) +
  geom_hline(yintercept = 0, linetype = "dashed", color = COLORS$muted) +
  scale_x_continuous(breaks = seq(0, 23, 3)) +
  labs(
    x     = "Hour of Day",
    y     = "ΔT: First Floor − Ground Floor (°C)",
    title = "Hourly Floor Temperature Difference",
    subtitle = "Line = mean, band = IQR. Heat rises — first floor tracks warmer."
  ) +
  theme_energy()

save_plot(p2, "39_hourly_floor_diff.png")

# ============================================================================
# Chart 3: Hourly temperature profiles by floor
# ============================================================================
floor_profiles <- floor_wide |>
  group_by(hour) |>
  summarize(
    ground = mean(ground, na.rm = TRUE),
    first  = mean(first, na.rm = TRUE),
    .groups = "drop"
  ) |>
  pivot_longer(c(ground, first), names_to = "floor", values_to = "temp") |>
  mutate(floor = ifelse(floor == "ground", "Ground Floor", "First Floor"))

p3 <- ggplot(floor_profiles, aes(x = hour, y = temp, color = floor)) +
  geom_line(linewidth = 1.2) +
  geom_point(size = 2) +
  scale_color_manual(values = c(
    "Ground Floor" = COLORS$charge,
    "First Floor"  = COLORS$import
  )) +
  scale_x_continuous(breaks = seq(0, 23, 3)) +
  labs(
    x     = "Hour of Day",
    y     = "Temperature (°C)",
    title = "Hourly Temperature Profiles by Floor",
    subtitle = "Average hourly temperature — ground floor vs first floor/attic",
    color = ""
  ) +
  theme_energy()

save_plot(p3, "39_floor_temp_profiles.png")

# ============================================================================
# Chart 4: Per-room comparison grouped by floor
# ============================================================================
room_summary <- room_data |>
  group_by(floor, room) |>
  summarize(
    mean_temp = mean(avg, na.rm = TRUE),
    p10 = quantile(avg, 0.10, na.rm = TRUE),
    p90 = quantile(avg, 0.90, na.rm = TRUE),
    .groups = "drop"
  ) |>
  arrange(floor, desc(mean_temp))

p4 <- ggplot(room_summary, aes(
  x = reorder(room, mean_temp),
  y = mean_temp,
  fill = floor
)) +
  geom_col(alpha = 0.7, width = 0.6) +
  geom_errorbar(aes(ymin = p10, ymax = p90), width = 0.2, color = "grey30") +
  geom_hline(yintercept = 21, linetype = "dashed", color = COLORS$muted) +
  annotate("text", x = 0.7, y = 21.3, label = "21°C target",
           color = COLORS$muted, size = 3, hjust = 0) +
  coord_flip(ylim = c(18, 28)) +
  scale_fill_manual(values = c(
    "Ground Floor" = COLORS$charge,
    "First Floor"  = COLORS$import
  )) +
  labs(
    x     = "",
    y     = "Temperature (°C)",
    title = "Room Temperatures by Floor",
    subtitle = "Bars = mean, whiskers = P10–P90 range",
    fill  = ""
  ) +
  theme_energy()

save_plot(p4, "39_room_by_floor.png")
