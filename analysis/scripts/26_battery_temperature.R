# ============================================================================
# 26_battery_temperature.R — Battery Feasibility: Workshop Temperature Analysis
# ============================================================================
# WHAT:    Analyzes whether an unheated, lightly insulated workshop (metal
#          garage) is suitable for battery storage year-round. Most LFP
#          batteries cannot charge below 0°C and cannot discharge below -10°C.
#
#          Uses measured workshop + outdoor temperature data to model:
#          1. Current (uninsulated) workshop daily minimum temps
#          2. Lightly insulated workshop (reduced thermal coupling)
#          3. Insulated + battery waste heat (sealed, unvented garage)
#          4. Insulated + battery heat + 50W constant heating pad
#
# INPUTS:  HP_OUTSIDE_TEMP (full year of outdoor data)
#          TEMP_WORKSHOP (workshop indoor temperature, may be shorter)
#
# OUTPUTS: output/26_battery_feasibility.png — annual temperature profile
#          output/26_battery_days_lost.png   — monthly days below thresholds
#          output/26_lost_pv_surplus.png     — PV surplus lost due to cold
#
# PHYSICS:
#   The daily minimum indoor temp depends on:
#     - Daily average outdoor temperature (baseline)
#     - Daily outdoor swing amplitude (avg - min)
#     - Damping factor: what fraction of the swing propagates indoors
#   damping = 1.0 → indoor min = outdoor min (no insulation)
#   damping = 0.0 → indoor min = outdoor avg (perfect insulation)
#
#   Heat source effect (steady-state):
#     ΔT = P / UA  where UA = thermal conductance of the envelope
#
# KEY ASSUMPTION:
#   Use MIN temperature (not average) — batteries need to survive the
#   coldest moment of each day, not the average.
# ============================================================================

source("analysis/helpers/load_data.R")

# ============================================================================
# Battery temperature thresholds
# ============================================================================
CHARGE_LIMIT    <- 0    # °C — most LFP batteries cannot charge below this
DISCHARGE_LIMIT <- -10  # °C — most LFP batteries cannot discharge below this

# ============================================================================
# Load outdoor and workshop temperature data
# ============================================================================
outdoor_raw <- load_stats_sensor(HP_OUTSIDE_TEMP) |>
  distinct(hour_bucket, .keep_all = TRUE)

workshop_raw <- load_stats_sensor(TEMP_WORKSHOP) |>
  distinct(hour_bucket, .keep_all = TRUE)

cat("\n=== Raw Data ===\n")
cat("  Outdoor temp hours: ", nrow(outdoor_raw), "\n")
cat("  Workshop temp hours:", nrow(workshop_raw), "\n")

if (nrow(outdoor_raw) < 100) {
  cat("Insufficient outdoor temperature data for annual analysis.\n")
  quit(save = "no")
}

# ============================================================================
# Daily aggregates — use min_val for worst-case within each hour
# ============================================================================
# min_val is the minimum reading within each hourly stats bucket.
# Taking min(min_val) across a day gives the lowest recorded temperature.
# If min_val is unavailable, fall back to avg.

outdoor_daily <- outdoor_raw |>
  mutate(
    date = as_date(hour_bucket),
    # Use min_val if available, otherwise avg
    hourly_min = ifelse(!is.na(min_val), min_val, avg)
  ) |>
  group_by(date) |>
  summarize(
    t_min = min(hourly_min, na.rm = TRUE),
    t_avg = mean(avg, na.rm = TRUE),
    t_max = max(ifelse(!is.na(max_val), max_val, avg), na.rm = TRUE),
    n_hours = n(),
    .groups = "drop"
  ) |>
  # Require at least 20 hours for a reliable daily min
  filter(n_hours >= 20, is.finite(t_min), is.finite(t_avg))

cat("\n=== Outdoor Daily Stats ===\n")
cat("  Days with data:    ", nrow(outdoor_daily), "\n")
cat("  Date range:        ", as.character(min(outdoor_daily$date)),
    "to", as.character(max(outdoor_daily$date)), "\n")
