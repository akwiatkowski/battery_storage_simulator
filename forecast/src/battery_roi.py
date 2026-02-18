"""Battery ROI analysis: per-config monthly breakdown and capacity sweep.

Usage:
    mise exec -- python -m forecast.src.battery_roi analyze --capacity 10 --power 5000 --cost-per-kwh 1500
    mise exec -- python -m forecast.src.battery_roi sweep --cost-per-kwh 1500
"""

import argparse

import numpy as np
import pandas as pd

from .backtest import load_data
from .config import load_config, project_root
from .optimize import BatteryParams, optimize_battery, simulate_no_battery


def _run_period(
    hourly: pd.DataFrame,
    params: BatteryParams,
    start: pd.Timestamp,
    end: pd.Timestamp,
) -> pd.DataFrame:
    """Run day-by-day LP optimal + no-battery over [start, end), return daily results."""
    results = []
    day = start

    while day < end:
        day_end = day + pd.Timedelta(days=1)
        mask = (hourly.index >= day) & (hourly.index < day_end)
        day_data = hourly[mask]

        if len(day_data) < 20:
            day = day_end
            continue

        net_load = day_data["net_load_w"].values
        price = day_data["price_pln_kwh"].values
        initial_soc = params.soc_min_wh

        no_batt = simulate_no_battery(net_load, price, params.export_coeff)
        opt = optimize_battery(net_load, price, params, initial_soc)

        results.append({
            "date": day,
            "no_batt_pln": no_batt.total_cost_pln,
            "opt_pln": opt.total_cost_pln,
        })

        day = day_end

    return pd.DataFrame(results)


def _parse_date_range(
    hourly: pd.DataFrame, start_str: str | None, end_str: str | None
) -> tuple[pd.Timestamp, pd.Timestamp]:
    """Parse start/end strings into UTC timestamps, defaulting to full data range."""
    if start_str:
        start = pd.Timestamp(start_str, tz="UTC")
    else:
        start = hourly.index.min().normalize()

    if end_str:
        end = pd.Timestamp(end_str, tz="UTC")
    else:
        end = hourly.index.max().normalize() + pd.Timedelta(days=1)

    return start, end


# ── analyze ──────────────────────────────────────────────────────────────────


def cmd_analyze(args):
    config = load_config()
    hourly = load_data(config)

    capacity_wh = args.capacity * 1000
    params = BatteryParams(
        capacity_wh=capacity_wh,
        max_power_w=args.power,
        soc_min_wh=capacity_wh * args.soc_min / 100,
        soc_max_wh=capacity_wh * args.soc_max / 100,
        export_coeff=args.export_coeff,
    )

    start, end = _parse_date_range(hourly, args.start, args.end)
    total_days = (end - start).days

    investment = args.capacity * args.cost_per_kwh

    print(f"\nBattery: {args.capacity} kWh, {args.power} W | Cost: {investment:,.0f} PLN")
    print(f"Period: {start.strftime('%Y-%m-%d')} to {end.strftime('%Y-%m-%d')} ({total_days} days)")

    daily = _run_period(hourly, params, start, end)

    if daily.empty:
        print("No days with sufficient data!")
        return

    # Monthly aggregation
    daily["month"] = daily["date"].dt.tz_localize(None).dt.to_period("M")
    monthly = daily.groupby("month").agg(
        no_batt_pln=("no_batt_pln", "sum"),
        opt_pln=("opt_pln", "sum"),
        days=("date", "count"),
    )
    monthly["savings"] = monthly["no_batt_pln"] - monthly["opt_pln"]
    monthly["savings_pct"] = (monthly["savings"] / monthly["no_batt_pln"] * 100).replace(
        [np.inf, -np.inf], 0
    )

    # Print table
    header = f"  {'Month':<10} {'No Batt':>9} {'Optimal':>9} {'Savings':>9} {'Savings%':>9}"
    print()
    print(header)
    print("  " + "-" * (len(header) - 2))

    for period, row in monthly.iterrows():
        print(
            f"  {str(period):<10} {row['no_batt_pln']:>9.1f} {row['opt_pln']:>9.1f}"
            f" {row['savings']:>9.1f} {row['savings_pct']:>8.1f}%"
        )

    total_no_batt = monthly["no_batt_pln"].sum()
    total_opt = monthly["opt_pln"].sum()
    total_savings = total_no_batt - total_opt
    total_pct = total_savings / total_no_batt * 100 if total_no_batt != 0 else 0

    print("  " + "-" * (len(header) - 2))
    print(
        f"  {'TOTAL':<10} {total_no_batt:>9.1f} {total_opt:>9.1f}"
        f" {total_savings:>9.1f} {total_pct:>8.1f}%"
    )

    actual_days = len(daily)
    annual_savings = total_savings * 365.25 / actual_days if actual_days > 0 else 0
    payback = investment / annual_savings if annual_savings > 0 else float("inf")
    roi_pct = annual_savings / investment * 100 if investment > 0 else 0

    print()
    print(f"  Annual savings (extrapolated): {annual_savings:,.0f} PLN")
    print(f"  Investment:                  {investment:,.0f} PLN")
    print(f"  Simple payback:               {payback:.1f} years")
    print(f"  Annual ROI:                    {roi_pct:.1f}%")

    # Plot
    _plot_analyze(monthly, investment, annual_savings, payback, args.capacity)


