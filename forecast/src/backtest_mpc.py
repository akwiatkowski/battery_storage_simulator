"""MPC Forecast Backtest: compare LP optimizer using ML forecasts vs actual data.

Replays historical days to evaluate the cost of imperfect forecasting — the gap
between what the controller achieves with predictions and what's theoretically
optimal with perfect information.

Usage:
    mise exec -- python -m forecast.src.backtest_mpc --days 14
    mise exec -- python -m forecast.src.backtest_mpc --days 14 --capacity 10 --power 5000 --plot
"""

import argparse
from datetime import date, timedelta

import numpy as np
import pandas as pd

from .config import load_config, project_root, python_root
from .data_loading import load_sensor_data, load_spot_prices
from .features import (
    build_pv_features,
    build_consumption_features,
    build_heating_features,
    build_dhw_features,
    build_spot_price_features,
)
from .models.lightgbm_model import LightGBMModel
from .optimize import BatteryParams, optimize_battery, simulate_no_battery
from .weather import fetch_historical


def load_models() -> dict[str, LightGBMModel]:
    """Load all 5 trained LightGBM models."""
    model_dir = python_root() / "models"
    models = {}
    for name in ("pv", "consumption", "heat_pump", "spot_price", "dhw"):
        models[name] = LightGBMModel.load(str(model_dir / f"{name}_model"))
    return models


def predict_model(model: LightGBMModel, features: pd.DataFrame) -> np.ndarray:
    """Predict using only the features the model was trained with."""
    return model.predict(features[model.feature_names])


def load_all_sensor_data(config: dict) -> dict[str, pd.Series]:
    """Load and resample all sensor data to hourly UTC series."""
    root = project_root()
    sensors = config["sensors"]

    sensor_defs = {
        "grid": (sensors["grid_power"], "grid_power.csv"),
        "pv": (sensors["pv_power"], "pv_power.csv"),
        "hp": (sensors["hp_heating"], "pump_heat_power_consumed.csv"),
        "dhw": (sensors["hp_dhw"], "pump_cwu_power_consumed.csv"),
    }

    hourly = {}
    for name, (sensor_id, legacy_file) in sensor_defs.items():
        df = load_sensor_data(
            sensor_id=sensor_id,
            legacy_path=root / "input" / legacy_file,
            recent_dir=root / "input" / "recent",
        )
        if df.empty:
            print(f"  Warning: no data for {name} ({sensor_id})")
            continue
        df = df.set_index("timestamp")
        df.index = df.index.tz_convert("UTC")
        hourly[name] = df["value"].resample("h").mean()
        print(f"  {name}: {len(hourly[name])} hourly samples")

    # Load spot prices
    spot_df = load_spot_prices(root / "input" / "recent" / "historic_spot_prices.csv")
    spot_df = spot_df.set_index("timestamp")
    if spot_df.index.tz is None:
        spot_df.index = spot_df.index.tz_localize("UTC")
    else:
        spot_df.index = spot_df.index.tz_convert("UTC")
    hourly["price"] = spot_df["value"].resample("h").mean()
    print(f"  price: {len(hourly['price'])} hourly samples")

    return hourly


def get_day_actuals(
    hourly: dict[str, pd.Series], day: pd.Timestamp
) -> dict[str, np.ndarray] | None:
    """Extract aligned hourly actual data for one day. Returns None if insufficient."""
    day_end = day + pd.Timedelta(days=1)

    # Grid and price are required
    for key in ("grid", "price"):
        if key not in hourly:
            return None

    grid = hourly["grid"].loc[day:day_end - pd.Timedelta(hours=1)].dropna()
    price = hourly["price"].loc[day:day_end - pd.Timedelta(hours=1)].dropna()

    # Intersect on common hours
    common_idx = grid.index.intersection(price.index)
    if len(common_idx) < 20:
        return None

    grid = grid.reindex(common_idx)
    price = price.reindex(common_idx)
    pv = hourly.get("pv", pd.Series(dtype=float)).reindex(common_idx).fillna(0).clip(lower=0)
    hp = hourly.get("hp", pd.Series(dtype=float)).reindex(common_idx).fillna(0).clip(lower=0)
    dhw = hourly.get("dhw", pd.Series(dtype=float)).reindex(common_idx).fillna(0).clip(lower=0)

    # Derive base consumption: grid + pv - hp - dhw
    consumption = (grid + pv - hp - dhw).clip(lower=0)

    return {
        "index": common_idx,
        "grid": grid.values,
        "pv": pv.values,
        "hp": hp.values,
        "dhw": dhw.values,
        "consumption": consumption.values,
        "price": price.values,
        "net_load": grid.values,  # grid = net load (positive=import)
    }


