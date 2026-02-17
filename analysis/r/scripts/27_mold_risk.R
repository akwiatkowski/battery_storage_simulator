# ============================================================================
# 27_mold_risk.R — Mold Risk Assessment
# ============================================================================
# WHAT:    Computes dewpoint from indoor temperature + humidity for each room.
#          Mold risk occurs when the gap between indoor air temp and dewpoint
#          is < 3 deg C, because cold surfaces (thermal bridges, window frames)
#          can reach dewpoint and condense moisture.
#
# INPUTS:  load_stats_sensor() for TEMP_* and HUM_* pairs per room,
#          NETATMO_OUTDOOR_TEMP for outdoor reference
#
# OUTPUTS: output/27_mold_risk_heatmap.png    — room x month % hours at risk
#          output/27_mold_risk_ranking.png     — total mold risk hours per room
#          output/27_mold_risk_daily.png       — hour-of-day risk profile
#          output/27_dewpoint_vs_outdoor.png   — dewpoint vs outdoor temp scatter
#
# HOW TO READ:
#   - Heatmap: darker cells = higher fraction of time with mold risk.
#     > 10% in a month is a concern; > 25% is critical.
#   - Ranking: longer bars = rooms with more total condensation risk hours.
#   - Daily pattern: morning peak expected — overnight cooling lowers surface
#     temps while humidity stays high from breathing/showering.
#   - Dewpoint vs outdoor: downward trend means cold weather drives up
#     condensation risk (walls get colder while indoor humidity stays high).
# ============================================================================

source("analysis/r/R/load_data.R")

# ============================================================================
# Physics: Magnus formula for dewpoint
# ============================================================================
# Given air temperature (deg C) and relative humidity (%),
# dewpoint is the temperature at which air becomes saturated.
# If a surface is at or below dewpoint, condensation occurs.
dewpoint <- function(temp, rh) {
  alpha <- log(rh / 100) + (17.27 * temp) / (237.3 + temp)
  237.3 * alpha / (17.27 - alpha)
}

# Mold risk threshold: surface temp ~ indoor temp - 3 deg C (thermal bridge)
# Risk when dewpoint > surface temp, i.e. (temp - dewpoint) < 3
MOLD_MARGIN <- 3  # deg C

# ============================================================================
# Load paired temperature + humidity for each room
# ============================================================================
room_pairs <- list(
  "Bedroom 1" = list(temp = TEMP_BEDROOM1, hum = HUM_BEDROOM1),
  "Bedroom 2" = list(temp = TEMP_BEDROOM2, hum = HUM_BEDROOM2),
  "Kitchen"   = list(temp = TEMP_KITCHEN,   hum = HUM_KITCHEN),
  "Office 1"  = list(temp = TEMP_OFFICE1,   hum = HUM_OFFICE1),
  "Office 2"  = list(temp = TEMP_OFFICE2,   hum = HUM_OFFICE2),
  "Bathroom"  = list(temp = TEMP_BATHROOM,  hum = HUM_BATHROOM),
  "Workshop"  = list(temp = TEMP_WORKSHOP,  hum = HUM_WORKSHOP)
)

room_data <- map2(names(room_pairs), room_pairs, function(name, sensors) {
  temp_df <- load_stats_sensor(sensors$temp) |>
    distinct(hour_bucket, .keep_all = TRUE) |>
    select(hour_bucket, temp = avg)

  hum_df <- load_stats_sensor(sensors$hum) |>
    distinct(hour_bucket, .keep_all = TRUE) |>
    select(hour_bucket, rh = avg)

  if (nrow(temp_df) < 20 || nrow(hum_df) < 20) {
    cat("  Skipping", name, "— insufficient data (temp:", nrow(temp_df),
        "hum:", nrow(hum_df), ")\n")
    return(tibble())
  }

  inner_join(temp_df, hum_df, by = "hour_bucket") |>
    filter(!is.na(temp), !is.na(rh), rh > 0, rh <= 100) |>
    mutate(
      room = name,
      dp = dewpoint(temp, rh),
      margin = temp - dp,
      at_risk = margin < MOLD_MARGIN
    )
}) |> bind_rows()

