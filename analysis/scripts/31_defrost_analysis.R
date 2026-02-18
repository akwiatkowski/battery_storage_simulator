# ============================================================================
# 31_defrost_analysis.R — Defrost Energy Budget
# ============================================================================
# WHAT:    Analyzes defrost cycles: when ice builds up on the outdoor evaporator,
#          the HP reverses flow (hot gas defrost), heating the outdoor coil while
#          momentarily cooling the house. This costs energy and reduces net COP.
#
#          Detection: during normal heating, outside_pipe_temp < outdoor_temp
#          (the coil is colder than air, absorbing heat). During defrost, the HP
#          sends hot refrigerant to the outdoor unit so outside_pipe_temp rises
#          well above outdoor_temp. We detect defrost as:
#            pipe_depression = outside_pipe_temp - outdoor_temp > 10 C
#          Combined with HP running (compressor speed > 0 or consumption > 0).
#
#          If pipe data is sparse, fallback: COP dropping below 1 while HP runs.
#
# INPUTS:  load_stats_sensor() for HP_OUTSIDE_PIPE_TEMP, HP_INSIDE_PIPE_TEMP,
#          HP_OUTSIDE_TEMP, HP_COMPRESSOR_SPEED, HP_COP_SENSOR, HP_CONSUMPTION
#
# OUTPUTS: output/31_defrost_by_temp.png        — defrost frequency by outdoor temp
#          output/31_defrost_duration.png        — histogram of defrost event duration
#          output/31_defrost_monthly_energy.png  — estimated defrost energy per month
#          output/31_defrost_pipe_example.png    — pipe temps during defrost events
#
# HOW TO READ:
#   - Frequency by temp: peak around -2 to +5 C = humid frost zone (expected)
#   - Duration: most defrosts are 3-10 minutes; long ones suggest icing problems
#   - Monthly energy: winter months dominate; high values = poor drainage or low airflow
#   - Pipe temps: outside pipe spikes up (hot gas), inside pipe dips (reversed flow)
# ============================================================================

source("analysis/helpers/load_data.R")

# ============================================================================
# Load sensor data
# ============================================================================
outside_pipe <- load_stats_sensor(HP_OUTSIDE_PIPE_TEMP) |>
  distinct(hour_bucket, .keep_all = TRUE)
inside_pipe  <- load_stats_sensor(HP_INSIDE_PIPE_TEMP) |>
  distinct(hour_bucket, .keep_all = TRUE)
outdoor_temp <- load_stats_sensor(HP_OUTSIDE_TEMP) |>
  distinct(hour_bucket, .keep_all = TRUE)
comp_speed   <- load_stats_sensor(HP_COMPRESSOR_SPEED) |>
  distinct(hour_bucket, .keep_all = TRUE)
cop_sensor   <- load_stats_sensor(HP_COP_SENSOR) |>
  distinct(hour_bucket, .keep_all = TRUE)
hp_cons      <- load_stats_sensor(HP_CONSUMPTION) |>
  distinct(hour_bucket, .keep_all = TRUE)

cat("\n=== Defrost Analysis Data ===\n")
cat("  Outside pipe temp:", nrow(outside_pipe), "hours\n")
cat("  Inside pipe temp: ", nrow(inside_pipe), "hours\n")
cat("  Outdoor temp:     ", nrow(outdoor_temp), "hours\n")
cat("  Compressor speed: ", nrow(comp_speed), "hours\n")
cat("  COP sensor:       ", nrow(cop_sensor), "hours\n")
cat("  HP consumption:   ", nrow(hp_cons), "hours\n")

# ============================================================================
# Primary detection: pipe temperature method
# ============================================================================
# During defrost: outside_pipe_temp >> outdoor_temp (hot gas sent to outdoor unit)
# pipe_elevation = outside_pipe_temp - outdoor_temp
# Normal heating: pipe_elevation < 0 (coil absorbing heat from air)
# Defrost: pipe_elevation > 10 C (hot gas melting ice)

