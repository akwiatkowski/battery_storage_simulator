"""MPC Battery Controller: continuous optimization loop with 24h horizon.

Fetches weather forecasts, predicts PV/consumption/HP/DHW/price using trained
LightGBM models, runs LP battery optimizer, and prints the recommended action
every --interval minutes.

Usage:
    mise exec -- python -m forecast.src.controller --capacity 10 --power 5000
    mise exec -- python -m forecast.src.controller --capacity 10 --power 5000 --interval 1
"""

import argparse
import time

import numpy as np
import pandas as pd

from .config import load_config, python_root
from .features import (
    build_pv_features,
    build_consumption_features,
    build_heating_features,
    build_dhw_features,
    build_spot_price_features,
)
from .models.lightgbm_model import LightGBMModel
from .optimize import BatteryParams, optimize_battery, simulate_no_battery
from .weather import fetch_forecast


class ForecastCache:
    """Cache weather + model predictions, refresh when stale."""

    def __init__(self, ttl_minutes: int = 60):
        self.ttl = ttl_minutes
        self._predictions: dict | None = None
        self._fetched_at: pd.Timestamp | None = None

    def get_predictions(self, config: dict, horizon_hours: int) -> dict:
        """Return cached predictions, re-fetch if stale."""
        now = pd.Timestamp.now(tz="UTC")
        if (
            self._fetched_at
            and (now - self._fetched_at).total_seconds() < self.ttl * 60
        ):
            return self._predictions

        try:
            self._predictions = forecast_all(config, horizon_hours)
            self._fetched_at = now
        except Exception as e:
            if self._predictions is not None:
                print(f"  Warning: forecast failed ({e}), reusing cached data")
                self._fetched_at = now  # extend TTL
            else:
                raise

        return self._predictions


def _load_models() -> dict[str, LightGBMModel]:
    """Load all 5 models from disk."""
    model_dir = python_root() / "models"
    models = {}
    for name in ("pv", "consumption", "heat_pump", "spot_price", "dhw"):
        models[name] = LightGBMModel.load(str(model_dir / f"{name}_model"))
    return models


def _predict_model(
    model: LightGBMModel, features: pd.DataFrame
) -> np.ndarray:
    """Predict using only the features the model was trained with."""
    expected = model.feature_names
    return model.predict(features[expected])


def forecast_all(config: dict, horizon: int) -> dict:
    """Fetch weather once + run all 5 models.

    Returns dict with keys pv/consumption/heat_pump/dhw/spot_price,
    each as a pd.Series indexed by UTC hourly timestamps (W or PLN/kWh).
    """
    loc = config["location"]
    capacity = config["pv_system"]["capacity_kwp"]

    print("  Fetching weather forecast...")
    weather_df = fetch_forecast(loc["latitude"], loc["longitude"], horizon)

    models = _load_models()

    # Common hourly UTC index for the horizon
    now = pd.Timestamp.now(tz="UTC").floor("h")
    full_idx = pd.date_range(now, periods=horizon, freq="h", tz="UTC")

    # --- PV ---
    pv_features = build_pv_features(weather_df, config)
    pv_pred = (_predict_model(models["pv"], pv_features) * capacity).clip(
        min=0
    )
    # PV features only cover daytime; fill nighttime with 0
    pv_series = pd.Series(pv_pred, index=pv_features.index).reindex(
        full_idx, fill_value=0.0
    )

    # --- Consumption (needs load_lag_1h: 2-pass autoregressive) ---
    cons_features = build_consumption_features(weather_df, config)
    cons_features = cons_features.reindex(full_idx, method="nearest")
    # First pass: use a reasonable default for lag
    cons_features["load_lag_1h"] = 300.0  # typical household average
    pass1 = _predict_model(models["consumption"], cons_features).clip(min=0)
    # Second pass: use shifted predictions as lag
    cons_features["load_lag_1h"] = (
        pd.Series(pass1, index=full_idx).shift(1).bfill()
    )
    cons_pred = _predict_model(models["consumption"], cons_features).clip(
        min=0
    )
    cons_series = pd.Series(cons_pred, index=full_idx)

    # --- Heat Pump ---
    hp_features = build_heating_features(weather_df, config)
    hp_features = hp_features.reindex(full_idx, method="nearest")
    # HP model trained at 6h resolution has temp_min feature
    if "temp_min" not in hp_features.columns:
        hp_features["temp_min"] = hp_features["temperature"]
    hp_pred = _predict_model(models["heat_pump"], hp_features).clip(min=0)
    hp_series = pd.Series(hp_pred, index=full_idx)

    # --- Spot Price (needs price lags: 2-pass) ---
    sp_features = build_spot_price_features(weather_df, config)
    sp_features = sp_features.reindex(full_idx, method="nearest")
    # First pass: use typical averages for lags
    sp_features["price_lag_1h"] = 0.4
    sp_features["price_lag_24h"] = 0.4
    sp_features["price_rolling_24h_mean"] = 0.4
    pass1_price = _predict_model(models["spot_price"], sp_features)
    # Second pass: use shifted predictions as lags
    price_s = pd.Series(pass1_price, index=full_idx)
    sp_features["price_lag_1h"] = price_s.shift(1).bfill()
    sp_features["price_lag_24h"] = price_s.shift(24).bfill()
    sp_features["price_rolling_24h_mean"] = price_s.rolling(
        24, min_periods=1
    ).mean()
    sp_pred = _predict_model(models["spot_price"], sp_features)
    sp_series = pd.Series(sp_pred, index=full_idx)

    # --- DHW (needs temperature from weather) ---
    dhw_features = build_dhw_features(full_idx, config, weather_df=weather_df)
    dhw_pred = _predict_model(models["dhw"], dhw_features).clip(min=0)
    dhw_series = pd.Series(dhw_pred, index=full_idx)

    return {
        "pv": pv_series,
        "consumption": cons_series,
        "heat_pump": hp_series,
        "dhw": dhw_series,
        "spot_price": sp_series,
    }


