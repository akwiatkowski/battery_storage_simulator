//! Battery Simulator — Rust/WASM core
//!
//! Three battery strategies running entirely client-side:
//!   1. DP Optimal — backward dynamic programming (200 SoC bins)
//!   2. Arbitrage — P33/P67 daily percentile heuristic
//!   3. Self-consumption — charge excess PV, discharge to offset import
//!
//! The single WASM export `simulate(days_json, params_json)` runs all three
//! strategies plus a no-battery baseline and returns JSON with SoC traces
//! and costs for Chart.js rendering.

use serde::{Deserialize, Serialize};
use wasm_bindgen::prelude::*;

// ── Input types (deserialized from JS JSON) ──────────────────────────────────

/// One day of hourly data: 20-24 values for net load and spot price.
#[derive(Deserialize)]
struct DayData {
    date: String,
    net_load_w: Vec<f64>,      // positive = grid import, negative = PV export
    price_pln_kwh: Vec<f64>,   // spot electricity price per hour
}

/// Battery configuration from the UI sliders.
#[derive(Deserialize)]
struct BatteryParams {
    capacity_kwh: f64,
    max_power_w: f64,       // max charge/discharge rate (W)
    soc_min_pct: f64,       // minimum SoC as % of capacity
    soc_max_pct: f64,       // maximum SoC as % of capacity
    export_coeff: f64,      // export revenue multiplier (0-1, accounts for grid fees)
}

// ── Output types (serialized back to JS) ─────────────────────────────────────

/// Result for a single battery strategy.
#[derive(Serialize)]
struct StrategyResult {
    soc_kwh: Vec<f64>,        // SoC after each hour (kWh)
    total_cost_pln: f64,      // net electricity cost over the period
}

/// Complete simulation result returned to JavaScript.
#[derive(Serialize)]
struct SimResult {
    hours: usize,                    // total data points
    dates: Vec<String>,              // date labels for x-axis
    net_load_w: Vec<f64>,            // flattened net load (for price/load chart)
    price_pln_kwh: Vec<f64>,         // flattened prices (for price/load chart)
    heuristic: StrategyResult,       // P33/P67 arbitrage
    self_consumption: StrategyResult,
    optimal: StrategyResult,         // DP-optimized schedule
    no_battery_cost_pln: f64,        // baseline cost without any battery
}

// ── Grid cost helper ─────────────────────────────────────────────────────────

/// Compute electricity cost for one hour given net load and battery action.
///
/// cost = (import_W × price - export_W × price × export_coeff) / 1000
///
/// `charge` and `discharge` are the battery's power draw this hour (Wh since 1h slots).
/// Positive `net` after battery = grid import; negative = grid export.
#[inline]
fn hour_cost(net_load: f64, charge: f64, discharge: f64, price: f64, export_coeff: f64) -> f64 {
    let net = net_load + charge - discharge;
    let imp = if net > 0.0 { net } else { 0.0 };
    let exp = if net < 0.0 { -net } else { 0.0 };
    (imp * price - exp * price * export_coeff) / 1000.0
}

// ── P33/P67 percentile ───────────────────────────────────────────────────────

/// Compute daily P33/P67 price thresholds using Go-compatible indexing.
///
/// Go uses `index = (n-1) * pct / 100` (integer truncation), which differs
/// from numpy's linear interpolation. We match Go for consistent results.
fn daily_percentiles(prices: &[f64]) -> (f64, f64) {
    let mut sorted: Vec<f64> = prices.to_vec();
    sorted.sort_by(|a, b| a.partial_cmp(b).unwrap_or(std::cmp::Ordering::Equal));
    let n = sorted.len();
    if n == 0 {
        return (0.0, 0.0);
    }
    let idx33 = ((n - 1) as f64 * 33.0 / 100.0) as usize;
    let idx67 = ((n - 1) as f64 * 67.0 / 100.0) as usize;
    (sorted[idx33], sorted[idx67])
}

// ── Strategy 1: Arbitrage heuristic (P33/P67) ───────────────────────────────
//
// Charges at max power when price <= P33 (cheapest third of the day),
// discharges at max power when price >= P67 (most expensive third).
// Can import from grid to charge (unlike self-consumption).
// Thresholds are computed per-day even in multi-day ranges.

