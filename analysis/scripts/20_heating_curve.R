# ============================================================================
# 20_heating_curve.R — Heating Curve Audit
# ============================================================================
# WHAT:    Analyzes the heat pump's weather-compensating heating curve.
#          The HP uses outdoor temperature to decide water supply temperature.
#          A wrong curve wastes energy (too steep) or underheats (too flat).
#
# INPUTS:  load_stats_sensor() for HP outside temp, Z1 target temp,
#          outlet temp, inlet temp, COP, and indoor room temperatures
#
# OUTPUTS: output/20_heating_curve.png       — actual curve: target water temp vs outdoor
#          output/20_curve_vs_comfort.png    — does the curve deliver indoor comfort?
#          output/20_overheating_map.png     — hour × outdoor temp heatmap of overshoot
#          output/20_curve_efficiency.png    — COP along the heating curve
#
# HOW TO READ:
#   - Heating curve: the line shows HP's configured outdoor→water temp mapping.
#     Steeper = more aggressive heating in cold weather.
#   - Comfort check: if indoor temps drop below target at cold outdoor temps,
#     the curve is too flat. If rooms overshoot, the curve is too steep.
#   - Overheating map: red cells = rooms warmer than target = wasted energy.
#   - COP along curve: shows efficiency cost of high water temps.
# ============================================================================

source("analysis/helpers/load_data.R")

# ============================================================================
# Load HP data
# ============================================================================
outdoor_temp  <- load_stats_sensor(HP_OUTSIDE_TEMP)
z1_target     <- load_stats_sensor(HP_Z1_TARGET_TEMP)
outlet_temp   <- load_stats_sensor(HP_OUTLET_TEMP)
inlet_temp    <- load_stats_sensor(HP_INLET_TEMP)
cop_sensor    <- load_stats_sensor(HP_COP_SENSOR)
hp_cons       <- load_stats_sensor(HP_CONSUMPTION)

cat("\n=== Heating Curve Data ===\n")
cat("  Outdoor temp:    ", nrow(outdoor_temp), "hours\n")
cat("  Z1 target temp:  ", nrow(z1_target), "hours\n")
cat("  Outlet temp:     ", nrow(outlet_temp), "hours\n")
cat("  COP sensor:      ", nrow(cop_sensor), "hours\n")

if (nrow(outdoor_temp) < 50 || nrow(z1_target) < 50) {
  cat("Insufficient data for heating curve analysis.\n")
  quit(save = "no")
}

# ============================================================================
# Build the combined heating curve dataset
# ============================================================================
# Deduplicate each sensor (stats CSVs can have overlapping exports)
outdoor_d  <- outdoor_temp |> select(hour_bucket, outdoor = avg) |> distinct(hour_bucket, .keep_all = TRUE)
target_d   <- z1_target |> select(hour_bucket, target_water = avg) |> distinct(hour_bucket, .keep_all = TRUE)
outlet_d   <- outlet_temp |> select(hour_bucket, actual_water = avg) |> distinct(hour_bucket, .keep_all = TRUE)
inlet_d    <- inlet_temp |> select(hour_bucket, return_water = avg) |> distinct(hour_bucket, .keep_all = TRUE)
cop_d      <- cop_sensor |> select(hour_bucket, cop = avg) |> distinct(hour_bucket, .keep_all = TRUE)
cons_d     <- hp_cons |> select(hour_bucket, consumption = avg) |> distinct(hour_bucket, .keep_all = TRUE)

curve_data <- outdoor_d |>
  inner_join(target_d, by = "hour_bucket") |>
  left_join(outlet_d, by = "hour_bucket") |>
  left_join(inlet_d, by = "hour_bucket") |>
  left_join(cop_d, by = "hour_bucket") |>
  left_join(cons_d, by = "hour_bucket") |>
  filter(!is.na(outdoor), !is.na(target_water)) |>
  mutate(
    hour = hour(hour_bucket),
    # Only heating season: outdoor < 15°C and target water > 20°C
    is_heating = outdoor < 15 & target_water > 20
  )

heating <- curve_data |> filter(is_heating)

cat("\n=== Heating Curve Dataset ===\n")
cat("  Total hours:    ", nrow(curve_data), "\n")
cat("  Heating hours:  ", nrow(heating), "\n")
cat("  Outdoor range:  ", round(min(heating$outdoor, na.rm = TRUE), 1),
    "to", round(max(heating$outdoor, na.rm = TRUE), 1), "°C\n")