def _plot_analyze(
    monthly: pd.DataFrame,
    investment: float,
    annual_savings: float,
    payback: float,
    capacity_kwh: float,
):
    try:
        import matplotlib.pyplot as plt
    except ImportError:
        print("matplotlib not available, skipping plot")
        return

    plt.style.use("ggplot")
    fig, ax1 = plt.subplots(figsize=(12, 6))

    months = [str(p) for p in monthly.index]
    savings = monthly["savings"].values
    cumulative = np.cumsum(savings)

    x = np.arange(len(months))

    ax1.bar(x, savings, color="#5bb88a", alpha=0.85, label="Monthly savings")
    ax1.set_xlabel("Month")
    ax1.set_ylabel("Savings (PLN)")
    ax1.set_xticks(x)
    ax1.set_xticklabels(months, rotation=45, ha="right")

    ax2 = ax1.twinx()
    ax2.plot(x, cumulative, color="#4a6fa5", linewidth=2.5, marker="o", markersize=5, label="Cumulative")
    ax2.set_ylabel("Cumulative savings (PLN)")

    lines1, labels1 = ax1.get_legend_handles_labels()
    lines2, labels2 = ax2.get_legend_handles_labels()
    ax1.legend(lines1 + lines2, labels1 + labels2, loc="upper left")

    ax1.set_title(
        f"Battery ROI: {capacity_kwh} kWh | "
        f"Annual savings: {annual_savings:,.0f} PLN | "
        f"Payback: {payback:.1f}y"
    )

    plt.tight_layout()
    out_path = project_root() / "docs" / "forecast" / "battery_roi_analyze.png"
    out_path.parent.mkdir(parents=True, exist_ok=True)
    plt.savefig(out_path, dpi=150)
    print(f"\nSaved plot to {out_path}")
    plt.close()


# ── sweep ────────────────────────────────────────────────────────────────────


def cmd_sweep(args):
    config = load_config()
    hourly = load_data(config)

    start, end = _parse_date_range(hourly, args.start, args.end)
    total_days = (end - start).days

    print(f"\nBattery Sweep: {args.min_capacity}–{args.max_capacity} kWh, {args.c_rate}C, {args.cost_per_kwh} PLN/kWh")
    print(f"Period: {start.strftime('%Y-%m-%d')} to {end.strftime('%Y-%m-%d')} ({total_days} days)")

    capacities = np.arange(args.min_capacity, args.max_capacity + args.step * 0.5, args.step)
    rows = []

    for cap_kwh in capacities:
        capacity_wh = cap_kwh * 1000
        power_w = capacity_wh * args.c_rate
        params = BatteryParams(
            capacity_wh=capacity_wh,
            max_power_w=power_w,
            soc_min_wh=capacity_wh * args.soc_min / 100,
            soc_max_wh=capacity_wh * args.soc_max / 100,
            export_coeff=args.export_coeff,
        )

        daily = _run_period(hourly, params, start, end)
        actual_days = len(daily)

        if actual_days == 0:
            continue

        total_savings = daily["no_batt_pln"].sum() - daily["opt_pln"].sum()
        annual_savings = total_savings * 365.25 / actual_days
        investment = cap_kwh * args.cost_per_kwh
        pln_kwh_yr = annual_savings / cap_kwh if cap_kwh > 0 else 0
        payback = investment / annual_savings if annual_savings > 0 else float("inf")
        roi_pct = annual_savings / investment * 100 if investment > 0 else 0

        rows.append({
            "capacity_kwh": cap_kwh,
            "power_w": power_w,
            "savings": total_savings,
            "annual_savings": annual_savings,
            "pln_kwh_yr": pln_kwh_yr,
            "payback": payback,
            "roi_pct": roi_pct,
        })

        print(f"  {cap_kwh:5.0f} kWh done ({total_savings:.1f} PLN savings)")

    if not rows:
        print("No results!")
        return

    df = pd.DataFrame(rows)
    best_idx = df["pln_kwh_yr"].idxmax()

    # Print table
    header = f"  {'Capacity':>8} {'Power':>7} {'Savings':>9} {'PLN/kWh/yr':>11} {'Payback':>9} {'ROI%':>7}"
    print()
    print(header)
    print("  " + "-" * (len(header) - 2))

    for i, row in df.iterrows():
        marker = "  ← best" if i == best_idx else ""
        print(
            f"  {row['capacity_kwh']:>6.0f} kWh {row['power_w']:>6.0f}W"
            f" {row['savings']:>9.1f} {row['pln_kwh_yr']:>11.1f}"
            f" {row['payback']:>8.1f}y {row['roi_pct']:>6.1f}%{marker}"
        )

    best = df.loc[best_idx]
    print()
    print(
        f"  Best ROI: {best['capacity_kwh']:.0f} kWh → "
        f"{best['pln_kwh_yr']:.1f} PLN/kWh/year "
        f"({best['roi_pct']:.1f}%, {best['payback']:.1f}y payback)"
    )

    _plot_sweep(df, best_idx)


