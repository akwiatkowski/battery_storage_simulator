# ============================================================================
# 19_workshop_thermal.R — Workshop Thermal Response Analysis
# ============================================================================
# WHAT:    Analyzes how outdoor temperature and humidity affect workshop
#          indoor temperature. Quantifies thermal coupling, lag, and the
#          independent effect of humidity on heat loss.
#
# INPUTS:  load_stats_sensor() for Netatmo outdoor temp/humidity,
#          workshop indoor temp/humidity, and workshop exterior temp.
#          Falls back to HP outside temp if Netatmo outdoor data is sparse.
#
# OUTPUTS: output/19_workshop_timeseries.png  — overlaid outdoor/indoor temps
#          output/19_workshop_scatter.png     — outdoor vs indoor, colored by humidity
#          output/19_workshop_lag.png         — cross-correlation lag analysis
#          output/19_workshop_humidity.png    — humidity effect on thermal response
#
# HOW TO READ:
#   - Time series: indoor follows outdoor with visible lag and damping
#   - Scatter: tighter cluster = stronger coupling, slope < 1 = thermal damping
#   - Lag: peak correlation at N hours = typical thermal response time
#   - Humidity: steeper slopes at high humidity = faster heat loss through walls
# ============================================================================

source("analysis/helpers/load_data.R")

# ============================================================================
# Load outdoor and indoor data
# ============================================================================
# Primary outdoor: Netatmo outdoor module
outdoor_temp <- load_stats_sensor(NETATMO_OUTDOOR_TEMP)
outdoor_hum  <- load_stats_sensor(NETATMO_OUTDOOR_HUM)

# Fallback outdoor: HP outside temp sensor (usually has more data)
hp_outside <- load_stats_sensor(HP_OUTSIDE_TEMP)

# Workshop indoor
workshop_temp <- load_stats_sensor(TEMP_WORKSHOP)
workshop_hum  <- load_stats_sensor(HUM_WORKSHOP)

# Workshop exterior sensor (mounted on workshop wall, outside)
workshop_ext  <- load_stats_sensor(TEMP_WORKSHOP_EXT)

cat("\n=== Workshop Thermal Data ===\n")
cat("  Netatmo outdoor temp: ", nrow(outdoor_temp), "hours\n")
cat("  Netatmo outdoor hum:  ", nrow(outdoor_hum), "hours\n")
cat("  HP outside temp:      ", nrow(hp_outside), "hours\n")
cat("  Workshop indoor temp: ", nrow(workshop_temp), "hours\n")
cat("  Workshop indoor hum:  ", nrow(workshop_hum), "hours\n")
cat("  Workshop ext temp:    ", nrow(workshop_ext), "hours\n")

# Use Netatmo outdoor if available, otherwise fall back to HP sensor
if (nrow(outdoor_temp) >= 20) {
  ext_temp <- outdoor_temp |> select(hour_bucket, ext_temp = avg)
  ext_source <- "Netatmo Outdoor"
} else if (nrow(hp_outside) >= 20) {
  ext_temp <- hp_outside |> select(hour_bucket, ext_temp = avg)
  ext_source <- "HP Outside"
} else {
  cat("Insufficient outdoor temperature data.\n")
  quit(save = "no")
}

if (nrow(workshop_temp) < 20) {
  cat("Insufficient workshop indoor temperature data.\n")
  quit(save = "no")
}

# Join all available data by hour
combined <- ext_temp |>
  inner_join(
    workshop_temp |> select(hour_bucket, indoor_temp = avg),
    by = "hour_bucket"
  ) |>
  filter(!is.na(ext_temp), !is.na(indoor_temp))

# Add outdoor humidity if available
if (nrow(outdoor_hum) >= 10) {
  combined <- combined |>
    left_join(
      outdoor_hum |> select(hour_bucket, ext_hum = avg),
      by = "hour_bucket"
    )
} else {
  combined$ext_hum <- NA_real_
}

# Add workshop exterior temp if available
if (nrow(workshop_ext) >= 10) {
  combined <- combined |>
    left_join(
      workshop_ext |> select(hour_bucket, wall_temp = avg),
      by = "hour_bucket"
    )
} else {
  combined$wall_temp <- NA_real_
}

