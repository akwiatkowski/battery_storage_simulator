# ============================================================================
# 29_thermal_comfort.R — Thermal Comfort Index
# ============================================================================
# WHAT:    Evaluates each room against standard comfort criteria (temperature
#          + humidity). Classifies every hour into one of: too cold, comfortable,
#          too warm, too dry, or too humid. Shows scatter against the ASHRAE
#          comfort zone and tracks comfort throughout the day.
#
# INPUTS:  load_stats_sensor() for TEMP_* and HUM_* pairs per room
#
# OUTPUTS: output/29_comfort_scatter.png   — temp vs RH scatter per room
#          output/29_comfort_score.png     — stacked bar: comfort breakdown
#          output/29_comfort_daily.png     — hourly comfort % across rooms
#
# HOW TO READ:
#   - Scatter: points inside the grey box (19-25 deg C, 30-70% RH) are
#     comfortable. Points outside are color-coded by discomfort type.
#   - Stacked bar: taller green section = more time comfortable. Red/blue
#     sections identify dominant discomfort type per room.
#   - Daily profile: dips in the line indicate hours when comfort drops
#     (typically early morning before heating kicks in, or afternoon overheating).
# ============================================================================

source("analysis/r/R/load_data.R")

# ============================================================================
# Comfort zone definitions
# ============================================================================
TEMP_LOW   <- 19   # deg C — below this is "too cold"
TEMP_HIGH  <- 25   # deg C — above this is "too warm"
RH_LOW     <- 30   # % — below this is "too dry"
RH_HIGH    <- 70   # % — above this is "too humid"

# Classification function: returns a single category per observation.
# Priority: temperature extremes first, then humidity.
classify_comfort <- function(temp, rh) {
  case_when(
    temp < TEMP_LOW                          ~ "Too cold",
    temp > TEMP_HIGH                         ~ "Too warm",
    rh < RH_LOW                              ~ "Too dry",
    rh > RH_HIGH                             ~ "Too humid",
    TRUE                                     ~ "Comfortable"
  )
}

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
      comfort = classify_comfort(temp, rh)
    )
}) |> bind_rows()

cat("\n=== Thermal Comfort Data ===\n")
room_data |>
  group_by(room) |>
  summarize(
    hours = n(),
    avg_temp = round(mean(temp, na.rm = TRUE), 1),
    avg_rh = round(mean(rh, na.rm = TRUE), 1),
    pct_comfortable = round(mean(comfort == "Comfortable", na.rm = TRUE) * 100, 1),
    .groups = "drop"
  ) |>
  print()

if (nrow(room_data) < 20) {
  cat("Insufficient room climate data for thermal comfort analysis.\n")
  quit(save = "no")
}

# Ordered factor for consistent coloring
comfort_levels <- c("Too cold", "Comfortable", "Too warm", "Too dry", "Too humid")
comfort_colors <- c(
  "Too cold"     = COLORS$charge,
  "Comfortable"  = COLORS$export,
  "Too warm"     = COLORS$import,
  "Too dry"      = COLORS$warning,
  "Too humid"    = COLORS$prediction
)
room_data$comfort <- factor(room_data$comfort, levels = comfort_levels)

# ============================================================================
# Chart 1: Comfort zone scatter — temp vs humidity per room (faceted)
# ============================================================================
# Subsample per room if too many points for readable scatter
scatter_data <- room_data |>
  group_by(room) |>
  sample_frac(1) |>
  slice_head(n = 2000) |>
  ungroup()

p1 <- ggplot(scatter_data, aes(x = temp, y = rh, color = comfort)) +
  # ASHRAE comfort zone rectangle
  annotate("rect",
    xmin = TEMP_LOW, xmax = TEMP_HIGH,
    ymin = RH_LOW, ymax = RH_HIGH,
    fill = COLORS$export, alpha = 0.08,
    color = COLORS$muted, linetype = "dashed", linewidth = 0.5
  ) +
  geom_point(alpha = 0.25, size = 0.8) +
  scale_color_manual(values = comfort_colors, name = "Category", drop = FALSE) +
  facet_wrap(~ room, ncol = 4) +
  labs(
    x     = "Temperature (\u00b0C)",
    y     = "Relative Humidity (%)",
    title = "Thermal Comfort: Temperature vs Humidity by Room",
    subtitle = paste0(
      "Dashed box = comfort zone (", TEMP_LOW, "-", TEMP_HIGH,
      "\u00b0C, ", RH_LOW, "-", RH_HIGH, "% RH). Points outside = uncomfortable."
    )
  ) +
  theme_energy() +
  theme(
    strip.text = element_text(face = "bold", size = 10),
    legend.position = "bottom"
  ) +
  guides(color = guide_legend(override.aes = list(alpha = 1, size = 3)))