cat("  Coldest daily min: ", round(min(outdoor_daily$t_min), 1), "°C\n")
cat("  Mean daily avg:    ", round(mean(outdoor_daily$t_avg), 1), "°C\n")

workshop_daily <- workshop_raw |>
  mutate(
    date = as_date(hour_bucket),
    hourly_min = ifelse(!is.na(min_val), min_val, avg)
  ) |>
  group_by(date) |>
  summarize(
    ws_min = min(hourly_min, na.rm = TRUE),
    ws_avg = mean(avg, na.rm = TRUE),
    n_hours = n(),
    .groups = "drop"
  ) |>
  filter(n_hours >= 20, is.finite(ws_min), is.finite(ws_avg))

cat("  Workshop days:     ", nrow(workshop_daily), "\n")

# ============================================================================
# Estimate thermal damping from measured data
# ============================================================================
# Damping = fraction of outdoor daily swing that propagates indoors.
# damping = (outdoor_avg - workshop_min) / (outdoor_avg - outdoor_min)
# High damping (→1) = indoor follows outdoor closely (poor insulation)
# Low damping (→0)  = indoor stays near daily average (good insulation)

joint <- outdoor_daily |>
  inner_join(workshop_daily, by = "date") |>
  mutate(
    outdoor_swing = t_avg - t_min,   # how far min dips below avg
    indoor_swing  = ws_avg - ws_min   # NOTE: use workshop avg, not outdoor avg
  )

# Also compute simple offset for reference
joint <- joint |>
  mutate(
    offset = ws_min - t_min,  # how much warmer the workshop is than outside
    # For damping calculation, use outdoor swing as denominator
    # indoor_swing relative to outdoor_swing tells us coupling
    damping = ifelse(outdoor_swing > 1.0,
                     (t_avg - ws_min) / outdoor_swing,
                     NA_real_)
  )

# Use median to be robust to outliers
measured_damping <- median(joint$damping, na.rm = TRUE)
measured_offset  <- median(joint$offset, na.rm = TRUE)

cat("\n=== Thermal Damping (current uninsulated workshop) ===\n")
cat("  Measured days:      ", sum(!is.na(joint$damping)), "\n")
cat("  Median damping:     ", round(measured_damping, 3),
    " (1.0 = follows outdoor exactly)\n")
cat("  Median min offset:  ", round(measured_offset, 1),
    "°C (workshop min - outdoor min)\n")

# If no workshop data, use conservative default (metal garage ≈ 0.85)
if (is.na(measured_damping) || sum(!is.na(joint$damping)) < 5) {
  measured_damping <- 0.85
  cat("  Using default damping: 0.85 (insufficient measured data)\n")
}

# ============================================================================
# Insulation and heat source parameters
# ============================================================================
# "Insulated a bit" = 50mm foam boards on walls + roof of metal garage.
# This reduces UA by roughly 5-8x. The damping factor drops proportionally.
INSULATION_FACTOR <- 3  # damping reduces by this factor with insulation
insulated_damping <- measured_damping / INSULATION_FACTOR

cat("\n=== Insulation Model ===\n")
cat("  Current damping:    ", round(measured_damping, 3), "\n")
cat("  Insulated damping:  ", round(insulated_damping, 3),
    " (÷", INSULATION_FACTOR, ")\n")

# UA estimation for the insulated garage.
# Typical single-car metal garage (~18m² floor, ~60m² walls+roof):
#   - Uninsulated metal: U ≈ 6 W/m²K → UA ≈ 360 W/K
#   - With 50mm foam:   U ≈ 0.6 W/m²K → UA ≈ 36 W/K
#   - Floor (concrete/ground): ~18m² × 0.8 = 14 W/K
#   - Total insulated: ~50 W/K
UA_INSULATED <- 50  # W/K — thermal conductance after insulation

