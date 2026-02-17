# ============================================================================
# load_data.R — Comprehensive data loader for all energy sensor data
# ============================================================================
# WHAT:  Loads and pre-processes all CSV data sources into ready-to-use
#        data frames. Sources theme.R and helpers.R automatically.
#
# INPUTS:
#   input/*.csv              — legacy HA exports (entity_id, state, last_changed)
#   input/recent/2026-*.csv  — high-resolution recent data (~60s intervals)
#   input/stats/*.csv        — HA long-term statistics (hourly avg/min/max)
#   input/recent/historic_spot_prices.csv — spot electricity prices
#
# OUTPUTS (data frames available after sourcing):
#   hourly       — hourly grid power (avg/max/min) with season, prices
#   cop_data     — heat pump COP joined with outdoor temperature
#   grid_legacy  — raw grid power from legacy CSV
#   spot_prices  — hourly spot prices (PLN/kWh)
#   legacy_*     — individual legacy sensor data frames (best-effort)
#
# HOW TO USE:
#   source("analysis/r/R/load_data.R")
#   # Now `hourly`, `cop_data`, etc. are available in your environment
# ============================================================================

library(tidyverse)

# Source shared theme and helpers
source("analysis/r/R/theme.R")
source("analysis/r/R/helpers.R")

# ============================================================================
# Sensor ID constants
# ============================================================================
# These match the entity_id values in the CSV files. The grid power sensor
# appears in both legacy CSVs (entity_id column) and recent/stats CSVs
# (sensor_id column).
GRID_SENSOR     <- "sensor.0x943469fffed2bf71_power"
VOLTAGE_SENSOR  <- "sensor.0x943469fffed2bf71_voltage"
PRICE_SENSOR    <- "sensor.spotprice_now"
PV_SENSOR       <- "sensor.hoymiles_gateway_solarh_3054300_real_power"
HP_CONSUMPTION  <- "sensor.panasonic_heat_pump_consumption"
HP_HEAT_POWER   <- "sensor.panasonic_heat_pump_main_heat_power_consumption"
HP_PRODUCTION   <- "sensor.panasonic_heat_pump_production"
HP_DHW_POWER    <- "sensor.panasonic_heat_pump_main_dhw_power_consumption"
HP_OUTSIDE_TEMP <- "sensor.panasonic_heat_pump_main_outside_temp"
HP_INLET_TEMP   <- "sensor.panasonic_heat_pump_main_main_inlet_temp"
HP_OUTLET_TEMP  <- "sensor.panasonic_heat_pump_main_main_outlet_temp"
HP_ZONE1_TEMP   <- "sensor.panasonic_heat_pump_main_z1_temp"
WASHING_SENSOR  <- "sensor.pralka_z2m_power"
OVEN_SENSOR     <- "sensor.piekarnik_z2m_power"
DRIER_SENSOR    <- "sensor.70_power"

# HP diagnostic sensors
HP_COP_SENSOR          <- "sensor.panasonic_heat_pump_cop"
HP_COMPRESSOR_SPEED    <- "sensor.panasonic_heat_pump_main_pump_speed"
HP_FAN_SPEED           <- "sensor.panasonic_heat_pump_main_fan1_motor_speed"
HP_HIGH_PRESSURE       <- "sensor.panasonic_heat_pump_main_high_pressure"
HP_DISCHARGE_TEMP      <- "sensor.panasonic_heat_pump_main_discharge_temp"
HP_PUMP_FLOW           <- "sensor.panasonic_heat_pump_main_pump_flow"
HP_DHW_TEMP            <- "sensor.panasonic_heat_pump_main_dhw_temp"
HP_Z1_TARGET_TEMP      <- "sensor.panasonic_heat_pump_main_z1_water_target_temp"
HP_INSIDE_PIPE_TEMP    <- "sensor.panasonic_heat_pump_main_inside_pipe_temp"
HP_OUTSIDE_PIPE_TEMP   <- "sensor.panasonic_heat_pump_main_outside_pipe_temp"
HP_HEATER_ROOM_HOURS   <- "sensor.panasonic_heat_pump_main_room_heater_operations_hours"
HP_HEATER_DHW_HOURS    <- "sensor.panasonic_heat_pump_main_dhw_heater_operations_hours"