def _plot_sweep(df: pd.DataFrame, best_idx: int):
    try:
        import matplotlib.pyplot as plt
    except ImportError:
        print("matplotlib not available, skipping plot")
        return

    plt.style.use("ggplot")
    fig, ax1 = plt.subplots(figsize=(12, 6))

    x = np.arange(len(df))
    capacities = df["capacity_kwh"].values
    pln_kwh_yr = df["pln_kwh_yr"].values
    payback = df["payback"].values

    colors = ["#e8b830" if i == best_idx else "#5bb88a" for i in df.index]
    ax1.bar(x, pln_kwh_yr, color=colors, alpha=0.85, label="PLN/kWh/year")
    ax1.set_xlabel("Battery capacity (kWh)")
    ax1.set_ylabel("PLN / kWh / year")
    ax1.set_xticks(x)
    ax1.set_xticklabels([f"{c:.0f}" for c in capacities], rotation=45, ha="right")

    ax2 = ax1.twinx()
    ax2.plot(x, payback, color="#4a6fa5", linewidth=2.5, marker="o", markersize=5, label="Payback (years)")
    ax2.set_ylabel("Payback (years)")

    lines1, labels1 = ax1.get_legend_handles_labels()
    lines2, labels2 = ax2.get_legend_handles_labels()
    ax1.legend(lines1 + lines2, labels1 + labels2, loc="upper right")

    best = df.loc[best_idx]
    ax1.set_title(
        f"Battery Size Sweep ({df['capacity_kwh'].min():.0f}–{df['capacity_kwh'].max():.0f} kWh) | "
        f"Best: {best['capacity_kwh']:.0f} kWh @ {best['pln_kwh_yr']:.1f} PLN/kWh/yr"
    )

    plt.tight_layout()
    out_path = project_root() / "docs" / "forecast" / "battery_roi_sweep.png"
    out_path.parent.mkdir(parents=True, exist_ok=True)
    plt.savefig(out_path, dpi=150)
    print(f"\nSaved plot to {out_path}")
    plt.close()


# ── main ─────────────────────────────────────────────────────────────────────


def main():
    parser = argparse.ArgumentParser(description="Battery ROI analysis")
    sub = parser.add_subparsers(dest="command", required=True)

    # analyze
    p_analyze = sub.add_parser("analyze", help="Monthly breakdown for a specific battery")
    p_analyze.add_argument("--capacity", type=float, required=True, help="Battery capacity (kWh)")
    p_analyze.add_argument("--power", type=float, required=True, help="Max charge/discharge power (W)")
    p_analyze.add_argument("--cost-per-kwh", type=float, required=True, help="Battery cost (PLN per kWh)")
    p_analyze.add_argument("--start", type=str, default=None, help="Start date (YYYY-MM-DD)")
    p_analyze.add_argument("--end", type=str, default=None, help="End date (YYYY-MM-DD)")
    p_analyze.add_argument("--soc-min", type=float, default=10.0, help="Min SoC (%%)")
    p_analyze.add_argument("--soc-max", type=float, default=90.0, help="Max SoC (%%)")
    p_analyze.add_argument("--export-coeff", type=float, default=0.8, help="Export coefficient (0-1)")

    # sweep
    p_sweep = sub.add_parser("sweep", help="Find optimal battery size for best ROI")
    p_sweep.add_argument("--min-capacity", type=float, default=5, help="Min capacity (kWh)")
    p_sweep.add_argument("--max-capacity", type=float, default=30, help="Max capacity (kWh)")
    p_sweep.add_argument("--step", type=float, default=1, help="Step size (kWh)")
    p_sweep.add_argument("--c-rate", type=float, default=0.3, help="C-rate for power constraint")
    p_sweep.add_argument("--cost-per-kwh", type=float, required=True, help="Battery cost (PLN per kWh)")
    p_sweep.add_argument("--start", type=str, default=None, help="Start date (YYYY-MM-DD)")
    p_sweep.add_argument("--end", type=str, default=None, help="End date (YYYY-MM-DD)")
    p_sweep.add_argument("--soc-min", type=float, default=10.0, help="Min SoC (%%)")
    p_sweep.add_argument("--soc-max", type=float, default=90.0, help="Max SoC (%%)")
    p_sweep.add_argument("--export-coeff", type=float, default=0.8, help="Export coefficient (0-1)")

    args = parser.parse_args()

    if args.command == "analyze":
        cmd_analyze(args)
    elif args.command == "sweep":
        cmd_sweep(args)


if __name__ == "__main__":
    main()
