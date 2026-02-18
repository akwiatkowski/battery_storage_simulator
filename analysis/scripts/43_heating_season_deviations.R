# ============================================================================
# 43_heating_season_deviations.R — Heating Season Temperature Deviations
# ============================================================================
# WHAT:    During heating season (outdoor <10°C), computes each room's average
#          temperature as "normal", then identifies when rooms run hotter or
#          colder. Correlates deviations with solar radiation, wind speed,
#          outdoor temp, and time of day to find the drivers.
#
# INPUTS:  load_stats_sensor() for room temps, HP data
#          input/weather/ for solar radiation, wind, outdoor temp
#
# OUTPUTS: docs/analysis/43_deviation_drivers.png        — what causes hot/cold
#          docs/analysis/43_deviation_heatmap.png        — hour × room deviation
#          docs/analysis/43_solar_overheating.png        — solar radiation → overheating
#          docs/analysis/43_wind_cold.png                — wind → underheating
#          docs/analysis/43_extreme_cold_outdoor.png     — when HP can't keep up
#          docs/analysis/43_deviation_calendar.png       — daily deviation calendar
# ============================================================================

source("analysis/helpers/load_data.R")

# ============================================================================
# 1. Load room temperatures
# ============================================================================
room_sensors <- list(
  "Living Room" = NETATMO_LIVING_TEMP,
  "Kitchen"     = TEMP_KITCHEN,
  "Olek"        = TEMP_OFFICE1,
  "Beata"       = TEMP_OFFICE2,
  "Bathroom"    = TEMP_BATHROOM,
  "Bedroom 1"   = TEMP_BEDROOM1,
  "Bedroom 2"   = TEMP_BEDROOM2
)

floor_map <- c(
  "Living Room" = "Ground", "Kitchen" = "Ground",
  "Olek" = "Ground", "Beata" = "Ground",
  "Bathroom" = "First", "Bedroom 1" = "First", "Bedroom 2" = "First"
)

room_data <- map2(names(room_sensors), room_sensors, function(name, sid) {
  df <- load_stats_sensor(sid)
  if (nrow(df) == 0) return(tibble())
  df |>
    distinct(hour_bucket, .keep_all = TRUE) |>
    mutate(room = name, floor = floor_map[name]) |>
    select(hour_bucket, temp = avg, room, floor)
}) |> bind_rows()

# ============================================================================
# 2. Load weather data
# ============================================================================
weather_files <- list.files("input/weather", pattern = "^poznan-.*\\.csv$", full.names = TRUE)

weather <- weather_files |>
  map(~ read_csv(.x, show_col_types = FALSE)) |>
  bind_rows() |>
  mutate(hour_bucket = as_datetime(timestamp)) |>
  select(
    hour_bucket,
    outdoor_temp = temperature_2m,
    cloud_cover,
    direct_rad = direct_radiation,
    diffuse_rad = diffuse_radiation,
    wind_speed = wind_speed_10m
  ) |>
  distinct(hour_bucket, .keep_all = TRUE) |>
  arrange(hour_bucket) |>
  mutate(total_rad = direct_rad + diffuse_rad)

cat("\n=== Weather data: ", nrow(weather), "hours\n")

# ============================================================================
# 3. Join and filter to heating season (outdoor <10°C)
# ============================================================================
combined <- room_data |>
  inner_join(weather, by = "hour_bucket") |>
  filter(outdoor_temp < 10) |>
  mutate(
    hour = hour(hour_bucket),
    date = as.Date(hour_bucket),
    month = month(hour_bucket, label = TRUE)
  )

cat("Heating season hours (outdoor <10°C):", nrow(combined) / length(unique(combined$room)), "per room\n")
cat("Date range:", format(min(combined$hour_bucket)), "to", format(max(combined$hour_bucket)), "\n")

# ============================================================================
# 4. Compute per-room "normal" and deviations
# ============================================================================
room_normals <- combined |>
  group_by(room, floor) |>
  summarize(normal_temp = mean(temp, na.rm = TRUE), .groups = "drop")

combined <- combined |>
  left_join(room_normals, by = c("room", "floor")) |>
  mutate(deviation = temp - normal_temp)

