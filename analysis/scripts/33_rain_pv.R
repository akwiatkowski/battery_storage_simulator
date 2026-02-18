# ============================================================================
# 33_rain_pv.R — Rain & PV Output Correlation
# ============================================================================
# WHAT:    Analyzes how rainfall affects PV generation (proxied by grid export).
#          Rainy days have lower solar irradiance, reducing PV output. Quantifies
#          the relationship across daily totals, rain intensity categories, and
#          monthly seasonal patterns.
#
# INPUTS:  load_stats_sensor() for NETATMO_RAIN (precipitation mm),
#          hourly grid power (avg_power) from load_data.R
#
# OUTPUTS: output/33_daily_pv_vs_rain.png    — scatter: daily PV vs rainfall
#          output/33_pv_by_rain_category.png  — box plot: PV by rain intensity
#          output/33_monthly_rain_pv.png      — monthly rain and PV side by side
#
# HOW TO READ:
#   - Daily scatter: downward loess trend confirms rain reduces PV output.
#     Dry days cluster at top-right of PV axis; rainy days at bottom.
#   - Box plot: compare median PV across dry / light / moderate / heavy rain.
#     Large drops from "dry" to "heavy" indicate strong weather sensitivity.
#   - Monthly facets: summer months should show high PV + low rain;
#     autumn/winter show low PV + more rain. Anti-correlation is expected.
# ============================================================================

source("analysis/helpers/load_data.R")

# ============================================================================
# Load rain data
# ============================================================================
rain_raw <- load_stats_sensor(NETATMO_RAIN) |> distinct(hour_bucket, .keep_all = TRUE)

cat("\n=== Rain Data ===\n")
cat("  Rain hours:  ", nrow(rain_raw), "\n")

if (nrow(rain_raw) < 20) {
  cat("Insufficient rain data (need >= 20 hours, have", nrow(rain_raw), ").\n")
  cat("Netatmo rain sensor may not be available. Skipping rain-PV analysis.\n")
  quit(save = "no")
}

# ============================================================================
# Build daily aggregates
# ============================================================================
# PV generation proxy: when grid power is negative, the house is exporting
# surplus PV. pmax(-avg_power, 0) captures the export portion in watts.
# Summing hourly watts over a day gives Wh (since each bucket = 1 hour).

daily_pv <- hourly |>
  filter(!is.na(avg_power)) |>
  mutate(
    date = as.Date(hour_bucket),
    pv_export_w = pmax(-avg_power, 0)
  ) |>
  group_by(date) |>
  summarize(
    pv_kwh = sum(pv_export_w) / 1000,   # Wh -> kWh
    hours = n(),
    .groups = "drop"
  ) |>
  filter(hours >= 12)  # only days with sufficient data coverage

daily_rain <- rain_raw |>
  mutate(date = as.Date(hour_bucket)) |>
  group_by(date) |>
  summarize(
    rain_mm = sum(pmax(avg, 0), na.rm = TRUE),  # total daily precipitation
    .groups = "drop"
  )

daily <- daily_pv |>
  inner_join(daily_rain, by = "date") |>
  mutate(
    month = month(date, label = TRUE),
    rain_cat = case_when(
      rain_mm == 0             ~ "Dry (0 mm)",
      rain_mm > 0 & rain_mm < 2 ~ "Light (<2 mm)",
      rain_mm >= 2 & rain_mm < 10 ~ "Moderate (2-10 mm)",
      rain_mm >= 10            ~ "Heavy (>10 mm)"
    ),
    rain_cat = factor(rain_cat, levels = c(
      "Dry (0 mm)", "Light (<2 mm)", "Moderate (2-10 mm)", "Heavy (>10 mm)"
    ))
  )

cat("  Matched days: ", nrow(daily), "\n")
cat("  PV range:     ", round(min(daily$pv_kwh), 1), "to",
    round(max(daily$pv_kwh), 1), "kWh/day\n")
cat("  Rain range:   ", round(min(daily$rain_mm), 1), "to",
    round(max(daily$rain_mm), 1), "mm/day\n")
cat("  Rain categories:\n")
print(table(daily$rain_cat))

if (nrow(daily) < 20) {
  cat("Insufficient matched daily data (need >= 20, have", nrow(daily), ").\n")
  quit(save = "no")
}