# Battery waste heat during charging:
# 10 kWh LFP battery charging at 3-5 kW, round-trip efficiency ~92-95%.
# During max charge: ~5% loss × 5000W = 250W as heat.
# During typical charge cycle (average): ~200-300W.
# Conservative estimate for consistent heat during operation.
BATTERY_HEAT_W <- 300  # W average during active charge/discharge

# Constant heating pad near batteries
HEATER_W <- 50  # W

battery_delta_t <- BATTERY_HEAT_W / UA_INSULATED
heater_delta_t  <- HEATER_W / UA_INSULATED

cat("  UA (insulated):     ", UA_INSULATED, "W/K\n")
cat("  Battery heat:       ", BATTERY_HEAT_W, "W →",
    round(battery_delta_t, 1), "°C steady-state rise\n")
cat("  Heater pad:         ", HEATER_W, "W →",
    round(heater_delta_t, 1), "°C steady-state rise\n")

# ============================================================================
# Predict workshop temperatures for all scenarios
# ============================================================================
# Model: indoor_min = outdoor_avg - damping × (outdoor_avg - outdoor_min)
#
# When damping = 1: indoor_min = outdoor_min (no buffering)
# When damping = 0: indoor_min = outdoor_avg (perfect buffering)
#
# Heat sources add a constant ΔT = P / UA to the insulated baseline.

predictions <- outdoor_daily |>
  mutate(
    doy = yday(date),
    month_label = month(date, label = TRUE),
    outdoor_swing = t_avg - t_min,

    # Scenario 1: Current uninsulated workshop
    current = t_avg - measured_damping * outdoor_swing,

    # Scenario 2: Insulated workshop, no heat
    insulated = t_avg - insulated_damping * outdoor_swing,

    # Scenario 3: Insulated + battery waste heat
    insulated_battery = insulated + battery_delta_t,

    # Scenario 4: Insulated + battery + 50W heater pad
    insulated_battery_heater = insulated_battery + heater_delta_t
  )

# Validate against measured data where available
if (nrow(joint) >= 5) {
  validation <- joint |>
    mutate(
      predicted_current = t_avg - measured_damping * (t_avg - t_min)
    )
  mae <- mean(abs(validation$predicted_current - validation$ws_min), na.rm = TRUE)
  cat("\n=== Model Validation ===\n")
  cat("  Validation days:     ", nrow(validation), "\n")
  cat("  MAE (predicted vs measured workshop min):", round(mae, 2), "°C\n")
}

# ============================================================================
# Summary statistics
# ============================================================================
cat("\n=== Days Below Battery Thresholds (annual) ===\n")

summarize_scenario <- function(temps, name) {
  days_no_charge    <- sum(temps < CHARGE_LIMIT, na.rm = TRUE)
  days_no_discharge <- sum(temps < DISCHARGE_LIMIT, na.rm = TRUE)
  total_days <- sum(!is.na(temps))
  cat(sprintf("  %-30s: %3d days no-charge (<0°C), %3d days no-discharge (<-10°C) [of %d]\n",
              name, days_no_charge, days_no_discharge, total_days))
}

summarize_scenario(predictions$t_min,                      "Outdoor (reference)")
summarize_scenario(predictions$current,                    "Current workshop")
summarize_scenario(predictions$insulated,                  "Insulated")
summarize_scenario(predictions$insulated_battery,          "Insulated + battery heat")
summarize_scenario(predictions$insulated_battery_heater,   "Insulated + battery + 50W")

# ============================================================================
# Chart 1: Annual temperature profile by day-of-year (averaged across years)
# ============================================================================
# Group by day-of-year (1-365) and average across years so we get one clean
# annual profile. Show only the most relevant scenarios as filled areas
# to highlight the problematic periods clearly.

scenario_levels <- c("Outdoor", "Current workshop", "Insulated",
                     "Insulated + battery", "Insulated + battery + 50W")
