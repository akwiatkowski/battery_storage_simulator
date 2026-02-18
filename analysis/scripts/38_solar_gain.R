# ============================================================================
# 38_solar_gain.R — Solar Gain & Indoor Temperature Analysis
# ============================================================================
# WHAT:    Analyzes how sunny weather causes indoor overheating on cold days.
#          Compares indoor temperatures on sunny vs cloudy cold days to quantify
#          passive solar gain and inform HP temperature offset decisions.
#
# INPUTS:  load_stats_sensor() for indoor temp sensors
#          Open-Meteo weather CSVs for direct_radiation, cloud_cover, temperature
#
# OUTPUTS: output/38_solar_gain_scatter.png    — indoor temp vs solar radiation
#          output/38_sunny_vs_cloudy.png       — hourly profiles sunny vs cloudy
#          output/38_overshoot_by_radiation.png — temp above setpoint vs radiation
#          output/38_solar_gain_heatmap.png    — hour × radiation → indoor temp
#
# HOW TO READ:
#   - Scatter: upward trend = solar gain heating the house
#   - Sunny vs cloudy: gap between lines = passive solar contribution
#   - Overshoot: positive values = room above setpoint → HP should reduce output
#   - Heatmap: bright cells = combinations where overheating occurs
# ============================================================================

source("analysis/helpers/load_data.R")

# ============================================================================
# 1. Load indoor temperature sensors
# ============================================================================
# Three groups per user request:
#   (a) Living room alone (south-facing windows, most solar gain)
#   (b) Average of kitchen + living + office 1 (main living areas)
#   (c) Average of all internal temperatures

living <- load_stats_sensor(NETATMO_LIVING_TEMP) |>
  distinct(hour_bucket, .keep_all = TRUE) |>
  select(hour_bucket, living_temp = avg)

kitchen <- load_stats_sensor(TEMP_KITCHEN) |>
  distinct(hour_bucket, .keep_all = TRUE) |>
  select(hour_bucket, kitchen_temp = avg)

office1 <- load_stats_sensor(TEMP_OFFICE1) |>
  distinct(hour_bucket, .keep_all = TRUE) |>
  select(hour_bucket, office1_temp = avg)

# All internal room sensors
all_room_sensors <- list(
  NETATMO_LIVING_TEMP, TEMP_KITCHEN, TEMP_OFFICE1,
  TEMP_BEDROOM1, TEMP_BEDROOM2, TEMP_OFFICE2, TEMP_BATHROOM
)

all_rooms <- map(all_room_sensors, function(sid) {
  load_stats_sensor(sid) |>
    distinct(hour_bucket, .keep_all = TRUE) |>
    select(hour_bucket, temp = avg)
}) |>
  reduce(full_join, by = "hour_bucket")

# Compute row-wise mean of all room temps
temp_cols <- paste0("temp.", c("x", "y", rep("", 5)))  # auto-generated suffixes
# Simpler: just compute mean manually across all loaded columns
all_rooms <- all_rooms |>
  rowwise() |>
  mutate(all_rooms_avg = mean(c_across(starts_with("temp")), na.rm = TRUE)) |>
  ungroup() |>
  select(hour_bucket, all_rooms_avg)

# Combine into one table
indoor <- living |>
  full_join(kitchen, by = "hour_bucket") |>
  full_join(office1, by = "hour_bucket") |>
  full_join(all_rooms, by = "hour_bucket") |>
  mutate(
    main_areas_avg = rowMeans(
      cbind(living_temp, kitchen_temp, office1_temp), na.rm = TRUE
    )
  )

cat("\n=== Indoor Temperature Data ===\n")
cat("  Living room:      ", sum(!is.na(indoor$living_temp)), "hours\n")
cat("  Kitchen:          ", sum(!is.na(indoor$kitchen_temp)), "hours\n")
cat("  Office 1:         ", sum(!is.na(indoor$office1_temp)), "hours\n")
cat("  All rooms avg:    ", sum(!is.na(indoor$all_rooms_avg)), "hours\n")

# ============================================================================
# 2. Load weather data (Open-Meteo cached CSVs)
# ============================================================================
weather_files <- list.files(
  "input/weather",
  pattern = "^poznan-.*\\.csv$",
  full.names = TRUE
)

weather <- weather_files |>
  map(~ read_csv(.x, show_col_types = FALSE)) |>
  bind_rows() |>
  mutate(hour_bucket = as_datetime(timestamp)) |>
  select(
    hour_bucket,
    outdoor_temp    = temperature_2m,
    cloud_cover,
    direct_rad      = direct_radiation,
    diffuse_rad     = diffuse_radiation,
    sunshine_dur    = sunshine_duration
  ) |>
  distinct(hour_bucket, .keep_all = TRUE) |>
  arrange(hour_bucket) |>
  mutate(total_rad = direct_rad + diffuse_rad)