combined <- combined |>
  mutate(
    hour = hour(hour_bucket),
    delta_t = indoor_temp - ext_temp
  )

cat("\n=== Combined dataset ===\n")
cat("  Joined hours:  ", nrow(combined), "\n")
cat("  Outdoor range: ", round(min(combined$ext_temp, na.rm = TRUE), 1),
    "to", round(max(combined$ext_temp, na.rm = TRUE), 1), "°C\n")
cat("  Indoor range:  ", round(min(combined$indoor_temp, na.rm = TRUE), 1),
    "to", round(max(combined$indoor_temp, na.rm = TRUE), 1), "°C\n")
cat("  Mean ΔT:       ", round(mean(combined$delta_t, na.rm = TRUE), 1), "°C\n")

# ============================================================================
# Chart 1: Time series overlay — outdoor, indoor, and wall temps
# ============================================================================
ts_data <- combined |>
  select(hour_bucket, ext_temp, indoor_temp, wall_temp) |>
  pivot_longer(
    cols = c(ext_temp, indoor_temp, wall_temp),
    names_to = "sensor",
    values_to = "temp"
  ) |>
  filter(!is.na(temp)) |>
  mutate(sensor = case_when(
    sensor == "ext_temp"    ~ paste0("Outdoor (", ext_source, ")"),
    sensor == "indoor_temp" ~ "Workshop Indoor",
    sensor == "wall_temp"   ~ "Workshop Exterior Wall"
  ))

color_map <- c(
  "Workshop Indoor"        = COLORS$charge,
  "Workshop Exterior Wall"  = COLORS$prediction
)
color_map[paste0("Outdoor (", ext_source, ")")] <- COLORS$import

p1 <- ggplot(ts_data, aes(x = hour_bucket, y = temp, color = sensor)) +
  geom_line(linewidth = 0.6, alpha = 0.8) +
  scale_color_manual(values = color_map) +
  labs(
    x     = "",
    y     = "Temperature (°C)",
    title = "Workshop Thermal Response — Time Series",
    subtitle = "Indoor temp follows outdoor with damping and lag",
    color = ""
  ) +
  theme_energy() +
  theme(legend.position = "bottom")

save_plot(p1, "19_workshop_timeseries.png")

# ============================================================================
# Chart 2: Scatter — outdoor temp vs indoor temp, colored by humidity
# ============================================================================
if (sum(!is.na(combined$ext_hum)) > 10) {
  # Has humidity data — use it for coloring
  scatter_data <- combined |> filter(!is.na(ext_hum))

  # Fit linear model
  model <- lm(indoor_temp ~ ext_temp, data = scatter_data)
  r_sq <- summary(model)$r.squared
  slope <- coef(model)[2]
  intercept <- coef(model)[1]

  cat("\n=== Linear Model: indoor ~ outdoor ===\n")
  cat("  R² =", round(r_sq, 3), "\n")
  cat("  Slope =", round(slope, 3), " (1.0 = perfect coupling)\n")
  cat("  Intercept =", round(intercept, 1), "°C (thermal offset)\n")

  p2 <- ggplot(scatter_data, aes(x = ext_temp, y = indoor_temp, color = ext_hum)) +
    geom_point(alpha = 0.5, size = 1.5) +
    scale_color_viridis_c(option = "plasma", name = "Outdoor\nHumidity %") +
    geom_smooth(method = "lm", color = COLORS$export, linewidth = 1, se = TRUE) +
    annotate("text",
      x = min(scatter_data$ext_temp) + 1,
      y = max(scatter_data$indoor_temp) - 0.5,
      label = paste0("R² = ", round(r_sq, 3),
                     "\nSlope = ", round(slope, 3),
                     "\nOffset = ", round(intercept, 1), "°C"),
      hjust = 0, size = 3, color = COLORS$muted
    ) +
    labs(
      x     = paste0("Outdoor Temperature (°C) — ", ext_source),
      y     = "Workshop Indoor Temperature (°C)",
      title = "Thermal Coupling: Outdoor vs Indoor",
      subtitle = "Slope < 1 = thermal damping by building envelope. Color = humidity."
    ) +
    theme_energy()

  save_plot(p2, "19_workshop_scatter.png")
} else {
  # No humidity — plain scatter
  model <- lm(indoor_temp ~ ext_temp, data = combined)
  r_sq <- summary(model)$r.squared
  slope <- coef(model)[2]
  intercept <- coef(model)[1]

  cat("\n=== Linear Model: indoor ~ outdoor ===\n")
  cat("  R² =", round(r_sq, 3), "\n")
  cat("  Slope =", round(slope, 3), "\n")
  cat("  Intercept =", round(intercept, 1), "°C\n")

  p2 <- ggplot(combined, aes(x = ext_temp, y = indoor_temp)) +
    geom_bin2d(bins = 30) +
    scale_fill_viridis_c(option = "plasma", trans = "log10") +
    geom_smooth(method = "lm", color = COLORS$export, linewidth = 1, se = TRUE) +
    annotate("text",
      x = min(combined$ext_temp) + 1,
      y = max(combined$indoor_temp) - 0.5,
      label = paste0("R² = ", round(r_sq, 3),
                     "\nSlope = ", round(slope, 3),
                     "\nOffset = ", round(intercept, 1), "°C"),
      hjust = 0, size = 3, color = COLORS$muted
    ) +
    labs(
      x     = paste0("Outdoor Temperature (°C) — ", ext_source),
      y     = "Workshop Indoor Temperature (°C)",
      title = "Thermal Coupling: Outdoor vs Indoor",
      subtitle = "Slope < 1 = thermal damping by building envelope",
      fill  = "Count"
    ) +
    theme_energy()

  save_plot(p2, "19_workshop_scatter.png")
}

