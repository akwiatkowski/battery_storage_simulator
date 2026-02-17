# ============================================================================
# helpers.R — Reusable computation functions for energy analysis
# ============================================================================
# Extracted from individual scripts to avoid duplication.
# Each function is used by 2+ scripts.
#
# Usage:
#   source("analysis/r/R/helpers.R")
#
# Exports:
#   compute_coverage_curve() — inverter sizing curve (scripts 03, 08)
#   compute_duration_curve() — power duration curve (script 07)
#   compute_clipping()       — clipped energy at each cap (scripts 05, 09)
#   annualize_factor()       — scale partial-year data to annual (script 09)
# ============================================================================

library(tidyverse)

# --- compute_coverage_curve() ------------------------------------------------
# For a vector of power readings, compute what % fall below each cap level.
# Returns a tibble with columns: cap_w, pct_covered.
#
# Args:
#   values — numeric vector of power values (e.g., max_power per hour)
#   levels — numeric vector of cap levels to test (default 500–8000 by 100)
#
# Used by: 03_peak_vs_average.R (inverter sizing), 08_seasonal_inverter.R
compute_coverage_curve <- function(values, levels = seq(500, 8000, by = 100)) {
  # map_dbl loops over each cap level and computes the fraction of values
  # that fall at or below that cap. mean() of a logical vector = proportion TRUE.
  tibble(
    cap_w       = levels,
    pct_covered = map_dbl(levels, ~ mean(values <= .x, na.rm = TRUE) * 100)
  )
}

# --- compute_duration_curve() ------------------------------------------------
# Sort values highest-to-lowest, assign each a rank and percent-of-time.
# Classic power engineering "duration curve" — shows how often a given
# power level is exceeded.
#
# Args:
#   values — numeric vector of power values
#
# Returns: tibble with columns: power, rank, pct_time
#
# Used by: 07_power_duration.R
compute_duration_curve <- function(values) {
  tibble(power = values) |>
    filter(!is.na(power), power > 0) |>
    arrange(desc(power)) |>
    mutate(
      rank     = row_number(),
      pct_time = rank / n() * 100
    )
}

# --- compute_clipping() ------------------------------------------------------
# For each cap level, compute how much energy (Wh) is clipped.
# "Clipping" = the power that exceeds the cap, lost per hour.
#
# Args:
#   peak_values — numeric vector of peak power values (W)
#   levels      — numeric vector of cap levels to evaluate
#
# Returns: tibble with columns: cap_w, total_wh, clipped_wh, pct_lost,
#          hours_clipped, pct_hours_clipped
#
# Used by: 05_export_clipping.R, 09_cost_of_clipping.R
compute_clipping <- function(peak_values, levels = seq(500, 8000, by = 100)) {
  total <- sum(peak_values, na.rm = TRUE)
  n_hours <- sum(!is.na(peak_values))

  # pmax(x - cap, 0) computes the excess above cap for each reading.
  # Since data is hourly, watts ≈ watt-hours per hour bucket.
  map_dfr(levels, function(cap) {
    clipped <- pmax(peak_values - cap, 0)
    tibble(
      cap_w              = cap,
      total_wh           = total,
      clipped_wh         = sum(clipped, na.rm = TRUE),
      pct_lost           = sum(clipped, na.rm = TRUE) / total * 100,
      hours_clipped      = sum(clipped > 0, na.rm = TRUE),
      pct_hours_clipped  = sum(clipped > 0, na.rm = TRUE) / n_hours * 100
    )
  })
}

# --- annualize_factor() ------------------------------------------------------
# Compute a scaling factor to extrapolate partial-year data to a full year.
# If the data spans 200 days, the factor is 365.25 / 200 ≈ 1.83.
#
# Args:
#   timestamps — vector of POSIXct timestamps
#
# Returns: numeric scalar (the multiplication factor)
#
# Used by: 09_cost_of_clipping.R
annualize_factor <- function(timestamps) {
  days_span <- as.numeric(difftime(max(timestamps), min(timestamps), units = "days"))
  if (days_span <= 0) return(1)
  365.25 / days_span
}