cat("\n=== Room Normals (heating season) ===\n")
room_normals |> mutate(normal_temp = round(normal_temp, 1)) |> arrange(floor, desc(normal_temp)) |> print()

# Also compute a whole-house average deviation per hour
house_avg <- combined |>
  group_by(hour_bucket, outdoor_temp, direct_rad, total_rad, wind_speed, cloud_cover, hour, date, month) |>
  summarize(
    mean_temp = mean(temp, na.rm = TRUE),
    mean_deviation = mean(deviation, na.rm = TRUE),
    n_rooms = n(),
    .groups = "drop"
  )

house_normal <- mean(house_avg$mean_temp, na.rm = TRUE)
cat("House average (heating season):", round(house_normal, 1), "°C\n")

# Classify conditions
house_avg <- house_avg |>
  mutate(
    sunny = direct_rad > 200,
    windy = wind_speed > 5,
    very_cold = outdoor_temp < -5,
    mild = outdoor_temp > 5,
    condition = case_when(
      sunny & !windy ~ "Sunny, calm",
      sunny & windy ~ "Sunny, windy",
      !sunny & windy ~ "Cloudy, windy",
      very_cold & !sunny ~ "Very cold, cloudy",
      TRUE ~ "Typical"
    )
  )

# ============================================================================
# Chart 1: Deviation drivers — boxplots by weather condition
# ============================================================================
condition_order <- house_avg |>
  group_by(condition) |>
  summarize(med = median(mean_deviation, na.rm = TRUE), n = n(), .groups = "drop") |>
  filter(n >= 20) |>
  arrange(med)

driver_data <- house_avg |>
  filter(condition %in% condition_order$condition)

p1 <- ggplot(driver_data, aes(
  x = factor(condition, levels = condition_order$condition),
  y = mean_deviation,
  fill = condition
)) +
  geom_boxplot(alpha = 0.7, outlier.size = 0.5) +
  geom_hline(yintercept = 0, linetype = "dashed", color = COLORS$muted) +
  coord_flip() +
  scale_fill_manual(values = c(
    "Very cold, cloudy" = COLORS$charge,
    "Cloudy, windy" = "#7ba7cc",
    "Typical" = "grey70",
    "Sunny, windy" = COLORS$pv,
    "Sunny, calm" = COLORS$import
  ), guide = "none") +
  labs(
    x = "",
    y = "Deviation from Heating Season Average (°C)",
    title = "What Drives Indoor Temperature Deviations?",
    subtitle = paste0("Heating season (outdoor <10°C). Normal = ", round(house_normal, 1),
                      "°C. Positive = warmer than average.")
  ) +
  theme_energy()

save_plot(p1, "43_deviation_drivers.png")

# ============================================================================
# Chart 2: Hour × Room deviation heatmap
# ============================================================================
hourly_room_dev <- combined |>
  group_by(room, hour) |>
  summarize(mean_dev = mean(deviation, na.rm = TRUE), .groups = "drop")

p2 <- ggplot(hourly_room_dev, aes(x = hour, y = room, fill = mean_dev)) +
  geom_tile(color = "white", linewidth = 0.3) +
  scale_fill_gradient2(
    low = COLORS$charge, mid = "white", high = COLORS$import,
    midpoint = 0, name = "Deviation (°C)"
  ) +
  scale_x_continuous(breaks = seq(0, 23, 2)) +
  labs(
    x = "Hour of Day",
    y = "",
    title = "When Are Rooms Hotter or Colder Than Normal?",
    subtitle = "Heating season only. Red = above average, blue = below average."
  ) +
  theme_energy() +
  theme(panel.grid = element_blank())

save_plot(p2, "43_deviation_heatmap.png")

# ============================================================================
# Chart 3: Solar radiation → overheating
# ============================================================================
# Bin solar radiation and show room deviation
solar_bins <- combined |>
  mutate(rad_bin = cut(direct_rad,
                       breaks = c(-1, 10, 100, 200, 400, 800),
                       labels = c("0-10\n(cloudy)", "10-100\n(overcast)",
                                  "100-200\n(hazy)", "200-400\n(partly sunny)",
                                  "400+\n(full sun)"))) |>
  filter(!is.na(rad_bin)) |>
  group_by(room, floor, rad_bin) |>
  summarize(
    mean_dev = mean(deviation, na.rm = TRUE),
    n = n(),
    .groups = "drop"
  ) |>
  filter(n >= 10)

