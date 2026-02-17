# ============================================================================
# 05_export_clipping.R — PV Export Clipping Loss
# ============================================================================
# WHAT:    Quantifies how much PV energy is lost ("clipped") when the inverter
#          caps export power at various levels.
#
# INPUTS:  hourly (from load_data.R — hourly grid power with min_power)
#
# OUTPUTS: output/05_export_clipping.png     — % energy lost vs cap
#          output/05_export_distribution.png — histogram of peak export power
#
# HOW TO READ:
#   - Clipping curve: steeper drop at low caps means those watts matter most
#   - Distribution histogram: shows the peak export values across all hours
#   - Vertical lines at 2/3/5 kW show where common inverters clip
#   - If most hours are below 3 kW, a 3 kW inverter wastes very little
# ============================================================================

source("analysis/r/R/load_data.R")

# Negative min_power = peak export within that hour.
# abs() converts to positive watts for easier reasoning.
export_hours <- hourly |>
  filter(min_power < 0) |>
  mutate(
    peak_export_w = abs(min_power),
    avg_export_w  = abs(pmin(avg_power, 0))
  ) |>
  filter(peak_export_w > 0)

cat("Hours with export:", nrow(export_hours), "\n")
cat("Average peak export:", round(mean(export_hours$peak_export_w)), "W\n")
cat("Max peak export:", max(export_hours$peak_export_w), "W\n")

# Use compute_clipping() from helpers.R
clipping <- compute_clipping(export_hours$peak_export_w)

# Print key thresholds
cat("\n=== Energy lost at common inverter sizes ===\n")
clipping |>
  filter(cap_w %in% c(1000, 2000, 3000, 4000, 5000, 6000, 8000)) |>
  mutate(lost_kwh = round(clipped_wh / 1000, 1), pct_lost = round(pct_lost, 1)) |>
  select(cap_w, lost_kwh, pct_lost, pct_hours_clipped) |>
  print()

# --- Chart 1: Clipping loss curve --------------------------------------------
p1 <- ggplot(clipping, aes(x = cap_w, y = pct_lost)) +
  geom_area(fill = COLORS$pv, alpha = 0.3) +
  geom_line(color = COLORS$pv, linewidth = 1.2) +
  labs(
    x        = "Inverter Export Cap (W)",
    y        = "PV Energy Lost (%)",
    title    = "PV Export Clipping Loss",
    subtitle = "How much solar energy is wasted at each inverter power limit?"
  ) +
  theme_energy()

save_plot(p1, "05_export_clipping.png")

# --- Chart 2: Histogram of peak export power ---------------------------------
p2 <- ggplot(export_hours, aes(x = peak_export_w)) +
  geom_histogram(bins = 50, fill = COLORS$pv, color = "white") +
  geom_vline(xintercept = c(2000, 3000, 5000), linetype = "dashed",
             color = c(COLORS$export, COLORS$heat_pump, COLORS$import)) +
  annotate("text", x = 2100, y = Inf, vjust = 2, label = "2 kW", color = COLORS$export) +
  annotate("text", x = 3100, y = Inf, vjust = 2, label = "3 kW", color = COLORS$heat_pump) +
  annotate("text", x = 5100, y = Inf, vjust = 2, label = "5 kW", color = COLORS$import) +
  labs(
    x        = "Peak Export Power (W)",
    y        = "Number of Hours",
    title    = "Distribution of Peak Export Power",
    subtitle = "Vertical lines show common inverter sizes"
  ) +
  theme_energy()

save_plot(p2, "05_export_distribution.png")
