# ============================================================================
# 28_ventilation.R — Ventilation Recommendation
# ============================================================================
# WHAT:    Compares indoor vs outdoor absolute humidity to identify when
#          opening windows would lower indoor moisture. Ventilation is
#          beneficial when outdoor absolute humidity (AH) is lower than
#          indoor AH — dry outside air replaces moist inside air.
#
# INPUTS:  load_stats_sensor() for all indoor TEMP_*/HUM_* pairs,
#          NETATMO_OUTDOOR_TEMP + NETATMO_OUTDOOR_HUM for outdoor
#          (falls back to HP_OUTSIDE_TEMP if Netatmo sparse, but then
#           humidity analysis is skipped)
#
# OUTPUTS: output/28_ventilation_daily.png    — indoor vs outdoor AH by hour
#          output/28_ventilation_heatmap.png  — month x hour ventilation window
#          output/28_ventilation_seasonal.png — avg hours/day helpful by season
#
# HOW TO READ:
#   - Daily chart: green zone = outdoor AH < indoor AH (ventilate now).
#     Wider green zone = more hours per day where windows help.
#   - Heatmap: darker cells = higher fraction of time ventilation helps.
#     Summer mornings are often ideal; winter nights rarely help.
#   - Seasonal bars: taller = more ventilation-friendly hours in that season.
# ============================================================================

source("analysis/r/R/load_data.R")

# ============================================================================
# Physics: Absolute humidity from temperature and RH
# ============================================================================
# Absolute humidity (g/m3) tells you the actual water content in air,
# independent of temperature. Two air masses with different temps can have
# the same RH but very different AH. AH comparison directly tells you
# whether exchanging air will add or remove moisture.
absolute_humidity <- function(temp, rh) {
  # Saturation vapor pressure (Pa) — Magnus approximation
  psat <- 610.78 * exp(17.27 * temp / (237.3 + temp))
  # Actual vapor pressure
  pv <- psat * rh / 100
  # Absolute humidity (g/m3)
  216.7 * pv / (temp + 273.15)
}

# ============================================================================
# Load outdoor temperature + humidity
# ============================================================================
outdoor_temp <- load_stats_sensor(NETATMO_OUTDOOR_TEMP) |>
  distinct(hour_bucket, .keep_all = TRUE) |>
  select(hour_bucket, outdoor_temp = avg)

outdoor_hum <- load_stats_sensor(NETATMO_OUTDOOR_HUM) |>
  distinct(hour_bucket, .keep_all = TRUE) |>
  select(hour_bucket, outdoor_rh = avg)

cat("\n=== Outdoor Data ===\n")
cat("  Netatmo outdoor temp hours:", nrow(outdoor_temp), "\n")
cat("  Netatmo outdoor hum hours: ", nrow(outdoor_hum), "\n")

# Check if we have outdoor humidity — required for this analysis
has_outdoor_hum <- nrow(outdoor_hum) >= 20

if (!has_outdoor_hum) {
  cat("No outdoor humidity data available (Netatmo outdoor humidity sensor).\n")
  cat("Cannot compute absolute humidity comparison — skipping ventilation analysis.\n")

  # Try HP outside temp for reference
  hp_outdoor <- load_stats_sensor(HP_OUTSIDE_TEMP) |>
    distinct(hour_bucket, .keep_all = TRUE)
  if (nrow(hp_outdoor) >= 20) {
    cat("HP_OUTSIDE_TEMP available (", nrow(hp_outdoor),
        " hours) but lacks humidity — analysis requires outdoor RH.\n")
  }
  quit(save = "no")
}

outdoor <- inner_join(outdoor_temp, outdoor_hum, by = "hour_bucket") |>
  filter(!is.na(outdoor_temp), !is.na(outdoor_rh),
         outdoor_rh > 0, outdoor_rh <= 100) |>
  mutate(outdoor_ah = absolute_humidity(outdoor_temp, outdoor_rh))

cat("  Outdoor combined hours:    ", nrow(outdoor), "\n")

if (nrow(outdoor) < 20) {
  cat("Insufficient outdoor temperature+humidity overlap.\n")
  quit(save = "no")
}

