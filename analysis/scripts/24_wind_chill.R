# ============================================================================
# 24_wind_chill.R — Wind Chill & Heat Loss Analysis
# ============================================================================
# WHAT:    Analyzes how wind speed affects heat pump COP and heating demand.
#          Wind strips heat from the outdoor unit's evaporator, reducing COP.
#          Also analyzes wind direction effects (prevailing wind vs building
#          orientation) and wind-driven indoor temperature drops.
#
# INPUTS:  load_stats_sensor() for Netatmo wind speed/angle, HP COP,
#          outdoor temp, HP consumption, indoor room temps
#          Falls back to HP outside temp if Netatmo wind data is sparse.
#
# OUTPUTS: output/24_wind_vs_cop.png        — COP at different wind speeds
#          output/24_wind_rose_heating.png   — wind direction vs heating demand
#          output/24_wind_indoor_effect.png  — wind speed vs indoor temp deviation
#          output/24_wind_power_demand.png   — HP power consumption vs wind speed
#
# HOW TO READ:
#   - Wind vs COP: COP drops at high wind = wind chill effect on evaporator
#   - Wind rose: certain directions cause more heating demand (exposed walls)
#   - Indoor effect: wind pushes indoor temps down despite HP running
#   - Power demand: HP works harder in windy conditions
# ============================================================================

source("analysis/helpers/load_data.R")

# ============================================================================
# Load wind and HP data
# ============================================================================
# Deduplicate all sensors (stats CSVs can have overlapping exports)
wind_speed   <- load_stats_sensor(NETATMO_WIND_SPEED) |> distinct(hour_bucket, .keep_all = TRUE)
wind_angle   <- load_stats_sensor(NETATMO_WIND_ANGLE) |> distinct(hour_bucket, .keep_all = TRUE)
gust_speed   <- load_stats_sensor(NETATMO_GUST_SPEED) |> distinct(hour_bucket, .keep_all = TRUE)
outdoor_temp <- load_stats_sensor(HP_OUTSIDE_TEMP) |> distinct(hour_bucket, .keep_all = TRUE)
cop_sensor   <- load_stats_sensor(HP_COP_SENSOR) |> distinct(hour_bucket, .keep_all = TRUE)
hp_cons      <- load_stats_sensor(HP_CONSUMPTION) |> distinct(hour_bucket, .keep_all = TRUE)
hp_heat      <- load_stats_sensor(HP_HEAT_POWER) |> distinct(hour_bucket, .keep_all = TRUE)

cat("\n=== Wind Data ===\n")
cat("  Wind speed:      ", nrow(wind_speed), "hours\n")
cat("  Wind angle:      ", nrow(wind_angle), "hours\n")
cat("  Gust speed:      ", nrow(gust_speed), "hours\n")
cat("  Outdoor temp:    ", nrow(outdoor_temp), "hours\n")
cat("  COP sensor:      ", nrow(cop_sensor), "hours\n")
cat("  HP consumption:  ", nrow(hp_cons), "hours\n")

# Wind data may be sparse (recently added sensors). Check availability.
has_wind <- nrow(wind_speed) >= 20

if (!has_wind) {
  cat("\nInsufficient wind speed data (need >= 20 hours, have", nrow(wind_speed), ").\n")
  cat("Wind sensors were recently added — data will accumulate over time.\n")
  cat("Skipping wind analysis for now.\n")
  quit(save = "no")
}

# ============================================================================
# Build combined dataset
# ============================================================================
wind_data <- wind_speed |>
  select(hour_bucket, wind_kmh = avg) |>
  left_join(wind_angle |> select(hour_bucket, wind_dir = avg), by = "hour_bucket") |>
  left_join(gust_speed |> select(hour_bucket, gust_kmh = avg), by = "hour_bucket") |>
  inner_join(outdoor_temp |> select(hour_bucket, outdoor = avg), by = "hour_bucket") |>
  left_join(cop_sensor |> select(hour_bucket, cop = avg), by = "hour_bucket") |>
  left_join(hp_cons |> select(hour_bucket, hp_power = avg), by = "hour_bucket") |>
  left_join(hp_heat |> select(hour_bucket, heat_power = avg), by = "hour_bucket") |>
  left_join(spot_prices, by = "hour_bucket") |>
  filter(!is.na(wind_kmh), !is.na(outdoor)) |>
  mutate(
    hour = hour(hour_bucket),
    month = month(hour_bucket, label = TRUE),
    is_heating = outdoor < 12,
    # Wind speed bins
    wind_bin = cut(wind_kmh,
      breaks = c(0, 5, 10, 15, 20, 50),
      labels = c("Calm (0-5)", "Light (5-10)", "Moderate (10-15)",
                 "Fresh (15-20)", "Strong (>20)"),
      include.lowest = TRUE
    ),
    # Wind direction sectors (8 compass points)
    wind_sector = case_when(
      is.na(wind_dir)           ~ NA_character_,
      wind_dir >= 337.5 | wind_dir < 22.5  ~ "N",
      wind_dir >= 22.5 & wind_dir < 67.5   ~ "NE",
      wind_dir >= 67.5 & wind_dir < 112.5  ~ "E",
      wind_dir >= 112.5 & wind_dir < 157.5 ~ "SE",
      wind_dir >= 157.5 & wind_dir < 202.5 ~ "S",
      wind_dir >= 202.5 & wind_dir < 247.5 ~ "SW",
      wind_dir >= 247.5 & wind_dir < 292.5 ~ "W",
      wind_dir >= 292.5 & wind_dir < 337.5 ~ "NW",
      TRUE ~ NA_character_
    )
  )

