# ============================================================================
# 42_extreme_temperature_patterns.R — Extreme Indoor Temperature Patterns
# ============================================================================
# WHAT:    Identifies when extreme temperatures (>27°C) occur in bedrooms and
#          offices. For offices, focuses on working hours (8-16). Analyzes
#          patterns by hour, day-of-week, month, and outdoor conditions.
#
# INPUTS:  load_stats_sensor() for room temps + outdoor temp
#
# OUTPUTS: docs/analysis/42_extreme_heatmap.png        — hour × month heatmap of hot hours
#          docs/analysis/42_extreme_by_room.png        — % extreme hours by room
#          docs/analysis/42_extreme_vs_outdoor.png     — extreme events vs outdoor temp
#          docs/analysis/42_extreme_weekday_pattern.png — day-of-week pattern
# ============================================================================

source("analysis/helpers/load_data.R")

EXTREME_THRESHOLD <- 27  # °C

# ============================================================================
# Define rooms and context
# ============================================================================
bedroom_sensors <- list(
  "Bedroom 1" = TEMP_BEDROOM1,
  "Bedroom 2" = TEMP_BEDROOM2
)

office_sensors <- list(
  "Olek"  = TEMP_OFFICE1,
  "Beata" = TEMP_OFFICE2
)

all_sensors <- c(bedroom_sensors, office_sensors)
room_type_map <- c(
  "Bedroom 1" = "Bedroom", "Bedroom 2" = "Bedroom",
  "Olek" = "Office", "Beata" = "Office"
)

# ============================================================================
# Load temperature data
# ============================================================================
room_data <- map2(names(all_sensors), all_sensors, function(name, sid) {
  df <- load_stats_sensor(sid)
  if (nrow(df) == 0) return(tibble())
  df |> mutate(
    room = name,
    type = room_type_map[name],
    hour = hour(hour_bucket),
    month = month(hour_bucket, label = TRUE),
    date = as.Date(hour_bucket),
    weekday = wday(hour_bucket, label = TRUE, week_start = 1),
    is_extreme = avg >= EXTREME_THRESHOLD,
    is_working_hour = hour >= 8 & hour < 16
  )
}) |> bind_rows()

# Load outdoor temp
outdoor <- load_stats_sensor(HP_OUTSIDE_TEMP)

cat("\n=== Extreme Temperature Analysis ===\n")
cat("Threshold:", EXTREME_THRESHOLD, "°C\n")
cat("Total room-hours:", nrow(room_data), "\n")

# For offices: only count working hours as relevant
office_data <- room_data |> filter(type == "Office", is_working_hour)
bedroom_data <- room_data |> filter(type == "Bedroom")

# Combined: offices during working hours, bedrooms anytime
relevant_data <- bind_rows(
  office_data |> mutate(context = "Office (8-16h)"),
  bedroom_data |> mutate(context = "Bedroom (all hours)")
)

cat("\n=== Extreme Hours by Room ===\n")
relevant_data |>
  group_by(room, context) |>
  summarize(
    total_hours = n(),
    extreme_hours = sum(is_extreme),
    pct_extreme = round(100 * extreme_hours / total_hours, 1),
    max_temp = round(max(avg, na.rm = TRUE), 1),
    .groups = "drop"
  ) |>
  arrange(desc(pct_extreme)) |>
  print()

if (sum(relevant_data$is_extreme) < 5) {
  cat("Very few extreme events detected. Lowering threshold.\n")
  # Try with a lower threshold
  EXTREME_THRESHOLD <- 26
  relevant_data <- relevant_data |>
    mutate(is_extreme = avg >= EXTREME_THRESHOLD)
  cat("Adjusted threshold:", EXTREME_THRESHOLD, "°C →",
      sum(relevant_data$is_extreme), "extreme hours\n")
}