# ============================================================================
# Load indoor temperature + humidity (average across all rooms)
# ============================================================================
room_pairs <- list(
  "Bedroom 1" = list(temp = TEMP_BEDROOM1, hum = HUM_BEDROOM1),
  "Bedroom 2" = list(temp = TEMP_BEDROOM2, hum = HUM_BEDROOM2),
  "Kitchen"   = list(temp = TEMP_KITCHEN,   hum = HUM_KITCHEN),
  "Office 1"  = list(temp = TEMP_OFFICE1,   hum = HUM_OFFICE1),
  "Office 2"  = list(temp = TEMP_OFFICE2,   hum = HUM_OFFICE2),
  "Bathroom"  = list(temp = TEMP_BATHROOM,  hum = HUM_BATHROOM),
  "Workshop"  = list(temp = TEMP_WORKSHOP,  hum = HUM_WORKSHOP)
)

indoor_data <- map2(names(room_pairs), room_pairs, function(name, sensors) {
  temp_df <- load_stats_sensor(sensors$temp) |>
    distinct(hour_bucket, .keep_all = TRUE) |>
    select(hour_bucket, temp = avg)

  hum_df <- load_stats_sensor(sensors$hum) |>
    distinct(hour_bucket, .keep_all = TRUE) |>
    select(hour_bucket, rh = avg)

  if (nrow(temp_df) < 20 || nrow(hum_df) < 20) {
    cat("  Skipping", name, "— insufficient data\n")
    return(tibble())
  }

  inner_join(temp_df, hum_df, by = "hour_bucket") |>
    filter(!is.na(temp), !is.na(rh), rh > 0, rh <= 100) |>
    mutate(room = name)
}) |> bind_rows()

if (nrow(indoor_data) < 20) {
  cat("Insufficient indoor climate data for ventilation analysis.\n")
  quit(save = "no")
}

# Average indoor temp + humidity across all rooms per hour
indoor_avg <- indoor_data |>
  group_by(hour_bucket) |>
  summarize(
    indoor_temp = mean(temp, na.rm = TRUE),
    indoor_rh = mean(rh, na.rm = TRUE),
    n_rooms = n(),
    .groups = "drop"
  ) |>
  filter(n_rooms >= 2) |>  # require at least 2 rooms for a meaningful average
  mutate(indoor_ah = absolute_humidity(indoor_temp, indoor_rh))

cat("  Indoor averaged hours:     ", nrow(indoor_avg), "\n")

# ============================================================================
# Join indoor and outdoor data
# ============================================================================
combined <- inner_join(indoor_avg, outdoor, by = "hour_bucket") |>
  filter(!is.na(indoor_ah), !is.na(outdoor_ah)) |>
  mutate(
    ventilate = outdoor_ah < indoor_ah,
    ah_diff = indoor_ah - outdoor_ah,  # positive = ventilation helps
    hour = hour(hour_bucket),
    month = month(hour_bucket, label = TRUE),
    season = case_when(
      month(hour_bucket) %in% c(12, 1, 2)  ~ "Winter",
      month(hour_bucket) %in% c(3, 4, 5)   ~ "Spring",
      month(hour_bucket) %in% c(6, 7, 8)   ~ "Summer",
      TRUE                                  ~ "Autumn"
    ),
    season = factor(season, levels = c("Spring", "Summer", "Autumn", "Winter"))
  )

cat("  Combined indoor+outdoor:   ", nrow(combined), "hours\n")
cat("  Hours ventilation helps:   ", sum(combined$ventilate),
    "(", round(mean(combined$ventilate) * 100, 1), "%)\n")

if (nrow(combined) < 20) {
  cat("Insufficient overlap between indoor and outdoor data.\n")
  quit(save = "no")
}

# ============================================================================
# Chart 1: Absolute humidity comparison — indoor vs outdoor by hour of day
# ============================================================================
hourly_ah <- combined |>
  group_by(hour) |>
  summarize(
    indoor_ah = mean(indoor_ah, na.rm = TRUE),
    outdoor_ah = mean(outdoor_ah, na.rm = TRUE),
    .groups = "drop"
  )

# Prepare data for the "ventilate" zone shading
hourly_ah <- hourly_ah |>
  mutate(
    zone_min = pmin(indoor_ah, outdoor_ah),
    zone_max = pmax(indoor_ah, outdoor_ah),
    beneficial = outdoor_ah < indoor_ah
  )

