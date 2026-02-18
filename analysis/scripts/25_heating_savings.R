# ============================================================================
# 25_heating_savings.R — Heating Energy, Temperature Savings & Cooling Projections
# ============================================================================
# WHAT:    Three analyses:
#          1) Heating energy (kWh/month) vs outdoor temperature at 0.1°C resolution
#          2) Weekly savings from reducing indoor temp by -1/-2/-3°C or enforcing 21°C
#          3) Cooling energy projections: current (+1.4°C) vs +2.7°C global warming
#
# INPUTS:  load_stats_sensor() for HP heating power, COP, outdoor temp,
#          indoor room temps, spot prices
#
# OUTPUTS: output/25_heating_energy_curve.png  — kWh/month vs outdoor temp
#          output/25_weekly_savings_kwh.png    — energy savings per scenario
#          output/25_weekly_savings_pln.png    — cost savings per scenario
#          output/25_cooling_energy.png        — annual cooling by target temp
#          output/25_cooling_monthly.png       — monthly cooling breakdown
# ============================================================================

source("analysis/helpers/load_data.R")

# ============================================================================
# Load data
# ============================================================================
hp_heat      <- load_stats_sensor(HP_HEAT_POWER) |> distinct(hour_bucket, .keep_all = TRUE)
hp_cons      <- load_stats_sensor(HP_CONSUMPTION) |> distinct(hour_bucket, .keep_all = TRUE)
outdoor_temp <- load_stats_sensor(HP_OUTSIDE_TEMP) |> distinct(hour_bucket, .keep_all = TRUE)

# Indoor temps — average across heated rooms
room_sensors <- list(
  "Bedroom 1" = TEMP_BEDROOM1,
  "Bedroom 2" = TEMP_BEDROOM2,
  "Kitchen"   = TEMP_KITCHEN,
  "Office 1"  = TEMP_OFFICE1,
  "Office 2"  = TEMP_OFFICE2,
  "Bathroom"  = TEMP_BATHROOM
)

room_data <- map2(names(room_sensors), room_sensors, function(name, sid) {
  df <- load_stats_sensor(sid)
  if (nrow(df) == 0) return(tibble())
  df |> distinct(hour_bucket, .keep_all = TRUE) |> select(hour_bucket, temp = avg)
}) |> bind_rows() |>
  group_by(hour_bucket) |>
  summarize(indoor_temp = mean(temp, na.rm = TRUE), .groups = "drop")

cat("\n=== Heating Savings Data ===\n")
cat("  HP heating power:", nrow(hp_heat), "hours\n")
cat("  HP consumption:  ", nrow(hp_cons), "hours\n")
cat("  Outdoor temp:    ", nrow(outdoor_temp), "hours\n")
cat("  Indoor temp avg: ", nrow(room_data), "hours\n")

# Use heating-specific power if available, otherwise total consumption
if (nrow(hp_heat) >= 100) {
  heat_source <- hp_heat |> select(hour_bucket, heat_w = avg)
  heat_label <- "Heating"
} else {
  heat_source <- hp_cons |> select(hour_bucket, heat_w = avg)
  heat_label <- "Total HP"
}

# Build combined dataset
cop_d <- load_stats_sensor(HP_COP_SENSOR) |>
  distinct(hour_bucket, .keep_all = TRUE) |>
  select(hour_bucket, cop = avg)

combined <- heat_source |>
  inner_join(outdoor_temp |> select(hour_bucket, outdoor = avg), by = "hour_bucket") |>
  left_join(room_data, by = "hour_bucket") |>
  left_join(cop_d, by = "hour_bucket") |>
  left_join(spot_prices, by = "hour_bucket") |>
  filter(!is.na(outdoor), !is.na(heat_w)) |>
  mutate(
    hour = hour(hour_bucket),
    date = as.Date(hour_bucket),
    week = floor_date(hour_bucket, "week"),
    month = floor_date(hour_bucket, "month"),
    is_heating = outdoor < 15,
    delta_t = if_else(!is.na(indoor_temp), indoor_temp - outdoor, NA_real_),
    heat_kwh = heat_w / 1000,
    heat_cost = if_else(!is.na(price), heat_kwh * price, NA_real_)
  )