def build_optimization_input(
    forecasts: dict, current_hour: pd.Timestamp, horizon: int
) -> tuple[np.ndarray, np.ndarray]:
    """Slice predictions from current_hour for horizon hours.

    Returns (net_load_w, price_pln_kwh) as numpy arrays.
    """
    idx = pd.date_range(current_hour, periods=horizon, freq="h", tz="UTC")

    consumption = forecasts["consumption"].reindex(idx, method="nearest").fillna(0).values
    hp = forecasts["heat_pump"].reindex(idx, method="nearest").fillna(0).values
    dhw = forecasts["dhw"].reindex(idx, method="nearest").fillna(0).values
    pv = forecasts["pv"].reindex(idx, fill_value=0).values
    price = forecasts["spot_price"].reindex(idx, method="nearest").fillna(0.4).values

    net_load = consumption + hp + dhw - pv
    return net_load, price


def print_full_schedule(
    forecasts: dict,
    params: BatteryParams,
    initial_soc_wh: float,
    horizon: int,
    timezone: str,
) -> None:
    """Print the complete hourly LP-optimal schedule."""
    now = pd.Timestamp.now(tz="UTC").floor("h")
    net_load, price = build_optimization_input(forecasts, now, horizon)

    # Summary totals (only for available hours)
    pv_kwh = forecasts["pv"].clip(lower=0).sum() / 1000
    cons_kwh = forecasts["consumption"].clip(lower=0).sum() / 1000
    hp_kwh = forecasts["heat_pump"].clip(lower=0).sum() / 1000
    dhw_kwh = forecasts["dhw"].clip(lower=0).sum() / 1000
    load_kwh = cons_kwh + hp_kwh + dhw_kwh

    print(f"PV forecast: {pv_kwh:.1f} kWh total")
    print(
        f"Load forecast: {load_kwh:.1f} kWh total "
        f"(consumption {cons_kwh:.1f} + HP {hp_kwh:.1f} + DHW {dhw_kwh:.1f})"
    )
    print(f"Price range: {price.min():.2f}\u2013{price.max():.2f} PLN/kWh")

    # Optimize
    opt = optimize_battery(net_load, price, params, initial_soc_wh)
    no_batt = simulate_no_battery(net_load, price, params.export_coeff)

    print(f"\n{horizon}h Optimal Schedule:")
    print(f"  {'Hour':<7} {'Price':>6} {'Action':<11} {'Power':>6} {'SoC':>15}")

    for t in range(len(net_load)):
        ts = (now + pd.Timedelta(hours=t)).tz_convert(timezone)
        p = price[t]
        charge = opt.charge_w[t]
        discharge = opt.discharge_w[t]
        soc_before = initial_soc_wh if t == 0 else opt.soc_wh[t - 1]
        soc_after = opt.soc_wh[t]
        soc_before_pct = soc_before / params.capacity_wh * 100
        soc_after_pct = soc_after / params.capacity_wh * 100

        if charge > 10:
            action, power_str = "CHARGE", f"{charge:.0f}W"
        elif discharge > 10:
            action, power_str = "DISCHARGE", f"{discharge:.0f}W"
        else:
            action, power_str = "HOLD", "0W"

        print(
            f"  {ts.strftime('%H:%M'):<7} {p:>6.2f} "
            f"{action:<11} {power_str:>6} "
            f"{soc_before_pct:>5.0f}% \u2192 {soc_after_pct:.0f}%"
        )

    savings = no_batt.total_cost_pln - opt.total_cost_pln
    if no_batt.total_cost_pln != 0:
        savings_pct = savings / abs(no_batt.total_cost_pln) * 100
    else:
        savings_pct = 0
    print(
        f"\nExpected savings: {savings:.1f} PLN vs no-battery "
        f"({savings_pct:.1f}% reduction)"
    )