use_pipe_method <- nrow(outside_pipe) >= 20 && nrow(outdoor_temp) >= 20

if (use_pipe_method) {
  defrost_data <- outside_pipe |>
    select(hour_bucket, pipe_avg = avg, pipe_max = max_val) |>
    inner_join(outdoor_temp |> select(hour_bucket, outdoor = avg), by = "hour_bucket") |>
    left_join(inside_pipe |> select(hour_bucket, inside_pipe = avg), by = "hour_bucket") |>
    left_join(comp_speed |> select(hour_bucket, speed = avg), by = "hour_bucket") |>
    left_join(cop_sensor |> select(hour_bucket, cop = avg), by = "hour_bucket") |>
    left_join(hp_cons |> select(hour_bucket, consumption = avg), by = "hour_bucket") |>
    filter(!is.na(pipe_avg), !is.na(outdoor)) |>
    mutate(
      # Use max_val for peak pipe temp within the hour (captures short defrosts)
      pipe_peak = ifelse(!is.na(pipe_max), pipe_max, pipe_avg),
      pipe_elevation_avg = pipe_avg - outdoor,
      pipe_elevation_max = pipe_peak - outdoor,
      # HP must be running for it to be a defrost (not just idle warm-up)
      hp_running = (!is.na(speed) & speed > 0) | (!is.na(consumption) & consumption > 100),
      # Defrost flag: pipe significantly above outdoor AND HP running
      is_defrost = pipe_elevation_max > 10 & hp_running,
      hour  = hour(hour_bucket),
      month = month(hour_bucket, label = TRUE),
      date  = as.Date(hour_bucket)
    )

  cat("\n=== Pipe-Based Defrost Detection ===\n")
  cat("  Hours analyzed:       ", nrow(defrost_data), "\n")
  cat("  Hours with HP running:", sum(defrost_data$hp_running, na.rm = TRUE), "\n")
  cat("  Defrost hours:        ", sum(defrost_data$is_defrost, na.rm = TRUE), "\n")
  cat("  Defrost rate:         ",
      round(sum(defrost_data$is_defrost, na.rm = TRUE) /
              max(1, sum(defrost_data$hp_running, na.rm = TRUE)) * 100, 1), "% of running hours\n")
} else {
  cat("Insufficient pipe temperature data, trying COP fallback.\n")
  # Fallback: defrost = COP < 1 while HP is consuming power
  if (nrow(cop_sensor) >= 20 && nrow(hp_cons) >= 20) {
    defrost_data <- cop_sensor |>
      select(hour_bucket, cop = avg) |>
      inner_join(hp_cons |> select(hour_bucket, consumption = avg), by = "hour_bucket") |>
      left_join(outdoor_temp |> select(hour_bucket, outdoor = avg), by = "hour_bucket") |>
      left_join(comp_speed |> select(hour_bucket, speed = avg), by = "hour_bucket") |>
      filter(!is.na(cop), !is.na(consumption)) |>
      mutate(
        hp_running = consumption > 100,
        is_defrost = hp_running & cop < 1 & cop > 0,
        hour  = hour(hour_bucket),
        month = month(hour_bucket, label = TRUE),
        date  = as.Date(hour_bucket),
        pipe_elevation_avg = NA_real_,
        pipe_elevation_max = NA_real_,
        pipe_avg = NA_real_,
        pipe_peak = NA_real_,
        inside_pipe = NA_real_
      )
    cat("  COP fallback hours: ", nrow(defrost_data), "\n")
    cat("  Defrost hours (COP < 1):", sum(defrost_data$is_defrost, na.rm = TRUE), "\n")
  } else {
    cat("Insufficient data for any defrost detection method. Exiting.\n")
    quit(save = "no")
  }
}

