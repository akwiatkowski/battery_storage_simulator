# ============================================================================
# 07_power_duration.R — Import/Export Power Duration Curves
# ============================================================================
# WHAT:    Sorts peak power values highest-to-lowest and plots "duration
#          curves" — a classic power engineering visualization showing how
#          often a given power level is exceeded.
#
# INPUTS:  hourly (from load_data.R — hourly grid power with max/min)
#
# OUTPUTS: output/07_import_duration.png   — import-side duration curve
#          output/07_export_duration.png   — export-side duration curve
#          output/07_combined_duration.png — both on one chart
#
# HOW TO READ:
#   - X axis = "% of time exceeded" — reading left-to-right, you see the
#     most extreme peaks first, then progressively common power levels
#   - Y axis = peak power (W)
#   - Dashed lines at 2/3/5 kW help assess inverter sizing
#   - The combined chart shows whether import or export drives sizing
# ============================================================================

source("analysis/r/R/load_data.R")

data <- hourly |> filter(!is.na(max_power))

# Use compute_duration_curve() from helpers.R for import peaks
import_curve <- compute_duration_curve(data$max_power)

# For export: use abs(min_power) where min_power < 0
export_values <- data |> filter(min_power < 0) |> pull(min_power) |> abs()
export_curve  <- compute_duration_curve(export_values)

# Print summary tables
cat("\n=== Import Power Duration ===\n")
cat("Peak import:", max(import_curve$power), "W\n")
for (pct in c(1, 5, 10, 25, 50)) {
  w <- import_curve |> filter(pct_time >= pct) |> slice(1) |> pull(power)
  cat(paste0("  Top ", pct, "% of hours need > ", round(w), " W\n"))
}

cat("\n=== Export Power Duration ===\n")
cat("Peak export:", max(export_curve$power), "W\n")
for (pct in c(1, 5, 10, 25, 50)) {
  w <- export_curve |> filter(pct_time >= pct) |> slice(1) |> pull(power)
  cat(paste0("  Top ", pct, "% of hours export > ", round(w), " W\n"))
}

# Reference lines for common inverter sizes
kw_lines <- c(2000, 3000, 5000)

# --- Chart 1: Import duration curve ------------------------------------------
p1 <- ggplot(import_curve, aes(x = pct_time, y = power)) +
  geom_area(fill = COLORS$import, alpha = 0.3) +
  geom_line(color = COLORS$import, linewidth = 0.8) +
  geom_hline(yintercept = kw_lines, linetype = "dashed", color = COLORS$muted) +
  annotate("text", x = 95, y = kw_lines + 200,
           label = c("2 kW", "3 kW", "5 kW"), color = "grey40", hjust = 1) +
  labs(
    x        = "% of Time Exceeded",
    y        = "Peak Import Power (W)",
    title    = "Import Power Duration Curve",
    subtitle = "How often do you actually need a given power level?"
  ) +
  theme_energy()

save_plot(p1, "07_import_duration.png")

# --- Chart 2: Export duration curve ------------------------------------------
p2 <- ggplot(export_curve, aes(x = pct_time, y = power)) +
  geom_area(fill = COLORS$pv, alpha = 0.3) +
  geom_line(color = COLORS$pv, linewidth = 0.8) +
  geom_hline(yintercept = kw_lines, linetype = "dashed", color = COLORS$muted) +
  annotate("text", x = 95, y = kw_lines + 200,
           label = c("2 kW", "3 kW", "5 kW"), color = "grey40", hjust = 1) +
  labs(
    x        = "% of Time Exceeded",
    y        = "Peak Export Power (W)",
    title    = "Export Power Duration Curve",
    subtitle = "How often does your PV export exceed a given power level?"
  ) +
  theme_energy()

save_plot(p2, "07_export_duration.png")

# --- Chart 3: Combined import + export --------------------------------------
combined <- bind_rows(
  import_curve |> mutate(direction = "Import"),
  export_curve |> mutate(direction = "Export")
)

p3 <- ggplot(combined, aes(x = pct_time, y = power, color = direction, fill = direction)) +
  geom_area(alpha = 0.2) +
  geom_line(linewidth = 0.8) +
  scale_color_manual(values = c("Import" = COLORS$import, "Export" = COLORS$pv)) +
  scale_fill_manual(values = c("Import" = COLORS$import, "Export" = COLORS$pv)) +
  labs(
    x        = "% of Time Exceeded",
    y        = "Peak Power (W)",
    title    = "Power Duration Curves \u2014 Import & Export",
    subtitle = "What inverter size covers both directions?",
    color    = "",
    fill     = ""
  ) +
  theme_energy()

save_plot(p3, "07_combined_duration.png")
