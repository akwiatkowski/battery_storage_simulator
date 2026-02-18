# ============================================================================
# 12_pv_self_consumption.R — PV Self-Consumption Decomposition
# ============================================================================
# WHAT:    Decomposes energy flow into self-consumed PV, exported PV, and
#          grid import. Answers: "how much of my solar do I actually use?"
#
# INPUTS:  legacy_pv, legacy_grid (from load_data.R)
#
# OUTPUTS: output/12_self_consumption_monthly.png — monthly breakdown
#          output/12_self_consumption_hourly.png  — hour-of-day profile
#          output/12_pv_utilization.png           — PV utilization rate over time
#
# HOW TO READ:
#   - Monthly stacked bars: green = self-consumed PV (used directly),
#     golden = exported PV (sent to grid). Higher green = better utilization
#   - Hourly profile: shows when PV is consumed vs exported during the day
#   - Utilization rate: % of PV generation that was self-consumed each month
# ============================================================================

source("analysis/helpers/load_data.R")

# Join PV generation with grid power by timestamp.
# Grid power: positive = import, negative = export.
# PV power: positive = generating.
# Self-consumed PV = PV generation that didn't leave the house.
pv_grid <- legacy_pv |>
  inner_join(legacy_grid |> select(timestamp, grid_power = power), by = "timestamp") |>
  filter(pv_power >= 0) |>
  mutate(
    # When grid is negative (exporting), the exported amount is abs(grid_power).
    # Self-consumed PV = total PV - what was exported.
    exported_pv = pmax(-grid_power, 0),  # only export portion (W)
    # Cap at PV generation (can't export more than we generate)
    exported_pv = pmin(exported_pv, pv_power),
    self_consumed_pv = pv_power - exported_pv,
    # Grid import is the positive grid power portion
    grid_import = pmax(grid_power, 0)
  )

cat("Matched PV+grid readings:", nrow(pv_grid), "\n")
cat("Total PV:", round(sum(pv_grid$pv_power) / 1000, 1), "kWh\n")
cat("Self-consumed:", round(sum(pv_grid$self_consumed_pv) / 1000, 1), "kWh\n")
cat("Exported:", round(sum(pv_grid$exported_pv) / 1000, 1), "kWh\n")
cat("Self-consumption rate:",
    round(sum(pv_grid$self_consumed_pv) / sum(pv_grid$pv_power) * 100, 1), "%\n")

# --- Chart 1: Monthly energy breakdown (stacked bar) ------------------------
monthly <- pv_grid |>
  mutate(month = floor_date(timestamp, "month")) |>
  group_by(month) |>
  summarize(
    self_consumed_kwh = sum(self_consumed_pv) / 1000,
    exported_kwh      = sum(exported_pv) / 1000,
    grid_import_kwh   = sum(grid_import) / 1000,
    total_pv_kwh      = sum(pv_power) / 1000,
    .groups           = "drop"
  ) |>
  # Reshape for stacked bar
  pivot_longer(
    cols      = c(self_consumed_kwh, exported_kwh),
    names_to  = "type",
    values_to = "kwh"
  ) |>
  mutate(type = recode(type,
    "self_consumed_kwh" = "Self-consumed",
    "exported_kwh"      = "Exported"
  ))

p1 <- ggplot(monthly, aes(x = month, y = kwh, fill = type)) +
  geom_col() +
  scale_fill_manual(values = c("Self-consumed" = COLORS$export,
                               "Exported"      = COLORS$pv)) +
  labs(
    x        = "",
    y        = "Energy (kWh)",
    title    = "Monthly PV Breakdown \u2014 Self-Consumed vs Exported",
    subtitle = "Green = used by the house, golden = sent to grid",
    fill     = ""
  ) +
  theme_energy()

save_plot(p1, "12_self_consumption_monthly.png")

# --- Chart 2: Hour-of-day self-consumption profile ---------------------------
hourly_sc <- pv_grid |>
  mutate(hour = hour(timestamp)) |>
  group_by(hour) |>
  summarize(
    avg_self_consumed = mean(self_consumed_pv),
    avg_exported      = mean(exported_pv),
    avg_pv            = mean(pv_power),
    avg_import        = mean(grid_import),
    .groups           = "drop"
  )

hourly_long <- hourly_sc |>
  pivot_longer(
    cols      = c(avg_self_consumed, avg_exported),
    names_to  = "type",
    values_to = "watts"
  ) |>
  mutate(type = recode(type,
    "avg_self_consumed" = "Self-consumed PV",
    "avg_exported"      = "Exported PV"
  ))

p2 <- ggplot(hourly_long, aes(x = hour, y = watts, fill = type)) +
  geom_area(alpha = 0.7) +
  scale_fill_manual(values = c("Self-consumed PV" = COLORS$export,
                               "Exported PV"      = COLORS$pv)) +
  scale_x_continuous(breaks = 0:23) +
  labs(
    x        = "Hour of Day",
    y        = "Average Power (W)",
    title    = "PV Self-Consumption by Hour of Day",
    subtitle = "Midday surplus is exported; morning/evening PV is consumed directly",
    fill     = ""
  ) +
  theme_energy()

save_plot(p2, "12_self_consumption_hourly.png")

# --- Chart 3: Monthly self-consumption rate ----------------------------------
monthly_rate <- pv_grid |>
  mutate(month = floor_date(timestamp, "month")) |>
  group_by(month) |>
  summarize(
    total_pv        = sum(pv_power),
    self_consumed   = sum(self_consumed_pv),
    rate            = self_consumed / total_pv * 100,
    .groups         = "drop"
  ) |>
  filter(total_pv > 0)

p3 <- ggplot(monthly_rate, aes(x = month, y = rate)) +
  geom_col(fill = COLORS$export, alpha = 0.7) +
  geom_hline(yintercept = c(25, 50, 75), linetype = "dashed", color = COLORS$muted) +
  scale_y_continuous(limits = c(0, 100)) +
  labs(
    x        = "",
    y        = "Self-Consumption Rate (%)",
    title    = "PV Utilization Rate by Month",
    subtitle = "How much of your solar generation do you actually use?"
  ) +
  theme_energy()

save_plot(p3, "12_pv_utilization.png")