cat("  Weather data:     ", nrow(weather), "hours (",
    format(min(weather$hour_bucket)), "to", format(max(weather$hour_bucket)), ")\n")

# ============================================================================
# 3. Join indoor + weather
# ============================================================================
combined <- indoor |>
  inner_join(weather, by = "hour_bucket") |>
  mutate(
    hour  = hour(hour_bucket),
    date  = as.Date(hour_bucket),
    month = month(hour_bucket)
  )

cat("  Combined (joined):", nrow(combined), "hours\n")

if (nrow(combined) < 200) {
  cat("Insufficient overlapping data for solar gain analysis.\n")
  quit(save = "no")
}

# ============================================================================
# 4. Classify days: cold (outdoor < 5°C daily mean) + sunny vs cloudy
# ============================================================================
# Daily aggregates for classification
daily_weather <- combined |>
  group_by(date) |>
  summarize(
    daily_mean_temp    = mean(outdoor_temp, na.rm = TRUE),
    daily_max_rad      = max(direct_rad, na.rm = TRUE),
    daily_total_rad    = sum(direct_rad, na.rm = TRUE),
    daily_mean_cloud   = mean(cloud_cover, na.rm = TRUE),
    daily_sunshine_hrs = sum(sunshine_dur, na.rm = TRUE) / 3600,
    .groups = "drop"
  )

# Cold days: daily mean outdoor temp < 5°C (heating season, real cold)
cold_days <- daily_weather |>
  filter(daily_mean_temp < 5)

cat("\n=== Cold Days (daily mean < 5°C) ===\n")
cat("  Total cold days: ", nrow(cold_days), "\n")

# Classify cold days as sunny or cloudy
# Sunny: daily mean cloud cover < 50% AND peak radiation > 200 W/m²
# Cloudy: daily mean cloud cover > 70% OR peak radiation < 100 W/m²
cold_days <- cold_days |>
  mutate(
    sky = case_when(
      daily_mean_cloud < 50 & daily_max_rad > 200 ~ "Sunny",
      daily_mean_cloud > 70 | daily_max_rad < 100  ~ "Cloudy",
      TRUE ~ "Mixed"
    )
  )

cat("  Sunny cold days: ", sum(cold_days$sky == "Sunny"), "\n")
cat("  Cloudy cold days:", sum(cold_days$sky == "Cloudy"), "\n")
cat("  Mixed cold days: ", sum(cold_days$sky == "Mixed"), "\n")

# Join classification back to hourly data
combined <- combined |>
  inner_join(cold_days |> select(date, sky, daily_mean_temp), by = "date")

# ============================================================================
# Chart 1: Indoor temp vs direct solar radiation on cold days
# ============================================================================
# Daytime hours only (8:00–17:00) when solar gain matters
daytime <- combined |>
  filter(hour >= 8, hour <= 17, direct_rad > 0)

if (nrow(daytime) > 50) {
  # Pivot to long format for faceting
  scatter_data <- daytime |>
    select(hour_bucket, direct_rad, outdoor_temp,
           `Living Room` = living_temp,
           `Main Areas (K+L+O1)` = main_areas_avg,
           `All Rooms` = all_rooms_avg) |>
    pivot_longer(
      c(`Living Room`, `Main Areas (K+L+O1)`, `All Rooms`),
      names_to = "group", values_to = "indoor_temp"
    ) |>
    filter(!is.na(indoor_temp))

  p1 <- ggplot(scatter_data, aes(x = direct_rad, y = indoor_temp)) +
    geom_point(aes(color = outdoor_temp), alpha = 0.3, size = 0.8) +
    geom_smooth(method = "loess", color = COLORS$import, linewidth = 1, se = TRUE,
                fill = COLORS$import, alpha = 0.15) +
    geom_hline(yintercept = 21, linetype = "dashed", color = COLORS$muted) +
    annotate("text", x = 50, y = 21.3, label = "21°C setpoint",
             color = COLORS$muted, size = 3, hjust = 0) +
    scale_color_gradient2(
      low = COLORS$charge, mid = COLORS$pv, high = COLORS$import,
      midpoint = 0, name = "Outdoor °C"
    ) +
    facet_wrap(~group, ncol = 3) +
    labs(
      x     = "Direct Solar Radiation (W/m²)",
      y     = "Indoor Temperature (°C)",
      title = "Indoor Temperature vs Solar Radiation on Cold Days",
      subtitle = "Daytime hours (8–17h), cold days only (daily mean < 5°C). Upward trend = solar gain."
    ) +
    theme_energy() +
    theme(legend.position = "right")

  save_plot(p1, "38_solar_gain_scatter.png", width = 14, height = 5)
}