cat("  Target water:   ", round(min(heating$target_water, na.rm = TRUE), 1),
    "to", round(max(heating$target_water, na.rm = TRUE), 1), "°C\n")

# ============================================================================
# Chart 1: The actual heating curve — target water temp vs outdoor temp
# ============================================================================
# This is the most important chart: what curve does the HP controller use?
# Each point is one hour; the trend line reveals the configured curve shape.

p1 <- ggplot(heating, aes(x = outdoor, y = target_water)) +
  geom_bin2d(bins = 40) +
  scale_fill_viridis_c(option = "plasma", trans = "log10") +
  geom_smooth(method = "loess", color = COLORS$import, linewidth = 1.2, se = TRUE) +
  # Add the actual outlet temp as a second trend
  {if (sum(!is.na(heating$actual_water)) > 50)
    geom_smooth(aes(y = actual_water), method = "loess",
                color = COLORS$charge, linewidth = 1, linetype = "dashed", se = FALSE)
  } +
  labs(
    x     = "Outdoor Temperature (°C)",
    y     = "Water Temperature (°C)",
    title = "Heating Curve: Water Target vs Outdoor Temperature",
    subtitle = "Solid red = target setpoint, dashed blue = actual outlet. Steeper = more aggressive.",
    fill  = "Hours"
  ) +
  theme_energy()

# Annotate with curve slope
curve_model <- lm(target_water ~ outdoor, data = heating)
slope <- round(coef(curve_model)[2], 2)
intercept <- round(coef(curve_model)[1], 1)

p1 <- p1 +
  annotate("text",
    x = max(heating$outdoor, na.rm = TRUE) - 1,
    y = max(heating$target_water, na.rm = TRUE) - 0.5,
    label = paste0("Slope = ", slope, " °C/°C\nIntercept = ", intercept, "°C"),
    hjust = 1, size = 3, color = COLORS$muted
  )

save_plot(p1, "20_heating_curve.png")

# ============================================================================
# Chart 2: Does the curve deliver comfort? Indoor temp vs outdoor temp.
# ============================================================================
# Load all indoor room temps and compute average indoor temp per hour
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
  df |> distinct(hour_bucket, .keep_all = TRUE) |>
    mutate(room = name) |> select(hour_bucket, room, indoor_temp = avg)
}) |> bind_rows()

if (nrow(room_data) > 50) {
  # Average across all rooms per hour
  avg_indoor <- room_data |>
    group_by(hour_bucket) |>
    summarize(avg_indoor = mean(indoor_temp, na.rm = TRUE), .groups = "drop")

  comfort <- heating |>
    inner_join(avg_indoor, by = "hour_bucket") |>
    filter(!is.na(avg_indoor)) |>
    mutate(
      overshoot = avg_indoor - 21,  # positive = too warm
      outdoor_bin = cut(outdoor, breaks = seq(-20, 15, by = 2.5), include.lowest = TRUE)
    )

  cat("\n=== Comfort Check ===\n")
  cat("  Hours with indoor data:", nrow(comfort), "\n")
  cat("  Mean indoor temp:      ", round(mean(comfort$avg_indoor, na.rm = TRUE), 1), "°C\n")
  cat("  Mean overshoot vs 21°C:", round(mean(comfort$overshoot, na.rm = TRUE), 1), "°C\n")
  cat("  Hours below 20°C:     ", sum(comfort$avg_indoor < 20, na.rm = TRUE), "\n")

  # Per-room comfort at different outdoor temp bins
  room_comfort <- room_data |>
    inner_join(
      heating |> select(hour_bucket, outdoor),
      by = "hour_bucket"
    ) |>
    filter(!is.na(indoor_temp)) |>
    mutate(outdoor_bin = cut(outdoor, breaks = seq(-20, 15, by = 5), include.lowest = TRUE))

  room_comfort_summary <- room_comfort |>
    group_by(room, outdoor_bin) |>
    summarize(
      mean_temp = mean(indoor_temp, na.rm = TRUE),
      p10_temp  = quantile(indoor_temp, 0.1, na.rm = TRUE),
      n = n(),
      .groups = "drop"
    ) |>
    filter(!is.na(outdoor_bin), n >= 5)

  p2 <- ggplot(room_comfort_summary, aes(x = outdoor_bin, y = mean_temp, fill = room)) +
    geom_col(position = "dodge", alpha = 0.8) +
    geom_hline(yintercept = 21, linetype = "dashed", color = COLORS$import, linewidth = 0.5) +
    annotate("text", x = 0.5, y = 21.3, label = "21°C target",
             color = COLORS$import, size = 3, hjust = 0) +
    scale_fill_brewer(palette = "Set2") +
    labs(
      x     = "Outdoor Temperature Bin (°C)",
      y     = "Mean Indoor Temperature (°C)",
      title = "Does the Heating Curve Deliver Comfort?",
      subtitle = "Room temps by outdoor temp bin. Below dashed line = underheating.",
      fill  = "Room"
    ) +
    theme_energy() +
    theme(axis.text.x = element_text(angle = 30, hjust = 1))

  save_plot(p2, "20_curve_vs_comfort.png")
} else {
  cat("Insufficient indoor temperature data for comfort analysis.\n")
}