p3 <- ggplot(solar_bins, aes(x = rad_bin, y = mean_dev, color = room, group = room)) +
  geom_line(linewidth = 0.9) +
  geom_point(size = 2) +
  geom_hline(yintercept = 0, linetype = "dashed", color = COLORS$muted) +
  facet_wrap(~floor) +
  scale_color_brewer(palette = "Set2") +
  labs(
    x = "Direct Solar Radiation (W/m²)",
    y = "Deviation from Normal (°C)",
    title = "Solar Radiation Drives Overheating",
    subtitle = "During heating season. Strong sun → rooms overshoot normal by 1-2°C.",
    color = ""
  ) +
  theme_energy() +
  theme(axis.text.x = element_text(size = 8))

save_plot(p3, "43_solar_overheating.png")

# ============================================================================
# Chart 4: Wind speed → underheating
# ============================================================================
wind_bins <- combined |>
  mutate(wind_bin = cut(wind_speed,
                        breaks = c(-1, 2, 4, 6, 8, 30),
                        labels = c("0-2\n(calm)", "2-4\n(light)", "4-6\n(moderate)",
                                   "6-8\n(fresh)", "8+\n(strong)"))) |>
  filter(!is.na(wind_bin)) |>
  group_by(room, floor, wind_bin) |>
  summarize(
    mean_dev = mean(deviation, na.rm = TRUE),
    n = n(),
    .groups = "drop"
  ) |>
  filter(n >= 10)

p4 <- ggplot(wind_bins, aes(x = wind_bin, y = mean_dev, color = room, group = room)) +
  geom_line(linewidth = 0.9) +
  geom_point(size = 2) +
  geom_hline(yintercept = 0, linetype = "dashed", color = COLORS$muted) +
  facet_wrap(~floor) +
  scale_color_brewer(palette = "Set2") +
  labs(
    x = "Wind Speed (m/s)",
    y = "Deviation from Normal (°C)",
    title = "Wind Speed Drives Underheating",
    subtitle = "During heating season. High wind → HP COP drops, rooms cool down.",
    color = ""
  ) +
  theme_energy() +
  theme(axis.text.x = element_text(size = 8))

save_plot(p4, "43_wind_underheating.png")

# ============================================================================
# Chart 5: Extreme cold outdoor — when HP struggles
# ============================================================================
outdoor_bins <- combined |>
  mutate(outdoor_bin = cut(outdoor_temp,
                           breaks = c(-30, -10, -5, 0, 5, 10),
                           labels = c("<-10°C", "-10 to -5°C", "-5 to 0°C",
                                      "0 to 5°C", "5 to 10°C"))) |>
  filter(!is.na(outdoor_bin)) |>
  group_by(room, floor, outdoor_bin) |>
  summarize(
    mean_dev = mean(deviation, na.rm = TRUE),
    mean_temp = mean(temp, na.rm = TRUE),
    n = n(),
    .groups = "drop"
  ) |>
  filter(n >= 5)

p5 <- ggplot(outdoor_bins, aes(x = outdoor_bin, y = mean_temp, color = room, group = room)) +
  geom_line(linewidth = 0.9) +
  geom_point(size = 2) +
  facet_wrap(~floor) +
  scale_color_brewer(palette = "Set2") +
  labs(
    x = "Outdoor Temperature",
    y = "Indoor Temperature (°C)",
    title = "Indoor Temperature vs Outdoor Cold",
    subtitle = "Does the HP keep up in extreme cold? Lower lines = rooms that suffer most.",
    color = ""
  ) +
  theme_energy() +
  theme(axis.text.x = element_text(angle = 30, hjust = 1))

save_plot(p5, "43_extreme_cold_outdoor.png")

