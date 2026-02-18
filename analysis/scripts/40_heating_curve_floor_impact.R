# ============================================================================
# 40_heating_curve_floor_impact.R — Heating Curve Impact on Floor Temperatures
# ============================================================================
# WHAT:    Analyzes how the heating curve (water supply temperature) affects
#          room temperatures on ground floor vs first floor. Models the impact
#          of lowering the curve to understand if first floor would get too cold.
#
# INPUTS:  load_stats_sensor() for room temps, HP water supply temp, outdoor temp
#
# OUTPUTS: docs/analysis/40_curve_sensitivity.png    — room temp vs supply temp
#          docs/analysis/40_curve_reduction.png      — predicted impact of curve change
#          docs/analysis/40_floor_sensitivity.png    — sensitivity by floor
#          docs/analysis/40_cold_risk.png            — hours below comfort threshold
# ============================================================================

source("analysis/helpers/load_data.R")

# ============================================================================
# Define floor groupings
# ============================================================================
ground_floor <- list(
  "Living Room" = NETATMO_LIVING_TEMP,
  "Kitchen"     = TEMP_KITCHEN,
  "Olek"        = TEMP_OFFICE1,
  "Beata"       = TEMP_OFFICE2
)

first_floor <- list(
  "Bathroom"  = TEMP_BATHROOM,
  "Bedroom 1" = TEMP_BEDROOM1,
  "Bedroom 2" = TEMP_BEDROOM2
)

all_rooms <- c(ground_floor, first_floor)
floor_map <- c(
  "Living Room" = "Ground Floor", "Kitchen" = "Ground Floor",
  "Olek" = "Ground Floor", "Beata" = "Ground Floor",
  "Bathroom" = "First Floor", "Bedroom 1" = "First Floor",
  "Bedroom 2" = "First Floor"
)

# ============================================================================
# Load HP supply temperature and outdoor temperature
# ============================================================================
supply_temp <- load_stats_sensor(HP_OUTLET_TEMP)
outdoor_temp <- load_stats_sensor(HP_OUTSIDE_TEMP)

cat("\n=== Heating Curve Data ===\n")
cat("Supply temp readings:", nrow(supply_temp), "\n")
cat("Outdoor temp readings:", nrow(outdoor_temp), "\n")

if (nrow(supply_temp) < 100 || nrow(outdoor_temp) < 100) {
  cat("Insufficient HP data for heating curve analysis.\n")
  quit(save = "no")
}

# ============================================================================
# Load all room data and join with HP data
# ============================================================================
room_data <- map2(names(all_rooms), all_rooms, function(name, sid) {
  df <- load_stats_sensor(sid)
  if (nrow(df) == 0) return(tibble())
  df |>
    mutate(room = name, floor = floor_map[name]) |>
    select(hour_bucket, room_temp = avg, room, floor)
}) |> bind_rows()

# Join everything on hour_bucket
hp_data <- supply_temp |>
  select(hour_bucket, supply = avg) |>
  distinct(hour_bucket, .keep_all = TRUE) |>
  inner_join(
    outdoor_temp |> select(hour_bucket, outdoor = avg) |> distinct(hour_bucket, .keep_all = TRUE),
    by = "hour_bucket"
  )

combined <- room_data |>
  inner_join(hp_data, by = "hour_bucket") |>
  filter(
    !is.na(room_temp), !is.na(supply), !is.na(outdoor),
    outdoor < 15,   # heating season only
    supply > 25     # HP actually running (not idle)
  )

cat("Combined observations:", nrow(combined), "\n")
cat("Rooms:", paste(unique(combined$room), collapse = ", "), "\n")

if (nrow(combined) < 200) {
  cat("Insufficient combined data for curve impact analysis.\n")
  quit(save = "no")
}

# ============================================================================
# Chart 1: Room temperature sensitivity to supply temperature
# ============================================================================
# For each room, fit: room_temp ~ supply + outdoor
# The supply coefficient tells us °C room change per °C supply change

