# ============================================================================
# 11_hp_delta_t.R — Heat Pump Temperature Lift & Efficiency
# ============================================================================
# WHAT:    Analyzes the relationship between water temperature lift (delta-T
#          = outlet - inlet) and heat pump efficiency. Higher lifts mean the
#          compressor works harder, lowering COP.
#
# INPUTS:  legacy_inlet_temp, legacy_outlet_temp, legacy_heat_consumed,
#          legacy_total_prod, legacy_cwu, legacy_ext_temp (from load_data.R)
#
# OUTPUTS: output/11_delta_t_vs_outdoor.png   — temperature lift vs outdoor temp
#          output/11_cop_vs_delta_t.png        — COP as function of delta-T
#          output/11_heating_vs_dhw.png        — heating vs DHW mode comparison
#
# HOW TO READ:
#   - Delta-T vs outdoor: shows how the pump adjusts water temp to match load
#   - COP vs delta-T: efficiency drops with higher temperature lifts
#   - Heating vs DHW: DHW cycles require high delta-T (to reach 45-55°C),
#     so they are less efficient than space heating
# ============================================================================

source("analysis/helpers/load_data.R")

# Join inlet and outlet temperatures with consumption and outdoor temp
hp_temps <- legacy_inlet_temp |>
  inner_join(legacy_outlet_temp, by = "timestamp") |>
  inner_join(legacy_ext_temp, by = "timestamp") |>
  mutate(
    delta_t = outlet_temp - inlet_temp   # temperature lift in °C
  ) |>
  filter(
    delta_t > 0, delta_t < 30,          # physically plausible range
    inlet_temp > 10, inlet_temp < 60,   # filter out sensor errors
    outlet_temp > 15, outlet_temp < 70
  )

cat("HP temperature readings:", nrow(hp_temps), "\n")
cat("Avg delta-T:", round(mean(hp_temps$delta_t), 1), "°C\n")
cat("Avg inlet:", round(mean(hp_temps$inlet_temp), 1), "°C\n")
cat("Avg outlet:", round(mean(hp_temps$outlet_temp), 1), "°C\n")

# --- Chart 1: Delta-T vs outdoor temperature --------------------------------
# As outdoor temp drops, the pump needs a higher outlet temp to maintain
# comfort, which increases delta-T.
p1 <- ggplot(hp_temps, aes(x = temp, y = delta_t)) +
  geom_bin2d(bins = 40) +
  scale_fill_gradient(low = COLORS$bg, high = COLORS$charge) +
  geom_smooth(method = "loess", span = 0.3, color = COLORS$text,
              linewidth = 1) +
  labs(
    x        = "Outdoor Temperature (\u00b0C)",
    y        = "Water Delta-T (\u00b0C, outlet \u2212 inlet)",
    title    = "Temperature Lift vs Outdoor Temperature",
    subtitle = "Colder weather \u2192 higher temperature lift \u2192 harder work for compressor"
  ) +
  theme_energy()

save_plot(p1, "11_delta_t_vs_outdoor.png")

# --- Chart 2: COP vs delta-T ------------------------------------------------
# Join with consumption + production to compute COP at each delta-T level
cop_delta <- hp_temps |>
  inner_join(legacy_heat_consumed, by = "timestamp") |>
  inner_join(legacy_total_prod, by = "timestamp") |>
  filter(consumption > 0, production > 0) |>
  mutate(cop = production / consumption) |>
  filter(cop > 0.8, cop < 10.0)

# Bin delta-T into 1°C buckets for cleaner visualization
cop_by_dt <- cop_delta |>
  mutate(dt_bin = round(delta_t)) |>
  group_by(dt_bin) |>
  summarize(
    avg_cop   = mean(cop),
    median_cop = median(cop),
    n         = n(),
    .groups   = "drop"
  ) |>
  filter(n >= 10)  # need enough readings to be meaningful

p2 <- ggplot(cop_by_dt, aes(x = dt_bin, y = avg_cop)) +
  geom_col(fill = COLORS$export, alpha = 0.7) +
  geom_line(aes(y = median_cop), color = COLORS$text, linewidth = 1) +
  geom_text(aes(label = n), vjust = -0.5, size = 2.5, color = COLORS$muted) +
  labs(
    x        = "Water Temperature Lift (\u00b0C)",
    y        = "Average COP",
    title    = "COP vs Temperature Lift",
    subtitle = "Bars = mean COP, line = median, numbers = sample count per bin"
  ) +
  theme_energy()

save_plot(p2, "11_cop_vs_delta_t.png")

# --- Chart 3: Heating vs DHW mode -------------------------------------------
# DHW (domestic hot water) cycles typically have:
#   - Higher outlet temp (45-55°C vs 25-35°C for heating)
#   - Higher delta-T
#   - Lower COP
# We use DHW consumption sensor to classify modes.
dhw_times <- legacy_cwu |>
  filter(cwu_power > 100) |>  # DHW cycle active when power > 100W
  select(timestamp)

cop_mode <- cop_delta |>
  mutate(
    # Classify as DHW if DHW sensor was active at that timestamp
    mode = if_else(timestamp %in% dhw_times$timestamp, "DHW", "Heating")
  )

mode_summary <- cop_mode |>
  group_by(mode) |>
  summarize(
    count        = n(),
    avg_cop      = mean(cop),
    avg_delta_t  = mean(delta_t),
    avg_outlet   = mean(outlet_temp),
    avg_inlet    = mean(inlet_temp),
    .groups      = "drop"
  )

cat("\n=== Heating vs DHW ===\n")
print(mode_summary)

p3 <- ggplot(cop_mode, aes(x = delta_t, y = cop, color = mode)) +
  geom_point(alpha = 0.05, size = 0.5) +
  geom_smooth(method = "loess", span = 0.5, linewidth = 1.2, se = FALSE) +
  scale_color_manual(values = c("Heating" = COLORS$charge, "DHW" = COLORS$heat_pump)) +
  labs(
    x        = "Temperature Lift (\u00b0C)",
    y        = "COP",
    title    = "Efficiency by Operating Mode",
    subtitle = "DHW requires higher temperature lifts \u2192 lower COP",
    color    = ""
  ) +
  theme_energy()

save_plot(p3, "11_heating_vs_dhw.png")
