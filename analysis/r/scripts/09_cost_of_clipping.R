# ============================================================================
# 09_cost_of_clipping.R — Monetary Impact of Undersized Inverter
# ============================================================================
# WHAT:    Combines peak power data with spot electricity prices to estimate
#          the annual cost of power clipping at each inverter size.
#
# INPUTS:  hourly (from load_data.R — hourly grid power + prices)
#
# OUTPUTS: output/09_clipping_cost.png    — annual cost vs inverter size
#          output/09_clipping_by_hour.png — when does clipping cost most
#          output/09_marginal_value.png   — diminishing returns per extra kW
#
# HOW TO READ:
#   - Cost chart: total annual PLN lost from both import clipping (can't use
#     inverter for peak demand) and export clipping (PV wasted)
#   - By-hour chart: shows which hours of the day cost the most from clipping
#     at 3 kW — high-price hours with big peaks are the worst
#   - Marginal value: how much money the next 1 kW of inverter capacity saves;
#     when the bars flatten, bigger isn't worth it
# ============================================================================

source("analysis/r/R/load_data.R")

# Only hours with both price data and meaningful power readings
data <- hourly |>
  filter(!is.na(price), !is.na(max_power))

cat("Hours with price data:", nrow(data), "\n")

# --- Compute clipping cost at each inverter cap level ------------------------
cap_levels <- seq(500, 8000, by = 100)

cost_impact <- map_dfr(cap_levels, function(cap) {
  data |>
    mutate(
      # Import clipping: peak demand exceeds inverter capacity
      import_clipped_w = pmax(max_power - cap, 0),
      # Export clipping: peak PV export exceeds inverter capacity
      export_clipped_w = pmax(abs(min_power) - cap, 0),
      # Monetary cost: clipped power × spot price (W/1000 = kW, × PLN/kWh)
      import_cost = import_clipped_w / 1000 * price,
      export_lost = export_clipped_w / 1000 * price
    ) |>
    summarize(
      inverter_w         = cap,
      import_clipped_kwh = sum(import_clipped_w) / 1000,
      export_clipped_kwh = sum(export_clipped_w) / 1000,
      import_cost_pln    = sum(import_cost),
      export_lost_pln    = sum(export_lost),
      total_cost_pln     = sum(import_cost) + sum(export_lost)
    )
})

# Scale partial-year data to annual estimate using annualize_factor()
af <- annualize_factor(data$hour_bucket)

cost_impact <- cost_impact |>
  mutate(
    annual_cost_pln   = total_cost_pln * af,
    annual_import_pln = import_cost_pln * af,
    annual_export_pln = export_lost_pln * af
  )

# Print key thresholds
cat("\n=== Annual cost of clipping at common inverter sizes ===\n")
cost_impact |>
  filter(inverter_w %in% c(1000, 2000, 3000, 4000, 5000, 6000, 8000)) |>
  select(inverter_w, annual_import_pln, annual_export_pln, annual_cost_pln) |>
  mutate(across(starts_with("annual"), ~ round(.x, 2))) |>
  print()

# --- Chart 1: Annual cost of clipping vs inverter size -----------------------
cost_long <- cost_impact |>
  select(inverter_w,
         "Import penalty" = annual_import_pln,
         "Lost export"    = annual_export_pln) |>
  pivot_longer(-inverter_w, names_to = "type", values_to = "pln")

p1 <- ggplot(cost_long, aes(x = inverter_w, y = pln, fill = type)) +
  geom_area(alpha = 0.6) +
  scale_fill_manual(values = c("Import penalty" = COLORS$import,
                               "Lost export"    = COLORS$pv)) +
  labs(
    x        = "Inverter Power Rating (W)",
    y        = "Annual Cost (PLN)",
    title    = "Annual Cost of Undersized Inverter",
    subtitle = "How much money do you lose from power clipping?",
    fill     = ""
  ) +
  theme_energy()

save_plot(p1, "09_clipping_cost.png")

# --- Chart 2: Clipping cost by hour of day (at 3 kW) ------------------------
hourly_cost <- data |>
  mutate(
    hour               = hour(hour_bucket),
    import_clipped_3kw = pmax(max_power - 3000, 0),
    export_clipped_3kw = pmax(abs(min_power) - 3000, 0),
    clipping_cost_3kw  = (import_clipped_3kw + export_clipped_3kw) / 1000 * price
  ) |>
  group_by(hour) |>
  summarize(
    avg_clipping_cost = mean(clipping_cost_3kw) * af,
    .groups           = "drop"
  )

p2 <- ggplot(hourly_cost, aes(x = hour, y = avg_clipping_cost)) +
  geom_col(fill = COLORS$import, alpha = 0.7) +
  scale_x_continuous(breaks = 0:23) +
  labs(
    x        = "Hour of Day",
    y        = "Annualized Clipping Cost (PLN)",
    title    = "When Does Clipping Cost You Most? (3 kW inverter)",
    subtitle = "Combining power peaks with spot prices \u2014 expensive peaks matter most"
  ) +
  theme_energy()

save_plot(p2, "09_clipping_by_hour.png", width = 12, height = 6)

# --- Chart 3: Marginal value of each extra kW --------------------------------
# lead(x, 10) looks 10 rows ahead (= 1 kW since step size is 100W).
# The difference is how much annual cost drops by adding that 1 kW.
marginal <- cost_impact |>
  mutate(
    savings_vs_next = annual_cost_pln - lead(annual_cost_pln, 10),
    midpoint_w      = inverter_w + 500
  ) |>
  filter(!is.na(savings_vs_next), savings_vs_next > 0)

p3 <- ggplot(marginal, aes(x = inverter_w, y = savings_vs_next)) +
  geom_col(fill = COLORS$export, alpha = 0.7) +
  labs(
    x        = "Inverter Size (W)",
    y        = "Annual Savings from +1 kW (PLN)",
    title    = "Marginal Value of Each Extra kW of Inverter",
    subtitle = "Diminishing returns \u2014 where does adding more capacity stop being worth it?"
  ) +
  theme_energy()

save_plot(p3, "09_marginal_value.png")