# Indoor climate sensors
TEMP_BEDROOM1   <- "sensor.lozeczko_zosii_z2m_temperature"
HUM_BEDROOM1    <- "sensor.lozeczko_zosii_z2m_humidity"
TEMP_BEDROOM2   <- "sensor.temperatura_pokoj_zosi_temperature"
HUM_BEDROOM2    <- "sensor.temperatura_pokoj_zosi_humidity"
TEMP_KITCHEN    <- "sensor.temperatura_w_kuchni_temperature"
HUM_KITCHEN     <- "sensor.temperatura_w_kuchni_humidity"
TEMP_OFFICE1    <- "sensor.termometr_olek_z2m_temperature"
HUM_OFFICE1     <- "sensor.termometr_olek_z2m_humidity"
TEMP_OFFICE2    <- "sensor.termometr_beata_z2m_temperature"
HUM_OFFICE2     <- "sensor.termometr_beata_z2m_humidity"
TEMP_BATHROOM   <- "sensor.termometr_lazienka_gorna_z2m_temperature"
HUM_BATHROOM    <- "sensor.termometr_lazienka_gorna_z2m_humidity"
TEMP_WORKSHOP   <- "sensor.warsztat_termometr_temperature"
HUM_WORKSHOP    <- "sensor.warsztat_termometr_humidity"
TEMP_WORKSHOP_EXT <- "sensor.warsztat_zewnatrz_termometr_temperature"

# Netatmo sensors
NETATMO_BEDROOM_TEMP   <- "sensor.unknown_70_ee_50_a9_6a_b8_sypialnia_temperature"
NETATMO_BEDROOM_HUM    <- "sensor.unknown_70_ee_50_a9_6a_b8_sypialnia_humidity"
NETATMO_BEDROOM_CO2    <- "sensor.unknown_70_ee_50_a9_6a_b8_sypialnia_carbon_dioxide"
NETATMO_LIVING_TEMP    <- "sensor.unknown_70_ee_50_a9_6a_b8_temperature"
NETATMO_LIVING_HUM     <- "sensor.unknown_70_ee_50_a9_6a_b8_humidity"
NETATMO_LIVING_CO2     <- "sensor.unknown_70_ee_50_a9_6a_b8_carbon_dioxide"
NETATMO_LIVING_PRESSURE <- "sensor.unknown_70_ee_50_a9_6a_b8_atmospheric_pressure"
NETATMO_LIVING_NOISE   <- "sensor.unknown_70_ee_50_a9_6a_b8_noise"
NETATMO_OUTDOOR_TEMP   <- "sensor.unknown_70_ee_50_a9_6a_b8_na_zewnatrz_temperature"
NETATMO_OUTDOOR_HUM    <- "sensor.unknown_70_ee_50_a9_6a_b8_na_zewnatrz_humidity"
NETATMO_WIND_SPEED     <- "sensor.unknown_70_ee_50_a9_6a_b8_wiatr_zachod_wind_speed"
NETATMO_WIND_ANGLE     <- "sensor.unknown_70_ee_50_a9_6a_b8_wiatr_zachod_wind_angle"
NETATMO_GUST_SPEED     <- "sensor.unknown_70_ee_50_a9_6a_b8_wiatr_zachod_gust_strength"
NETATMO_GUST_ANGLE     <- "sensor.unknown_70_ee_50_a9_6a_b8_wiatr_zachod_gust_angle"
NETATMO_RAIN           <- "sensor.unknown_70_ee_50_a9_6a_b8_deszcz_precipitation"

# Grid power quality sensors
POWER_FACTOR_SENSOR    <- "sensor.0x943469fffed2bf71_power_factor"
REACTIVE_POWER_SENSOR  <- "sensor.0x943469fffed2bf71_power_reactive"
REACTIVE_ENERGY_SENSOR <- "sensor.0x943469fffed2bf71_energy_reactive"

