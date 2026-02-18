"""Backtest battery strategies: LP optimal vs P33/P67 heuristic vs no-battery.

Usage:
    mise exec -- python -m analysis.python.src.backtest --days 30
    mise exec -- python -m analysis.python.src.backtest --days 30 --capacity 10 --power 5000 --plot
"""

import argparse

import numpy as np
import pandas as pd

from .config import load_config, project_root
from .data_loading import load_sensor_data, load_spot_prices
from .optimize import (
    BatteryParams,
    optimize_battery,
    prepare_hourly_data,
    simulate_heuristic,
    simulate_no_battery,
)


def load_data(config: dict) -> pd.DataFrame:
    """Load grid power and spot prices, return aligned hourly DataFrame."""
    root = project_root()
    sensors = config["sensors"]

    print("Loading grid power...")
    grid_df = load_sensor_data(
        sensor_id=sensors["grid_power"],
        legacy_path=root / "input" / "grid_power.csv",
        recent_dir=root / "input" / "recent",
    )
    print(f"  {len(grid_df)} readings")

    print("Loading spot prices...")
    spot_df = load_spot_prices(root / "input" / "recent" / "historic_spot_prices.csv")
    print(f"  {len(spot_df)} readings")

    print("Aligning to hourly...")
    hourly = prepare_hourly_data(grid_df, spot_df)
    print(f"  {len(hourly)} aligned hours ({hourly.index.min()} to {hourly.index.max()})")

    return hourly


def run_backtest(
    hourly: pd.DataFrame,
    params: BatteryParams,
    n_days: int,
) -> pd.DataFrame:
    """Run day-by-day comparison, return results DataFrame."""
    # Use the last n_days of data
    end_date = hourly.index.max().normalize() + pd.Timedelta(days=1)
    start_date = end_date - pd.Timedelta(days=n_days)

    results = []
    day = start_date

    while day < end_date:
        day_end = day + pd.Timedelta(days=1)
        mask = (hourly.index >= day) & (hourly.index < day_end)
        day_data = hourly[mask]

        if len(day_data) < 20:  # need at least 20 hours
            day = day_end
            continue

        net_load = day_data["net_load_w"].values
        price = day_data["price_pln_kwh"].values

        initial_soc = params.soc_min_wh

        no_batt = simulate_no_battery(net_load, price, params.export_coeff)
        heur = simulate_heuristic(net_load, price, params, initial_soc)
        opt = optimize_battery(net_load, price, params, initial_soc)

        results.append({
            "date": day.strftime("%Y-%m-%d"),
            "hours": len(day_data),
            "no_batt_pln": no_batt.total_cost_pln,
            "heur_pln": heur.total_cost_pln,
            "opt_pln": opt.total_cost_pln,
            "opt_status": opt.status,
        })

        day = day_end

    return pd.DataFrame(results)


def print_results(df: pd.DataFrame):
    """Print ASCII comparison table."""
    if df.empty:
        print("No days with sufficient data!")
        return

    header = (
        f"{'Date':<12} {'No Batt':>9} {'Heuristic':>10} {'Optimal':>9}"
        f" {'Heur Save':>10} {'Opt Save':>9} {'Gap':>6}"
    )
    print()
    print(header)
    print("-" * len(header))

    for _, row in df.iterrows():
        no_batt = row["no_batt_pln"]
        heur = row["heur_pln"]
        opt = row["opt_pln"]

        heur_save_pct = (1 - heur / no_batt) * 100 if no_batt != 0 else 0
        opt_save_pct = (1 - opt / no_batt) * 100 if no_batt != 0 else 0
        gap_pct = opt_save_pct - heur_save_pct

        print(
            f"{row['date']:<12} {no_batt:>9.2f} {heur:>10.2f} {opt:>9.2f}"
            f" {heur_save_pct:>+9.1f}% {opt_save_pct:>+8.1f}% {gap_pct:>+5.1f}%"
        )

    print("-" * len(header))

    # Totals
    total_no_batt = df["no_batt_pln"].sum()
    total_heur = df["heur_pln"].sum()
    total_opt = df["opt_pln"].sum()
    heur_save = (1 - total_heur / total_no_batt) * 100 if total_no_batt != 0 else 0
    opt_save = (1 - total_opt / total_no_batt) * 100 if total_no_batt != 0 else 0
    gap = opt_save - heur_save

    print(
        f"{'TOTAL':<12} {total_no_batt:>9.2f} {total_heur:>10.2f} {total_opt:>9.2f}"
        f" {heur_save:>+9.1f}% {opt_save:>+8.1f}% {gap:>+5.1f}%"
    )
    print()
    print(f"  Heuristic total savings: {total_no_batt - total_heur:.2f} PLN")
    print(f"  Optimal total savings:   {total_no_batt - total_opt:.2f} PLN")
    print(f"  Opportunity gap:         {total_heur - total_opt:.2f} PLN")