# ============================================================================
# Chart 6: Daily deviation calendar — identify specific problem days
# ============================================================================
daily_dev <- house_avg |>
  group_by(date) |>
  summarize(
    mean_dev = mean(mean_deviation, na.rm = TRUE),
    mean_outdoor = mean(outdoor_temp, na.rm = TRUE),
    mean_rad = mean(direct_rad, na.rm = TRUE),
    mean_wind = mean(wind_speed, na.rm = TRUE),
    hours = n(),
    .groups = "drop"
  ) |>
  filter(hours >= 12) |>  # need most of a day
  mutate(
    week = isoweek(date),
    weekday = wday(date, label = TRUE, week_start = 1),
    month = month(date, label = TRUE),
    year_month = format(date, "%Y-%m")
  )

p6 <- ggplot(daily_dev, aes(x = weekday, y = reorder(year_month, desc(year_month)),
                             fill = mean_dev)) +
  geom_tile(color = "white", linewidth = 0.5) +
  scale_fill_gradient2(
    low = COLORS$charge, mid = "white", high = COLORS$import,
    midpoint = 0, name = "Deviation\n(°C)",
    limits = c(-2, 2), oob = scales::squish
  ) +
  labs(
    x = "",
    y = "",
    title = "Daily Temperature Deviation Calendar",
    subtitle = "Heating season days only. Red = warmer than normal, blue = colder."
  ) +
  theme_energy() +
  theme(panel.grid = element_blank())

save_plot(p6, "43_deviation_calendar.png", height = 8)

# ============================================================================
# Summary: top 10 hottest and coldest days
# ============================================================================
cat("\n=== Top 10 Hottest Heating Days (above normal) ===\n")
daily_dev |>
  arrange(desc(mean_dev)) |>
  head(10) |>
  mutate(across(where(is.numeric), ~round(., 1))) |>
  select(date, mean_dev, mean_outdoor, mean_rad, mean_wind) |>
  print()

cat("\n=== Top 10 Coldest Heating Days (below normal) ===\n")
daily_dev |>
  arrange(mean_dev) |>
  head(10) |>
  mutate(across(where(is.numeric), ~round(., 1))) |>
  select(date, mean_dev, mean_outdoor, mean_rad, mean_wind) |>
  print()

# ============================================================================
# Correlation summary — what matters most?
# ============================================================================
cat("\n=== Correlation with House Average Deviation ===\n")
cat("  Solar radiation:  r =", round(cor(house_avg$direct_rad, house_avg$mean_deviation, use = "complete"), 3), "\n")
cat("  Total radiation:  r =", round(cor(house_avg$total_rad, house_avg$mean_deviation, use = "complete"), 3), "\n")
cat("  Wind speed:       r =", round(cor(house_avg$wind_speed, house_avg$mean_deviation, use = "complete"), 3), "\n")
cat("  Outdoor temp:     r =", round(cor(house_avg$outdoor_temp, house_avg$mean_deviation, use = "complete"), 3), "\n")
cat("  Cloud cover:      r =", round(cor(house_avg$cloud_cover, house_avg$mean_deviation, use = "complete"), 3), "\n")

# Which factor has the biggest effect?
cat("\n=== Practical Impact ===\n")
sunny_dev <- mean(house_avg$mean_deviation[house_avg$sunny & !house_avg$windy], na.rm = TRUE)
windy_dev <- mean(house_avg$mean_deviation[house_avg$windy & !house_avg$sunny], na.rm = TRUE)
cold_dev <- mean(house_avg$mean_deviation[house_avg$very_cold], na.rm = TRUE)
mild_dev <- mean(house_avg$mean_deviation[house_avg$mild], na.rm = TRUE)

cat("  Sunny & calm:   ", sprintf("%+.2f°C", sunny_dev), "\n")
cat("  Cloudy & windy: ", sprintf("%+.2f°C", windy_dev), "\n")
cat("  Very cold (<-5): ", sprintf("%+.2f°C", cold_dev), "\n")
cat("  Mild (5-10°C):  ", sprintf("%+.2f°C", mild_dev), "\n")
cat("  Range (max effect):", sprintf("%.2f°C", sunny_dev - windy_dev), "\n")