fn run_heuristic(
    net_load: &[f64],
    price: &[f64],
    max_power_w: f64,
    soc_min_wh: f64,
    soc_max_wh: f64,
    export_coeff: f64,
    initial_soc_wh: f64,
    day_boundaries: &[usize],   // index where each new day starts
) -> StrategyResult {
    let t = net_load.len();
    let mut soc_kwh = Vec::with_capacity(t);
    let mut current_soc = initial_soc_wh;
    let mut total_cost = 0.0;

    // Pre-compute P33/P67 thresholds for each day's price slice
    let mut thresholds: Vec<(f64, f64)> = Vec::new();
    for i in 0..day_boundaries.len() {
        let start = day_boundaries[i];
        let end = if i + 1 < day_boundaries.len() {
            day_boundaries[i + 1]
        } else {
            t
        };
        thresholds.push(daily_percentiles(&price[start..end]));
    }

    // Map each hour to its day's index (for threshold lookup)
    let mut hour_day = vec![0usize; t];
    for i in 0..day_boundaries.len() {
        let start = day_boundaries[i];
        let end = if i + 1 < day_boundaries.len() {
            day_boundaries[i + 1]
        } else {
            t
        };
        for h in start..end {
            hour_day[h] = i;
        }
    }

    // Forward simulation: charge/discharge based on price vs thresholds
    for i in 0..t {
        let p = price[i];
        let (p33, p67) = thresholds[hour_day[i]];

        let charge;
        let discharge;

        if p <= p33 {
            // Cheap hour: charge as much as possible (capped by power and headroom)
            charge = max_power_w.min(soc_max_wh - current_soc).max(0.0);
            discharge = 0.0;
        } else if p >= p67 {
            // Expensive hour: discharge as much as possible
            charge = 0.0;
            discharge = max_power_w.min(current_soc - soc_min_wh).max(0.0);
        } else {
            // Mid-price: hold
            charge = 0.0;
            discharge = 0.0;
        }

        current_soc += charge - discharge;
        soc_kwh.push(current_soc / 1000.0);
        total_cost += hour_cost(net_load[i], charge, discharge, p, export_coeff);
    }

    StrategyResult {
        soc_kwh,
        total_cost_pln: total_cost,
    }
}

// ── Strategy 2: Self-consumption ─────────────────────────────────────────────
//
// Charges only from excess PV (when net_load < 0, i.e., house is exporting).
// Discharges to offset grid import (when net_load > 0).
// Never imports from grid to charge — purely PV-driven.

fn run_self_consumption(
    net_load: &[f64],
    price: &[f64],
    max_power_w: f64,
    soc_min_wh: f64,
    soc_max_wh: f64,
    export_coeff: f64,
    initial_soc_wh: f64,
) -> StrategyResult {
    let t = net_load.len();
    let mut soc_kwh = Vec::with_capacity(t);
    let mut current_soc = initial_soc_wh;
    let mut total_cost = 0.0;

    for i in 0..t {
        let nl = net_load[i];

        let charge;
        let discharge;

        if nl < 0.0 {
            // Excess PV production: divert to battery instead of exporting
            charge = (-nl).min(max_power_w).min(soc_max_wh - current_soc).max(0.0);
            discharge = 0.0;
        } else {
            // Net consumption: discharge battery to reduce grid import
            charge = 0.0;
            discharge = nl.min(max_power_w).min(current_soc - soc_min_wh).max(0.0);
        }

        current_soc += charge - discharge;
        soc_kwh.push(current_soc / 1000.0);
        total_cost += hour_cost(nl, charge, discharge, price[i], export_coeff);
    }

    StrategyResult {
        soc_kwh,
        total_cost_pln: total_cost,
    }
}