def _format_action(
    action: str,
    power_w: float,
    energy_kwh: float,
    soc_before_pct: float,
    soc_after_pct: float,
    interval_min: int,
) -> str:
    """Format a single action line for console output."""
    ts = pd.Timestamp.now(tz="UTC").strftime("%H:%M:%S")
    if action == "HOLD":
        return (
            f"{ts} \u2502 HOLD"
            f"{'':<24}\u2502"
            f"{'':<12}\u2502 SoC {soc_before_pct:.1f}%"
        )
    sign = "+" if action == "CHARGE" else "-"
    return (
        f"{ts} \u2502 {action:<10} {power_w:.0f}W for {interval_min}min "
        f"\u2502 {sign}{abs(energy_kwh):.2f} kWh "
        f"\u2502 SoC {soc_before_pct:.1f}% \u2192 {soc_after_pct:.1f}%"
    )


def run_cycle(
    cache: ForecastCache,
    config: dict,
    params: BatteryParams,
    current_soc_wh: float,
    interval_min: int,
    horizon: int,
) -> tuple[str, float]:
    """One MPC cycle: get predictions, optimize, extract first action.

    Returns (formatted_action_string, new_soc_wh).
    """
    forecasts = cache.get_predictions(config, horizon)
    now = pd.Timestamp.now(tz="UTC").floor("h")
    net_load, price = build_optimization_input(forecasts, now, horizon)

    # Optimize from current SoC
    try:
        opt = optimize_battery(net_load, price, params, current_soc_wh)
        if opt.status != "optimal":
            raise RuntimeError(f"LP status: {opt.status}")
    except Exception as e:
        print(f"  Warning: optimizer failed ({e}), falling back to HOLD")
        soc_pct = current_soc_wh / params.capacity_wh * 100
        return (
            _format_action("HOLD", 0, 0, soc_pct, soc_pct, interval_min),
            current_soc_wh,
        )

    # Extract hour 0's action, scale energy by interval fraction
    charge_w = opt.charge_w[0]
    discharge_w = opt.discharge_w[0]
    fraction = interval_min / 60.0
    soc_before_pct = current_soc_wh / params.capacity_wh * 100

    if charge_w > 10:
        energy_wh = min(
            charge_w * fraction, params.soc_max_wh - current_soc_wh
        )
        energy_wh = max(0, energy_wh)
        new_soc = current_soc_wh + energy_wh
        action_str = _format_action(
            "CHARGE",
            charge_w,
            energy_wh / 1000,
            soc_before_pct,
            new_soc / params.capacity_wh * 100,
            interval_min,
        )
    elif discharge_w > 10:
        energy_wh = min(
            discharge_w * fraction, current_soc_wh - params.soc_min_wh
        )
        energy_wh = max(0, energy_wh)
        new_soc = current_soc_wh - energy_wh
        action_str = _format_action(
            "DISCHARGE",
            discharge_w,
            energy_wh / 1000,
            soc_before_pct,
            new_soc / params.capacity_wh * 100,
            interval_min,
        )
    else:
        new_soc = current_soc_wh
        action_str = _format_action(
            "HOLD", 0, 0, soc_before_pct, soc_before_pct, interval_min
        )

    return action_str, new_soc