cat("\n=== Mold Risk Data ===\n")
room_data |>
  group_by(room) |>
  summarize(
    hours = n(),
    avg_temp = round(mean(temp, na.rm = TRUE), 1),
    avg_rh = round(mean(rh, na.rm = TRUE), 1),
    avg_dp = round(mean(dp, na.rm = TRUE), 1),
    pct_at_risk = round(mean(at_risk, na.rm = TRUE) * 100, 1),
    .groups = "drop"
  ) |>
  print()

if (nrow(room_data) < 20) {
  cat("Insufficient room climate data for mold risk analysis.\n")
  quit(save = "no")
}

# Workshop is an unheated metal garage — not a living space.
# Separate it so it doesn't skew indoor room averages.
indoor_data   <- room_data |> filter(room != "Workshop")
workshop_data <- room_data |> filter(room == "Workshop")
has_workshop  <- nrow(workshop_data) >= 20

# ============================================================================
# Chart 1: Dewpoint proximity heatmap — room x month, % hours at risk
# ============================================================================
monthly_risk <- room_data |>
  mutate(
    month = month(hour_bucket, label = TRUE),
    # Tag workshop distinctly in the y-axis label
    room = if_else(room == "Workshop", "Workshop (garage)", room)
  ) |>
  group_by(room, month) |>
  summarize(
    pct_risk = mean(at_risk, na.rm = TRUE) * 100,
    n_hours = n(),
    .groups = "drop"
  ) |>
  # Only keep months with >= 20 hours of data

  filter(n_hours >= 20)

p1 <- ggplot(monthly_risk, aes(x = month, y = room, fill = pct_risk)) +
  geom_tile(color = "white", linewidth = 0.5) +
  geom_text(aes(label = sprintf("%.0f%%", pct_risk)),
            color = ifelse(monthly_risk$pct_risk > 15, "white", COLORS$text),
            size = 3.5) +
  scale_fill_gradient(
    low = COLORS$bg, high = COLORS$import,
    name = "% hours at risk",
    limits = c(0, NA)
  ) +
  labs(
    x     = "",
    y     = "",
    title = "Mold Risk: Dewpoint Proximity by Room and Month",
    subtitle = paste0(
      "% of hours where (indoor temp - dewpoint) < ", MOLD_MARGIN,
      "\u00b0C. Higher = more surface condensation risk on thermal bridges."
    )
  ) +
  theme_energy() +
  theme(
    panel.grid = element_blank(),
    axis.text.y = element_text(size = 11)
  )

save_plot(p1, "27_mold_risk_heatmap.png", width = 11, height = 6)

# ============================================================================
# Chart 2: Worst-case room ranking — total mold risk hours
# ============================================================================
risk_ranking <- room_data |>
  mutate(room = if_else(room == "Workshop", "Workshop (garage)", room)) |>
  group_by(room) |>
  summarize(
    risk_hours = sum(at_risk, na.rm = TRUE),
    total_hours = n(),
    pct_risk = mean(at_risk, na.rm = TRUE) * 100,
    .groups = "drop"
  ) |>
  arrange(desc(risk_hours))

p2 <- ggplot(risk_ranking, aes(x = reorder(room, risk_hours), y = risk_hours)) +
  geom_col(fill = COLORS$import, alpha = 0.75, width = 0.6) +
  geom_text(aes(label = paste0(round(pct_risk, 1), "%")),
            hjust = -0.15, color = COLORS$text, size = 3.5) +
  coord_flip() +
  labs(
    x     = "",
    y     = "Total Hours at Mold Risk",
    title = "Mold Risk Ranking by Room",
    subtitle = paste0(
      "Total hours where (temp - dewpoint) < ", MOLD_MARGIN,
      "\u00b0C. Labels show % of total hours."
    )
  ) +
  # Extend x axis to make room for labels
  scale_y_continuous(expand = expansion(mult = c(0, 0.15))) +
  theme_energy()

save_plot(p2, "27_mold_risk_ranking.png")

# ============================================================================
# Chart 3: Daily pattern — hour-of-day mold risk profile (indoor rooms only)
# ============================================================================
# Workshop excluded — its 64% risk rate would dominate the average and
# obscure the much lower (but actionable) risk in living spaces.
hourly_risk <- indoor_data |>
  mutate(hour = hour(hour_bucket)) |>
  group_by(hour) |>
  summarize(
    pct_risk = mean(at_risk, na.rm = TRUE) * 100,
    .groups = "drop"
  )