scenario_colors <- c(
  "Outdoor"                    = COLORS$muted,
  "Current workshop"           = COLORS$import,
  "Insulated"                  = COLORS$charge,
  "Insulated + battery"        = COLORS$pv,
  "Insulated + battery + 50W"  = COLORS$export
)

# Average each scenario by day-of-year
doy_avg <- predictions |>
  group_by(doy) |>
  summarize(
    outdoor       = mean(t_min, na.rm = TRUE),
    current       = mean(current, na.rm = TRUE),
    insulated     = mean(insulated, na.rm = TRUE),
    ins_battery   = mean(insulated_battery, na.rm = TRUE),
    ins_batt_heat = mean(insulated_battery_heater, na.rm = TRUE),
    .groups = "drop"
  ) |>
  arrange(doy) |>
  # Apply 7-day rolling average for smooth curves
  mutate(
    outdoor       = zoo::rollmean(outdoor,       k = 7, fill = NA, align = "center"),
    current       = zoo::rollmean(current,       k = 7, fill = NA, align = "center"),
    insulated     = zoo::rollmean(insulated,     k = 7, fill = NA, align = "center"),
    ins_battery   = zoo::rollmean(ins_battery,   k = 7, fill = NA, align = "center"),
    ins_batt_heat = zoo::rollmean(ins_batt_heat, k = 7, fill = NA, align = "center")
  ) |>
  filter(!is.na(outdoor))

# Build month tick positions: day-of-year for the 1st of each month
month_breaks <- tibble(
  month = 1:12,
  label = month.abb,
  doy = yday(as.Date(paste0("2025-", month, "-01")))
)

# Filled areas showing progressively warmer scenarios
# Outdoor → current → insulated → ins+batt → ins+batt+heat
# Each band shows the "gain" from that upgrade

p1 <- ggplot(doy_avg, aes(x = doy)) +
  # Area: outdoor to insulated+battery+heater (full improvement span, light)
  geom_ribbon(aes(ymin = outdoor, ymax = ins_batt_heat),
              fill = COLORS$export, alpha = 0.10) +
  # Area: outdoor to insulated+battery
  geom_ribbon(aes(ymin = outdoor, ymax = ins_battery),
              fill = COLORS$pv, alpha = 0.10) +
  # Area: outdoor to insulated
  geom_ribbon(aes(ymin = outdoor, ymax = insulated),
              fill = COLORS$charge, alpha = 0.10) +

  # Highlight: fill below 0°C for ins+batt+heat (days still blocked)
  geom_ribbon(
    data = doy_avg |> filter(ins_batt_heat < CHARGE_LIMIT),
    aes(ymin = ins_batt_heat, ymax = CHARGE_LIMIT),
    fill = COLORS$import, alpha = 0.35
  ) +

  # Lines
  geom_line(aes(y = outdoor, color = "Outdoor"),
            linewidth = 0.6, alpha = 0.7) +
  geom_line(aes(y = insulated, color = "Insulated"),
            linewidth = 0.8) +
  geom_line(aes(y = ins_battery, color = "Insulated + battery"),
            linewidth = 1.0) +
  geom_line(aes(y = ins_batt_heat, color = "Insulated + battery + 50W"),
            linewidth = 1.2) +

  # Threshold lines
  geom_hline(yintercept = CHARGE_LIMIT, linetype = "dashed",
             color = COLORS$charge, linewidth = 0.6) +
  geom_hline(yintercept = DISCHARGE_LIMIT, linetype = "dashed",
             color = COLORS$import, linewidth = 0.6) +

  # Threshold labels
  annotate("text", x = 200, y = CHARGE_LIMIT + 1,
           label = "Charge limit (0\u00b0C)", color = COLORS$charge,
           size = 3.5, fontface = "bold") +
  annotate("text", x = 200, y = DISCHARGE_LIMIT + 1,
           label = "Discharge limit (-10\u00b0C)", color = COLORS$import,
           size = 3.5, fontface = "bold") +

  # Problematic zone label
  {if (any(doy_avg$ins_batt_heat < CHARGE_LIMIT, na.rm = TRUE))
    annotate("text",
      x = median(doy_avg$doy[doy_avg$ins_batt_heat < CHARGE_LIMIT], na.rm = TRUE),
      y = min(doy_avg$ins_batt_heat, na.rm = TRUE) - 1.5,
      label = "Still too cold\nto charge",
      color = COLORS$import, size = 3.5, fontface = "italic")
  } +

  scale_color_manual(values = scenario_colors) +
  scale_x_continuous(
    breaks = month_breaks$doy,
    labels = month_breaks$label,
    expand = c(0.01, 0)
  ) +
  labs(
    x     = "Day of Year",
    y     = "Daily Minimum Temperature (\u00b0C)",
    title = "Battery Feasibility: Workshop Temperature Through the Year",
    subtitle = paste0(
      "7-day rolling average of daily min temps, averaged across years. ",
      "Battery heat: +", round(battery_delta_t, 1),
      "\u00b0C, heater: +", round(heater_delta_t, 1), "\u00b0C. ",
      "Red zone = still below charge limit."
    ),
    color = ""
  ) +
  theme_energy() +
  theme(legend.position = "bottom",
        legend.text = element_text(size = 10))