heating <- combined |> filter(is_heating)

# Data span in months for normalization
data_months <- as.numeric(difftime(max(heating$hour_bucket), min(heating$hour_bucket), units = "days")) / 30.44
if (data_months < 1) data_months <- 1

cat("\n=== Heating Season Dataset ===\n")
cat("  Heating hours:    ", nrow(heating), "\n")
cat("  With indoor temp: ", sum(!is.na(heating$indoor_temp)), "\n")
cat("  With spot price:  ", sum(!is.na(heating$price)), "\n")
cat("  Outdoor range:    ", round(min(heating$outdoor), 1), "to",
    round(max(heating$outdoor), 1), "°C\n")
cat("  Data span:        ", round(data_months, 1), "months\n")
if (sum(!is.na(heating$indoor_temp)) > 0) {
  cat("  Mean indoor:      ", round(mean(heating$indoor_temp, na.rm = TRUE), 1), "°C\n")
  cat("  Mean ΔT:          ", round(mean(heating$delta_t, na.rm = TRUE), 1), "°C\n")
}

# ============================================================================
# Chart 1: Heating energy vs outdoor temperature — kWh per 30-day month
# ============================================================================
# For each 0.1°C bin: mean power × hours at this bin / data_months = kWh/month
# This shows how much energy each outdoor temp level costs per month.

HOURS_PER_MONTH <- 720  # 30 days × 24 hours

heating_binned <- heating |>
  mutate(outdoor_bin = round(outdoor * 10) / 10) |>
  group_by(outdoor_bin) |>
  summarize(
    mean_heat_kw  = mean(heat_w, na.rm = TRUE) / 1000,
    p25_heat_kw   = quantile(heat_w, 0.25, na.rm = TRUE) / 1000,
    p75_heat_kw   = quantile(heat_w, 0.75, na.rm = TRUE) / 1000,
    total_hours   = n(),
    .groups = "drop"
  ) |>
  filter(total_hours >= 3) |>
  mutate(
    # kWh per 30-day month at this constant outdoor temp
    kwh_per_month = mean_heat_kw * HOURS_PER_MONTH,
    p25_kwh       = p25_heat_kw * HOURS_PER_MONTH,
    p75_kwh       = p75_heat_kw * HOURS_PER_MONTH
  )

cat("\n=== Heating Energy Curve ===\n")
cat("  Temperature bins (0.1°C):", nrow(heating_binned), "\n")
cat("  Range:", round(min(heating_binned$outdoor_bin), 1), "to",
    round(max(heating_binned$outdoor_bin), 1), "°C\n")
coldest <- heating_binned |> slice_min(outdoor_bin, n = 1)
cat("  At", coldest$outdoor_bin, "°C:", round(coldest$kwh_per_month),
    "kWh/month\n")

p1 <- ggplot(heating_binned, aes(x = outdoor_bin)) +
  geom_ribbon(aes(ymin = p25_kwh, ymax = p75_kwh),
              fill = COLORS$heat_pump, alpha = 0.2) +
  geom_point(aes(y = kwh_per_month), color = COLORS$heat_pump, alpha = 0.3, size = 0.5) +
  geom_smooth(aes(y = kwh_per_month), method = "loess", span = 0.3,
              color = COLORS$import, linewidth = 1.5, se = FALSE) +
  scale_x_continuous(breaks = seq(-15, 15, by = 2.5)) +
  labs(
    x     = "Outdoor Temperature (°C)",
    y     = paste0(heat_label, " Energy (kWh / 30-day month)"),
    title = "Monthly Heating Energy vs Outdoor Temperature",
    subtitle = "If outdoor temp stayed constant for 30 days, how many kWh? Band = IQR. 0.1°C bins."
  ) +
  theme_energy()

save_plot(p1, "25_heating_energy_curve.png")

# ============================================================================
# Chart 2: Weekly savings from temperature reduction
# ============================================================================
# Model: heating energy proportional to ΔT (indoor - outdoor).
# Reducing indoor by X°C → savings = X / ΔT fraction of consumption.