def generate_forecast(
    models: dict[str, LightGBMModel],
    config: dict,
    day: pd.Timestamp,
    actuals: dict[str, np.ndarray],
) -> dict[str, np.ndarray]:
    """Generate ML forecasts for one day, mimicking what the live controller does."""
    loc = config["location"]
    capacity = config["pv_system"]["capacity_kwp"]
    idx = actuals["index"]
    T = len(idx)

    # Load weather for this day
    cache_dir = project_root() / "input" / "weather"
    day_date = day.date() if hasattr(day, 'date') else day
    weather_df = fetch_historical(
        loc["latitude"], loc["longitude"],
        day_date, day_date,
        cache_dir,
    )
    if weather_df.empty:
        raise ValueError(f"No weather data for {day_date}")

    # --- PV ---
    pv_features = build_pv_features(weather_df, config)
    pv_pred = (predict_model(models["pv"], pv_features) * capacity).clip(min=0)
    pv_series = pd.Series(pv_pred, index=pv_features.index).reindex(idx, fill_value=0.0)

    # --- Consumption (2-pass autoregressive) ---
    cons_features = build_consumption_features(weather_df, config)
    cons_features = cons_features.reindex(idx, method="nearest")
    # First pass with default lag
    cons_features["load_lag_1h"] = 300.0
    pass1 = predict_model(models["consumption"], cons_features).clip(min=0)
    # Second pass with shifted predictions
    cons_features["load_lag_1h"] = pd.Series(pass1, index=idx).shift(1).bfill()
    # Use actual prior-hour load if available
    if len(actuals["consumption"]) > 0:
        cons_features.iloc[0, cons_features.columns.get_loc("load_lag_1h")] = (
            actuals["consumption"][0]
        )
    cons_pred = predict_model(models["consumption"], cons_features).clip(min=0)

    # --- Heat Pump ---
    hp_features = build_heating_features(weather_df, config)
    hp_features = hp_features.reindex(idx, method="nearest")
    if "temp_min" not in hp_features.columns:
        hp_features["temp_min"] = hp_features["temperature"]
    hp_pred = predict_model(models["heat_pump"], hp_features).clip(min=0)

    # --- DHW ---
    dhw_features = build_dhw_features(idx, config, weather_df=weather_df)
    dhw_pred = predict_model(models["dhw"], dhw_features).clip(min=0)

    # --- Spot Price (2-pass autoregressive) ---
    sp_features = build_spot_price_features(weather_df, config)
    sp_features = sp_features.reindex(idx, method="nearest")
    sp_features["price_lag_1h"] = 0.4
    sp_features["price_lag_24h"] = 0.4
    sp_features["price_rolling_24h_mean"] = 0.4
    pass1_price = predict_model(models["spot_price"], sp_features)
    price_s = pd.Series(pass1_price, index=idx)
    sp_features["price_lag_1h"] = price_s.shift(1).bfill()
    sp_features["price_lag_24h"] = price_s.shift(24).bfill()
    sp_features["price_rolling_24h_mean"] = price_s.rolling(24, min_periods=1).mean()
    # Use actual prior prices as lags where available
    if len(actuals["price"]) > 0:
        sp_features.iloc[0, sp_features.columns.get_loc("price_lag_1h")] = (
            actuals["price"][0]
        )
    sp_pred = predict_model(models["spot_price"], sp_features)

    # Compute forecast net load
    forecast_net = cons_pred + hp_pred + dhw_pred - pv_series.values

    return {
        "pv": pv_series.values,
        "consumption": cons_pred,
        "hp": hp_pred,
        "dhw": dhw_pred,
        "price": sp_pred,
        "net_load": forecast_net,
    }


