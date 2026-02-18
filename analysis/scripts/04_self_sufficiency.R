# ============================================================================
# 04_self_sufficiency.R — Battery Self-Sufficiency Curve
# ============================================================================
# WHAT:    Simulates a simple battery for self-consumption and plots what %
#          of hours can go off-grid at each battery capacity.
#
# INPUTS:  hourly (from load_data.R — hourly grid power with avg)
#
# OUTPUTS: output/04_self_sufficiency.png
#
# HOW TO READ:
#   - X axis = battery capacity in kWh
#   - Y axis = % of hours where the battery covered all grid import
#   - Steep initial rise = first few kWh are very valuable
#   - Flattening curve = diminishing returns from larger batteries
#   - Dashed lines at 50%, 75%, 90% for reference
# ============================================================================

source("analysis/helpers/load_data.R")

# Prepare data: net energy per hour (positive = need grid, negative = surplus)
# Since avg_power is in watts and each bucket is 1 hour, W × 1h = Wh directly.
data <- hourly |>
  filter(!is.na(avg_power)) |>
  mutate(
    net_wh    = avg_power,          # W * 1h = Wh
    import_wh = pmax(net_wh, 0),    # only the import portion
    export_wh = pmax(-net_wh, 0)    # only the export portion
  )

# Simulate battery at each capacity: charge from export, discharge for import.
# This is a simple sequential simulation — we loop through each hour and
# update the state of charge (SoC). The for-loop is necessary because each
# hour's SoC depends on the previous hour's result (sequential dependency).
battery_sizes <- seq(0, 20, by = 0.5)  # kWh

results <- map_dfr(battery_sizes, function(cap_kwh) {
  cap_wh        <- cap_kwh * 1000
  soc           <- cap_wh * 0.5   # start at 50% state of charge
  hours_offgrid <- 0
  total_hours   <- nrow(data)

  for (i in seq_len(total_hours)) {
    if (data$net_wh[i] > 0) {
      # Importing: try to discharge battery to cover demand
      discharge <- min(soc, data$net_wh[i])
      soc <- soc - discharge
      # If battery fully covered the import, count as off-grid
      if (discharge >= data$net_wh[i]) hours_offgrid <- hours_offgrid + 1
    } else {
      # Exporting: charge battery from surplus PV
      charge <- min(cap_wh - soc, data$export_wh[i])
      soc <- soc + charge
      hours_offgrid <- hours_offgrid + 1  # no grid needed this hour
    }
  }

  tibble(
    battery_kwh = cap_kwh,
    offgrid_pct = hours_offgrid / total_hours * 100
  )
})

# Print key thresholds
cat("\n=== Self-sufficiency by battery size ===\n")
results |> filter(battery_kwh %in% c(0, 2, 5, 8, 10, 15, 20)) |> print()

# Plot the self-sufficiency curve
p <- ggplot(results, aes(x = battery_kwh, y = offgrid_pct)) +
  geom_line(color = COLORS$export, linewidth = 1.5) +
  # Mark every 5 kWh with a dot for easy reading
  geom_point(data = results |> filter(battery_kwh %% 5 == 0),
             color = COLORS$export, size = 3) +
  geom_hline(yintercept = c(50, 75, 90), linetype = "dashed", color = "grey60") +
  scale_x_continuous(breaks = seq(0, 20, 2)) +
  scale_y_continuous(limits = c(0, 100)) +
  labs(
    x        = "Battery Capacity (kWh)",
    y        = "Hours Off-Grid (%)",
    title    = "Self-Sufficiency vs Battery Size",
    subtitle = "What % of hours can you avoid grid import entirely?"
  ) +
  theme_energy()

save_plot(p, "04_self_sufficiency.png")