n_defrost <- sum(defrost_data$is_defrost, na.rm = TRUE)
if (n_defrost < 20) {
  cat("Only", n_defrost, "defrost hours detected. Insufficient for analysis.\n")
  quit(save = "no")
}

# ============================================================================
# Chart 1: Defrost frequency by outdoor temperature
# ============================================================================
# Bin outdoor temperature and count defrost events per bin
defrost_by_temp <- defrost_data |>
  filter(!is.na(outdoor), hp_running) |>
  mutate(
    temp_bin = cut(outdoor,
                   breaks = seq(-25, 25, by = 2),
                   include.lowest = TRUE)
  ) |>
  filter(!is.na(temp_bin)) |>
  group_by(temp_bin) |>
  summarize(
    total_hours   = n(),
    defrost_hours = sum(is_defrost, na.rm = TRUE),
    defrost_pct   = mean(is_defrost, na.rm = TRUE) * 100,
    .groups = "drop"
  ) |>
  filter(total_hours >= 5) |>
  mutate(
    temp_mid = as.numeric(sub("\\[?\\(?(-?[0-9.]+),.*", "\\1", as.character(temp_bin))) + 1
  )

if (nrow(defrost_by_temp) > 3) {
  p1 <- ggplot(defrost_by_temp, aes(x = temp_mid, y = defrost_pct)) +
    geom_col(fill = COLORS$charge, alpha = 0.8) +
    geom_text(aes(label = defrost_hours), vjust = -0.3, size = 3, color = COLORS$muted) +
    labs(
      x     = "Outdoor Temperature (\u00b0C)",
      y     = "% of Running Hours with Defrost",
      title = "Defrost Frequency by Outdoor Temperature",
      subtitle = paste0(
        "Peak expected around -2 to +5\u00b0C (humid frost zone). ",
        "Numbers above bars = total defrost hours in that bin."
      )
    ) +
    theme_energy()

  save_plot(p1, "31_defrost_by_temp.png")
}

# ============================================================================
# Chart 2: Defrost event duration distribution
# ============================================================================
# Group consecutive defrost hours into events, measure duration
defrost_events <- defrost_data |>
  arrange(hour_bucket) |>
  mutate(
    hours_gap = as.numeric(difftime(hour_bucket, lag(hour_bucket), units = "hours")),
    # A new event starts when there is a gap > 1 hour or not defrost -> defrost
    is_new_event = is.na(hours_gap) | hours_gap > 1 | (!lag(is_defrost, default = FALSE) & is_defrost)
  ) |>
  filter(is_defrost) |>
  mutate(event_id = cumsum(is_new_event))

event_summary <- defrost_events |>
  group_by(event_id) |>
  summarize(
    start_time  = min(hour_bucket),
    duration_h  = n(),
    avg_outdoor = mean(outdoor, na.rm = TRUE),
    avg_cop     = mean(cop, na.rm = TRUE),
    avg_pipe_elevation = mean(pipe_elevation_avg, na.rm = TRUE),
    month = month(min(hour_bucket), label = TRUE),
    .groups = "drop"
  )

cat("\n=== Defrost Event Summary ===\n")
cat("  Total events:      ", nrow(event_summary), "\n")
cat("  Median duration:   ", median(event_summary$duration_h), "hours\n")
cat("  Mean duration:     ", round(mean(event_summary$duration_h), 1), "hours\n")
cat("  Max duration:      ", max(event_summary$duration_h), "hours\n")