def print_explanation(params: BatteryParams, capacity_kwh: float):
    """Print explanation header."""
    print("=== MPC Forecast Backtest ===")
    print()
    print("This script replays historical days to evaluate battery optimization performance.")
    print()
    print("For each day, it:")
    print("  1. Loads actual sensor data (grid power, PV, heat pump, DHW, spot prices)")
    print("  2. Generates ML forecasts using the same models the live controller uses")
    print("  3. Runs the LP battery optimizer on both forecast and actual data")
    print("  4. Compares costs across 4 scenarios")
    print()
    print("Scenarios:")
    print("  No Battery    — baseline cost without any battery")
    print("  Perfect Info  — LP optimizer with actual measured data (theoretical best)")
    print("  MPC Forecast  — LP optimizer with ML-predicted load and prices (realistic)")
    print("  MPC + Prices  — LP optimizer with predicted load but actual prices")
    print()
    soc_min_pct = params.soc_min_wh / params.capacity_wh * 100
    soc_max_pct = params.soc_max_wh / params.capacity_wh * 100
    eff_pct = params.round_trip_efficiency * 100
    print(
        f"Battery: {capacity_kwh:.1f} kWh, {params.max_power_w:.0f} W, "
        f"SoC {soc_min_pct:.0f}-{soc_max_pct:.0f}%, efficiency {eff_pct:.0f}%"
    )
    print(
        f"Export coefficient: {params.export_coeff:.2f} "
        f"(receive {params.export_coeff*100:.0f}% of spot price on export)"
    )
    print()
    print("Model Accuracy section shows how well each ML model predicted reality:")
    print("  MAE        — Mean Absolute Error (average hourly prediction error)")
    print("  kWh/day    — total daily energy for that component")
    print("                PV = solar production, Consumption = household load (excl HP),")
    print("                HP = heat pump heating, DHW = hot water heating")
    print()
    print("Cost Table shows daily electricity cost under each scenario:")
    print("  Savings %  — reduction vs no-battery baseline")
    print("  Gap        — money left on the table due to imperfect forecasts")
    print()


def run_backtest(
    config: dict,
    params: BatteryParams,
    n_days: int,
    do_plot: bool = False,
) -> tuple[pd.DataFrame, pd.DataFrame]:
    """Run day-by-day MPC backtest. Returns (cost_results, accuracy_results)."""
    print("Loading sensor data...")
    hourly = load_all_sensor_data(config)

    print("\nLoading ML models...")
    models = load_models()
    for name, model in models.items():
        metrics = model.train_metrics
        r2 = metrics.get("r2", 0)
        mae = metrics.get("mae", 0)
        print(f"  {name}: R²={r2:.2f}, MAE={mae:.1f}")

    # Determine date range: last n_days with data from both grid + price
    grid_series = hourly.get("grid")
    price_series = hourly.get("price")
    if grid_series is None or grid_series.empty:
        raise ValueError("No grid power data available")
    if price_series is None or price_series.empty:
        raise ValueError("No spot price data available")
    data_end = min(grid_series.index.max(), price_series.index.max())
    end_date = data_end.normalize() + pd.Timedelta(days=1)
    start_date = end_date - pd.Timedelta(days=n_days)

    print(f"\nBacktest period: {start_date.date()} to {(end_date - pd.Timedelta(days=1)).date()}")
    print()

    cost_results = []
    accuracy_results = []

    day = start_date
    while day < end_date:
        day_str = day.strftime("%Y-%m-%d")
        actuals = get_day_actuals(hourly, day)

        if actuals is None:
            day += pd.Timedelta(days=1)
            continue

        T = len(actuals["index"])
        initial_soc = params.soc_min_wh

        # Generate ML forecast
        try:
            forecast = generate_forecast(models, config, day, actuals)
        except Exception as e:
            print(f"  {day_str}: forecast failed ({e}), skipping")
            day += pd.Timedelta(days=1)
            continue

        # --- 4 Scenarios ---
        # 1. No battery (actual data)
        no_batt = simulate_no_battery(
            actuals["net_load"], actuals["price"], params.export_coeff
        )

        # 2. Perfect info: LP on actual data
        perfect = optimize_battery(
            actuals["net_load"], actuals["price"], params, initial_soc
        )

        # 3. MPC forecast: optimize on forecast, simulate on actuals
        mpc_opt = optimize_battery(
            forecast["net_load"], forecast["price"], params, initial_soc
        )
        mpc_cost = _simulate_schedule_on_actuals(
            mpc_opt, actuals["net_load"], actuals["price"], params, initial_soc
        )

        # 4. MPC + known prices: forecast load, actual prices
        mpc_prices_opt = optimize_battery(
            forecast["net_load"], actuals["price"], params, initial_soc
        )
        mpc_prices_cost = _simulate_schedule_on_actuals(
            mpc_prices_opt, actuals["net_load"], actuals["price"], params, initial_soc
        )

        cost_results.append({
            "date": day_str,
            "hours": T,
            "no_batt_pln": no_batt.total_cost_pln,
            "perfect_pln": perfect.total_cost_pln,
            "mpc_pln": mpc_cost.total_cost_pln,
            "mpc_prices_pln": mpc_prices_cost.total_cost_pln,
        })

        # Model accuracy for this day
        for model_name, pred_key, actual_key, unit in [
            ("PV", "pv", "pv", "W"),
            ("Consumption", "consumption", "consumption", "W"),
            ("Heat Pump", "hp", "hp", "W"),
            ("DHW", "dhw", "dhw", "W"),
            ("Spot Price", "price", "price", "PLN/kWh"),
        ]:
            pred = forecast[pred_key]
            actual = actuals[actual_key]
            mae = float(np.mean(np.abs(pred - actual)))
            pred_kwh = float(np.sum(pred)) / 1000 if unit == "W" else float(np.mean(pred))
            actual_kwh = float(np.sum(actual)) / 1000 if unit == "W" else float(np.mean(actual))
            accuracy_results.append({
                "date": day_str,
                "model": model_name,
                "mae": mae,
                "pred_daily": pred_kwh,
                "actual_daily": actual_kwh,
                "unit": unit,
            })

        print(
            f"  {day_str}: no_batt={no_batt.total_cost_pln:6.2f}  "
            f"perfect={perfect.total_cost_pln:6.2f}  "
            f"mpc={mpc_cost.total_cost_pln:6.2f}  "
            f"mpc+price={mpc_prices_cost.total_cost_pln:6.2f}"
        )

        if do_plot:
            plot_day(day_str, actuals, forecast, perfect, mpc_cost, mpc_prices_cost, params)

        day += pd.Timedelta(days=1)

    return pd.DataFrame(cost_results), pd.DataFrame(accuracy_results)