savings_data <- heating |>
  filter(!is.na(indoor_temp), !is.na(delta_t), delta_t > 2, heat_w > 0)

if (nrow(savings_data) < 50) {
  cat("Insufficient data with indoor temps for savings analysis.\n")
} else {
  scenarios <- savings_data |>
    mutate(
      frac_1c = pmin(1 / delta_t, 1),
      frac_2c = pmin(2 / delta_t, 1),
      frac_3c = pmin(3 / delta_t, 1),
      reduction_21c = pmax(indoor_temp - 21, 0),
      frac_21c = pmin(reduction_21c / delta_t, 1),
      save_1c_kwh  = heat_kwh * frac_1c,
      save_2c_kwh  = heat_kwh * frac_2c,
      save_3c_kwh  = heat_kwh * frac_3c,
      save_21c_kwh = heat_kwh * frac_21c,
      save_1c_pln  = if_else(!is.na(price), save_1c_kwh * price, NA_real_),
      save_2c_pln  = if_else(!is.na(price), save_2c_kwh * price, NA_real_),
      save_3c_pln  = if_else(!is.na(price), save_3c_kwh * price, NA_real_),
      save_21c_pln = if_else(!is.na(price), save_21c_kwh * price, NA_real_)
    )

  # Aggregate by week with week-of-year label
  weekly <- scenarios |>
    mutate(
      week_num = isoweek(hour_bucket),
      month_num = month(hour_bucket),
      year = year(hour_bucket),
      # Week number within month: 1-based
      month_start_week = isoweek(floor_date(hour_bucket, "month")),
      week_in_month = week_num - month_start_week + 1,
      # Label: "Jan-W1", "Feb-W2", etc.
      week_label = paste0(format(hour_bucket, "%b"), "-W", pmax(week_in_month, 1))
    ) |>
    group_by(week, week_label, year) |>
    summarize(
      avg_outdoor  = mean(outdoor, na.rm = TRUE),
      avg_indoor   = mean(indoor_temp, na.rm = TRUE),
      total_kwh    = sum(heat_kwh, na.rm = TRUE),
      total_cost   = sum(heat_cost, na.rm = TRUE),
      s1c_kwh  = sum(save_1c_kwh, na.rm = TRUE),
      s2c_kwh  = sum(save_2c_kwh, na.rm = TRUE),
      s3c_kwh  = sum(save_3c_kwh, na.rm = TRUE),
      s21c_kwh = sum(save_21c_kwh, na.rm = TRUE),
      s1c_pln  = sum(save_1c_pln, na.rm = TRUE),
      s2c_pln  = sum(save_2c_pln, na.rm = TRUE),
      s3c_pln  = sum(save_3c_pln, na.rm = TRUE),
      s21c_pln = sum(save_21c_pln, na.rm = TRUE),
      hours = n(),
      .groups = "drop"
    ) |>
    filter(hours >= 20) |>
    arrange(week)

  cat("\n=== Weekly Savings Summary ===\n")
  cat("  Weeks analyzed:", nrow(weekly), "\n")
  cat("  Total heating: ", round(sum(weekly$total_kwh)), "kWh,",
      round(sum(weekly$total_cost, na.rm = TRUE)), "PLN\n")
  cat("  Savings -1°C:  ", round(sum(weekly$s1c_kwh)), "kWh,",
      round(sum(weekly$s1c_pln, na.rm = TRUE)), "PLN\n")
  cat("  Savings -2°C:  ", round(sum(weekly$s2c_kwh)), "kWh,",
      round(sum(weekly$s2c_pln, na.rm = TRUE)), "PLN\n")
  cat("  Savings -3°C:  ", round(sum(weekly$s3c_kwh)), "kWh,",
      round(sum(weekly$s3c_pln, na.rm = TRUE)), "PLN\n")
  cat("  Savings ->21C: ", round(sum(weekly$s21c_kwh)), "kWh,",
      round(sum(weekly$s21c_pln, na.rm = TRUE)), "PLN\n")

  scenario_levels <- c("-1C", "-2C", "-3C", "21C target")
  scenario_labels <- c("-1\u00b0C", "-2\u00b0C", "-3\u00b0C", "\u219221\u00b0C")
  scenario_colors <- setNames(
    c(COLORS$charge, COLORS$export, COLORS$pv, COLORS$import),
    scenario_levels
  )

  # --- Apply 3-week rolling average for smoother curves ---
  SMOOTH_WINDOW <- 3

  weekly <- weekly |>
    arrange(week) |>
    mutate(
      s1c_kwh_smooth  = zoo::rollmean(s1c_kwh,  k = SMOOTH_WINDOW, fill = NA, align = "center"),
      s2c_kwh_smooth  = zoo::rollmean(s2c_kwh,  k = SMOOTH_WINDOW, fill = NA, align = "center"),
      s3c_kwh_smooth  = zoo::rollmean(s3c_kwh,  k = SMOOTH_WINDOW, fill = NA, align = "center"),
      s21c_kwh_smooth = zoo::rollmean(s21c_kwh, k = SMOOTH_WINDOW, fill = NA, align = "center"),
      s1c_pln_smooth  = zoo::rollmean(s1c_pln,  k = SMOOTH_WINDOW, fill = NA, align = "center"),
      s2c_pln_smooth  = zoo::rollmean(s2c_pln,  k = SMOOTH_WINDOW, fill = NA, align = "center"),
      s3c_pln_smooth  = zoo::rollmean(s3c_pln,  k = SMOOTH_WINDOW, fill = NA, align = "center"),
      s21c_pln_smooth = zoo::rollmean(s21c_pln, k = SMOOTH_WINDOW, fill = NA, align = "center")
    )

  # --- Plot 2a: Weekly kWh savings — smoothed line chart, faceted by scenario ---
  weekly_long_kwh <- weekly |>
    select(week, week_label,
           `-1C` = s1c_kwh, `-2C` = s2c_kwh,
           `-3C` = s3c_kwh, `21C target` = s21c_kwh) |>
    pivot_longer(cols = -c(week, week_label),
                 names_to = "scenario", values_to = "savings_kwh") |>
    mutate(scenario = factor(scenario, levels = scenario_levels))

  weekly_smooth_kwh <- weekly |>
    select(week, week_label,
           `-1C` = s1c_kwh_smooth, `-2C` = s2c_kwh_smooth,
           `-3C` = s3c_kwh_smooth, `21C target` = s21c_kwh_smooth) |>
    pivot_longer(cols = -c(week, week_label),
                 names_to = "scenario", values_to = "savings_kwh") |>
    mutate(scenario = factor(scenario, levels = scenario_levels))

  p2a <- ggplot() +
    # Raw data as faint points
    geom_point(data = weekly_long_kwh,
               aes(x = week, y = savings_kwh, color = scenario),
               size = 1, alpha = 0.3) +
    # Smoothed rolling average as solid line
    geom_line(data = weekly_smooth_kwh,
              aes(x = week, y = savings_kwh, color = scenario),
              linewidth = 1.1, alpha = 0.9) +
    scale_color_manual(values = scenario_colors) +
    scale_x_datetime(date_labels = "%b", date_breaks = "1 month") +
    facet_wrap(~scenario, ncol = 2, scales = "free_y") +
    labs(
      x     = "",
      y     = "Savings (kWh / week)",
      title = "Weekly Heating Savings — Energy",
      subtitle = paste0("3-week rolling average. Estimated from \u0394T proportional model. Heating season only.")
    ) +
    theme_energy() +
    theme(legend.position = "none",
          axis.text.x = element_text(angle = 30, hjust = 1, size = 8))

  save_plot(p2a, "25_weekly_savings_kwh.png", height = 7)

  # --- Plot 2b: Weekly PLN savings — smoothed line chart, faceted by scenario ---
  weekly_long_pln <- weekly |>
    select(week, week_label,
           `-1C` = s1c_pln, `-2C` = s2c_pln,
           `-3C` = s3c_pln, `21C target` = s21c_pln) |>
    pivot_longer(cols = -c(week, week_label),
                 names_to = "scenario", values_to = "savings_pln") |>
    mutate(scenario = factor(scenario, levels = scenario_levels))

  weekly_smooth_pln <- weekly |>
    select(week, week_label,
           `-1C` = s1c_pln_smooth, `-2C` = s2c_pln_smooth,
           `-3C` = s3c_pln_smooth, `21C target` = s21c_pln_smooth) |>
    pivot_longer(cols = -c(week, week_label),
                 names_to = "scenario", values_to = "savings_pln") |>
    mutate(scenario = factor(scenario, levels = scenario_levels))

  p2b <- ggplot() +
    geom_point(data = weekly_long_pln,
               aes(x = week, y = savings_pln, color = scenario),
               size = 1, alpha = 0.3) +
    geom_line(data = weekly_smooth_pln,
              aes(x = week, y = savings_pln, color = scenario),
              linewidth = 1.1, alpha = 0.9) +
    scale_color_manual(values = scenario_colors) +
    scale_x_datetime(date_labels = "%b", date_breaks = "1 month") +
    facet_wrap(~scenario, ncol = 2, scales = "free_y") +
    labs(
      x     = "",
      y     = "Savings (PLN / week)",
      title = "Weekly Heating Savings — Cost (Spot Price)",
      subtitle = paste0("3-week rolling average \u00d7 hourly spot prices. Colder weeks = larger absolute savings.")
    ) +
    theme_energy() +
    theme(legend.position = "none",
          axis.text.x = element_text(angle = 30, hjust = 1, size = 8))

  save_plot(p2b, "25_weekly_savings_pln.png", height = 7)

  # --- Monthly summary table ---
  monthly <- scenarios |>
    mutate(month_label = format(hour_bucket, "%Y-%m")) |>
    group_by(month_label) |>
    summarize(
      avg_outdoor = round(mean(outdoor, na.rm = TRUE), 1),
      avg_indoor  = round(mean(indoor_temp, na.rm = TRUE), 1),
      heat_kwh    = round(sum(heat_kwh, na.rm = TRUE)),
      heat_pln    = round(sum(heat_cost, na.rm = TRUE)),
      s1c_kwh     = round(sum(save_1c_kwh, na.rm = TRUE)),
      s2c_kwh     = round(sum(save_2c_kwh, na.rm = TRUE)),
      s3c_kwh     = round(sum(save_3c_kwh, na.rm = TRUE)),
      s21c_kwh    = round(sum(save_21c_kwh, na.rm = TRUE)),
      s1c_pln     = round(sum(save_1c_pln, na.rm = TRUE)),
      s2c_pln     = round(sum(save_2c_pln, na.rm = TRUE)),
      s3c_pln     = round(sum(save_3c_pln, na.rm = TRUE)),
      s21c_pln    = round(sum(save_21c_pln, na.rm = TRUE)),
      .groups = "drop"
    )

  cat("\n=== Monthly Breakdown ===\n")
  print(monthly |> select(month_label, avg_outdoor, avg_indoor, heat_kwh, heat_pln,
                           s1c_pln, s2c_pln, s3c_pln, s21c_pln))
}