save_plot(p1, "26_battery_feasibility.png", width = 12, height = 7)

# ============================================================================
# Chart 2: Days below threshold per month — stacked comparison
# ============================================================================
monthly_summary <- predictions |>
  mutate(month_label = month(date, label = TRUE)) |>
  group_by(month_label) |>
  summarize(
    total_days = n(),
    # No-charge days (< 0°C) per scenario
    outdoor_no_charge          = sum(t_min < CHARGE_LIMIT, na.rm = TRUE),
    current_no_charge          = sum(current < CHARGE_LIMIT, na.rm = TRUE),
    insulated_no_charge        = sum(insulated < CHARGE_LIMIT, na.rm = TRUE),
    ins_battery_no_charge      = sum(insulated_battery < CHARGE_LIMIT, na.rm = TRUE),
    ins_batt_heat_no_charge    = sum(insulated_battery_heater < CHARGE_LIMIT, na.rm = TRUE),
    # No-discharge days (< -10°C) per scenario
    outdoor_no_discharge       = sum(t_min < DISCHARGE_LIMIT, na.rm = TRUE),
    current_no_discharge       = sum(current < DISCHARGE_LIMIT, na.rm = TRUE),
    insulated_no_discharge     = sum(insulated < DISCHARGE_LIMIT, na.rm = TRUE),
    ins_battery_no_discharge   = sum(insulated_battery < DISCHARGE_LIMIT, na.rm = TRUE),
    ins_batt_heat_no_discharge = sum(insulated_battery_heater < DISCHARGE_LIMIT, na.rm = TRUE),
    .groups = "drop"
  )

cat("\n=== Monthly No-Charge Days (< 0°C) ===\n")
print(monthly_summary |>
        select(month_label, total_days,
               outdoor_no_charge, current_no_charge,
               insulated_no_charge, ins_battery_no_charge,
               ins_batt_heat_no_charge))

# Pivot for plotting — show no-charge days by scenario
monthly_long <- monthly_summary |>
  select(month_label,
         Outdoor = outdoor_no_charge,
         `Current workshop` = current_no_charge,
         Insulated = insulated_no_charge,
         `Insulated + battery` = ins_battery_no_charge,
         `Insulated + battery + 50W` = ins_batt_heat_no_charge
  ) |>
  pivot_longer(
    cols = -month_label,
    names_to = "scenario",
    values_to = "days_no_charge"
  )

monthly_long$scenario <- factor(monthly_long$scenario, levels = scenario_levels)