def _simulate_schedule_on_actuals(
    opt_result,
    actual_net_load: np.ndarray,
    actual_price: np.ndarray,
    params: BatteryParams,
    initial_soc_wh: float,
):
    """Apply an optimized charge/discharge schedule to actual net load and prices.

    The optimizer planned for forecast data, but we evaluate against reality.
    Re-simulate SoC with the planned schedule clamped to actual constraints.
    """
    from .optimize import OptimizeResult

    T = len(actual_net_load)
    eta = np.sqrt(params.round_trip_efficiency)
    charge = np.zeros(T)
    discharge = np.zeros(T)
    imp = np.zeros(T)
    exp = np.zeros(T)
    soc = np.zeros(T)
    current_soc = initial_soc_wh

    for t in range(T):
        # Clamp planned schedule to actual SoC limits
        planned_charge = opt_result.charge_w[t] if t < len(opt_result.charge_w) else 0
        planned_discharge = opt_result.discharge_w[t] if t < len(opt_result.discharge_w) else 0

        # Feasibility: can we actually charge/discharge this much?
        max_charge = (params.soc_max_wh - current_soc) / eta
        max_discharge = (current_soc - params.soc_min_wh) * eta

        c = min(max(planned_charge, 0), params.max_power_w, max_charge)
        d = min(max(planned_discharge, 0), params.max_power_w, max_discharge)

        # If both charge and discharge planned, pick the larger
        if c > 0 and d > 0:
            if c > d:
                d = 0
            else:
                c = 0

        charge[t] = c
        discharge[t] = d
        current_soc = current_soc + c * eta - d / eta
        soc[t] = current_soc

        # Grid flows with actual net load
        net = actual_net_load[t] + c - d
        if net >= 0:
            imp[t] = net
        else:
            exp[t] = -net

    cost = (imp * actual_price - exp * actual_price * params.export_coeff) / 1000.0

    return OptimizeResult(
        charge_w=charge,
        discharge_w=discharge,
        import_w=imp,
        export_w=exp,
        soc_wh=soc,
        cost_pln=cost,
        total_cost_pln=float(cost.sum()),
        status="simulated",
    )