# Per-circuit voltage sensors
VOLTAGE_OFFICE1  <- "sensor.olek_tylne_cieple_oswietlenie_voltage"
VOLTAGE_OFFICE2  <- "sensor.beata_biurko_voltage"
VOLTAGE_EXTERNAL <- "sensor.obciazenie_zewnetrzne_1_voltage"
VOLTAGE_LIVING_LAMP  <- "sensor.salon_oswietlenie_w_szafie_podschodowe_voltage"
VOLTAGE_LIVING_MEDIA <- "sensor.salon_tv_i_media_voltage"

# ============================================================================
# Generic legacy CSV loader
# ============================================================================
# Legacy HA export format: entity_id, state, last_changed
# state is stored as character (can be "unavailable" etc.), so we parse
# it to numeric and drop NA rows.
#
# Args:
#   filename — path relative to project root, e.g. "input/grid_power.csv"
#   col_name — name for the numeric state column (default "value")
#
# Returns: tibble with columns: timestamp (POSIXct), <col_name> (numeric)
load_legacy_csv <- function(filename, col_name = "value") {
  tryCatch({
    df <- read_csv(filename, show_col_types = FALSE) |>
      select(last_changed, value = state) |>
      mutate(
        timestamp = as_datetime(last_changed),   # parse ISO 8601
        value     = as.numeric(value)            # "unavailable" → NA
      ) |>
      filter(!is.na(value)) |>
      select(timestamp, value) |>
      arrange(timestamp)

    # Rename 'value' to the caller's preferred column name
    names(df)[names(df) == "value"] <- col_name
    df
  }, error = function(e) {
    warning(paste("Could not load", filename, ":", e$message))
    tibble(timestamp = as.POSIXct(character()), placeholder = numeric())
  })
}

# ============================================================================
# 1. Grid power — hourly aggregates from multiple sources
# ============================================================================

# --- Source A: High-resolution recent CSVs (sensor_id, value, updated_ts) ----
# These are ~60-second interval readings fetched from Home Assistant.
# We aggregate them into hourly buckets with avg/max/min.
recent_files <- list.files("input/recent", pattern = "^2026.*\\.csv$", full.names = TRUE)

hourly_from_recent <- tryCatch({
  if (length(recent_files) == 0) {
    tibble(hour_bucket = as.POSIXct(character()),
           avg_power = numeric(), max_power = numeric(),
           min_power = numeric(), readings = integer())
  } else {
    recent_files |>
      map(~ read_csv(.x, show_col_types = FALSE,
                      col_types = cols(value = col_character()))) |>
      bind_rows() |>
      filter(sensor_id == GRID_SENSOR) |>
      mutate(value = as.numeric(value)) |>
      filter(!is.na(value)) |>
      mutate(timestamp = as_datetime(updated_ts)) |>
      select(timestamp, power = value) |>
      distinct(timestamp, .keep_all = TRUE) |>
      arrange(timestamp) |>
      # floor_date rounds each timestamp down to its hour boundary,
      # so 14:37:22 → 14:00:00. This creates hourly buckets.
      mutate(hour_bucket = floor_date(timestamp, "hour")) |>
      group_by(hour_bucket) |>
      summarize(
        avg_power = mean(power),
        max_power = max(power),
        min_power = min(power),
        readings  = n(),
        .groups   = "drop"
      )
  }
}, error = function(e) {
  warning(paste("Recent CSV loading failed:", e$message))
  tibble(hour_bucket = as.POSIXct(character()),
         avg_power = numeric(), max_power = numeric(),
         min_power = numeric(), readings = integer())
})

# --- Source B: HA long-term statistics (already hourly) ----------------------
stats_files <- list.files("input/stats", pattern = "\\.csv$", full.names = TRUE)