# ============================================================================
# Chart 3: Overheating heatmap — hour of day × outdoor temp bin
# ============================================================================
# Shows when the house overheats (excess energy) vs when it's too cold
if (exists("comfort") && nrow(comfort) > 50) {
  overshoot_map <- comfort |>
    mutate(
      outdoor_bin = cut(outdoor, breaks = seq(-20, 15, by = 2.5), include.lowest = TRUE),
      hour_bin = factor(hour)
    ) |>
    filter(!is.na(outdoor_bin)) |>
    group_by(outdoor_bin, hour_bin) |>
    summarize(
      avg_overshoot = mean(overshoot, na.rm = TRUE),
      n = n(),
      .groups = "drop"
    ) |>
    filter(n >= 3)

  p3 <- ggplot(overshoot_map, aes(x = hour_bin, y = outdoor_bin, fill = avg_overshoot)) +
    geom_tile(color = "white", linewidth = 0.3) +
    scale_fill_gradient2(
      low = COLORS$charge, mid = "white", high = COLORS$import,
      midpoint = 0, name = "Overshoot (°C)\nvs 21°C target"
    ) +
    labs(
      x     = "Hour of Day",
      y     = "Outdoor Temperature (°C)",
      title = "Overheating Map: When Is the Curve Wrong?",
      subtitle = "Red = rooms too warm (curve too steep). Blue = too cold (curve too flat)."
    ) +
    theme_energy() +
    theme(axis.text.x = element_text(size = 9))

  save_plot(p3, "20_overheating_map.png")
}

# ============================================================================
# Chart 4: COP along the heating curve — efficiency cost of high water temps
# ============================================================================
# Higher target water temp → lower COP. This quantifies the penalty.
cop_curve <- heating |>
  filter(!is.na(cop), cop > 0.5, cop < 10, !is.na(target_water)) |>
  mutate(
    water_bin = cut(target_water,
      breaks = seq(20, 55, by = 2.5),
      include.lowest = TRUE
    )
  ) |>
  filter(!is.na(water_bin))

if (nrow(cop_curve) > 30) {
  cop_by_water <- cop_curve |>
    group_by(water_bin) |>
    summarize(
      mean_cop = mean(cop, na.rm = TRUE),
      median_cop = median(cop, na.rm = TRUE),
      p25_cop = quantile(cop, 0.25, na.rm = TRUE),
      p75_cop = quantile(cop, 0.75, na.rm = TRUE),
      mean_outdoor = mean(outdoor, na.rm = TRUE),
      n = n(),
      .groups = "drop"
    ) |>
    filter(n >= 5)

  cat("\n=== COP vs Water Temperature ===\n")
  print(cop_by_water |> select(water_bin, mean_cop, mean_outdoor, n))

  p4 <- ggplot(cop_curve, aes(x = target_water, y = cop)) +
    geom_bin2d(bins = 35) +
    scale_fill_viridis_c(option = "plasma", trans = "log10") +
    geom_smooth(method = "loess", color = COLORS$export, linewidth = 1.2, se = TRUE) +
    geom_vline(xintercept = 35, linetype = "dotted", color = COLORS$warning) +
    annotate("text", x = 35.5, y = max(cop_curve$cop) * 0.95,
             label = "35°C threshold", color = COLORS$warning, size = 3, hjust = 0) +
    labs(
      x     = "Target Water Temperature (°C)",
      y     = "COP",
      title = "COP Along the Heating Curve",
      subtitle = "Higher water temp = lower COP. Each degree costs efficiency.",
      fill  = "Hours"
    ) +
    theme_energy()

  save_plot(p4, "20_curve_efficiency.png")
} else {
  cat("Insufficient COP data for curve efficiency analysis.\n")
}