p2 <- ggplot(monthly_long,
             aes(x = month_label, y = days_no_charge, fill = scenario)) +
  geom_col(position = "dodge", alpha = 0.8) +
  scale_fill_manual(values = scenario_colors) +
  labs(
    x     = "",
    y     = "Days Below 0°C (no charging)",
    title = "Monthly Days When Battery Cannot Charge",
    subtitle = "Number of days per month where daily minimum falls below 0°C.",
    fill  = ""
  ) +
  theme_energy() +
  theme(legend.text = element_text(size = 9))

save_plot(p2, "26_battery_days_lost.png", width = 12, height = 6)

# ============================================================================
# Print summary table
# ============================================================================
annual_totals <- monthly_summary |>
  summarize(
    across(ends_with("_no_charge"), sum),
    across(ends_with("_no_discharge"), sum)
  )

cat("\n=== ANNUAL SUMMARY ===\n")
cat(sprintf("  %-30s  No-charge  No-discharge\n", "Scenario"))
cat(sprintf("  %-30s  %3d days   %3d days\n", "Outdoor",
            annual_totals$outdoor_no_charge, annual_totals$outdoor_no_discharge))
cat(sprintf("  %-30s  %3d days   %3d days\n", "Current workshop",
            annual_totals$current_no_charge, annual_totals$current_no_discharge))
cat(sprintf("  %-30s  %3d days   %3d days\n", "Insulated",
            annual_totals$insulated_no_charge, annual_totals$insulated_no_discharge))
cat(sprintf("  %-30s  %3d days   %3d days\n", "Insulated + battery",
            annual_totals$ins_battery_no_charge, annual_totals$ins_battery_no_discharge))
cat(sprintf("  %-30s  %3d days   %3d days\n", "Insulated + battery + 50W",
            annual_totals$ins_batt_heat_no_charge, annual_totals$ins_batt_heat_no_discharge))

# ============================================================================
# Chart 3: PV surplus lost to cold — hourly cross-reference
# ============================================================================
# When PV exports (grid power < 0) and workshop is too cold to charge,
# that surplus energy is wasted. This quantifies the real cost.
#
# Hourly temperature model:
#   Fit workshop_avg ~ outdoor_avg from measured data, then apply insulation
#   and heat source offsets. Use hourly avg (not daily min) because PV export
#   is a daytime phenomenon — the minimum typically occurs at night.
#   However, for conservative battery safety, use min_val where available.

# Fit hourly model from overlapping workshop + outdoor data
hourly_joint <- outdoor_raw |>
  select(hour_bucket, outdoor_avg = avg, outdoor_min = min_val) |>
  inner_join(
    workshop_raw |> select(hour_bucket, ws_avg = avg, ws_min = min_val),
    by = "hour_bucket"
  ) |>
  filter(!is.na(outdoor_avg), !is.na(ws_avg))

if (nrow(hourly_joint) >= 20) {
  # Fit hourly model: workshop min = a × outdoor avg + b
  # Use outdoor avg (not min) because PV surplus happens during daytime
  # when outdoor temps are closer to average than minimum.
  # But predict workshop min (conservative for battery).
  hourly_model <- lm(ws_avg ~ outdoor_avg, data = hourly_joint)
  hourly_slope     <- coef(hourly_model)[2]
  hourly_intercept <- coef(hourly_model)[1]

  cat("\n=== Hourly Temperature Model ===\n")
  cat("  workshop_avg = ", round(hourly_slope, 3), " × outdoor_avg + ",
      round(hourly_intercept, 2), "\n")
  cat("  R² =", round(summary(hourly_model)$r.squared, 3), "\n")
} else {
  # Fallback: use daily damping model parameters applied to hourly data
  hourly_slope     <- 1 - (1 - measured_damping) * 0.5  # less damping at hourly
  hourly_intercept <- measured_offset * 0.7
  cat("\n=== Hourly Model (fallback from daily damping) ===\n")
  cat("  slope:", round(hourly_slope, 3), "  intercept:", round(hourly_intercept, 2), "\n")
}