hourly_from_stats <- tryCatch({
  if (length(stats_files) == 0) {
    tibble(hour_bucket = as.POSIXct(character()),
           avg_power = numeric(), max_power = numeric(),
           min_power = numeric(), readings = integer())
  } else {
    stats_files |>
      map(~ read_csv(.x, show_col_types = FALSE)) |>
      bind_rows() |>
      filter(sensor_id == GRID_SENSOR) |>
      mutate(hour_bucket = as_datetime(start_time)) |>
      select(hour_bucket, avg_power = avg, max_power = max_val, min_power = min_val) |>
      mutate(readings = NA_integer_)
  }
}, error = function(e) {
  warning(paste("Stats CSV loading failed:", e$message))
  tibble(hour_bucket = as.POSIXct(character()),
         avg_power = numeric(), max_power = numeric(),
         min_power = numeric(), readings = integer())
})

# --- Combine and deduplicate (recent data listed first so it wins) -----------
# distinct() keeps the first occurrence, so recent high-res data takes
# priority over stats for overlapping hours.
hourly <- bind_rows(hourly_from_recent, hourly_from_stats) |>
  arrange(hour_bucket) |>
  distinct(hour_bucket, .keep_all = TRUE) |>
  mutate(
    hour   = hour(hour_bucket),
    month  = month(hour_bucket, label = TRUE),
    # Assign meteorological seasons based on calendar month
    season = case_when(
      month(hour_bucket) %in% c(12, 1, 2)  ~ "Winter",
      month(hour_bucket) %in% c(3, 4, 5)   ~ "Spring",
      month(hour_bucket) %in% c(6, 7, 8)   ~ "Summer",
      TRUE                                  ~ "Autumn"
    ),
    # factor() with explicit levels controls ordering in charts
    season = factor(season, levels = c("Spring", "Summer", "Autumn", "Winter"))
  )

# ============================================================================
# 2. Spot prices
# ============================================================================
spot_prices <- tryCatch({
  read_csv("input/recent/historic_spot_prices.csv", show_col_types = FALSE) |>
    mutate(hour_bucket = floor_date(as_datetime(updated_ts), "hour")) |>
    select(hour_bucket, price = value) |>
    distinct(hour_bucket, .keep_all = TRUE) |>
    arrange(hour_bucket)
}, error = function(e) {
  warning(paste("Spot prices loading failed:", e$message))
  tibble(hour_bucket = as.POSIXct(character()), price = numeric())
})

# Join prices into hourly grid data
hourly <- hourly |> left_join(spot_prices, by = "hour_bucket")

# ============================================================================
# 3. Legacy sensor CSVs — load all available files
# ============================================================================
# Each legacy CSV has the same format: entity_id, state, last_changed.
# We load each with a descriptive column name. tryCatch inside load_legacy_csv
# means missing files produce a warning, not an error.

legacy_grid         <- load_legacy_csv("input/grid_power.csv", "power")
legacy_pv           <- load_legacy_csv("input/pv_power.csv", "pv_power")
legacy_ext_temp     <- load_legacy_csv("input/pump_ext_temp.csv", "temp")
legacy_heat_consumed <- load_legacy_csv("input/pump_heat_power_consumed.csv", "consumption")
legacy_total_prod   <- load_legacy_csv("input/pump_total_production.csv", "production")
legacy_total_cons   <- load_legacy_csv("input/pump_total_consumption.csv", "total_consumption")
legacy_cwu          <- load_legacy_csv("input/pump_cwu_power_consumed.csv", "cwu_power")
legacy_inlet_temp   <- load_legacy_csv("input/pump_inlet_temp.csv", "inlet_temp")
legacy_outlet_temp  <- load_legacy_csv("input/pump_outlet_temp.csv", "outlet_temp")
legacy_zone1_temp   <- load_legacy_csv("input/pump_zone1_temp.csv", "zone1_temp")

# ============================================================================
# 3b. Recent data helpers — load any sensor from high-res or stats CSVs
# ============================================================================