heating_wind <- wind_data |> filter(is_heating)

cat("\n=== Wind + Heating Dataset ===\n")
cat("  Total hours:    ", nrow(wind_data), "\n")
cat("  Heating hours:  ", nrow(heating_wind), "\n")
cat("  Wind speed range:", round(min(wind_data$wind_kmh, na.rm = TRUE), 1),
    "to", round(max(wind_data$wind_kmh, na.rm = TRUE), 1), "km/h\n")

# ============================================================================
# Chart 1: COP at different wind speeds (controlling for outdoor temp)
# ============================================================================
cop_wind <- heating_wind |>
  filter(!is.na(cop), cop > 0.5, cop < 10, !is.na(wind_bin))

if (nrow(cop_wind) > 20) {
  # Summary by wind bin
  cop_by_wind <- cop_wind |>
    group_by(wind_bin) |>
    summarize(
      mean_cop     = mean(cop, na.rm = TRUE),
      median_cop   = median(cop, na.rm = TRUE),
      mean_outdoor = mean(outdoor, na.rm = TRUE),
      mean_wind    = mean(wind_kmh, na.rm = TRUE),
      n = n(),
      .groups = "drop"
    )

  cat("\n=== COP by Wind Speed ===\n")
  print(cop_by_wind)

  p1 <- ggplot(cop_wind, aes(x = wind_kmh, y = cop)) +
    geom_bin2d(bins = 30) +
    scale_fill_viridis_c(option = "plasma", trans = "log10") +
    geom_smooth(method = "loess", color = COLORS$export, linewidth = 1.2, se = TRUE) +
    labs(
      x     = "Wind Speed (km/h)",
      y     = "COP",
      title = "Wind Speed Effect on Heat Pump COP",
      subtitle = "Wind strips heat from evaporator coil. Heating season only (outdoor < 12°C).",
      fill  = "Hours"
    ) +
    theme_energy()

  save_plot(p1, "24_wind_vs_cop.png")

  # Bonus: control for outdoor temp by using residuals
  # Fit COP ~ outdoor, then check if wind explains residual variation
  model_base <- lm(cop ~ outdoor, data = cop_wind)
  cop_wind$cop_residual <- residuals(model_base)

  model_wind <- lm(cop_residual ~ wind_kmh, data = cop_wind)
  wind_effect <- round(coef(model_wind)[2], 4)
  cat("\n=== Wind Effect (controlling for outdoor temp) ===\n")
  cat("  COP residual per km/h wind:", wind_effect, "\n")
  cat("  Interpretation: +10 km/h wind →", round(wind_effect * 10, 3), "COP change\n")
  cat("  R² of wind on residuals:", round(summary(model_wind)$r.squared, 4), "\n")
}

# ============================================================================
# Chart 2: Wind rose — direction vs heating demand
# ============================================================================
if (sum(!is.na(heating_wind$wind_sector)) > 20) {
  wind_rose <- heating_wind |>
    filter(!is.na(wind_sector), !is.na(hp_power), hp_power > 0) |>
    group_by(wind_sector) |>
    summarize(
      mean_power   = mean(hp_power, na.rm = TRUE),
      mean_wind    = mean(wind_kmh, na.rm = TRUE),
      mean_outdoor = mean(outdoor, na.rm = TRUE),
      n = n(),
      .groups = "drop"
    ) |>
    filter(n >= 3)

  # Order sectors for proper circular layout
  sector_order <- c("N", "NE", "E", "SE", "S", "SW", "W", "NW")
  wind_rose$wind_sector <- factor(wind_rose$wind_sector, levels = sector_order)

  cat("\n=== Wind Direction vs Heating Demand ===\n")
  print(wind_rose |> arrange(desc(mean_power)))

  p2 <- ggplot(wind_rose, aes(x = wind_sector, y = mean_power, fill = mean_power)) +
    geom_col(width = 0.7, alpha = 0.8) +
    scale_fill_gradient(low = COLORS$export, high = COLORS$import, name = "HP Power (W)") +
    coord_polar(start = 0) +
    labs(
      x     = "",
      y     = "Mean HP Power (W)",
      title = "Wind Direction vs Heating Demand",
      subtitle = "Taller bars = more HP power needed from that wind direction."
    ) +
    theme_energy() +
    theme(axis.text.y = element_blank())

  save_plot(p2, "24_wind_rose_heating.png")
}