# ============================================================================
# Chart 2: Hourly indoor temp profile — sunny vs cloudy cold days
# ============================================================================
# Compare the hourly temperature profile on sunny vs cloudy cold days
profile_data <- combined |>
  filter(sky %in% c("Sunny", "Cloudy")) |>
  select(hour, sky,
         `Living Room` = living_temp,
         `Main Areas (K+L+O1)` = main_areas_avg,
         `All Rooms` = all_rooms_avg) |>
  pivot_longer(
    c(`Living Room`, `Main Areas (K+L+O1)`, `All Rooms`),
    names_to = "group", values_to = "indoor_temp"
  ) |>
  filter(!is.na(indoor_temp))

hourly_profiles <- profile_data |>
  group_by(hour, sky, group) |>
  summarize(
    mean_temp = mean(indoor_temp, na.rm = TRUE),
    se_temp   = sd(indoor_temp, na.rm = TRUE) / sqrt(n()),
    n         = n(),
    .groups   = "drop"
  )

if (nrow(hourly_profiles) > 20) {
  p2 <- ggplot(hourly_profiles, aes(x = hour, y = mean_temp, color = sky)) +
    geom_ribbon(aes(ymin = mean_temp - se_temp, ymax = mean_temp + se_temp,
                    fill = sky), alpha = 0.15, color = NA) +
    geom_line(linewidth = 1.2) +
    geom_point(size = 1.5) +
    geom_hline(yintercept = 21, linetype = "dashed", color = COLORS$muted) +
    scale_color_manual(
      values = c("Sunny" = COLORS$pv, "Cloudy" = COLORS$charge),
      name = ""
    ) +
    scale_fill_manual(
      values = c("Sunny" = COLORS$pv, "Cloudy" = COLORS$charge),
      name = ""
    ) +
    scale_x_continuous(breaks = seq(0, 23, 3)) +
    facet_wrap(~group, ncol = 3) +
    labs(
      x     = "Hour of Day",
      y     = "Indoor Temperature (°C)",
      title = "Indoor Temperature: Sunny vs Cloudy Cold Days",
      subtitle = "Cold days (daily mean < 5°C). Gap = passive solar heating contribution."
    ) +
    theme_energy()

  save_plot(p2, "38_sunny_vs_cloudy.png", width = 14, height = 5)

  # Print the key finding: max temperature difference
  diff_by_hour <- hourly_profiles |>
    select(hour, sky, group, mean_temp) |>
    pivot_wider(names_from = sky, values_from = mean_temp) |>
    mutate(solar_gain = Sunny - Cloudy)

  cat("\n=== Solar Gain: Sunny - Cloudy Temperature Difference ===\n")
  diff_by_hour |>
    filter(!is.na(solar_gain)) |>
    group_by(group) |>
    summarize(
      max_gain    = round(max(solar_gain, na.rm = TRUE), 2),
      max_gain_hr = hour[which.max(solar_gain)],
      mean_gain   = round(mean(solar_gain[hour >= 10 & hour <= 16], na.rm = TRUE), 2),
      .groups = "drop"
    ) |>
    print()
}

# ============================================================================
# Chart 3: Temperature overshoot above setpoint by radiation level
# ============================================================================
# Bucket direct radiation into categories and show indoor temp distribution
overshoot_data <- combined |>
  filter(hour >= 9, hour <= 16) |>
  mutate(
    rad_bucket = cut(
      direct_rad,
      breaks = c(-1, 50, 150, 300, 500, Inf),
      labels = c("0-50", "50-150", "150-300", "300-500", "500+")
    ),
    living_overshoot = living_temp - 21,
    main_overshoot   = main_areas_avg - 21,
    all_overshoot    = all_rooms_avg - 21
  ) |>
  filter(!is.na(rad_bucket))

overshoot_long <- overshoot_data |>
  select(rad_bucket,
         `Living Room` = living_overshoot,
         `Main Areas (K+L+O1)` = main_overshoot,
         `All Rooms` = all_overshoot) |>
  pivot_longer(
    c(`Living Room`, `Main Areas (K+L+O1)`, `All Rooms`),
    names_to = "group", values_to = "overshoot"
  ) |>
  filter(!is.na(overshoot))