# ============================================================================
# Chart 3: Cooling energy — current (+1.4°C) vs +2.7°C global (+4°C Europe)
# ============================================================================
# Current climate is already +1.4°C above pre-industrial.
# +2.7°C global warming → ~+4°C in Europe → shift outdoor temps by +2.6°C
# (the delta from current +1.4°C to the +4°C European level).

COOLING_COP   <- 3.5   # typical split AC
WARMING_DELTA <- 2.6   # +4°C Europe minus current +1.4°C = +2.6°C shift

summer_data <- combined |>
  filter(month(hour_bucket) %in% c(5, 6, 7, 8, 9)) |>
  filter(!is.na(outdoor))

if (nrow(summer_data) > 100) {
  targets <- c(21, 22, 23, 24)

  # Estimate building UA from heating data (thermal output / delta_t)
  ua_data <- heating |>
    filter(!is.na(indoor_temp), !is.na(delta_t), delta_t > 3, heat_w > 100,
           !is.na(cop), cop > 0.5, cop < 10)

  if (nrow(ua_data) > 50) {
    ua_estimate <- median(ua_data$heat_w * ua_data$cop / ua_data$delta_t, na.rm = TRUE)
  } else {
    ua_estimate <- 150  # fallback W/°C
  }

  cat("\n=== Cooling Estimate ===\n")
  cat("  Estimated UA:", round(ua_estimate), "W/°C\n")
  cat("  Cooling COP:", COOLING_COP, "\n")
  cat("  Warming shift: +", WARMING_DELTA, "°C (current→+2.7°C global)\n")

  cooling_scenarios <- map_dfr(targets, function(tgt) {
    bind_rows(
      summer_data |>
        mutate(
          scenario = "Current climate (+1.4\u00b0C)",
          target = tgt,
          excess = pmax(outdoor - tgt, 0),
          cool_kwh = excess * ua_estimate / COOLING_COP / 1000
        ),
      summer_data |>
        mutate(
          scenario = "+2.7\u00b0C global (+4\u00b0C Europe)",
          target = tgt,
          outdoor_warm = outdoor + WARMING_DELTA,
          excess = pmax(outdoor_warm - tgt, 0),
          cool_kwh = excess * ua_estimate / COOLING_COP / 1000
        )
    )
  })

  # Season totals (annualized)
  cooling_season <- cooling_scenarios |>
    group_by(scenario, target) |>
    summarize(
      season_kwh = sum(cool_kwh, na.rm = TRUE) / data_months * 12,
      hours_cooling = sum(excess > 0, na.rm = TRUE) / data_months * 12,
      .groups = "drop"
    ) |>
    mutate(target_label = paste0(target, "\u00b0C"))

  cat("\n=== Annual Cooling Energy Estimate (May-Sep) ===\n")
  print(cooling_season)

  target_colors <- c(
    "21\u00b0C" = COLORS$charge,
    "22\u00b0C" = COLORS$export,
    "23\u00b0C" = COLORS$pv,
    "24\u00b0C" = COLORS$discharge
  )

  p3 <- ggplot(cooling_season, aes(x = target_label, y = season_kwh, fill = target_label)) +
    geom_col(alpha = 0.8, width = 0.6) +
    geom_text(aes(label = paste0(round(season_kwh), " kWh")),
              vjust = -0.3, size = 3.5, color = COLORS$text) +
    scale_fill_manual(values = target_colors) +
    facet_wrap(~scenario) +
    labs(
      x     = "Target Indoor Temperature",
      y     = "Annual Cooling Energy (kWh, May\u2013Sep)",
      title = "Estimated Cooling Energy by Target Temperature",
      subtitle = paste0("Building UA = ", round(ua_estimate), " W/\u00b0C, AC COP = ",
                        COOLING_COP, ". Right: outdoor temps shifted +",
                        WARMING_DELTA, "\u00b0C."),
      fill  = ""
    ) +
    theme_energy() +
    theme(legend.position = "none")

  save_plot(p3, "25_cooling_energy.png")

  # Monthly breakdown
  cooling_monthly <- cooling_scenarios |>
    mutate(month_label = format(hour_bucket, "%b")) |>
    group_by(scenario, target, month_label) |>
    summarize(
      total_kwh = sum(cool_kwh, na.rm = TRUE) / data_months * 12,
      .groups = "drop"
    ) |>
    mutate(
      month_label = factor(month_label, levels = c("May", "Jun", "Jul", "Aug", "Sep")),
      target_label = paste0(target, "\u00b0C")
    )

  p3b <- ggplot(cooling_monthly, aes(x = month_label, y = total_kwh, fill = target_label)) +
    geom_col(position = "dodge", alpha = 0.8) +
    scale_fill_manual(values = target_colors) +
    facet_wrap(~scenario) +
    labs(
      x     = "",
      y     = "Cooling Energy (kWh / month, annualized)",
      title = "Monthly Cooling Energy \u2014 Current vs +2.7\u00b0C Global Warming",
      subtitle = "Lower target = more cooling. July/August dominate.",
      fill  = "Target"
    ) +
    theme_energy()

  save_plot(p3b, "25_cooling_monthly.png")
} else {
  cat("Insufficient summer data for cooling estimate.\n")
}