def print_accuracy(accuracy_df: pd.DataFrame):
    """Print model accuracy summary."""
    if accuracy_df.empty:
        return

    print("\nModel Accuracy (daily averages):")
    header = f"  {'Model':<14} {'MAE':>10}  {'Pred kWh/day':>13}  {'Actual kWh/day':>15}"
    print(header)
    print("  " + "-" * (len(header) - 2))

    for model_name in ("PV", "Consumption", "Heat Pump", "DHW", "Spot Price"):
        rows = accuracy_df[accuracy_df["model"] == model_name]
        if rows.empty:
            continue
        mae = rows["mae"].mean()
        pred = rows["pred_daily"].mean()
        actual = rows["actual_daily"].mean()
        unit = rows["unit"].iloc[0]

        if unit == "W":
            print(f"  {model_name:<14} {mae:>7.0f} W   {pred:>10.1f}        {actual:>10.1f}")
        else:
            print(f"  {model_name:<14} {mae:>5.2f} PLN  {pred:>10.2f}        {actual:>10.2f}")


def print_cost_results(cost_df: pd.DataFrame):
    """Print daily cost comparison table."""
    if cost_df.empty:
        print("No days with sufficient data!")
        return

    print()
    header = (
        f"{'Date':<12} {'No Batt':>8} {'Perfect':>8} "
        f"{'MPC':>8} {'MPC+Price':>9}  "
        f"{'MPC Save':>9} {'Perf Save':>10} {'Gap':>6}"
    )
    print(header)
    print("-" * len(header))

    for _, row in cost_df.iterrows():
        nb = row["no_batt_pln"]
        perf = row["perfect_pln"]
        mpc = row["mpc_pln"]
        mpc_p = row["mpc_prices_pln"]

        mpc_save = (1 - mpc / nb) * 100 if nb != 0 else 0
        perf_save = (1 - perf / nb) * 100 if nb != 0 else 0
        gap = perf_save - mpc_save

        print(
            f"{row['date']:<12} {nb:>8.2f} {perf:>8.2f} "
            f"{mpc:>8.2f} {mpc_p:>9.2f}  "
            f"{mpc_save:>+8.1f}% {perf_save:>+9.1f}% {gap:>+5.1f}%"
        )

    print("-" * len(header))

    # Totals
    t_nb = cost_df["no_batt_pln"].sum()
    t_perf = cost_df["perfect_pln"].sum()
    t_mpc = cost_df["mpc_pln"].sum()
    t_mpc_p = cost_df["mpc_prices_pln"].sum()
    mpc_save = (1 - t_mpc / t_nb) * 100 if t_nb != 0 else 0
    perf_save = (1 - t_perf / t_nb) * 100 if t_nb != 0 else 0
    gap = perf_save - mpc_save

    print(
        f"{'TOTAL':<12} {t_nb:>8.2f} {t_perf:>8.2f} "
        f"{t_mpc:>8.2f} {t_mpc_p:>9.2f}  "
        f"{mpc_save:>+8.1f}% {perf_save:>+9.1f}% {gap:>+5.1f}%"
    )

    n_days = len(cost_df)
    forecast_penalty = t_mpc - t_perf
    print()
    print(
        f"Forecast penalty: {forecast_penalty:.2f} PLN / {n_days} days "
        f"= {forecast_penalty / n_days:.2f} PLN/day"
    )


