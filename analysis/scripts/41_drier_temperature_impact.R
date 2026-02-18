# ============================================================================
# 41_drier_temperature_impact.R — Drier Impact on Room Temperatures
# ============================================================================
# WHAT:    Analyzes how running the drier in the first-floor bathroom affects
#          temperatures in bedrooms (first floor) and ground floor rooms.
#          Detects drier cycles, then measures temperature change during and
#          after each cycle across all rooms.
#
# INPUTS:  load_recent_sensor() for drier power + all room temp sensors
#
# OUTPUTS: docs/analysis/41_drier_temp_impact.png     — temp change by room during drier
#          docs/analysis/41_drier_temp_timeline.png   — example drier event with temps
#          docs/analysis/41_drier_floor_comparison.png — first floor vs ground floor impact
#          docs/analysis/41_drier_recovery.png        — temp recovery time after drier stops
# ============================================================================

source("analysis/helpers/load_data.R")

# ============================================================================
# Detect drier run cycles
# ============================================================================
drier_raw <- load_recent_sensor(DRIER_SENSOR)

cat("\n=== Drier Data ===\n")
cat("Total drier readings:", nrow(drier_raw), "\n")

if (nrow(drier_raw) < 100) {
  cat("Insufficient drier data for analysis.\n")
  quit(save = "no")
}

# Detect cycles: drier typically draws >50W when running
detect_cycles <- function(data, threshold = 50, gap_minutes = 15) {
  if (nrow(data) == 0) return(tibble())

  data |>
    filter(value > threshold) |>
    arrange(timestamp) |>
    mutate(
      gap = as.numeric(difftime(timestamp, lag(timestamp), units = "mins")),
      new_cycle = is.na(gap) | gap > gap_minutes,
      cycle_id = cumsum(new_cycle)
    ) |>
    group_by(cycle_id) |>
    summarize(
      start = min(timestamp),
      end = max(timestamp),
      duration_min = as.numeric(difftime(max(timestamp), min(timestamp), units = "mins")),
      avg_power = mean(value),
      max_power = max(value),
      readings = n(),
      energy_wh = mean(value) * duration_min / 60,
      .groups = "drop"
    ) |>
    filter(duration_min >= 10, readings >= 5)  # real drier cycles ≥10 min
}

cycles <- detect_cycles(drier_raw)
cat("Drier cycles detected:", nrow(cycles), "\n")
cat("Typical duration:", round(median(cycles$duration_min)), "min\n")
cat("Typical power:", round(median(cycles$avg_power)), "W\n")

if (nrow(cycles) < 3) {
  cat("Too few drier cycles for meaningful analysis.\n")
  quit(save = "no")
}

# ============================================================================
# Load all room temperature data (high-res recent for precise timing)
# ============================================================================
room_sensors <- list(
  "Bathroom"    = TEMP_BATHROOM,
  "Bedroom 1"   = TEMP_BEDROOM1,
  "Bedroom 2"   = TEMP_BEDROOM2,
  "Living Room"  = NETATMO_LIVING_TEMP,
  "Kitchen"      = TEMP_KITCHEN,
  "Olek"         = TEMP_OFFICE1,
  "Beata"        = TEMP_OFFICE2
)

floor_map <- c(
  "Bathroom" = "First Floor", "Bedroom 1" = "First Floor",
  "Bedroom 2" = "First Floor",
  "Living Room" = "Ground Floor", "Kitchen" = "Ground Floor",
  "Olek" = "Ground Floor", "Beata" = "Ground Floor"
)

# Load recent high-res data for each room
room_recent <- map2(names(room_sensors), room_sensors, function(name, sid) {
  df <- load_recent_sensor(sid)
  if (nrow(df) == 0) return(tibble())
  df |> mutate(room = name, floor = floor_map[name])
}) |> bind_rows()

cat("Room temp readings:", nrow(room_recent), "\n")
cat("Rooms with data:", paste(unique(room_recent$room), collapse = ", "), "\n")