# ============================================================================
# Chart 3: Wind effect on indoor temperatures
# ============================================================================
# Load indoor temps and check if windy hours cause indoor temp drops
room_sensors <- list(
  "Bedroom 1" = TEMP_BEDROOM1,
  "Kitchen"   = TEMP_KITCHEN,
  "Office 1"  = TEMP_OFFICE1,
  "Bathroom"  = TEMP_BATHROOM
)

avg_indoor <- map2(names(room_sensors), room_sensors, function(name, sid) {
  df <- load_stats_sensor(sid)
  if (nrow(df) == 0) return(tibble())
  df |> distinct(hour_bucket, .keep_all = TRUE) |> select(hour_bucket, temp = avg)
}) |> bind_rows() |>
  group_by(hour_bucket) |>
  summarize(indoor_temp = mean(temp, na.rm = TRUE), .groups = "drop")

indoor_wind <- heating_wind |>
  inner_join(avg_indoor, by = "hour_bucket") |>
  filter(!is.na(indoor_temp), !is.na(wind_bin))

if (nrow(indoor_wind) > 20) {
  # Indoor temp deviation from mean, by wind speed
  indoor_wind <- indoor_wind |>
    mutate(
      indoor_deviation = indoor_temp - mean(indoor_temp, na.rm = TRUE)
    )

  indoor_by_wind <- indoor_wind |>
    group_by(wind_bin) |>
    summarize(
      mean_indoor   = mean(indoor_temp, na.rm = TRUE),
      mean_deviation = mean(indoor_deviation, na.rm = TRUE),
      mean_outdoor  = mean(outdoor, na.rm = TRUE),
      n = n(),
      .groups = "drop"
    )

  cat("\n=== Wind Effect on Indoor Temp ===\n")
  print(indoor_by_wind)

  p3 <- ggplot(indoor_wind, aes(x = wind_kmh, y = indoor_temp, color = outdoor)) +
    geom_point(alpha = 0.3, size = 1) +
    scale_color_viridis_c(option = "plasma", name = "Outdoor (°C)") +
    geom_smooth(method = "lm", color = COLORS$import, linewidth = 1.2, se = TRUE) +
    labs(
      x     = "Wind Speed (km/h)",
      y     = "Average Indoor Temperature (°C)",
      title = "Wind Speed vs Indoor Temperature",
      subtitle = "Heating season. Downward slope = wind pushes indoor temp down despite HP."
    ) +
    theme_energy()

  save_plot(p3, "24_wind_indoor_effect.png")
}

# ============================================================================
# Chart 4: HP power demand vs wind speed (controlling for outdoor temp)
# ============================================================================
power_wind <- heating_wind |>
  filter(!is.na(hp_power), hp_power > 0, !is.na(wind_bin))

if (nrow(power_wind) > 20) {
  # Bin outdoor temp to control for it
  power_wind <- power_wind |>
    mutate(
      outdoor_bin = cut(outdoor, breaks = c(-20, -5, 0, 5, 10, 15),
                        labels = c("< -5°C", "-5 to 0°C", "0 to 5°C",
                                   "5 to 10°C", "10 to 15°C"),
                        include.lowest = TRUE)
    ) |>
    filter(!is.na(outdoor_bin))

  p4 <- ggplot(power_wind, aes(x = wind_kmh, y = hp_power, color = outdoor_bin)) +
    geom_point(alpha = 0.15, size = 0.8) +
    geom_smooth(method = "loess", se = FALSE, linewidth = 1.2) +
    scale_color_brewer(palette = "RdYlBu", direction = 1) +
    labs(
      x     = "Wind Speed (km/h)",
      y     = "HP Power Consumption (W)",
      title = "HP Power Demand vs Wind Speed",
      subtitle = "Each line = outdoor temp range. Rising lines = wind increases heating demand.",
      color = "Outdoor Temp"
    ) +
    theme_energy()

  save_plot(p4, "24_wind_power_demand.png")
} else {
  cat("Insufficient HP power + wind data for power demand analysis.\n")
}