save_plot(p1, "29_comfort_scatter.png", width = 14, height = 8)

# ============================================================================
# Chart 2: Comfort score by room — stacked bar breakdown
# ============================================================================
comfort_breakdown <- room_data |>
  group_by(room, comfort) |>
  summarize(hours = n(), .groups = "drop") |>
  group_by(room) |>
  mutate(
    total = sum(hours),
    pct = hours / total * 100
  ) |>
  ungroup()

# Order rooms by comfortable %
room_order <- comfort_breakdown |>
  filter(comfort == "Comfortable") |>
  arrange(desc(pct)) |>
  pull(room)

comfort_breakdown$room <- factor(comfort_breakdown$room, levels = rev(room_order))

p2 <- ggplot(comfort_breakdown, aes(x = room, y = pct, fill = comfort)) +
  geom_col(alpha = 0.85, width = 0.65) +
  # Add % labels for the comfortable segment
  geom_text(
    data = comfort_breakdown |> filter(comfort == "Comfortable"),
    aes(label = paste0(round(pct, 0), "%")),
    position = position_stack(vjust = 0.5),
    color = "white", size = 3.5, fontface = "bold"
  ) +
  scale_fill_manual(values = comfort_colors, name = "", drop = FALSE) +
  coord_flip() +
  labs(
    x     = "",
    y     = "% of Time",
    title = "Comfort Score by Room",
    subtitle = paste0(
      "Stacked: too cold (<", TEMP_LOW, "\u00b0C), comfortable (",
      TEMP_LOW, "-", TEMP_HIGH, "\u00b0C & ", RH_LOW, "-", RH_HIGH,
      "% RH), too warm (>", TEMP_HIGH, "\u00b0C), too dry (<",
      RH_LOW, "%), too humid (>", RH_HIGH, "%)."
    )
  ) +
  theme_energy()

save_plot(p2, "29_comfort_score.png", width = 11, height = 6)

# ============================================================================
# Chart 3: Daily comfort profile — % of rooms comfortable by hour of day
# ============================================================================
# For each hour, what fraction of all room-observations are comfortable?
hourly_comfort <- room_data |>
  filter(room != "Workshop") |>
  mutate(hour = hour(hour_bucket)) |>
  group_by(hour) |>
  summarize(
    pct_comfortable = mean(comfort == "Comfortable", na.rm = TRUE) * 100,
    pct_cold = mean(comfort == "Too cold", na.rm = TRUE) * 100,
    pct_warm = mean(comfort == "Too warm", na.rm = TRUE) * 100,
    pct_dry = mean(comfort == "Too dry", na.rm = TRUE) * 100,
    pct_humid = mean(comfort == "Too humid", na.rm = TRUE) * 100,
    .groups = "drop"
  )

# Pivot for stacked area — exclude Workshop (unoccupied, skews results)
hourly_long <- hourly_comfort |>
  pivot_longer(
    cols = starts_with("pct_"),
    names_to = "category",
    values_to = "pct"
  ) |>
  mutate(
    category = case_when(
      category == "pct_comfortable" ~ "Comfortable",
      category == "pct_cold"        ~ "Too cold",
      category == "pct_warm"        ~ "Too warm",
      category == "pct_dry"         ~ "Too dry",
      category == "pct_humid"       ~ "Too humid"
    ),
    category = factor(category, levels = comfort_levels)
  ) |>
  filter(!is.na(pct), pct > 0 | category == "Comfortable")

p3 <- ggplot(hourly_long, aes(x = hour, y = pct, fill = category)) +
  geom_col(alpha = 0.7, position = "stack", width = 0.9) +
  scale_fill_manual(values = comfort_colors, name = "", drop = FALSE) +
  scale_x_continuous(breaks = seq(0, 23, 3)) +
  scale_y_continuous(limits = c(0, 100)) +
  labs(
    x     = "Hour of Day",
    y     = "% of Room-Hours",
    title = "Daily Comfort Profile: When Are Rooms Comfortable?",
    subtitle = paste0(
      "Stacked area shows comfort breakdown across all rooms by hour. ",
      "Green = comfortable. Dips indicate discomfort peaks."
    )
  ) +
  theme_energy()

save_plot(p3, "29_comfort_daily.png")

# ============================================================================
# Summary
# ============================================================================
cat("\n=== Overall Comfort Summary ===\n")
overall <- room_data |>
  count(comfort) |>
  mutate(pct = round(n / sum(n) * 100, 1))
print(overall)

cat("\n=== Thermal Comfort Analysis Complete ===\n")