# Build hourly dataset: outdoor temp + grid power + spot price
# Grid power < 0 means PV is exporting (surplus available for battery)
hourly_analysis <- outdoor_raw |>
  select(hour_bucket, outdoor_avg = avg) |>
  inner_join(
    hourly |> select(hour_bucket, grid_power = avg_power, price),
    by = "hour_bucket"
  ) |>
  filter(!is.na(outdoor_avg), !is.na(grid_power)) |>
  mutate(
    # PV surplus: only when grid is negative (exporting)
    pv_surplus_w = pmax(-grid_power, 0),
    has_surplus = pv_surplus_w > 0,

    # Predict workshop temp under each scenario
    ws_current   = hourly_slope * outdoor_avg + hourly_intercept,
    ws_insulated = outdoor_avg + (ws_current - outdoor_avg) * INSULATION_FACTOR,
    ws_ins_batt  = ws_insulated + battery_delta_t,
    ws_ins_batt_heat = ws_insulated + battery_delta_t + heater_delta_t,

    # Is charging blocked in each scenario?
    blocked_current   = ws_current < CHARGE_LIMIT,
    blocked_insulated = ws_insulated < CHARGE_LIMIT,
    blocked_ins_batt  = ws_ins_batt < CHARGE_LIMIT,
    blocked_ins_batt_heat = ws_ins_batt_heat < CHARGE_LIMIT,

    # Lost PV surplus (Wh) — energy that could have charged the battery
    # but was exported instead because battery is too cold
    lost_current_wh   = ifelse(has_surplus & blocked_current, pv_surplus_w, 0),
    lost_insulated_wh = ifelse(has_surplus & blocked_insulated, pv_surplus_w, 0),
    lost_ins_batt_wh  = ifelse(has_surplus & blocked_ins_batt, pv_surplus_w, 0),
    lost_ins_batt_heat_wh = ifelse(has_surplus & blocked_ins_batt_heat, pv_surplus_w, 0),

    # Lost value (PLN) — surplus × spot price (price is already PLN/kWh)
    price_pln_kwh = ifelse(!is.na(price), price, 0),
    lost_current_pln   = lost_current_wh / 1000 * price_pln_kwh,
    lost_insulated_pln = lost_insulated_wh / 1000 * price_pln_kwh,
    lost_ins_batt_pln  = lost_ins_batt_wh / 1000 * price_pln_kwh,
    lost_ins_batt_heat_pln = lost_ins_batt_heat_wh / 1000 * price_pln_kwh,

    month_label = month(hour_bucket, label = TRUE)
  )

cat("\n=== PV Surplus vs Cold Workshop ===\n")
cat("  Total hours analyzed: ", nrow(hourly_analysis), "\n")
cat("  Hours with PV surplus:", sum(hourly_analysis$has_surplus), "\n")
cat("  Total PV surplus:     ", round(sum(hourly_analysis$pv_surplus_w) / 1000, 1), "kWh\n")

# Summarize lost energy per scenario
lost_summary <- hourly_analysis |>
  summarize(
    hours_surplus = sum(has_surplus),
    total_surplus_kwh = sum(pv_surplus_w) / 1000,

    blocked_hours_current   = sum(has_surplus & blocked_current),
    blocked_hours_insulated = sum(has_surplus & blocked_insulated),
    blocked_hours_ins_batt  = sum(has_surplus & blocked_ins_batt),
    blocked_hours_ins_batt_heat = sum(has_surplus & blocked_ins_batt_heat),

    lost_kwh_current   = sum(lost_current_wh) / 1000,
    lost_kwh_insulated = sum(lost_insulated_wh) / 1000,
    lost_kwh_ins_batt  = sum(lost_ins_batt_wh) / 1000,
    lost_kwh_ins_batt_heat = sum(lost_ins_batt_heat_wh) / 1000,

    lost_pln_current   = sum(lost_current_pln),
    lost_pln_insulated = sum(lost_insulated_pln),
    lost_pln_ins_batt  = sum(lost_ins_batt_pln),
    lost_pln_ins_batt_heat = sum(lost_ins_batt_heat_pln)
  )