def main():
    parser = argparse.ArgumentParser(description="MPC Battery Controller")
    parser.add_argument(
        "--capacity", type=float, default=10.0, help="Battery capacity (kWh)"
    )
    parser.add_argument(
        "--power", type=float, default=5000, help="Max charge/discharge (W)"
    )
    parser.add_argument(
        "--soc-min", type=float, default=10, help="Min SoC (%%)"
    )
    parser.add_argument(
        "--soc-max", type=float, default=90, help="Max SoC (%%)"
    )
    parser.add_argument(
        "--soc-initial", type=float, default=50, help="Starting SoC (%%)"
    )
    parser.add_argument(
        "--export-coeff", type=float, default=0.8, help="Export coefficient"
    )
    parser.add_argument(
        "--interval", type=int, default=15, help="Replan interval (minutes)"
    )
    parser.add_argument(
        "--horizon", type=int, default=24, help="Optimization horizon (hours)"
    )
    args = parser.parse_args()

    config = load_config()
    timezone = config["location"]["timezone"]

    capacity_wh = args.capacity * 1000
    params = BatteryParams(
        capacity_wh=capacity_wh,
        max_power_w=args.power,
        soc_min_wh=capacity_wh * args.soc_min / 100,
        soc_max_wh=capacity_wh * args.soc_max / 100,
        export_coeff=args.export_coeff,
    )

    current_soc_wh = capacity_wh * args.soc_initial / 100
    initial_soc_pct = args.soc_initial

    print("=== Battery Controller Started ===")
    print(
        f"Battery: {args.capacity} kWh, {args.power:.0f} W | "
        f"SoC: {initial_soc_pct:.1f}% | Interval: {args.interval}min"
    )
    print()

    # Initial forecast + full schedule
    cache = ForecastCache(ttl_minutes=60)
    forecasts = cache.get_predictions(config, args.horizon)
    print_full_schedule(forecasts, params, current_soc_wh, args.horizon, timezone)

    print(f"\n\u2500\u2500\u2500 Live Control (Ctrl+C to stop) \u2500\u2500\u2500")
    print()

    # Session tracking
    start_time = time.time()
    cycle_count = 0
    charge_count = 0
    discharge_count = 0
    hold_count = 0
    total_charged_kwh = 0.0
    total_discharged_kwh = 0.0

    try:
        while True:
            action_str, new_soc = run_cycle(
                cache, config, params, current_soc_wh, args.interval, args.horizon
            )
            print(action_str)

            # Track stats
            cycle_count += 1
            energy_delta_wh = new_soc - current_soc_wh
            if energy_delta_wh > 0:
                charge_count += 1
                total_charged_kwh += energy_delta_wh / 1000
            elif energy_delta_wh < 0:
                discharge_count += 1
                total_discharged_kwh += abs(energy_delta_wh) / 1000
            else:
                hold_count += 1

            current_soc_wh = new_soc
            time.sleep(args.interval * 60)

    except KeyboardInterrupt:
        elapsed = time.time() - start_time
        hours = int(elapsed // 3600)
        minutes = int((elapsed % 3600) // 60)
        final_soc_pct = current_soc_wh / capacity_wh * 100

        print(f"\n\u2500\u2500\u2500 Session Summary \u2500\u2500\u2500")
        if hours > 0:
            print(f"Duration: {hours}h {minutes}min ({cycle_count} cycles)")
        else:
            print(f"Duration: {minutes}min ({cycle_count} cycles)")
        print(
            f"Actions: {charge_count} charge, "
            f"{discharge_count} discharge, {hold_count} hold"
        )
        print(
            f"Energy: +{total_charged_kwh:.1f} kWh charged, "
            f"-{total_discharged_kwh:.1f} kWh discharged"
        )
        print(f"SoC: {initial_soc_pct:.1f}% \u2192 {final_soc_pct:.1f}%")


if __name__ == "__main__":
    main()