if (nrow(room_recent) < 1000) {
  cat("Insufficient high-res room data. Trying stats data.\n")
  room_recent <- map2(names(room_sensors), room_sensors, function(name, sid) {
    df <- load_stats_sensor(sid)
    if (nrow(df) == 0) return(tibble())
    df |>
      rename(timestamp = hour_bucket, value = avg) |>
      mutate(room = name, floor = floor_map[name])
  }) |> bind_rows()
}

# ============================================================================
# Measure temperature change during each drier cycle
# ============================================================================
# For each cycle, find room temp at start, end, and 1h after end
measure_impact <- function(cycle, room_data, window_before_min = 30, window_after_min = 60) {
  map(seq_len(nrow(cycle)), function(i) {
    cyc <- cycle[i, ]

    # Time windows
    before_start <- cyc$start - minutes(window_before_min)
    after_end <- cyc$end + minutes(window_after_min)

    # Get temps before, during, and after for each room
    room_data |>
      filter(timestamp >= before_start, timestamp <= after_end) |>
      mutate(
        phase = case_when(
          timestamp < cyc$start ~ "before",
          timestamp <= cyc$end ~ "during",
          TRUE ~ "after"
        )
      ) |>
      group_by(room, floor, phase) |>
      summarize(
        mean_temp = mean(value, na.rm = TRUE),
        n = n(),
        .groups = "drop"
      ) |>
      mutate(cycle_id = i, duration_min = cyc$duration_min)
  }) |> bind_rows()
}

impact <- measure_impact(cycles, room_recent)

if (nrow(impact) < 10) {
  cat("Insufficient overlap between drier cycles and room data.\n")
  quit(save = "no")
}

# Compute temperature deltas
temp_deltas <- impact |>
  pivot_wider(names_from = phase, values_from = c(mean_temp, n)) |>
  filter(!is.na(mean_temp_before), !is.na(mean_temp_during)) |>
  mutate(
    delta_during = mean_temp_during - mean_temp_before,
    delta_after = ifelse(!is.na(mean_temp_after), mean_temp_after - mean_temp_before, NA_real_)
  )

cat("\n=== Temperature Impact During Drier Operation ===\n")
temp_deltas |>
  group_by(room, floor) |>
  summarize(
    cycles = n(),
    mean_delta = round(mean(delta_during, na.rm = TRUE), 2),
    median_delta = round(median(delta_during, na.rm = TRUE), 2),
    .groups = "drop"
  ) |>
  arrange(desc(mean_delta)) |>
  print()

# ============================================================================
# Chart 1: Temperature change by room during drier operation
# ============================================================================
room_order <- temp_deltas |>
  group_by(room, floor) |>
  summarize(med = median(delta_during, na.rm = TRUE), .groups = "drop") |>
  arrange(med)

p1 <- ggplot(temp_deltas, aes(
  x = factor(room, levels = room_order$room),
  y = delta_during,
  fill = floor
)) +
  geom_boxplot(alpha = 0.7, outlier.size = 0.8) +
  geom_hline(yintercept = 0, linetype = "dashed", color = COLORS$muted) +
  coord_flip() +
  scale_fill_manual(values = c(
    "Ground Floor" = COLORS$charge,
    "First Floor"  = COLORS$import
  )) +
  labs(
    x = "",
    y = "Temperature Change During Drier (°C)",
    title = "Room Temperature Impact of Running the Drier",
    subtitle = paste0("Based on ", nrow(cycles), " drier cycles. Positive = warmer during drier operation."),
    fill = ""
  ) +
  theme_energy()

save_plot(p1, "41_drier_temp_impact.png")

# ============================================================================
# Chart 2: Example timeline — drier event with room temperatures
# ============================================================================
# Pick the longest cycle with good data coverage
best_cycle_id <- temp_deltas |>
  group_by(cycle_id) |>
  summarize(rooms = n_distinct(room), dur = first(duration_min), .groups = "drop") |>
  filter(rooms >= 3) |>
  slice_max(dur, n = 1) |>
  pull(cycle_id)