models <- combined |>
  group_by(room, floor) |>
  summarize(
    coef_supply = coef(lm(room_temp ~ supply + outdoor))[["supply"]],
    coef_outdoor = coef(lm(room_temp ~ supply + outdoor))[["outdoor"]],
    r_squared = summary(lm(room_temp ~ supply + outdoor))$r.squared,
    mean_temp = mean(room_temp, na.rm = TRUE),
    n_obs = n(),
    .groups = "drop"
  ) |>
  arrange(desc(coef_supply))

cat("\n=== Room Sensitivity to Supply Temperature ===\n")
cat("(°C room temp change per °C supply temp change, controlling for outdoor temp)\n")
models |>
  mutate(
    coef_supply = round(coef_supply, 3),
    coef_outdoor = round(coef_outdoor, 3),
    r_squared = round(r_squared, 3)
  ) |>
  print()

p1 <- ggplot(models, aes(
  x = reorder(room, coef_supply),
  y = coef_supply,
  fill = floor
)) +
  geom_col(alpha = 0.7, width = 0.6) +
  coord_flip() +
  scale_fill_manual(values = c(
    "Ground Floor" = COLORS$charge,
    "First Floor"  = COLORS$import
  )) +
  labs(
    x     = "",
    y     = "Sensitivity (°C room / °C supply)",
    title = "Room Temperature Sensitivity to Heating Curve",
    subtitle = "Higher = more affected by supply temperature change. From linear model controlling for outdoor temp.",
    fill  = ""
  ) +
  theme_energy()

save_plot(p1, "40_curve_sensitivity.png")

# ============================================================================
# Chart 2: Predicted impact of lowering heating curve
# ============================================================================
# Simulate: if supply temp is reduced by 1, 2, 3, 5°C, what happens to each room?
reductions <- c(1, 2, 3, 5)

impact <- expand_grid(room = models$room, reduction = reductions) |>
  left_join(models |> select(room, floor, coef_supply, mean_temp), by = "room") |>
  mutate(
    temp_drop = coef_supply * reduction,
    predicted_temp = mean_temp - temp_drop
  )

p2 <- ggplot(impact, aes(
  x = factor(reduction),
  y = temp_drop,
  color = room,
  group = room
)) +
  geom_line(linewidth = 1) +
  geom_point(size = 2.5) +
  facet_wrap(~floor) +
  scale_color_brewer(palette = "Set2") +
  labs(
    x     = "Heating Curve Reduction (°C)",
    y     = "Predicted Room Temperature Drop (°C)",
    title = "Impact of Lowering Heating Curve on Room Temperatures",
    subtitle = "Based on linear regression: room_temp ~ supply_temp + outdoor_temp",
    color = ""
  ) +
  theme_energy()

save_plot(p2, "40_curve_reduction.png")

# ============================================================================
# Chart 3: Floor-level sensitivity comparison
# ============================================================================
floor_sensitivity <- models |>
  group_by(floor) |>
  summarize(
    mean_sensitivity = mean(coef_supply),
    min_sensitivity = min(coef_supply),
    max_sensitivity = max(coef_supply),
    mean_temp = mean(mean_temp),
    .groups = "drop"
  )

# Show predicted temp at different curve reductions, by floor
floor_impact <- expand_grid(floor = c("Ground Floor", "First Floor"), reduction = 0:5) |>
  left_join(floor_sensitivity, by = "floor") |>
  mutate(predicted_temp = mean_temp - mean_sensitivity * reduction)