# Cache all recent CSV data once (read + bind is expensive).
# Scripts that need specific sensors use load_recent_sensor() below.
.recent_all <- tryCatch({
  if (length(recent_files) == 0) tibble()
  else {
    recent_files |>
      map(~ read_csv(.x, show_col_types = FALSE,
                      col_types = cols(value = col_character()))) |>
      bind_rows() |>
      mutate(value = as.numeric(value)) |>
      filter(!is.na(value)) |>
      mutate(timestamp = as_datetime(updated_ts))
  }
}, error = function(e) { tibble() })

.stats_all <- tryCatch({
  if (length(stats_files) == 0) tibble()
  else {
    stats_files |>
      map(~ read_csv(.x, show_col_types = FALSE)) |>
      bind_rows()
  }
}, error = function(e) { tibble() })

# load_recent_sensor() — Extract raw high-res readings for one sensor.
# Returns tibble with: timestamp, value (sorted by time, deduplicated).
load_recent_sensor <- function(sid) {
  if (nrow(.recent_all) == 0) return(tibble(timestamp = as.POSIXct(character()), value = numeric()))
  .recent_all |>
    filter(sensor_id == sid) |>
    select(timestamp, value) |>
    distinct(timestamp, .keep_all = TRUE) |>
    arrange(timestamp)
}

# load_stats_sensor() — Extract hourly stats for one sensor.
# Returns tibble with: hour_bucket, avg, min_val, max_val.
load_stats_sensor <- function(sid) {
  if (nrow(.stats_all) == 0) return(tibble(hour_bucket = as.POSIXct(character()),
                                           avg = numeric(), min_val = numeric(), max_val = numeric()))
  .stats_all |>
    filter(sensor_id == sid) |>
    mutate(hour_bucket = as_datetime(start_time)) |>
    select(hour_bucket, avg, min_val, max_val) |>
    arrange(hour_bucket)
}

# ============================================================================
# 4. Pre-computed COP data (heat pump analysis)
# ============================================================================
# COP = Coefficient of Performance = heat_output / electrical_input.
# We join consumption, production, and outdoor temperature by timestamp,
# then filter for heating conditions (temp < 14°C) and sane COP range.

cop_data <- tryCatch({
  legacy_heat_consumed |>
    inner_join(legacy_total_prod, by = "timestamp") |>
    inner_join(legacy_ext_temp, by = "timestamp") |>
    filter(
      temp < 14,                    # only heating season
      consumption > 0,              # pump running
      production > 0                # producing heat
    ) |>
    mutate(cop = production / consumption) |>
    filter(cop > 0.8, cop < 10.0) |>  # discard physically implausible values
    mutate(
      month       = month(timestamp, label = TRUE),
      hour        = hour(timestamp),
      time_of_day = case_when(
        hour >= 6 & hour < 12  ~ "Morning (6-12)",
        hour >= 12 & hour < 18 ~ "Afternoon (12-18)",
        hour >= 18 & hour < 24 ~ "Evening (18-24)",
        TRUE                   ~ "Night (0-6)"
      )
    )
}, error = function(e) {
  warning(paste("COP data computation failed:", e$message))
  tibble()
})

# ============================================================================
# 5. Pre-computed grid_legacy with hour/weekday (for heatmap)
# ============================================================================
grid_legacy <- legacy_grid |>
  mutate(
    hour    = hour(timestamp),
    weekday = wday(timestamp, label = TRUE, week_start = 1),
    month   = month(timestamp)
  )

# ============================================================================
# Summary
# ============================================================================
cat("=== Data Loaded ===\n")
cat("  hourly:       ", nrow(hourly), "hours |",
    format(min(hourly$hour_bucket)), "to", format(max(hourly$hour_bucket)),
    "| With prices:", sum(!is.na(hourly$price)), "\n")
cat("  cop_data:     ", nrow(cop_data), "readings\n")
cat("  grid_legacy:  ", nrow(grid_legacy), "readings\n")
cat("  spot_prices:  ", nrow(spot_prices), "rows\n")
cat("  legacy CSVs:   grid=", nrow(legacy_grid),
    " pv=", nrow(legacy_pv),
    " temp=", nrow(legacy_ext_temp), "\n")