def plot_day_comparison(
    hourly: pd.DataFrame,
    params: BatteryParams,
    day_str: str,
):
    """Plot SoC traces and price for a single day."""
    try:
        import matplotlib.pyplot as plt
    except ImportError:
        print("matplotlib not available, skipping plot")
        return

    day = pd.Timestamp(day_str, tz="UTC")
    day_end = day + pd.Timedelta(days=1)
    mask = (hourly.index >= day) & (hourly.index < day_end)
    day_data = hourly[mask]

    if len(day_data) < 20:
        print(f"Not enough data for {day_str}")
        return

    net_load = day_data["net_load_w"].values
    price = day_data["price_pln_kwh"].values
    hours = np.arange(len(day_data))
    initial_soc = params.soc_min_wh

    heur = simulate_heuristic(net_load, price, params, initial_soc)
    opt = optimize_battery(net_load, price, params, initial_soc)

    fig, (ax1, ax2) = plt.subplots(2, 1, figsize=(12, 6), sharex=True)

    # SoC comparison
    ax1.plot(hours, heur.soc_wh / 1000, label=f"Heuristic ({heur.total_cost_pln:.2f} PLN)", linewidth=2)
    ax1.plot(hours, opt.soc_wh / 1000, label=f"Optimal ({opt.total_cost_pln:.2f} PLN)", linewidth=2)
    ax1.axhline(y=params.soc_min_wh / 1000, color="gray", linestyle="--", alpha=0.5)
    ax1.axhline(y=params.soc_max_wh / 1000, color="gray", linestyle="--", alpha=0.5)
    ax1.set_ylabel("SoC (kWh)")
    ax1.legend()
    ax1.set_title(f"Battery Schedule Comparison — {day_str}")

    # Price + net load
    ax2_price = ax2
    ax2_load = ax2.twinx()
    ax2_price.bar(hours, price, alpha=0.3, color="#e8b830", label="Spot price")
    ax2_load.plot(hours, net_load / 1000, color="#e87c6c", linewidth=1.5, label="Net load")
    ax2_price.set_xlabel("Hour")
    ax2_price.set_ylabel("Price (PLN/kWh)")
    ax2_load.set_ylabel("Net load (kW)")
    ax2_price.legend(loc="upper left")
    ax2_load.legend(loc="upper right")

    plt.tight_layout()
    out_path = f"analysis/python/output/backtest_{day_str}.png"
    plt.savefig(out_path, dpi=150)
    print(f"Saved plot to {out_path}")
    plt.close()


def main():
    parser = argparse.ArgumentParser(description="Battery strategy backtest")
    parser.add_argument("--days", type=int, default=30, help="Number of days to backtest")
    parser.add_argument("--capacity", type=float, default=10.0, help="Battery capacity (kWh)")
    parser.add_argument("--power", type=float, default=5000.0, help="Max charge/discharge power (W)")
    parser.add_argument("--soc-min", type=float, default=10.0, help="Min SoC (%%)")
    parser.add_argument("--soc-max", type=float, default=90.0, help="Max SoC (%%)")
    parser.add_argument("--export-coeff", type=float, default=0.8, help="Export coefficient (0-1)")
    parser.add_argument("--plot", type=str, default=None, help="Plot a specific day (YYYY-MM-DD)")
    args = parser.parse_args()

    capacity_wh = args.capacity * 1000
    params = BatteryParams(
        capacity_wh=capacity_wh,
        max_power_w=args.power,
        soc_min_wh=capacity_wh * args.soc_min / 100,
        soc_max_wh=capacity_wh * args.soc_max / 100,
        export_coeff=args.export_coeff,
    )

    print(f"Battery: {args.capacity} kWh, {args.power} W, SoC {args.soc_min}-{args.soc_max}%")
    print(f"Export coefficient: {args.export_coeff}")
    print()

    config = load_config()
    hourly = load_data(config)

    if args.plot:
        plot_day_comparison(hourly, params, args.plot)
        return

    print(f"\nRunning {args.days}-day backtest...")
    results = run_backtest(hourly, params, args.days)
    print_results(results)


if __name__ == "__main__":
    main()
