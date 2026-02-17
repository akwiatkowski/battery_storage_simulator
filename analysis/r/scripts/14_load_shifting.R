# ============================================================================
# 14_load_shifting.R — Appliance Load Shifting Potential
# ============================================================================
# WHAT:    Detects washing machine, drier, and oven run cycles from high-res
#          sensor data, overlays with spot prices, and estimates savings from
#          shifting usage to cheaper hours.
#
# INPUTS:  load_recent_sensor() for appliance sensors, spot_prices
#
# OUTPUTS: output/14_cycle_times.png        — when appliances run (hour heatmap)
#          output/14_shifting_savings.png   — cost per cycle: actual vs optimal
#          output/14_best_hours.png         — cheapest hours for each appliance
#
# HOW TO READ:
#   - Cycle times: heatmap shows which hours appliances typically run
#   - Shifting savings: bar pairs show actual cost vs best-possible cost
#     per run cycle — the gap is the savings from better timing
#   - Best hours: the cheapest window to run each appliance type
# ============================================================================

source("analysis/r/R/load_data.R")

# ============================================================================
# Detect run cycles from power data
# ============================================================================
# A "cycle" = contiguous period where power exceeds a threshold.
# We group nearby readings (within gap_minutes) into single cycles and
# compute energy, duration, and cost for each.
#
# Args:
#   data       — tibble with timestamp, value columns
#   threshold  — minimum watts to count as "running" (filters standby)
#   gap_minutes — merge readings within this gap into one cycle
#   label      — human-readable name for the appliance
detect_cycles <- function(data, threshold = 10, gap_minutes = 10, label = "Appliance") {
  if (nrow(data) == 0) return(tibble())

  runs <- data |>
    filter(value > threshold) |>
    arrange(timestamp) |>
    mutate(
      # Time gap to previous reading (in minutes)
      gap = as.numeric(difftime(timestamp, lag(timestamp), units = "mins")),
      # Start a new cycle if gap > gap_minutes or it's the first reading
      new_cycle = is.na(gap) | gap > gap_minutes,
      # Assign a cycle ID by cumulating the new_cycle flags
      cycle_id = cumsum(new_cycle)
    ) |>
    group_by(cycle_id) |>
    summarize(
      start    = min(timestamp),
      end      = max(timestamp),
      duration_min = as.numeric(difftime(max(timestamp), min(timestamp), units = "mins")),
      avg_power = mean(value),
      max_power = max(value),
      readings  = n(),
      # Energy: sum of (power × time_interval). Approximate with mean power × duration.
      energy_wh = mean(value) * duration_min / 60,
      .groups   = "drop"
    ) |>
    filter(duration_min >= 3, readings >= 3) |>  # filter noise spikes
    mutate(
      appliance = label,
      hour      = hour(start),
      weekday   = wday(start, label = TRUE, week_start = 1)
    )

  # Join with hourly spot price (use the hour the cycle started)
  runs |>
    mutate(price_bucket = floor_date(start, "hour")) |>
    left_join(spot_prices, by = c("price_bucket" = "hour_bucket")) |>
    mutate(
      cost_pln = energy_wh / 1000 * price  # Wh to kWh × PLN/kWh
    )
}

# Load appliance data and detect cycles
washing_raw <- load_recent_sensor(WASHING_SENSOR)
drier_raw   <- load_recent_sensor(DRIER_SENSOR)
oven_raw    <- load_recent_sensor(OVEN_SENSOR)

washing_cycles <- detect_cycles(washing_raw, threshold = 5, gap_minutes = 15,
                                label = "Washing machine")
drier_cycles   <- detect_cycles(drier_raw, threshold = 5, gap_minutes = 15,
                                label = "Drier")
oven_cycles    <- detect_cycles(oven_raw, threshold = 50, gap_minutes = 10,
                                label = "Oven")

all_cycles <- bind_rows(washing_cycles, drier_cycles, oven_cycles) |>
  filter(!is.na(price))

cat("\n=== Detected Cycles ===\n")
all_cycles |>
  group_by(appliance) |>
  summarize(
    cycles     = n(),
    avg_min    = round(mean(duration_min)),
    avg_wh     = round(mean(energy_wh)),
    avg_cost   = round(mean(cost_pln, na.rm = TRUE), 3),
    total_kwh  = round(sum(energy_wh) / 1000, 1),
    .groups    = "drop"
  ) |>
  print()

# ============================================================================
# Charts
# ============================================================================

