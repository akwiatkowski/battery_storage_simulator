# ============================================================================
# 02_grid_heatmap.R — Weekday x Hour Grid Power Heatmap
# ============================================================================
# WHAT:    Creates a heatmap showing average grid power by day of week and
#          hour of day. Blue/green = exporting to grid, red = importing.
#
# INPUTS:  grid_legacy (from load_data.R — legacy grid power readings with
#          hour and weekday columns)
#
# OUTPUTS: output/02_grid_heatmap.png
#
# HOW TO READ:
#   - Green cells = net export (PV generation exceeds consumption)
#   - White cells = near zero (balanced)
#   - Red/coral cells = net import (consuming from grid)
#   - Look for the midday green band (PV export) and evening red band (cooking,
#     heating, no sun)
# ============================================================================

source("analysis/helpers/load_data.R")

# Compute average power for each (weekday, hour) combination.
# group_by splits the data into 7 × 24 = 168 groups, and summarize
# computes the mean within each group.
heatmap_data <- grid_legacy |>
  group_by(weekday, hour) |>
  summarize(avg_power = mean(power), .groups = "drop")

# geom_tile draws one rectangle per (weekday, hour) cell.
# scale_fill_power() is our custom diverging scale: green ↔ white ↔ coral.
p <- ggplot(heatmap_data, aes(x = hour, y = weekday, fill = avg_power)) +
  geom_tile() +
  scale_fill_power() +
  scale_x_continuous(breaks = 0:23) +
  labs(
    x     = "Hour of Day",
    y     = "",
    title = "Average Grid Power \u2014 Weekday \u00d7 Hour",
    subtitle = "Green = export (PV surplus) | Coral = import (grid consumption)"
  ) +
  theme_energy()

save_plot(p, "02_grid_heatmap.png", width = 12, height = 5)