if (length(best_cycle_id) > 0) {
  cyc <- cycles[best_cycle_id[1], ]
  window <- 60  # minutes before and after

  timeline_data <- room_recent |>
    filter(
      timestamp >= (cyc$start - minutes(window)),
      timestamp <= (cyc$end + minutes(window)),
      room %in% c("Bathroom", "Bedroom 1", "Bedroom 2")
    )

  if (nrow(timeline_data) > 20) {
    p2 <- ggplot(timeline_data, aes(x = timestamp, y = value, color = room)) +
      geom_line(linewidth = 0.8) +
      annotate("rect",
               xmin = cyc$start, xmax = cyc$end,
               ymin = -Inf, ymax = Inf,
               fill = COLORS$heat_pump, alpha = 0.15) +
      annotate("text",
               x = cyc$start + (cyc$end - cyc$start) / 2,
               y = max(timeline_data$value, na.rm = TRUE) + 0.2,
               label = paste0("Drier ON (", round(cyc$duration_min), " min)"),
               color = COLORS$heat_pump, size = 3.5) +
      scale_color_brewer(palette = "Set2") +
      labs(
        x = "",
        y = "Temperature (°C)",
        title = "First Floor Temperatures During a Drier Cycle",
        subtitle = format(cyc$start, "%Y-%m-%d %H:%M"),
        color = ""
      ) +
      theme_energy()

    save_plot(p2, "41_drier_temp_timeline.png")
  }
}

# ============================================================================
# Chart 3: Floor-level comparison — aggregated impact
# ============================================================================
floor_impact <- temp_deltas |>
  group_by(floor, cycle_id) |>
  summarize(mean_delta = mean(delta_during, na.rm = TRUE), .groups = "drop")

p3 <- ggplot(floor_impact, aes(x = floor, y = mean_delta, fill = floor)) +
  geom_boxplot(alpha = 0.7, width = 0.5) +
  geom_hline(yintercept = 0, linetype = "dashed", color = COLORS$muted) +
  scale_fill_manual(values = c(
    "Ground Floor" = COLORS$charge,
    "First Floor"  = COLORS$import
  )) +
  labs(
    x = "",
    y = "Average Temperature Change (°C)",
    title = "Drier Impact: First Floor vs Ground Floor",
    subtitle = "Each point = one drier cycle's average room temp change on that floor",
    fill = ""
  ) +
  theme_energy()

save_plot(p3, "41_drier_floor_comparison.png")

# ============================================================================
# Chart 4: Temperature recovery after drier stops
# ============================================================================
if (any(!is.na(temp_deltas$delta_after))) {
  recovery <- temp_deltas |>
    filter(!is.na(delta_after), !is.na(delta_during)) |>
    mutate(recovery_pct = ifelse(delta_during != 0,
                                  (1 - delta_after / delta_during) * 100, NA)) |>
    filter(!is.na(recovery_pct))

  if (nrow(recovery) > 5) {
    recovery_summary <- recovery |>
      group_by(room, floor) |>
      summarize(
        mean_during = mean(delta_during, na.rm = TRUE),
        mean_after_1h = mean(delta_after, na.rm = TRUE),
        .groups = "drop"
      ) |>
      pivot_longer(c(mean_during, mean_after_1h), names_to = "phase", values_to = "delta") |>
      mutate(phase = ifelse(phase == "mean_during", "During Drier", "1h After Stop"))

    p4 <- ggplot(recovery_summary, aes(
      x = factor(room, levels = room_order$room),
      y = delta,
      fill = factor(phase, levels = c("During Drier", "1h After Stop"))
    )) +
      geom_col(position = position_dodge(width = 0.6), alpha = 0.7, width = 0.5) +
      geom_hline(yintercept = 0, linetype = "dashed", color = COLORS$muted) +
      coord_flip() +
      scale_fill_manual(values = c(
        "During Drier" = COLORS$import,
        "1h After Stop" = COLORS$charge
      )) +
      labs(
        x = "",
        y = "Temperature Change vs Baseline (°C)",
        title = "Temperature Recovery After Drier Stops",
        subtitle = "Comparing temperature shift during drier vs 1 hour after it stops",
        fill = ""
      ) +
      theme_energy()

    save_plot(p4, "41_drier_recovery.png")
  }
}
