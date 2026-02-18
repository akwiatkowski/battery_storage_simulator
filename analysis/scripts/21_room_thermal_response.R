# ============================================================================
# 21_room_thermal_response.R — Room-by-Room Thermal Response Analysis
# ============================================================================
# WHAT:    Advanced room-level thermal analysis: cooling rates when HP cycles
#          off, thermal inertia ranking, night temperature drops, and
#          temperature uniformity across the house.
#
# INPUTS:  load_stats_sensor() for all indoor room temps, outdoor temp,
#          HP consumption (to detect on/off periods)
#
# OUTPUTS: output/21_cooling_rates.png       — how fast each room cools (°C/h)
#          output/21_thermal_inertia.png     — inertia ranking: time to lose 1°C
#          output/21_night_drop.png          — overnight temperature drop per room
#          output/21_uniformity.png          — inter-room temp spread over time
#
# HOW TO READ:
#   - Cooling rates: steeper = worse insulation, room loses heat faster
#   - Thermal inertia: higher bars = room holds heat longer
#   - Night drop: bigger drop = more heat loss overnight (external walls, windows)
#   - Uniformity: wider spread = some rooms too hot, others too cold
# ============================================================================

source("analysis/helpers/load_data.R")

# ============================================================================
# Load all room temperature data
# ============================================================================
room_sensors <- list(
  "Bedroom 1" = TEMP_BEDROOM1,
  "Bedroom 2" = TEMP_BEDROOM2,
  "Kitchen"   = TEMP_KITCHEN,
  "Office 1"  = TEMP_OFFICE1,
  "Office 2"  = TEMP_OFFICE2,
  "Bathroom"  = TEMP_BATHROOM
)

room_data <- map2(names(room_sensors), room_sensors, function(name, sid) {
  df <- load_stats_sensor(sid)
  if (nrow(df) == 0) return(tibble())
  df |>
    distinct(hour_bucket, .keep_all = TRUE) |>
    mutate(room = name) |>
    select(hour_bucket, room, temp = avg, temp_min = min_val, temp_max = max_val)
}) |> bind_rows()

outdoor     <- load_stats_sensor(HP_OUTSIDE_TEMP) |> distinct(hour_bucket, .keep_all = TRUE)
hp_cons     <- load_stats_sensor(HP_CONSUMPTION) |> distinct(hour_bucket, .keep_all = TRUE)

cat("\n=== Room Thermal Response Data ===\n")
room_data |>
  group_by(room) |>
  summarize(hours = n(), .groups = "drop") |>
  print()

cat("  Outdoor temp:    ", nrow(outdoor), "hours\n")
cat("  HP consumption:  ", nrow(hp_cons), "hours\n")

if (nrow(room_data) < 100) {
  cat("Insufficient room temperature data.\n")
  quit(save = "no")
}

# ============================================================================
# Chart 1: Cooling rates — hourly temperature change per room
# ============================================================================
# Compute hour-to-hour temperature change. Negative = cooling.
# Filter for cooling periods: HP consumption low + temp dropping.

room_changes <- room_data |>
  arrange(room, hour_bucket) |>
  group_by(room) |>
  mutate(
    temp_change = temp - lag(temp),
    hours_gap = as.numeric(difftime(hour_bucket, lag(hour_bucket), units = "hours"))
  ) |>
  ungroup() |>
  filter(!is.na(temp_change), hours_gap == 1)  # only consecutive hours

# Join outdoor temp for context
room_changes <- room_changes |>
  left_join(outdoor |> select(hour_bucket, outdoor_temp = avg), by = "hour_bucket") |>
  left_join(hp_cons |> select(hour_bucket, hp_power = avg), by = "hour_bucket")

# Cooling periods: temp dropping AND low HP consumption (coasting)
cooling <- room_changes |>
  filter(
    temp_change < 0,                              # temperature falling
    !is.na(hp_power), hp_power < 200,             # HP mostly off
    !is.na(outdoor_temp), outdoor_temp < 12        # heating season
  ) |>
  mutate(
    cooling_rate = -temp_change,  # positive number = degrees lost per hour
    delta_t = temp - outdoor_temp  # indoor-outdoor gap
  ) |>
  filter(delta_t > 2, cooling_rate < 3)  # sanity: exclude glitches

