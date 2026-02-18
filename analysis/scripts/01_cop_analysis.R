# ============================================================================
# 01_cop_analysis.R — Heat Pump COP vs Outdoor Temperature
# ============================================================================
# WHAT:    Plots the heat pump's Coefficient of Performance (COP) against
#          outdoor temperature, with monthly and time-of-day breakdowns.
#
# INPUTS:  cop_data (from load_data.R — joined pump consumption, production,
#          and outdoor temperature)
#
# OUTPUTS: output/01_cop_vs_temp.png      — COP scatter with loess trend
#          output/01_cop_by_month.png     — COP faceted by calendar month
#          output/01_cop_by_time.png      — COP faceted by time of day
#
# HOW TO READ:
#   - Higher COP = more efficient (3.0 means 3 kW heat per 1 kW electricity)
#   - COP drops as outdoor temperature drops (heat pump works harder)
#   - The loess curve shows the average trend through the cloud of readings
# ============================================================================

source("analysis/helpers/load_data.R")

# --- Chart 1: Overall COP vs temperature ------------------------------------
# geom_bin2d creates a 2D histogram — color intensity shows reading density.
# geom_smooth adds a loess (locally weighted) regression line to show the
# average COP at each temperature.
p1 <- ggplot(cop_data, aes(x = temp, y = cop)) +
  geom_bin2d(bins = 40) +
  scale_fill_gradient(low = COLORS$bg, high = COLORS$heat_pump) +
  geom_smooth(method = "loess", span = 0.3, color = COLORS$text) +
  labs(
    x        = "Outdoor Temperature (\u00b0C)",
    y        = "COP",
    title    = "Heat Pump COP vs Temperature",
    subtitle = "Higher COP = more efficient | Drops sharply below 0\u00b0C"
  ) +
  theme_energy()

save_plot(p1, "01_cop_vs_temp.png")

# --- Chart 2: COP faceted by month ------------------------------------------
# facet_wrap creates a separate panel for each month, showing how the COP
# curve shifts seasonally. Winter months cluster at low temps with lower COP.
p2 <- ggplot(cop_data, aes(x = temp, y = cop)) +
  geom_bin2d(bins = 25) +
  scale_fill_gradient(low = COLORS$bg, high = COLORS$heat_pump) +
  geom_smooth(method = "loess", span = 0.5, color = COLORS$text) +
  facet_wrap(~month) +
  labs(
    x        = "Outdoor Temperature (\u00b0C)",
    y        = "COP",
    title    = "Heat Pump COP vs Temperature \u2014 by Month",
    subtitle = "Each panel shows one calendar month"
  ) +
  theme_energy()

save_plot(p2, "01_cop_by_month.png", width = 12, height = 8)

# --- Chart 3: COP faceted by time of day ------------------------------------
# Shows whether COP varies by time of day (e.g., morning defrost cycles,
# afternoon solar warming).
p3 <- ggplot(cop_data, aes(x = temp, y = cop)) +
  geom_bin2d(bins = 25) +
  scale_fill_gradient(low = COLORS$bg, high = COLORS$heat_pump) +
  geom_smooth(method = "loess", span = 0.5, color = COLORS$text) +
  facet_wrap(~time_of_day) +
  labs(
    x        = "Outdoor Temperature (\u00b0C)",
    y        = "COP",
    title    = "Heat Pump COP vs Temperature \u2014 by Time of Day",
    subtitle = "Morning defrost vs afternoon warmth"
  ) +
  theme_energy()

save_plot(p3, "01_cop_by_time.png", width = 12, height = 8)