# ============================================================================
# Chart 1: Daily PV vs rainfall scatter with loess
# ============================================================================
p1 <- ggplot(daily, aes(x = rain_mm, y = pv_kwh)) +
  geom_point(aes(color = month), alpha = 0.5, size = 1.5) +
  geom_smooth(method = "loess", color = COLORS$pv, linewidth = 1.2,
              se = TRUE, fill = COLORS$pv, alpha = 0.15) +
  scale_color_brewer(palette = "Set3", name = "Month") +
  labs(
    x     = "Daily Rainfall (mm)",
    y     = "Daily PV Generation (kWh, grid export proxy)",
    title = "Daily PV Output vs Rainfall",
    subtitle = "Loess trend shows PV drops with increasing rain. Dry days dominate high-PV range."
  ) +
  theme_energy()

save_plot(p1, "33_daily_pv_vs_rain.png")

# ============================================================================
# Chart 2: PV output by rain category — box plot
# ============================================================================
# Filter to only categories with enough data
cat_counts <- daily |> count(rain_cat)
valid_cats <- cat_counts |> filter(n >= 5) |> pull(rain_cat)
daily_filtered <- daily |> filter(rain_cat %in% valid_cats)

cat("\n=== PV by Rain Category ===\n")
daily_filtered |>
  group_by(rain_cat) |>
  summarize(
    n = n(),
    median_pv = round(median(pv_kwh), 2),
    mean_pv   = round(mean(pv_kwh), 2),
    .groups = "drop"
  ) |>
  print()

p2 <- ggplot(daily_filtered, aes(x = rain_cat, y = pv_kwh, fill = rain_cat)) +
  geom_boxplot(alpha = 0.7, outlier.alpha = 0.4) +
  scale_fill_manual(values = c(
    "Dry (0 mm)"           = COLORS$pv,
    "Light (<2 mm)"        = COLORS$charge,
    "Moderate (2-10 mm)"   = COLORS$muted,
    "Heavy (>10 mm)"       = COLORS$import
  )) +
  labs(
    x     = "Rain Category",
    y     = "Daily PV Generation (kWh)",
    title = "PV Output by Rain Intensity",
    subtitle = "Dry days produce the most solar. Heavy rain cuts output substantially."
  ) +
  theme_energy() +
  theme(legend.position = "none")

save_plot(p2, "33_pv_by_rain_category.png")

# ============================================================================
# Chart 3: Monthly rain vs PV — faceted comparison
# ============================================================================
monthly_pv <- daily |>
  mutate(month_date = floor_date(date, "month")) |>
  group_by(month_date) |>
  summarize(
    total_pv_kwh = sum(pv_kwh),
    total_rain_mm = sum(rain_mm),
    n_days = n(),
    .groups = "drop"
  ) |>
  filter(n_days >= 10)  # only months with decent coverage

if (nrow(monthly_pv) >= 3) {
  # Reshape for faceted display
  monthly_long <- monthly_pv |>
    pivot_longer(
      cols = c(total_pv_kwh, total_rain_mm),
      names_to = "metric",
      values_to = "value"
    ) |>
    mutate(
      metric = recode(metric,
        "total_pv_kwh"  = "PV Generation (kWh)",
        "total_rain_mm" = "Rainfall (mm)"
      )
    )

  p3 <- ggplot(monthly_long, aes(x = month_date, y = value,
                                  fill = metric)) +
    geom_col(alpha = 0.75) +
    facet_wrap(~ metric, ncol = 1, scales = "free_y") +
    scale_fill_manual(values = c(
      "PV Generation (kWh)" = COLORS$pv,
      "Rainfall (mm)"       = COLORS$charge
    )) +
    scale_x_date(date_labels = "%b %Y", date_breaks = "1 month") +
    labs(
      x     = "",
      y     = "",
      title = "Monthly PV Generation vs Rainfall",
      subtitle = "Anti-correlation expected: sunny months have high PV and low rain."
    ) +
    theme_energy() +
    theme(
      legend.position = "none",
      axis.text.x = element_text(angle = 45, hjust = 1),
      strip.text = element_text(face = "bold", size = 12)
    )

  save_plot(p3, "33_monthly_rain_pv.png", height = 8)
} else {
  cat("Insufficient monthly data for rain vs PV comparison (need >= 3 months).\n")
}

cat("\n=== Rain & PV Analysis Complete ===\n")