// ── Strategy 3: DP Optimal ───────────────────────────────────────────────────
//
// Dynamic programming optimizer that finds the minimum-cost battery schedule.
// Equivalent to the Python LP optimizer (scipy linprog) but implemented as
// backward DP with discretized SoC — no external solver needed.
//
// Algorithm:
//   1. Discretize SoC range [soc_min, soc_max] into N_BINS levels.
//   2. Backward sweep: for each hour t from T-1 to 0, for each SoC bin s,
//      find the transition to bin s' that minimizes:
//        hour_cost(t, s→s') + dp_next[s']
//      Only bins reachable within max_power_w are considered.
//   3. Forward trace: starting from initial SoC bin, follow the policy
//      to reconstruct the optimal SoC path.
//
// Complexity: O(T × N_BINS × max_reachable_bins)
//   Typical: 24h × 200 bins × ~100 reachable = 480K ops → <1ms in WASM
//
// Discretization error: <0.5% with 200 bins (50 Wh resolution for 10 kWh battery).

const N_BINS: usize = 200;

fn run_optimal(
    net_load: &[f64],
    price: &[f64],
    max_power_w: f64,
    soc_min_wh: f64,
    soc_max_wh: f64,
    export_coeff: f64,
    initial_soc_wh: f64,
) -> StrategyResult {
    let t = net_load.len();
    if t == 0 {
        return StrategyResult {
            soc_kwh: vec![],
            total_cost_pln: 0.0,
        };
    }

    let soc_range = soc_max_wh - soc_min_wh;
    if soc_range <= 0.0 {
        // Degenerate case: no usable capacity
        let mut total = 0.0;
        for i in 0..t {
            total += hour_cost(net_load[i], 0.0, 0.0, price[i], export_coeff);
        }
        return StrategyResult {
            soc_kwh: vec![soc_min_wh / 1000.0; t],
            total_cost_pln: total,
        };
    }

    let bin_wh = soc_range / N_BINS as f64;  // Wh per bin

    // Conversion helpers between bin index and Wh
    let bin_to_wh = |b: usize| -> f64 { soc_min_wh + b as f64 * bin_wh };
    let wh_to_bin = |wh: f64| -> usize {
        let b = ((wh - soc_min_wh) / bin_wh).round() as isize;
        b.max(0).min(N_BINS as isize) as usize
    };

    // Max number of bins reachable in one hour (bounded by max charge/discharge power)
    let max_bin_delta = (max_power_w / bin_wh).ceil() as usize;

    let inf = f64::MAX / 2.0;

    // dp_next[s] = min cost from hour (t+1) to end, starting at bin s
    // dp_curr[s] = min cost from hour t to end, starting at bin s
    // We only need two arrays (not T×N_BINS) since we sweep backward
    let mut dp_next = vec![inf; N_BINS + 1];
    let mut dp_curr = vec![inf; N_BINS + 1];

    // policy[t][s] = which SoC bin to transition to at hour t from bin s
    let mut policy = vec![vec![0u16; N_BINS + 1]; t];

    // Terminal condition: zero cost at the end regardless of SoC
    for s in 0..=N_BINS {
        dp_next[s] = 0.0;
    }

    // ── Backward sweep: fill dp table from last hour to first ──
    for hour in (0..t).rev() {
        let nl = net_load[hour];
        let p = price[hour];

        for s in 0..=N_BINS {
            let soc_wh = bin_to_wh(s);
            let mut best_cost = inf;
            let mut best_next = s as u16;

            // Only check bins reachable within one hour's charge/discharge
            let s_lo = if s >= max_bin_delta { s - max_bin_delta } else { 0 };
            let s_hi = (s + max_bin_delta).min(N_BINS);

            for s2 in s_lo..=s_hi {
                let soc2_wh = bin_to_wh(s2);
                let delta = soc2_wh - soc_wh; // positive = charging, negative = discharging

                let (charge, discharge) = if delta >= 0.0 {
                    (delta, 0.0)
                } else {
                    (0.0, -delta)
                };

                let cost = hour_cost(nl, charge, discharge, p, export_coeff) + dp_next[s2];

                if cost < best_cost {
                    best_cost = cost;
                    best_next = s2 as u16;
                }
            }

            dp_curr[s] = best_cost;
            policy[hour][s] = best_next;
        }

        // Swap arrays: current becomes next for the preceding hour
        std::mem::swap(&mut dp_curr, &mut dp_next);
    }

    // ── Forward trace: reconstruct optimal SoC path ──
    let mut soc_kwh = Vec::with_capacity(t);
    let mut current_bin = wh_to_bin(initial_soc_wh);
    let mut total_cost = 0.0;

    for hour in 0..t {
        let next_bin = policy[hour][current_bin] as usize;
        let soc_wh = bin_to_wh(current_bin);
        let soc2_wh = bin_to_wh(next_bin);
        let delta = soc2_wh - soc_wh;

        let (charge, discharge) = if delta >= 0.0 {
            (delta, 0.0)
        } else {
            (0.0, -delta)
        };

        total_cost += hour_cost(net_load[hour], charge, discharge, price[hour], export_coeff);
        soc_kwh.push(soc2_wh / 1000.0);
        current_bin = next_bin;
    }

    StrategyResult {
        soc_kwh,
        total_cost_pln: total_cost,
    }
}