p3 <- ggplot(floor_impact, aes(x = reduction, y = predicted_temp, color = floor)) +
  geom_line(linewidth = 1.2) +
  geom_point(size = 2.5) +
  geom_hline(yintercept = 20, linetype = "dashed", color = COLORS$warning, linewidth = 0.5) +
  annotate("text", x = 0.2, y = 19.7, label = "20°C comfort minimum",
           color = COLORS$warning, size = 3, hjust = 0) +
  geom_hline(yintercept = 18, linetype = "dotted", color = COLORS$import, linewidth = 0.5) +
  annotate("text", x = 0.2, y = 17.7, label = "18°C discomfort threshold",
           color = COLORS$import, size = 3, hjust = 0) +
  scale_color_manual(values = c(
    "Ground Floor" = COLORS$charge,
    "First Floor"  = COLORS$import
  )) +
  scale_x_continuous(breaks = 0:5) +
  labs(
    x     = "Heating Curve Reduction (°C)",
    y     = "Predicted Average Temperature (°C)",
    title = "Floor Temperature vs Heating Curve Reduction",
    subtitle = "Which floor hits comfort limits first when curve is lowered?",
    color = ""
  ) +
  theme_energy()

save_plot(p3, "40_floor_sensitivity.png")

# ============================================================================
# Chart 4: Cold risk — hours below thresholds at different curve reductions
# ============================================================================
# Use actual hourly data to estimate how many hours would fall below thresholds
thresholds <- c(20, 19, 18)

cold_risk <- map(reductions, function(red) {
  combined |>
    mutate(
      # Predict adjusted temp: current - (sensitivity × reduction)
      adj_temp = room_temp - {
        s <- models$coef_supply[match(room, models$room)]
        s * red
      }
    ) |>
    group_by(room, floor) |>
    summarize(
      reduction = red,
      below_20 = sum(adj_temp < 20) / n() * 100,
      below_19 = sum(adj_temp < 19) / n() * 100,
      below_18 = sum(adj_temp < 18) / n() * 100,
      .groups = "drop"
    )
}) |> bind_rows()

# Add baseline (no reduction)
baseline <- combined |>
  group_by(room, floor) |>
  summarize(
    reduction = 0,
    below_20 = sum(room_temp < 20) / n() * 100,
    below_19 = sum(room_temp < 19) / n() * 100,
    below_18 = sum(room_temp < 18) / n() * 100,
    .groups = "drop"
  )

cold_risk <- bind_rows(baseline, cold_risk) |>
  pivot_longer(
    cols = starts_with("below_"),
    names_to = "threshold",
    values_to = "pct_hours"
  ) |>
  mutate(threshold = case_when(
    threshold == "below_20" ~ "<20°C",
    threshold == "below_19" ~ "<19°C",
    threshold == "below_18" ~ "<18°C"
  ))

# Focus on <20°C threshold — most relevant
cold_20 <- cold_risk |>
  filter(threshold == "<20°C")

p4 <- ggplot(cold_20, aes(x = factor(reduction), y = pct_hours, fill = room)) +
  geom_col(position = position_dodge(width = 0.7), alpha = 0.8, width = 0.6) +
  facet_wrap(~floor) +
  scale_fill_brewer(palette = "Set2") +
  labs(
    x     = "Heating Curve Reduction (°C)",
    y     = "% of Heating Hours Below 20°C",
    title = "Cold Risk: Hours Below 20°C at Different Curve Reductions",
    subtitle = "Heating season only (outdoor <15°C, HP running). First floor rooms at higher risk.",
    fill  = ""
  ) +
  theme_energy()

save_plot(p4, "40_cold_risk.png")

# ============================================================================
# Summary
# ============================================================================
cat("\n=== Heating Curve Impact Summary ===\n")
cat("\nFloor sensitivity (°C room drop per °C curve reduction):\n")
print(floor_sensitivity |> select(floor, mean_sensitivity, mean_temp) |>
        mutate(across(where(is.numeric), ~round(., 3))))

cat("\nSafe curve reduction (keeping all rooms ≥20°C):\n")
models |>
  mutate(
    margin = mean_temp - 20,
    safe_reduction = margin / coef_supply
  ) |>
  select(room, floor, mean_temp, coef_supply, safe_reduction) |>
  mutate(across(where(is.numeric), ~round(., 1))) |>
  arrange(safe_reduction) |>
  print()