# ============================================================================
# Chart 1: Hour × Month heatmap of extreme temperature frequency
# ============================================================================
# Separate heatmaps for offices (working hours) and bedrooms
heatmap_data <- bind_rows(
  # Offices: all hours but highlight working hours
  room_data |>
    filter(type == "Office") |>
    group_by(month, hour) |>
    summarize(
      pct_extreme = 100 * mean(is_extreme, na.rm = TRUE),
      .groups = "drop"
    ) |>
    mutate(panel = "Offices (Olek + Beata)"),
  # Bedrooms: all hours
  room_data |>
    filter(type == "Bedroom") |>
    group_by(month, hour) |>
    summarize(
      pct_extreme = 100 * mean(is_extreme, na.rm = TRUE),
      .groups = "drop"
    ) |>
    mutate(panel = "Bedrooms")
)

p1 <- ggplot(heatmap_data, aes(x = hour, y = month, fill = pct_extreme)) +
  geom_tile(color = "white", linewidth = 0.3) +
  facet_wrap(~panel, ncol = 1) +
  scale_fill_gradient2(
    low = COLORS$charge, mid = COLORS$pv, high = COLORS$import,
    midpoint = max(heatmap_data$pct_extreme) / 2,
    name = paste0("% hours ≥", EXTREME_THRESHOLD, "°C")
  ) +
  scale_x_continuous(breaks = seq(0, 23, 2)) +
  # Mark working hours for offices
  annotate("rect", xmin = 7.5, xmax = 16.5, ymin = 0.5, ymax = Inf,
           fill = NA, color = COLORS$muted, linetype = "dashed", linewidth = 0.4) +
  labs(
    x = "Hour of Day",
    y = "",
    title = paste0("When Does Temperature Exceed ", EXTREME_THRESHOLD, "°C?"),
    subtitle = "Hour × month heatmap. Dashed box = working hours (8-16h)."
  ) +
  theme_energy() +
  theme(panel.grid = element_blank())

save_plot(p1, "42_extreme_heatmap.png", height = 8)

# ============================================================================
# Chart 2: Percentage of extreme hours by room
# ============================================================================
room_extreme <- relevant_data |>
  group_by(room, context) |>
  summarize(
    total = n(),
    extreme = sum(is_extreme),
    pct = 100 * extreme / total,
    .groups = "drop"
  ) |>
  arrange(desc(pct))

p2 <- ggplot(room_extreme, aes(
  x = reorder(room, pct),
  y = pct,
  fill = context
)) +
  geom_col(alpha = 0.7, width = 0.6) +
  geom_text(aes(label = paste0(round(pct, 1), "%")), hjust = -0.1, size = 3.5) +
  coord_flip() +
  scale_fill_manual(values = c(
    "Office (8-16h)" = COLORS$heat_pump,
    "Bedroom (all hours)" = COLORS$prediction
  )) +
  labs(
    x = "",
    y = paste0("% of Hours ≥", EXTREME_THRESHOLD, "°C"),
    title = paste0("Extreme Temperature Frequency (≥", EXTREME_THRESHOLD, "°C)"),
    subtitle = "Offices: working hours only (8-16h). Bedrooms: all hours.",
    fill = ""
  ) +
  theme_energy()

save_plot(p2, "42_extreme_by_room.png")