// ── No-battery baseline ──────────────────────────────────────────────────────

/// Compute total cost without any battery — direct grid import/export.
fn no_battery_cost(net_load: &[f64], price: &[f64], export_coeff: f64) -> f64 {
    let mut total = 0.0;
    for i in 0..net_load.len() {
        total += hour_cost(net_load[i], 0.0, 0.0, price[i], export_coeff);
    }
    total
}

// ── WASM export ──────────────────────────────────────────────────────────────

/// Main entry point called from JavaScript.
///
/// Takes JSON arrays of day data and battery parameters, runs all strategies,
/// returns a JSON string with SoC traces and cost comparisons.
///
/// Input `days_json`: `[{date, net_load_w: [f64], price_pln_kwh: [f64]}, ...]`
/// Input `params_json`: `{capacity_kwh, max_power_w, soc_min_pct, soc_max_pct, export_coeff}`
#[wasm_bindgen]
pub fn simulate(days_json: &str, params_json: &str) -> String {
    let days: Vec<DayData> = serde_json::from_str(days_json).unwrap_or_default();
    let params: BatteryParams = serde_json::from_str(params_json).unwrap_or(BatteryParams {
        capacity_kwh: 10.0,
        max_power_w: 5000.0,
        soc_min_pct: 10.0,
        soc_max_pct: 90.0,
        export_coeff: 0.8,
    });

    // Flatten multi-day data into contiguous arrays for simulation.
    // Track day boundaries so the heuristic can compute per-day percentiles.
    let mut all_net_load: Vec<f64> = Vec::new();
    let mut all_price: Vec<f64> = Vec::new();
    let mut dates: Vec<String> = Vec::new();
    let mut day_boundaries: Vec<usize> = Vec::new();

    for day in &days {
        day_boundaries.push(all_net_load.len());
        dates.push(day.date.clone());
        all_net_load.extend_from_slice(&day.net_load_w);
        all_price.extend_from_slice(&day.price_pln_kwh);
    }

    // Convert percentage-based params to absolute Wh values
    let capacity_wh = params.capacity_kwh * 1000.0;
    let soc_min_wh = capacity_wh * params.soc_min_pct / 100.0;
    let soc_max_wh = capacity_wh * params.soc_max_pct / 100.0;
    let initial_soc = soc_min_wh;  // start at minimum SoC

    // Run all three strategies on the same data
    let heuristic = run_heuristic(
        &all_net_load, &all_price,
        params.max_power_w, soc_min_wh, soc_max_wh,
        params.export_coeff, initial_soc, &day_boundaries,
    );

    let self_consumption = run_self_consumption(
        &all_net_load, &all_price,
        params.max_power_w, soc_min_wh, soc_max_wh,
        params.export_coeff, initial_soc,
    );

    let optimal = run_optimal(
        &all_net_load, &all_price,
        params.max_power_w, soc_min_wh, soc_max_wh,
        params.export_coeff, initial_soc,
    );

    let no_batt_cost = no_battery_cost(&all_net_load, &all_price, params.export_coeff);

    // Pack everything into JSON for the frontend
    let result = SimResult {
        hours: all_net_load.len(),
        dates,
        net_load_w: all_net_load,
        price_pln_kwh: all_price,
        heuristic,
        self_consumption,
        optimal,
        no_battery_cost_pln: no_batt_cost,
    };

    serde_json::to_string(&result).unwrap_or_default()
}