# Since data is hourly, durations are in whole hours. Short defrosts (minutes)
# appear as 1-hour events. Show histogram.
if (nrow(event_summary) > 10) {
  p2 <- ggplot(event_summary, aes(x = duration_h)) +
    geom_histogram(binwidth = 1, fill = COLORS$charge, alpha = 0.8, color = "white") +
    geom_vline(xintercept = median(event_summary$duration_h),
               linetype = "dashed", color = COLORS$import) +
    annotate("text",
      x = median(event_summary$duration_h) + 0.5,
      y = max(table(event_summary$duration_h)) * 0.9,
      label = paste0("Median: ", median(event_summary$duration_h), "h"),
      color = COLORS$import, size = 3.5, hjust = 0
    ) +
    labs(
      x     = "Defrost Event Duration (hours)",
      y     = "Number of Events",
      title = "Defrost Event Duration Distribution",
      subtitle = paste0(
        "Hourly resolution: real defrosts last 3-10 min, so most appear as 1-hour events. ",
        nrow(event_summary), " events detected."
      )
    ) +
    theme_energy()

  save_plot(p2, "31_defrost_duration.png")
}

# ============================================================================
# Chart 3: Monthly defrost energy estimate
# ============================================================================
# During defrost, the HP consumes power but produces no useful heat.
# Energy penalty = consumption during defrost hours.
# If we have consumption data, use it directly. Otherwise estimate from
# average HP power.

defrost_with_cons <- defrost_data |>
  filter(is_defrost) |>
  mutate(
    # consumption is in W (hourly average), so Wh = W * 1h
    defrost_energy_wh = ifelse(!is.na(consumption), consumption, 1500)
  )

monthly_defrost <- defrost_with_cons |>
  group_by(month) |>
  summarize(
    defrost_hours  = n(),
    defrost_energy_kwh = sum(defrost_energy_wh, na.rm = TRUE) / 1000,
    avg_outdoor    = mean(outdoor, na.rm = TRUE),
    .groups = "drop"
  )

# Also get total HP energy per month for context
monthly_hp <- hp_cons |>
  mutate(month = month(hour_bucket, label = TRUE)) |>
  group_by(month) |>
  summarize(
    total_hp_kwh = sum(avg, na.rm = TRUE) / 1000,
    .groups = "drop"
  )

monthly_defrost <- monthly_defrost |>
  left_join(monthly_hp, by = "month") |>
  mutate(
    defrost_pct = ifelse(!is.na(total_hp_kwh) & total_hp_kwh > 0,
                         defrost_energy_kwh / total_hp_kwh * 100, NA_real_)
  )

cat("\n=== Monthly Defrost Energy ===\n")
print(monthly_defrost |>
        select(month, defrost_hours, defrost_energy_kwh, total_hp_kwh, defrost_pct) |>
        mutate(across(where(is.numeric), ~ round(., 1))))

total_defrost_kwh <- sum(monthly_defrost$defrost_energy_kwh, na.rm = TRUE)

p3 <- ggplot(monthly_defrost, aes(x = month, y = defrost_energy_kwh)) +
  geom_col(fill = COLORS$charge, alpha = 0.8) +
  {if (any(!is.na(monthly_defrost$defrost_pct)))
    geom_text(
      aes(label = ifelse(!is.na(defrost_pct),
                         paste0(round(defrost_pct, 1), "%"), "")),
      vjust = -0.3, size = 3, color = COLORS$muted
    )
  } +
  labs(
    x     = "",
    y     = "Estimated Defrost Energy (kWh)",
    title = "Monthly Defrost Energy Budget",
    subtitle = paste0(
      "Energy consumed during defrost hours (wasted for heating). ",
      "Total: ", round(total_defrost_kwh, 1), " kWh. ",
      "Labels show % of total HP consumption."
    )
  ) +
  theme_energy()

save_plot(p3, "31_defrost_monthly_energy.png")

# ============================================================================
# Chart 4: Pipe temperature during defrost — example time series
# ============================================================================
# Pick 2-3 defrost events with the best data coverage and plot a time window
# around each showing pipe temps, outdoor temp, and compressor speed.