# ============================================================================
# Chart 3: Cross-correlation — thermal lag analysis
# ============================================================================
# How many hours does the workshop indoor temp lag behind outdoor changes?
# Use CCF (cross-correlation function) between outdoor and indoor temps.
if (nrow(combined) >= 24) {
  # Ensure data is sorted and evenly spaced (hourly)
  ts_sorted <- combined |> arrange(hour_bucket)

  # Compute rate of change (first difference) to focus on dynamics not levels
  ts_sorted <- ts_sorted |>
    mutate(
      ext_change  = ext_temp - lag(ext_temp),
      in_change   = indoor_temp - lag(indoor_temp)
    ) |>
    filter(!is.na(ext_change), !is.na(in_change))

  if (nrow(ts_sorted) >= 20) {
    # CCF: positive lag = indoor lags behind outdoor
    max_lag <- min(24, nrow(ts_sorted) %/% 4)
    ccf_result <- ccf(ts_sorted$ext_change, ts_sorted$in_change,
                      lag.max = max_lag, plot = FALSE)

    ccf_df <- tibble(
      lag_hours = as.numeric(ccf_result$lag),
      correlation = as.numeric(ccf_result$acf)
    )

    # Find peak positive lag (indoor responding to outdoor)
    peak <- ccf_df |>
      filter(lag_hours >= 0) |>
      slice_max(correlation, n = 1)

    cat("\n=== Cross-Correlation ===\n")
    cat("  Peak lag:", peak$lag_hours, "hours\n")
    cat("  Peak r:  ", round(peak$correlation, 3), "\n")

    p3 <- ggplot(ccf_df, aes(x = lag_hours, y = correlation)) +
      geom_col(aes(fill = lag_hours >= 0), width = 0.8, alpha = 0.7, show.legend = FALSE) +
      scale_fill_manual(values = c("TRUE" = COLORS$charge, "FALSE" = COLORS$muted)) +
      geom_vline(xintercept = peak$lag_hours, linetype = "dashed", color = COLORS$import) +
      annotate("text",
        x = peak$lag_hours + 0.5, y = max(ccf_df$correlation) * 0.9,
        label = paste0("Peak at ", peak$lag_hours, "h\nr = ",
                       round(peak$correlation, 3)),
        hjust = 0, size = 3, color = COLORS$import
      ) +
      scale_x_continuous(breaks = seq(-max_lag, max_lag, by = 2)) +
      labs(
        x     = "Lag (hours, positive = indoor lags outdoor)",
        y     = "Cross-Correlation (hourly changes)",
        title = "Thermal Lag — How Fast Does the Workshop Respond?",
        subtitle = "Peak at positive lag = hours for outdoor change to reach indoor"
      ) +
      theme_energy()

    save_plot(p3, "19_workshop_lag.png")
  } else {
    cat("Insufficient data for lag analysis.\n")
  }
} else {
  cat("Insufficient data for lag analysis (need >= 24 hours).\n")
}