if (nrow(cooling) > 30) {
  cooling_summary <- cooling |>
    group_by(room) |>
    summarize(
      median_rate = median(cooling_rate, na.rm = TRUE),
      p25_rate    = quantile(cooling_rate, 0.25, na.rm = TRUE),
      p75_rate    = quantile(cooling_rate, 0.75, na.rm = TRUE),
      mean_delta  = mean(delta_t, na.rm = TRUE),
      n = n(),
      .groups = "drop"
    ) |>
    filter(n >= 5) |>
    arrange(desc(median_rate))

  cat("\n=== Cooling Rates (HP off, heating season) ===\n")
  print(cooling_summary)

  p1 <- ggplot(cooling_summary, aes(x = reorder(room, median_rate), y = median_rate)) +
    geom_col(fill = COLORS$charge, alpha = 0.7, width = 0.6) +
    geom_errorbar(aes(ymin = p25_rate, ymax = p75_rate), width = 0.2, color = COLORS$import) +
    coord_flip() +
    labs(
      x     = "",
      y     = "Cooling Rate (°C/hour, HP off)",
      title = "Room Cooling Rates — Which Rooms Lose Heat Fastest?",
      subtitle = "Higher = worse insulation. Measured during HP-off periods in heating season.",
    ) +
    theme_energy()

  save_plot(p1, "21_cooling_rates.png")
} else {
  cat("Insufficient cooling period data (need HP off + heating season).\n")
  # Fallback: use all temp drops
  cooling_all <- room_changes |>
    filter(temp_change < 0, !is.na(outdoor_temp), outdoor_temp < 12) |>
    mutate(cooling_rate = -temp_change) |>
    filter(cooling_rate < 3)

  if (nrow(cooling_all) > 30) {
    cooling_summary <- cooling_all |>
      group_by(room) |>
      summarize(
        median_rate = median(cooling_rate, na.rm = TRUE),
        p25_rate    = quantile(cooling_rate, 0.25, na.rm = TRUE),
        p75_rate    = quantile(cooling_rate, 0.75, na.rm = TRUE),
        n = n(),
        .groups = "drop"
      ) |>
      filter(n >= 5) |>
      arrange(desc(median_rate))

    p1 <- ggplot(cooling_summary, aes(x = reorder(room, median_rate), y = median_rate)) +
      geom_col(fill = COLORS$charge, alpha = 0.7, width = 0.6) +
      geom_errorbar(aes(ymin = p25_rate, ymax = p75_rate), width = 0.2, color = COLORS$import) +
      coord_flip() +
      labs(
        x     = "",
        y     = "Cooling Rate (°C/hour)",
        title = "Room Cooling Rates — Which Rooms Lose Heat Fastest?",
        subtitle = "Higher = worse insulation. All temperature-drop hours in heating season.",
      ) +
      theme_energy()

    save_plot(p1, "21_cooling_rates.png")
  }
}

# ============================================================================
# Chart 2: Thermal inertia — normalized cooling rate per °C of delta-T
# ============================================================================
# Thermal inertia = how long the room takes to cool, normalized by the
# indoor-outdoor gap. Lower rate per degree = better insulation/thermal mass.

if (nrow(cooling) > 30) {
  # Normalize: cooling_rate / delta_t gives rate per degree of driving force
  inertia <- cooling |>
    filter(delta_t > 3) |>
    mutate(normalized_rate = cooling_rate / delta_t) |>
    filter(normalized_rate < 0.5)  # sanity

  inertia_summary <- inertia |>
    group_by(room) |>
    summarize(
      median_norm_rate = median(normalized_rate, na.rm = TRUE),
      # Time to lose 1°C at median rate (hours)
      hours_per_degree = 1 / median(cooling_rate, na.rm = TRUE),
      n = n(),
      .groups = "drop"
    ) |>
    filter(n >= 5) |>
    arrange(hours_per_degree)

  cat("\n=== Thermal Inertia (hours to lose 1°C) ===\n")
  print(inertia_summary)

  p2 <- ggplot(inertia_summary, aes(x = reorder(room, hours_per_degree), y = hours_per_degree)) +
    geom_col(fill = COLORS$export, alpha = 0.7, width = 0.6) +
    geom_text(aes(label = paste0(round(hours_per_degree, 1), "h")),
              hjust = -0.2, size = 3.5, color = COLORS$text) +
    coord_flip() +
    labs(
      x     = "",
      y     = "Hours to Lose 1°C (HP off)",
      title = "Thermal Inertia Ranking",
      subtitle = "Higher = room holds heat longer. Based on HP-off cooling periods."
    ) +
    theme_energy()

  save_plot(p2, "21_thermal_inertia.png")
}

# ============================================================================
# Chart 3: Night temperature drop — 22:00 to 06:00
# ============================================================================
# How much does each room cool overnight?
night_data <- room_data |>
  mutate(
    date = as.Date(hour_bucket),
    hour = hour(hour_bucket)
  )

# Evening snapshot (22:00) and morning snapshot (06:00 next day)
evening <- night_data |>
  filter(hour == 22) |>
  select(date, room, evening_temp = temp)