if (nrow(all_cycles) > 0) {

# --- Chart 1: When appliances run (hour-of-day frequency) --------------------
cycle_hours <- all_cycles |>
  group_by(appliance, hour) |>
  summarize(count = n(), .groups = "drop")

p1 <- ggplot(cycle_hours, aes(x = hour, y = count, fill = appliance)) +
  geom_col(position = "dodge", alpha = 0.8) +
  scale_fill_manual(values = c(
    "Washing machine" = COLORS$charge,
    "Drier"           = COLORS$heat_pump,
    "Oven"            = COLORS$import
  )) +
  scale_x_continuous(breaks = 0:23) +
  labs(
    x        = "Hour of Day",
    y        = "Number of Cycles",
    title    = "When Do Appliances Run?",
    subtitle = "Current usage pattern \u2014 are you running them at expensive hours?",
    fill     = ""
  ) +
  theme_energy()

save_plot(p1, "14_cycle_times.png")

# --- Chart 2: Actual cost vs optimal cost per cycle --------------------------
# For each cycle, find the cheapest hour of the same day it could have run.
optimal <- all_cycles |>
  mutate(date = as.Date(start)) |>
  left_join(
    spot_prices |>
      mutate(date = as.Date(hour_bucket)) |>
      group_by(date) |>
      summarize(cheapest_price = min(price), .groups = "drop"),
    by = "date"
  ) |>
  mutate(
    optimal_cost = energy_wh / 1000 * cheapest_price,
    savings      = cost_pln - optimal_cost
  ) |>
  filter(!is.na(savings))

savings_summary <- optimal |>
  group_by(appliance) |>
  summarize(
    cycles          = n(),
    avg_actual_cost = mean(cost_pln),
    avg_optimal_cost = mean(optimal_cost),
    avg_savings     = mean(savings),
    total_savings   = sum(savings),
    .groups         = "drop"
  )

cat("\n=== Shifting Potential ===\n")
print(savings_summary)

savings_long <- savings_summary |>
  select(appliance,
         "Actual"  = avg_actual_cost,
         "Optimal" = avg_optimal_cost) |>
  pivot_longer(-appliance, names_to = "scenario", values_to = "cost")

p2 <- ggplot(savings_long, aes(x = appliance, y = cost * 1000, fill = scenario)) +
  geom_col(position = "dodge") +
  scale_fill_manual(values = c("Actual" = COLORS$import, "Optimal" = COLORS$export)) +
  labs(
    x        = "",
    y        = "Average Cost per Cycle (gr)",
    title    = "Appliance Cost per Cycle \u2014 Actual vs Optimal Timing",
    subtitle = "Green = cheapest possible hour of the same day",
    fill     = ""
  ) +
  theme_energy()

save_plot(p2, "14_shifting_savings.png")

# --- Chart 3: Best hours to run each appliance (price profile) ---------------
# Average spot price by hour, annotated with current usage peaks.
price_by_hour <- spot_prices |>
  mutate(hour = hour(hour_bucket)) |>
  group_by(hour) |>
  summarize(avg_price = mean(price, na.rm = TRUE), .groups = "drop")

# Find peak usage hour per appliance
peak_hours <- all_cycles |>
  group_by(appliance) |>
  count(hour) |>
  slice_max(n, n = 1) |>
  ungroup()

p3 <- ggplot(price_by_hour, aes(x = hour, y = avg_price)) +
  geom_col(fill = COLORS$muted, alpha = 0.3) +
  geom_line(color = COLORS$import, linewidth = 1) +
  geom_vline(data = peak_hours,
             aes(xintercept = hour, color = appliance),
             linetype = "dashed", linewidth = 0.8) +
  scale_color_manual(values = c(
    "Washing machine" = COLORS$charge,
    "Drier"           = COLORS$heat_pump,
    "Oven"            = COLORS$import
  )) +
  scale_x_continuous(breaks = 0:23) +
  annotate("rect", xmin = 1.5, xmax = 5.5, ymin = -Inf, ymax = Inf,
           fill = COLORS$export, alpha = 0.1) +
  annotate("text", x = 3.5, y = max(price_by_hour$avg_price) * 0.95,
           label = "Cheapest\nwindow", color = COLORS$export, size = 3) +
  labs(
    x        = "Hour of Day",
    y        = "Average Spot Price (PLN/kWh)",
    title    = "Best Hours for Deferrable Loads",
    subtitle = "Dashed lines = current peak usage hour per appliance",
    color    = "Peak usage"
  ) +
  theme_energy()

save_plot(p3, "14_best_hours.png")

} else {
  cat("No appliance cycles detected in recent data.\n")
}