# If workshop data exists, compute its daily pattern for comparison
if (has_workshop) {
  hourly_workshop <- workshop_data |>
    mutate(hour = hour(hour_bucket)) |>
    group_by(hour) |>
    summarize(
      pct_risk = mean(at_risk, na.rm = TRUE) * 100,
      .groups = "drop"
    )
}

p3 <- ggplot(hourly_risk, aes(x = hour, y = pct_risk)) +
  geom_area(fill = COLORS$import, alpha = 0.25) +
  geom_line(aes(linetype = "Indoor rooms"), color = COLORS$import, linewidth = 1.2) +
  geom_point(color = COLORS$import, size = 2) +
  {if (has_workshop) list(
    geom_line(data = hourly_workshop, aes(linetype = "Workshop (garage)"),
              color = COLORS$muted, linewidth = 1),
    geom_point(data = hourly_workshop, color = COLORS$muted, size = 1.5, shape = 17)
  )} +
  scale_linetype_manual(values = c("Indoor rooms" = "solid", "Workshop (garage)" = "dashed"),
                        name = "") +
  scale_x_continuous(breaks = seq(0, 23, 3)) +
  labs(
    x     = "Hour of Day",
    y     = "% of Room-Hours at Risk",
    title = "Mold Risk: Daily Pattern",
    subtitle = paste0(
      "Indoor rooms (solid) vs workshop garage (dashed). ",
      "Morning peak expected — overnight cooling drops surface temps."
    )
  ) +
  theme_energy()

save_plot(p3, "27_mold_risk_daily.png")

# ============================================================================
# Chart 4: Dewpoint vs outdoor temperature scatter
# ============================================================================
outdoor <- load_stats_sensor(NETATMO_OUTDOOR_TEMP) |>
  distinct(hour_bucket, .keep_all = TRUE) |>
  select(hour_bucket, outdoor_temp = avg)

if (nrow(outdoor) < 20) {
  cat("Netatmo outdoor data sparse, trying HP_OUTSIDE_TEMP as fallback.\n")
  outdoor <- load_stats_sensor(HP_OUTSIDE_TEMP) |>
    distinct(hour_bucket, .keep_all = TRUE) |>
    select(hour_bucket, outdoor_temp = avg)
}

if (nrow(outdoor) >= 20) {
  # Compute average indoor dewpoint per hour (indoor rooms only, not workshop)
  avg_dewpoint <- indoor_data |>
    group_by(hour_bucket) |>
    summarize(
      avg_dp = mean(dp, na.rm = TRUE),
      avg_margin = mean(margin, na.rm = TRUE),
      .groups = "drop"
    )

  scatter_data <- avg_dewpoint |>
    inner_join(outdoor, by = "hour_bucket") |>
    filter(!is.na(outdoor_temp), !is.na(avg_dp))

  if (nrow(scatter_data) >= 20) {
    # Subsample for plotting if too many points
    plot_data <- if (nrow(scatter_data) > 5000) {
      slice_sample(scatter_data, n = 5000)
    } else {
      scatter_data
    }

    p4 <- ggplot(plot_data, aes(x = outdoor_temp, y = avg_dp, color = avg_margin)) +
      geom_point(alpha = 0.3, size = 0.8) +
      geom_smooth(method = "lm", color = COLORS$muted, linewidth = 1, se = TRUE) +
      geom_hline(yintercept = 0, linetype = "dotted", color = COLORS$muted) +
      scale_color_gradient2(
        low = COLORS$import, mid = COLORS$pv, high = COLORS$export,
        midpoint = MOLD_MARGIN,
        name = "Margin (\u00b0C)"
      ) +
      labs(
        x     = "Outdoor Temperature (\u00b0C)",
        y     = "Avg Indoor Dewpoint (\u00b0C)",
        title = "Indoor Dewpoint vs Outdoor Temperature",
        subtitle = paste0(
          "Color = margin (temp - dewpoint). Red = high condensation risk. ",
          "Cold weather raises risk when walls cool down."
        )
      ) +
      theme_energy()

    save_plot(p4, "27_dewpoint_vs_outdoor.png")
  } else {
    cat("Insufficient overlap between indoor climate and outdoor data for scatter plot.\n")
  }
} else {
  cat("Insufficient outdoor temperature data for dewpoint vs outdoor chart.\n")
}

cat("\n=== Mold Risk Analysis Complete ===\n")