morning <- night_data |>
  filter(hour == 6) |>
  mutate(date = date - 1) |>  # match to the previous evening
  select(date, room, morning_temp = temp)

night_drop <- evening |>
  inner_join(morning, by = c("date", "room")) |>
  mutate(drop = evening_temp - morning_temp) |>
  filter(!is.na(drop), drop > -2, drop < 5)  # sanity bounds

# Add outdoor temp at evening for filtering to heating season
evening_outdoor <- outdoor |>
  mutate(date = as.Date(hour_bucket), hour = hour(hour_bucket)) |>
  filter(hour == 22) |>
  select(date, outdoor_evening = avg)

night_drop <- night_drop |>
  left_join(evening_outdoor, by = "date") |>
  filter(!is.na(outdoor_evening), outdoor_evening < 12)  # heating season only

if (nrow(night_drop) > 20) {
  night_summary <- night_drop |>
    group_by(room) |>
    summarize(
      median_drop = median(drop, na.rm = TRUE),
      p25_drop    = quantile(drop, 0.25, na.rm = TRUE),
      p75_drop    = quantile(drop, 0.75, na.rm = TRUE),
      n = n(),
      .groups = "drop"
    ) |>
    filter(n >= 3) |>
    arrange(desc(median_drop))

  cat("\n=== Night Temperature Drop (22:00 → 06:00) ===\n")
  print(night_summary)

  p3 <- ggplot(night_summary, aes(x = reorder(room, median_drop), y = median_drop)) +
    geom_col(fill = COLORS$discharge, alpha = 0.7, width = 0.6) +
    geom_errorbar(aes(ymin = p25_drop, ymax = p75_drop), width = 0.2, color = COLORS$import) +
    geom_text(aes(label = paste0(round(median_drop, 2), "°C")),
              hjust = -0.2, size = 3.5, color = COLORS$text) +
    coord_flip() +
    labs(
      x     = "",
      y     = "Temperature Drop (°C, 22:00 → 06:00)",
      title = "Overnight Temperature Drop by Room",
      subtitle = "Heating season only. Larger drop = more heat loss through walls/windows."
    ) +
    theme_energy()

  save_plot(p3, "21_night_drop.png")
} else {
  cat("Insufficient overnight data for night drop analysis.\n")
}

# ============================================================================
# Chart 4: Temperature uniformity — inter-room spread over time
# ============================================================================
# For each hour, compute the spread (max - min) across all rooms.
# Wide spread = some rooms too hot, others too cold = poor heat distribution.
uniformity <- room_data |>
  group_by(hour_bucket) |>
  summarize(
    n_rooms   = n_distinct(room),
    max_temp  = max(temp, na.rm = TRUE),
    min_temp  = min(temp, na.rm = TRUE),
    spread    = max_temp - min_temp,
    mean_temp = mean(temp, na.rm = TRUE),
    sd_temp   = sd(temp, na.rm = TRUE),
    .groups   = "drop"
  ) |>
  filter(n_rooms >= 3) |>
  left_join(outdoor |> select(hour_bucket, outdoor_temp = avg), by = "hour_bucket") |>
  mutate(hour = hour(hour_bucket))

if (nrow(uniformity) > 50) {
  # Hourly profile of inter-room spread
  spread_by_hour <- uniformity |>
    group_by(hour) |>
    summarize(
      median_spread = median(spread, na.rm = TRUE),
      p25_spread    = quantile(spread, 0.25, na.rm = TRUE),
      p75_spread    = quantile(spread, 0.75, na.rm = TRUE),
      .groups = "drop"
    )

  cat("\n=== Temperature Uniformity ===\n")
  cat("  Mean inter-room spread:", round(mean(uniformity$spread, na.rm = TRUE), 1), "°C\n")
  cat("  Median spread:         ", round(median(uniformity$spread, na.rm = TRUE), 1), "°C\n")

  p4 <- ggplot(spread_by_hour, aes(x = hour, y = median_spread)) +
    geom_ribbon(aes(ymin = p25_spread, ymax = p75_spread),
                fill = COLORS$charge, alpha = 0.3) +
    geom_line(color = COLORS$charge, linewidth = 1.2) +
    geom_point(color = COLORS$charge, size = 2) +
    scale_x_continuous(breaks = seq(0, 23, 3)) +
    labs(
      x     = "Hour of Day",
      y     = "Inter-Room Temperature Spread (°C)",
      title = "Temperature Uniformity Across Rooms",
      subtitle = "Lower = more uniform heating. Band = IQR. Spikes = heating distribution problems."
    ) +
    theme_energy()

  save_plot(p4, "21_uniformity.png")
} else {
  cat("Insufficient multi-room data for uniformity analysis.\n")
}