if (nrow(overshoot_long) > 50) {
  p3 <- ggplot(overshoot_long, aes(x = rad_bucket, y = overshoot)) +
    geom_hline(yintercept = 0, linetype = "dashed", color = COLORS$muted) +
    geom_boxplot(aes(fill = rad_bucket), alpha = 0.6, outlier.alpha = 0.2,
                 outlier.size = 0.5) +
    scale_fill_manual(
      values = c(
        "0-50"    = COLORS$charge,
        "50-150"  = "#8cc4e8",
        "150-300" = COLORS$pv,
        "300-500" = COLORS$discharge,
        "500+"    = COLORS$import
      ),
      guide = "none"
    ) +
    facet_wrap(~group, ncol = 3) +
    labs(
      x     = "Direct Solar Radiation (W/m²)",
      y     = "Temperature Above 21°C Setpoint (°C)",
      title = "Indoor Temperature Overshoot by Solar Radiation Level",
      subtitle = "Cold days, daytime (9–16h). Positive = overheating from solar gain."
    ) +
    theme_energy()

  save_plot(p3, "38_overshoot_by_radiation.png", width = 14, height = 5)

  # Print median overshoot per radiation bucket
  cat("\n=== Overshoot Summary (Living Room) ===\n")
  overshoot_data |>
    filter(!is.na(living_overshoot)) |>
    group_by(rad_bucket) |>
    summarize(
      n              = n(),
      median_temp    = round(median(living_temp, na.rm = TRUE), 1),
      median_over    = round(median(living_overshoot, na.rm = TRUE), 1),
      pct_over_21    = round(100 * mean(living_overshoot > 0, na.rm = TRUE), 0),
      pct_over_22    = round(100 * mean(living_overshoot > 1, na.rm = TRUE), 0),
      .groups = "drop"
    ) |>
    print()
}

# ============================================================================
# Chart 4: Hour × radiation heatmap of living room temperature
# ============================================================================
heatmap_data <- combined |>
  filter(hour >= 7, hour <= 19) |>
  mutate(
    rad_bin = cut(
      direct_rad,
      breaks = c(-1, 0, 100, 200, 300, 400, 500, 700, Inf),
      labels = c("0", "1-100", "100-200", "200-300", "300-400", "400-500", "500-700", "700+")
    )
  ) |>
  filter(!is.na(rad_bin), !is.na(living_temp))

heatmap_summary <- heatmap_data |>
  group_by(hour, rad_bin) |>
  summarize(
    mean_temp = mean(living_temp, na.rm = TRUE),
    n = n(),
    .groups = "drop"
  ) |>
  filter(n >= 3)

if (nrow(heatmap_summary) > 20) {
  p4 <- ggplot(heatmap_summary, aes(x = hour, y = rad_bin, fill = mean_temp)) +
    geom_tile(color = "white", linewidth = 0.5) +
    geom_text(aes(label = round(mean_temp, 1)), size = 3, color = "grey20") +
    scale_fill_gradient2(
      low = COLORS$charge, mid = "white", high = COLORS$import,
      midpoint = 21, name = "Mean °C"
    ) +
    scale_x_continuous(breaks = 7:19) +
    labs(
      x     = "Hour of Day",
      y     = "Direct Solar Radiation (W/m²)",
      title = "Living Room Temperature by Hour & Solar Radiation",
      subtitle = "Cold days only. Red = above 21°C setpoint (solar gain overheating)."
    ) +
    theme_energy() +
    theme(
      panel.grid = element_blank(),
      legend.position = "right"
    )

  save_plot(p4, "38_solar_gain_heatmap.png", width = 12, height = 6)
}

# ============================================================================
# Summary: Suggested HP temperature offset
# ============================================================================
cat("\n=== Solar Gain Summary & HP Offset Suggestion ===\n")

if (nrow(daytime) > 50) {
  # Compute median overshoot for high-radiation hours
  high_rad <- combined |>
    filter(hour >= 10, hour <= 15, direct_rad > 300, !is.na(living_temp))

  if (nrow(high_rad) > 10) {
    median_overshoot <- median(high_rad$living_temp - 21, na.rm = TRUE)
    p90_overshoot    <- quantile(high_rad$living_temp - 21, 0.9, na.rm = TRUE)
    cat("  High radiation (>300 W/m², 10-15h) on cold days:\n")
    cat("    Median living room temp:      ", round(median(high_rad$living_temp, na.rm = TRUE), 1), "°C\n")
    cat("    Median overshoot above 21°C:  ", round(median_overshoot, 1), "°C\n")
    cat("    P90 overshoot:                ", round(p90_overshoot, 1), "°C\n")
    cat("    Suggested HP offset:          -", round(max(0, median_overshoot), 1), "°C\n")
    cat("    (Reduce target temp by this amount on sunny cold days)\n")
  }
}