if (use_pipe_method && nrow(inside_pipe) > 20) {
  # Pick events that have inside pipe data for richer visualization
  events_with_data <- defrost_events |>
    filter(!is.na(inside_pipe), !is.na(pipe_avg)) |>
    group_by(event_id) |>
    summarize(
      start_time = min(hour_bucket),
      n_with_data = n(),
      .groups = "drop"
    ) |>
    filter(n_with_data >= 1) |>
    arrange(desc(n_with_data)) |>
    head(3)

  if (nrow(events_with_data) > 0) {
    # Build time windows: event +/- 6 hours for context
    windows <- events_with_data |>
      mutate(
        window_start = start_time - hours(6),
        window_end   = start_time + hours(12),
        event_label  = paste("Event", row_number(),
                             format(start_time, "%Y-%m-%d %H:%M"))
      )

    # Extract data for each window
    example_data <- map_dfr(1:nrow(windows), function(i) {
      w <- windows[i, ]
      defrost_data |>
        filter(hour_bucket >= w$window_start, hour_bucket <= w$window_end) |>
        mutate(event_label = w$event_label)
    })

    if (nrow(example_data) > 10) {
      # Pivot pipe temps and outdoor into long format for line chart
      example_long <- example_data |>
        select(hour_bucket, event_label, is_defrost,
               `Outside Pipe` = pipe_avg,
               `Inside Pipe` = inside_pipe,
               `Outdoor Air` = outdoor) |>
        pivot_longer(
          cols = c(`Outside Pipe`, `Inside Pipe`, `Outdoor Air`),
          names_to = "measurement",
          values_to = "temp_c"
        ) |>
        filter(!is.na(temp_c))

      temp_colors <- c(
        "Outside Pipe" = COLORS$import,
        "Inside Pipe"  = COLORS$charge,
        "Outdoor Air"  = COLORS$muted
      )

      # Highlight defrost periods
      defrost_bands <- example_data |>
        filter(is_defrost) |>
        mutate(
          xmin = hour_bucket - minutes(30),
          xmax = hour_bucket + minutes(30)
        )

      p4 <- ggplot(example_long, aes(x = hour_bucket, y = temp_c, color = measurement)) +
        {if (nrow(defrost_bands) > 0)
          geom_rect(data = defrost_bands,
                    aes(xmin = xmin, xmax = xmax, ymin = -Inf, ymax = Inf),
                    inherit.aes = FALSE,
                    fill = COLORS$warning, alpha = 0.2)
        } +
        geom_line(linewidth = 0.9) +
        geom_point(size = 1.5, alpha = 0.7) +
        facet_wrap(~ event_label, scales = "free_x", ncol = 1) +
        scale_color_manual(values = temp_colors) +
        labs(
          x     = "",
          y     = "Temperature (\u00b0C)",
          title = "Pipe Temperatures During Defrost Events",
          subtitle = "Yellow bands = defrost detected. Outside pipe spikes above outdoor = hot gas defrost.",
          color = ""
        ) +
        theme_energy() +
        theme(strip.text = element_text(face = "bold", size = 10))

      save_plot(p4, "31_defrost_pipe_example.png", width = 12, height = 10)
    }
  } else {
    cat("No defrost events with complete pipe data for example chart.\n")
  }
} else {
  cat("Insufficient inside pipe data for defrost example chart.\n")
}

# ============================================================================
# Summary
# ============================================================================
cat("\n=== DEFROST ANALYSIS SUMMARY ===\n")
cat("  Detection method:     ", ifelse(use_pipe_method, "pipe temperature", "COP fallback"), "\n")
cat("  Total defrost hours:  ", n_defrost, "\n")
cat("  Total defrost events: ", nrow(event_summary), "\n")
cat("  Total defrost energy: ", round(total_defrost_kwh, 1), "kWh\n")
if (nrow(defrost_by_temp) > 0) {
  peak_bin <- defrost_by_temp |> arrange(desc(defrost_pct)) |> head(1)
  cat("  Peak defrost temp:    ", round(peak_bin$temp_mid, 0), "\u00b0C (",
      round(peak_bin$defrost_pct, 1), "% of running hours)\n")
}
