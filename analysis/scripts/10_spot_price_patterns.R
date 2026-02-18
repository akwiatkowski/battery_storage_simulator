# ============================================================================
# 10_spot_price_patterns.R — 8 Years of Spot Electricity Prices
# ============================================================================
# WHAT:    Analyzes historical spot price patterns (2018-2026) to reveal
#          hourly, weekly, and seasonal price structure.
#
# INPUTS:  spot_prices (from load_data.R — hourly PLN/kWh since 2018)
#
# OUTPUTS: output/10_price_hour_profile.png   — avg price by hour of day
#          output/10_price_heatmap.png        — month x hour heatmap
#          output/10_price_yoy.png            — year-over-year comparison
#          output/10_price_volatility.png     — price spike frequency
#
# HOW TO READ:
#   - Hour profile: prices are cheapest at night (2-5 AM), most expensive
#     at morning (7-9 AM) and evening (17-20 PM) peaks
#   - Heatmap: color intensity shows where cheap/expensive hours cluster
#   - YoY: 2022 energy crisis is clearly visible as a spike
#   - Volatility: how often prices exceed 2x/3x the daily average
# ============================================================================

source("analysis/helpers/load_data.R")

# Prepare price data with time features
prices <- spot_prices |>
  filter(!is.na(price), price > 0) |>
  mutate(
    hour    = hour(hour_bucket),
    weekday = wday(hour_bucket, label = TRUE, week_start = 1),
    month   = month(hour_bucket, label = TRUE),
    year    = year(hour_bucket),
    # weekend flag for weekday vs weekend comparison
    day_type = if_else(wday(hour_bucket, week_start = 1) >= 6,
                       "Weekend", "Weekday")
  )

cat("Price data:", nrow(prices), "hours |",
    min(prices$year), "-", max(prices$year), "\n")

# --- Chart 1: Average price by hour of day ----------------------------------
# Weekday vs weekend profiles show the demand-driven price pattern.
hourly_price <- prices |>
  group_by(hour, day_type) |>
  summarize(avg_price = mean(price), .groups = "drop")

p1 <- ggplot(hourly_price, aes(x = hour, y = avg_price, color = day_type)) +
  geom_line(linewidth = 1.2) +
  geom_point(size = 2) +
  scale_color_manual(values = c("Weekday" = COLORS$import, "Weekend" = COLORS$charge)) +
  scale_x_continuous(breaks = 0:23) +
  labs(
    x        = "Hour of Day",
    y        = "Average Price (PLN/kWh)",
    title    = "Electricity Price by Hour of Day",
    subtitle = "Weekday vs weekend \u2014 morning and evening peaks drive costs",
    color    = ""
  ) +
  theme_energy()

save_plot(p1, "10_price_hour_profile.png")

# --- Chart 2: Month x hour heatmap ------------------------------------------
# Shows the full seasonal + hourly price structure in one view.
monthly_hourly <- prices |>
  group_by(month, hour) |>
  summarize(avg_price = mean(price), .groups = "drop")

p2 <- ggplot(monthly_hourly, aes(x = hour, y = month, fill = avg_price)) +
  geom_tile() +
  scale_fill_gradient(low = COLORS$export, high = COLORS$import,
                      name = "PLN/kWh") +
  scale_x_continuous(breaks = 0:23) +
  labs(
    x     = "Hour of Day",
    y     = "",
    title = "Average Spot Price \u2014 Month \u00d7 Hour",
    subtitle = "Green = cheap hours, coral = expensive hours"
  ) +
  theme_energy()

save_plot(p2, "10_price_heatmap.png", width = 12, height = 5)

# --- Chart 3: Year-over-year hourly profiles ---------------------------------
# Each year's hourly profile shows structural shifts (e.g., 2022 crisis).
yoy <- prices |>
  group_by(year, hour) |>
  summarize(avg_price = mean(price), .groups = "drop")

p3 <- ggplot(yoy, aes(x = hour, y = avg_price, color = factor(year))) +
  geom_line(linewidth = 0.9) +
  scale_x_continuous(breaks = 0:23) +
  scale_color_viridis_d(option = "turbo") +
  labs(
    x        = "Hour of Day",
    y        = "Average Price (PLN/kWh)",
    title    = "Spot Price Profile by Year",
    subtitle = "Year-over-year evolution of hourly price structure",
    color    = "Year"
  ) +
  theme_energy()

save_plot(p3, "10_price_yoy.png")

# --- Chart 4: Price volatility — daily spread and spike frequency ------------
# Compute daily stats: min, max, spread, and how often price > 2x daily avg
daily_stats <- prices |>
  group_by(date = as.Date(hour_bucket)) |>
  summarize(
    avg_price  = mean(price),
    min_price  = min(price),
    max_price  = max(price),
    spread     = max_price - min_price,
    # Ratio of cheapest to most expensive hour within each day
    max_min_ratio = if_else(min_price > 0, max_price / min_price, NA_real_),
    .groups = "drop"
  ) |>
  mutate(year = year(date))

p4 <- ggplot(daily_stats |> filter(!is.na(max_min_ratio)),
             aes(x = date, y = max_min_ratio)) +
  geom_point(alpha = 0.15, size = 0.5, color = COLORS$import) +
  geom_smooth(method = "loess", span = 0.1, color = COLORS$text,
              linewidth = 0.8, se = FALSE) +
  geom_hline(yintercept = c(2, 5), linetype = "dashed", color = COLORS$muted) +
  annotate("text", x = min(daily_stats$date) + 60, y = 2.3,
           label = "2x spread", color = COLORS$muted) +
  annotate("text", x = min(daily_stats$date) + 60, y = 5.3,
           label = "5x spread", color = COLORS$muted) +
  labs(
    x        = "",
    y        = "Daily Max/Min Price Ratio",
    title    = "Daily Price Volatility Over Time",
    subtitle = "Higher ratio = more opportunity for arbitrage and load shifting"
  ) +
  theme_energy()

save_plot(p4, "10_price_volatility.png", width = 12, height = 5)