cat("\n=== Lost PV Surplus Due to Cold (annual) ===\n")
cat(sprintf("  %-30s  %6.1f kWh  %6.1f PLN  %4d hours blocked\n",
            "Current workshop",
            lost_summary$lost_kwh_current, lost_summary$lost_pln_current,
            lost_summary$blocked_hours_current))
cat(sprintf("  %-30s  %6.1f kWh  %6.1f PLN  %4d hours blocked\n",
            "Insulated",
            lost_summary$lost_kwh_insulated, lost_summary$lost_pln_insulated,
            lost_summary$blocked_hours_insulated))
cat(sprintf("  %-30s  %6.1f kWh  %6.1f PLN  %4d hours blocked\n",
            "Insulated + battery",
            lost_summary$lost_kwh_ins_batt, lost_summary$lost_pln_ins_batt,
            lost_summary$blocked_hours_ins_batt))
cat(sprintf("  %-30s  %6.1f kWh  %6.1f PLN  %4d hours blocked\n",
            "Insulated + battery + 50W",
            lost_summary$lost_kwh_ins_batt_heat, lost_summary$lost_pln_ins_batt_heat,
            lost_summary$blocked_hours_ins_batt_heat))

# Monthly breakdown of lost PV surplus
monthly_lost <- hourly_analysis |>
  group_by(month_label) |>
  summarize(
    surplus_kwh = sum(pv_surplus_w) / 1000,
    lost_current = sum(lost_current_wh) / 1000,
    lost_insulated = sum(lost_insulated_wh) / 1000,
    lost_ins_batt = sum(lost_ins_batt_wh) / 1000,
    lost_ins_batt_heat = sum(lost_ins_batt_heat_wh) / 1000,
    .groups = "drop"
  )

# Pivot for chart
monthly_lost_long <- monthly_lost |>
  select(month_label,
         `Current workshop` = lost_current,
         Insulated = lost_insulated,
         `Insulated + battery` = lost_ins_batt,
         `Insulated + battery + 50W` = lost_ins_batt_heat
  ) |>
  pivot_longer(
    cols = -month_label,
    names_to = "scenario",
    values_to = "lost_kwh"
  )

# Reuse scenario ordering (without "Outdoor" which isn't relevant here)
pv_scenario_levels <- c("Current workshop", "Insulated",
                        "Insulated + battery", "Insulated + battery + 50W")
monthly_lost_long$scenario <- factor(monthly_lost_long$scenario,
                                     levels = pv_scenario_levels)

pv_scenario_colors <- scenario_colors[pv_scenario_levels]

# Add total PV surplus as reference bars behind
p3 <- ggplot() +
  # Background: total PV surplus per month (light grey)
  geom_col(data = monthly_lost,
           aes(x = month_label, y = surplus_kwh),
           fill = COLORS$border, alpha = 0.5, width = 0.85) +
  # Lost surplus per scenario
  geom_col(data = monthly_lost_long |> filter(lost_kwh > 0),
           aes(x = month_label, y = lost_kwh, fill = scenario),
           position = "dodge", alpha = 0.85) +
  scale_fill_manual(values = pv_scenario_colors) +
  labs(
    x     = "",
    y     = "PV Surplus (kWh)",
    title = "PV Surplus Lost to Cold Battery Restrictions",
    subtitle = paste0(
      "Grey = total monthly PV surplus. Colored = surplus lost because workshop < 0°C. ",
      "Total surplus: ", round(lost_summary$total_surplus_kwh, 0), " kWh."
    ),
    fill  = ""
  ) +
  theme_energy() +
  theme(legend.text = element_text(size = 9))

save_plot(p3, "26_lost_pv_surplus.png", width = 12, height = 6)