def plot_day(
    day_str: str,
    actuals: dict,
    forecast: dict,
    perfect,
    mpc,
    mpc_prices,
    params: BatteryParams,
):
    """Generate per-day comparison plot."""
    try:
        import matplotlib.pyplot as plt
    except ImportError:
        print("  matplotlib not available, skipping plot")
        return

    import os

    out_dir = project_root() / "docs" / "forecast"
    os.makedirs(out_dir, exist_ok=True)

    T = len(actuals["index"])
    hours = np.arange(T)

    fig, axes = plt.subplots(3, 1, figsize=(14, 10), sharex=True)

    # Row 1: SoC traces
    ax = axes[0]
    ax.plot(hours, perfect.soc_wh / 1000, label=f"Perfect ({perfect.total_cost_pln:.2f} PLN)",
            color="#5bb88a", linewidth=2)
    ax.plot(hours, mpc.soc_wh / 1000, label=f"MPC ({mpc.total_cost_pln:.2f} PLN)",
            color="#64b5f6", linewidth=2)
    ax.plot(hours, mpc_prices.soc_wh / 1000, label=f"MPC+Price ({mpc_prices.total_cost_pln:.2f} PLN)",
            color="#9b8fd8", linewidth=1.5, linestyle="--")
    ax.axhline(y=params.soc_min_wh / 1000, color="gray", linestyle="--", alpha=0.5)
    ax.axhline(y=params.soc_max_wh / 1000, color="gray", linestyle="--", alpha=0.5)
    ax.set_ylabel("SoC (kWh)")
    ax.legend(fontsize=9)
    ax.set_title(f"MPC Forecast Backtest — {day_str}")

    # Row 2: Predicted vs actual per model
    ax = axes[1]
    model_items = [
        ("PV", "pv", "#e8b830"),
        ("Consumption", "consumption", "#e87c6c"),
        ("Heat Pump", "hp", "#e8884c"),
    ]
    for label, key, color in model_items:
        ax.plot(hours, actuals[key] / 1000, color=color, linewidth=1.5, label=f"{label} actual")
        ax.plot(hours, forecast[key] / 1000, color=color, linewidth=1, linestyle="--",
                alpha=0.7, label=f"{label} forecast")
    ax.set_ylabel("Power (kW)")
    ax.legend(fontsize=8, ncol=3)

    # Row 3: Net load + spot price
    ax = axes[2]
    ax2 = ax.twinx()
    ax.plot(hours, actuals["net_load"] / 1000, color="#e87c6c", linewidth=1.5, label="Actual net load")
    ax.plot(hours, forecast["net_load"] / 1000, color="#e87c6c", linewidth=1, linestyle="--",
            alpha=0.7, label="Forecast net load")
    ax2.bar(hours, actuals["price"], alpha=0.2, color="#e8b830", label="Actual price")
    ax2.plot(hours, forecast["price"], color="#9b8fd8", linewidth=1, linestyle="--",
             alpha=0.7, label="Forecast price")
    ax.set_xlabel("Hour")
    ax.set_ylabel("Net load (kW)")
    ax2.set_ylabel("Price (PLN/kWh)")
    ax.legend(loc="upper left", fontsize=8)
    ax2.legend(loc="upper right", fontsize=8)

    plt.tight_layout()
    out_path = out_dir / f"backtest_mpc_{day_str}.png"
    plt.savefig(str(out_path), dpi=150)
    print(f"  Saved plot: {out_path}")
    plt.close()


def main():
    parser = argparse.ArgumentParser(description="MPC Forecast Backtest")
    parser.add_argument("--days", type=int, default=14, help="Days to backtest")
    parser.add_argument("--capacity", type=float, default=10.0, help="Battery kWh")
    parser.add_argument("--power", type=float, default=5000, help="Max W")
    parser.add_argument("--soc-min", type=float, default=10, help="Min SoC (%%)")
    parser.add_argument("--soc-max", type=float, default=90, help="Max SoC (%%)")
    parser.add_argument("--export-coeff", type=float, default=0.8, help="Export coefficient")
    parser.add_argument(
        "--efficiency", type=float, default=0.90,
        help="Round-trip efficiency (grid->battery->grid)"
    )
    parser.add_argument("--plot", action="store_true", help="Generate per-day PNG plots")
    args = parser.parse_args()

    capacity_wh = args.capacity * 1000
    params = BatteryParams(
        capacity_wh=capacity_wh,
        max_power_w=args.power,
        soc_min_wh=capacity_wh * args.soc_min / 100,
        soc_max_wh=capacity_wh * args.soc_max / 100,
        export_coeff=args.export_coeff,
        round_trip_efficiency=args.efficiency,
    )

    config = load_config()
    print_explanation(params, args.capacity)

    cost_df, accuracy_df = run_backtest(config, params, args.days, args.plot)

    print_accuracy(accuracy_df)
    print_cost_results(cost_df)


if __name__ == "__main__":
    main()