# ============================================================================
# Chart 3: Extreme events vs outdoor temperature
# ============================================================================
if (nrow(outdoor) > 100) {
  with_outdoor <- room_data |>
    inner_join(
      outdoor |> select(hour_bucket, outdoor_temp = avg) |> distinct(hour_bucket, .keep_all = TRUE),
      by = "hour_bucket"
    ) |>
    filter(!is.na(outdoor_temp))

  # Bin outdoor temp and compute % extreme per bin per room type
  outdoor_bins <- with_outdoor |>
    mutate(
      outdoor_bin = cut(outdoor_temp,
                        breaks = seq(-15, 40, 5),
                        include.lowest = TRUE)
    ) |>
    filter(!is.na(outdoor_bin)) |>
    group_by(type, outdoor_bin) |>
    summarize(
      n = n(),
      pct_extreme = 100 * mean(is_extreme, na.rm = TRUE),
      .groups = "drop"
    ) |>
    filter(n >= 10)  # need enough data per bin

  p3 <- ggplot(outdoor_bins, aes(x = outdoor_bin, y = pct_extreme, fill = type)) +
    geom_col(position = position_dodge(width = 0.7), alpha = 0.7, width = 0.6) +
    scale_fill_manual(values = c(
      "Office" = COLORS$heat_pump,
      "Bedroom" = COLORS$prediction
    )) +
    labs(
      x = "Outdoor Temperature (°C)",
      y = paste0("% of Hours ≥", EXTREME_THRESHOLD, "°C"),
      title = "Extreme Indoor Temperature vs Outdoor Conditions",
      subtitle = "Higher outdoor temps drive more extreme indoor events",
      fill = ""
    ) +
    theme_energy() +
    theme(axis.text.x = element_text(angle = 45, hjust = 1))

  save_plot(p3, "42_extreme_vs_outdoor.png")

  # Also: scatter of indoor vs outdoor for extreme events
  extreme_events <- with_outdoor |>
    filter(is_extreme) |>
    group_by(room, type) |>
    summarize(
      count = n(),
      mean_outdoor = mean(outdoor_temp, na.rm = TRUE),
      min_outdoor = min(outdoor_temp, na.rm = TRUE),
      .groups = "drop"
    )

  cat("\n=== Outdoor Conditions During Extreme Events ===\n")
  print(extreme_events)

  # Interesting: extreme events that happen even in mild/cold weather
  cold_extreme <- with_outdoor |>
    filter(is_extreme, outdoor_temp < 15)

  if (nrow(cold_extreme) > 0) {
    cat("\n=== Extreme Events Despite Cool Weather (<15°C outdoor) ===\n")
    cold_extreme |>
      group_by(room, type) |>
      summarize(
        count = n(),
        mean_indoor = round(mean(avg, na.rm = TRUE), 1),
        mean_outdoor = round(mean(outdoor_temp, na.rm = TRUE), 1),
        .groups = "drop"
      ) |>
      print()
  }
}

# ============================================================================
# Chart 4: Day-of-week pattern
# ============================================================================
weekday_pattern <- relevant_data |>
  group_by(room, context, weekday) |>
  summarize(
    pct_extreme = 100 * mean(is_extreme, na.rm = TRUE),
    .groups = "drop"
  )

p4 <- ggplot(weekday_pattern, aes(x = weekday, y = pct_extreme, fill = room)) +
  geom_col(position = position_dodge(width = 0.7), alpha = 0.7, width = 0.6) +
  facet_wrap(~context, scales = "free_y", ncol = 1) +
  scale_fill_brewer(palette = "Set2") +
  labs(
    x = "",
    y = paste0("% of Hours ≥", EXTREME_THRESHOLD, "°C"),
    title = "Extreme Temperature by Day of Week",
    subtitle = "Are weekends worse? Offices: working hours only.",
    fill = ""
  ) +
  theme_energy()

save_plot(p4, "42_extreme_weekday_pattern.png", height = 8)

# ============================================================================
# Summary
# ============================================================================
cat("\n=== Extreme Temperature Summary ===\n")
cat("Threshold:", EXTREME_THRESHOLD, "°C\n\n")

# Peak hours for extreme events
peak_hours <- relevant_data |>
  filter(is_extreme) |>
  count(hour) |>
  arrange(desc(n)) |>
  head(5)

cat("Peak hours for extreme events:\n")
print(peak_hours)

# Peak months
peak_months <- relevant_data |>
  filter(is_extreme) |>
  count(month) |>
  arrange(desc(n)) |>
  head(5)

cat("\nPeak months for extreme events:\n")
print(peak_months)