# ============================================================================
# Chart 4: Humidity effect on thermal response
# ============================================================================
# Does higher outdoor humidity cause faster heat loss (lower indoor temp
# at the same outdoor temp)?
# Approach: bin by humidity quartile, fit indoor ~ outdoor per bin
hum_data <- combined |> filter(!is.na(ext_hum), ext_hum > 0)

if (nrow(hum_data) >= 30) {
  # Create humidity bins
  hum_data <- hum_data |>
    mutate(
      hum_bin = cut(ext_hum,
        breaks = quantile(ext_hum, probs = c(0, 0.33, 0.67, 1), na.rm = TRUE),
        labels = c("Low", "Medium", "High"),
        include.lowest = TRUE
      )
    ) |>
    filter(!is.na(hum_bin))

  # Summary per bin
  hum_summary <- hum_data |>
    group_by(hum_bin) |>
    summarize(
      n = n(),
      avg_hum = mean(ext_hum, na.rm = TRUE),
      avg_delta_t = mean(delta_t, na.rm = TRUE),
      avg_indoor = mean(indoor_temp, na.rm = TRUE),
      .groups = "drop"
    )

  cat("\n=== Humidity Effect ===\n")
  print(hum_summary)

  # Fit model: indoor ~ outdoor_temp + humidity
  model_hum <- lm(indoor_temp ~ ext_temp + ext_hum, data = hum_data)
  cat("\n=== Model: indoor ~ outdoor + humidity ===\n")
  print(summary(model_hum)$coefficients)
  hum_coef <- coef(model_hum)["ext_hum"]
  cat("\nHumidity coefficient:", round(hum_coef, 4),
      "°C per 1% humidity\n")
  cat("Interpretation: +10% humidity →",
      round(hum_coef * 10, 2), "°C indoor temp change\n")

  p4 <- ggplot(hum_data, aes(x = ext_temp, y = indoor_temp, color = hum_bin)) +
    geom_point(alpha = 0.4, size = 1.2) +
    geom_smooth(method = "lm", se = FALSE, linewidth = 1) +
    scale_color_manual(
      values = c(
        "Low"    = COLORS$pv,
        "Medium" = COLORS$charge,
        "High"   = COLORS$prediction
      ),
      labels = c(
        "Low"    = paste0("Low (<", round(quantile(hum_data$ext_hum, 0.33)), "%)"),
        "Medium" = paste0("Medium"),
        "High"   = paste0("High (>", round(quantile(hum_data$ext_hum, 0.67)), "%)")
      )
    ) +
    annotate("text",
      x = min(hum_data$ext_temp) + 0.5,
      y = max(hum_data$indoor_temp) - 0.3,
      label = paste0("Humidity effect: ", round(hum_coef, 4),
                     " °C per 1% RH\n+10% RH → ",
                     round(hum_coef * 10, 2), "°C"),
      hjust = 0, size = 2.8, color = COLORS$muted
    ) +
    labs(
      x     = "Outdoor Temperature (°C)",
      y     = "Workshop Indoor Temperature (°C)",
      title = "Humidity Effect on Workshop Temperature",
      subtitle = "Regression lines per humidity tercile — steeper at high humidity = faster heat loss",
      color = "Outdoor Humidity"
    ) +
    theme_energy() +
    theme(legend.position = "bottom")

  save_plot(p4, "19_workshop_humidity.png")
} else {
  cat("Insufficient humidity data for humidity effect analysis.\n")
  cat("(need >=30 joined hours with outdoor humidity, have", nrow(hum_data), ")\n")
}