p1 <- ggplot(hourly_ah, aes(x = hour)) +
  # Shade zone where ventilation helps (outdoor < indoor)
  geom_ribbon(
    data = hourly_ah |> filter(beneficial),
    aes(ymin = outdoor_ah, ymax = indoor_ah),
    fill = COLORS$export, alpha = 0.25
  ) +
  # Shade zone where ventilation hurts (outdoor > indoor)
  geom_ribbon(
    data = hourly_ah |> filter(!beneficial),
    aes(ymin = indoor_ah, ymax = outdoor_ah),
    fill = COLORS$import, alpha = 0.15
  ) +
  # Lines
  geom_line(aes(y = indoor_ah, color = "Indoor"), linewidth = 1.2) +
  geom_line(aes(y = outdoor_ah, color = "Outdoor"), linewidth = 1.2) +
  geom_point(aes(y = indoor_ah, color = "Indoor"), size = 2) +
  geom_point(aes(y = outdoor_ah, color = "Outdoor"), size = 2) +
  # Annotation
  annotate("text",
    x = hourly_ah$hour[which.max(hourly_ah$indoor_ah - hourly_ah$outdoor_ah)],
    y = max(hourly_ah$indoor_ah) + 0.3,
    label = "Ventilate\n(outdoor drier)", color = COLORS$export,
    size = 3.5, fontface = "bold"
  ) +
  scale_color_manual(
    values = c("Indoor" = COLORS$heat_pump, "Outdoor" = COLORS$charge),
    name = ""
  ) +
  scale_x_continuous(breaks = seq(0, 23, 3)) +
  labs(
    x     = "Hour of Day",
    y     = "Absolute Humidity (g/m\u00b3)",
    title = "Ventilation Guide: Indoor vs Outdoor Absolute Humidity",
    subtitle = paste0(
      "Green zone = outdoor air drier than indoor (ventilate). ",
      "Average across all rooms and ", length(unique(combined$month)), " months."
    )
  ) +
  theme_energy()

save_plot(p1, "28_ventilation_daily.png")

# ============================================================================
# Chart 2: Monthly ventilation window — heatmap: month x hour
# ============================================================================
monthly_hourly <- combined |>
  group_by(month, hour) |>
  summarize(
    pct_beneficial = mean(ventilate, na.rm = TRUE) * 100,
    n_hours = n(),
    .groups = "drop"
  ) |>
  filter(n_hours >= 5)  # require some data per cell

p2 <- ggplot(monthly_hourly, aes(x = hour, y = month, fill = pct_beneficial)) +
  geom_tile(color = "white", linewidth = 0.4) +
  geom_text(aes(label = sprintf("%.0f", pct_beneficial)),
            size = 2.8,
            color = ifelse(monthly_hourly$pct_beneficial > 60, "white", COLORS$text)) +
  scale_fill_gradient(
    low = COLORS$bg, high = COLORS$export,
    name = "% time ventilation helps",
    limits = c(0, 100)
  ) +
  scale_x_continuous(breaks = seq(0, 23, 3)) +
  labs(
    x     = "Hour of Day",
    y     = "",
    title = "Ventilation Window: When Does Opening Windows Help?",
    subtitle = "% of hours when outdoor absolute humidity < indoor. Darker = better ventilation window."
  ) +
  theme_energy() +
  theme(panel.grid = element_blank())

save_plot(p2, "28_ventilation_heatmap.png", width = 12, height = 6)

# ============================================================================
# Chart 3: Seasonal strategy — avg hours per day when ventilation helps
# ============================================================================
seasonal_hours <- combined |>
  mutate(date = as.Date(hour_bucket)) |>
  group_by(season, date) |>
  summarize(
    hours_beneficial = sum(ventilate, na.rm = TRUE),
    .groups = "drop"
  ) |>
  group_by(season) |>
  summarize(
    avg_hours = mean(hours_beneficial, na.rm = TRUE),
    sd_hours = sd(hours_beneficial, na.rm = TRUE),
    n_days = n(),
    .groups = "drop"
  ) |>
  filter(n_days >= 5)

p3 <- ggplot(seasonal_hours, aes(x = season, y = avg_hours, fill = season)) +
  geom_col(alpha = 0.75, width = 0.6) +
  geom_errorbar(
    aes(ymin = pmax(avg_hours - sd_hours, 0), ymax = avg_hours + sd_hours),
    width = 0.2, color = COLORS$text, alpha = 0.6
  ) +
  geom_text(aes(label = sprintf("%.1f h", avg_hours)),
            vjust = -0.8, color = COLORS$text, size = 4) +
  scale_fill_manual(values = SEASON_COLORS) +
  scale_y_continuous(expand = expansion(mult = c(0, 0.15))) +
  labs(
    x     = "",
    y     = "Avg Hours per Day",
    title = "Seasonal Ventilation Strategy",
    subtitle = "Average hours per day when opening windows lowers indoor moisture. Error bars = 1 SD."
  ) +
  theme_energy() +
  theme(legend.position = "none")

save_plot(p3, "28_ventilation_seasonal.png")

cat("\n=== Ventilation Analysis Complete ===\n")
