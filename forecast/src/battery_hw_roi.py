"""Battery hardware ROI comparison: Dyness vs Pylontech with real hardware costs.

Enumerates valid battery configurations for both brands, runs LP optimization
on historical data, and generates ROI comparison graphs.

Hardware assumptions:
  - Deye hybrid inverter: 6000 PLN (required for both, 5kW max)
  - Dyness HV9640 PRO: 3.84 kWh module, 3500 PLN each
    - BMS per tower: 2000 PLN
    - Min 2, max 6 modules per tower
  - Pylontech H3 starter (H3 + BMS + 3 modules): 16509.77 PLN
    - Pylontech H3 FH10050: 5.12 kWh module, 5447.61 PLN each
    - Max 6 modules per tower
  - Multi-tower: each additional tower needs its own BMS and same number of modules

Usage:
    mise exec -- python -m forecast.src.battery_hw_roi
    mise exec -- python -m forecast.src.battery_hw_roi --start 2024-07-01 --end 2025-02-01
    mise exec -- python -m forecast.src.battery_hw_roi --max-towers 3
"""

import argparse
from dataclasses import dataclass

import numpy as np
import pandas as pd

from .backtest import load_data
from .config import load_config, project_root
from .optimize import BatteryParams, optimize_battery, simulate_no_battery

# ── Hardware constants ──────────────────────────────────────────────────────

INVERTER_COST_PLN = 6000
INVERTER_MAX_POWER_W = 5000

DYNESS_MODULE_KWH = 3.84
DYNESS_MODULE_COST = 3500
DYNESS_BMS_COST = 2000
DYNESS_MIN_PER_TOWER = 2
DYNESS_MAX_PER_TOWER = 6

PYLONTECH_STARTER_COST = 16509.77
PYLONTECH_STARTER_MODULES = 3
PYLONTECH_MODULE_KWH = 5.12
PYLONTECH_MODULE_COST = 5447.61
PYLONTECH_MIN_PER_TOWER = 3  # starter has 3
PYLONTECH_MAX_PER_TOWER = 6


# ── Configuration model ────────────────────────────────────────────────────

@dataclass
class HardwareConfig:
    brand: str
    towers: int
    modules_per_tower: int
    capacity_kwh: float
    hardware_cost_pln: float
    max_power_w: float = INVERTER_MAX_POWER_W

    @property
    def total_modules(self) -> int:
        return self.towers * self.modules_per_tower

    @property
    def label(self) -> str:
        mod_str = f"{self.total_modules}mod"
        if self.towers > 1:
            mod_str += f" ({self.towers}x{self.modules_per_tower})"
        return f"{self.brand} {self.capacity_kwh:.1f}kWh {mod_str}"

    @property
    def short_label(self) -> str:
        return f"{self.brand[:3]} {self.capacity_kwh:.1f}"


def generate_dyness_configs(max_towers: int = 2) -> list[HardwareConfig]:
    """Generate all valid Dyness battery configurations."""
    configs = []
    for towers in range(1, max_towers + 1):
        for mpt in range(DYNESS_MIN_PER_TOWER, DYNESS_MAX_PER_TOWER + 1):
            total = towers * mpt
            capacity = total * DYNESS_MODULE_KWH
            cost = (
                INVERTER_COST_PLN
                + towers * DYNESS_BMS_COST
                + total * DYNESS_MODULE_COST
            )
            configs.append(HardwareConfig(
                brand="Dyness",
                towers=towers,
                modules_per_tower=mpt,
                capacity_kwh=capacity,
                hardware_cost_pln=cost,
            ))
    return configs


def generate_pylontech_configs(max_towers: int = 2) -> list[HardwareConfig]:
    """Generate all valid Pylontech battery configurations."""
    configs = []
    for towers in range(1, max_towers + 1):
        for mpt in range(PYLONTECH_MIN_PER_TOWER, PYLONTECH_MAX_PER_TOWER + 1):
            total = towers * mpt
            capacity = total * PYLONTECH_MODULE_KWH
            extra_modules_per_tower = mpt - PYLONTECH_STARTER_MODULES
            cost = (
                INVERTER_COST_PLN
                + towers * PYLONTECH_STARTER_COST
                + towers * max(0, extra_modules_per_tower) * PYLONTECH_MODULE_COST
            )
            configs.append(HardwareConfig(
                brand="Pylontech",
                towers=towers,
                modules_per_tower=mpt,
                capacity_kwh=capacity,
                hardware_cost_pln=cost,
            ))
    return configs


# ── Optimization engine ────────────────────────────────────────────────────

def run_config(
    hourly: pd.DataFrame,
    config: HardwareConfig,
    start: pd.Timestamp,
    end: pd.Timestamp,
    soc_min_pct: float = 10,
    soc_max_pct: float = 90,
    export_coeff: float = 0.8,
) -> dict:
    """Run LP optimization for a single hardware configuration."""
    capacity_wh = config.capacity_kwh * 1000
    params = BatteryParams(
        capacity_wh=capacity_wh,
        max_power_w=config.max_power_w,
        soc_min_wh=capacity_wh * soc_min_pct / 100,
        soc_max_wh=capacity_wh * soc_max_pct / 100,
        export_coeff=export_coeff,
    )

    total_no_batt = 0.0
    total_opt = 0.0
    day_count = 0
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

        no_batt = simulate_no_battery(net_load, price, export_coeff)
        opt = optimize_battery(net_load, price, params, initial_soc)

        total_no_batt += no_batt.total_cost_pln
        total_opt += opt.total_cost_pln
        day_count += 1
        day = day_end

    total_savings = total_no_batt - total_opt
    annual_savings = total_savings * 365.25 / day_count if day_count > 0 else 0
    payback = config.hardware_cost_pln / annual_savings if annual_savings > 0 else float("inf")
    roi_pct = annual_savings / config.hardware_cost_pln * 100 if config.hardware_cost_pln > 0 else 0
    pln_per_kwh_yr = annual_savings / config.capacity_kwh if config.capacity_kwh > 0 else 0

    return {
        "config": config,
        "days": day_count,
        "total_savings": total_savings,
        "annual_savings": annual_savings,
        "payback_years": payback,
        "roi_pct": roi_pct,
        "pln_per_kwh_yr": pln_per_kwh_yr,
        "hardware_cost": config.hardware_cost_pln,
        "capacity_kwh": config.capacity_kwh,
    }


# ── Main ────────────────────────────────────────────────────────────────────

def main():
    parser = argparse.ArgumentParser(
        description="Battery hardware ROI comparison: Dyness vs Pylontech"
    )
    parser.add_argument(
        "--start", type=str, default=None, help="Start date (YYYY-MM-DD)"
    )
    parser.add_argument(
        "--end", type=str, default=None, help="End date (YYYY-MM-DD)"
    )
    parser.add_argument(
        "--max-towers", type=int, default=2,
        help="Max towers per brand (default: 2)"
    )
    parser.add_argument(
        "--soc-min", type=float, default=10, help="Min SoC (%%)"
    )
    parser.add_argument(
        "--soc-max", type=float, default=90, help="Max SoC (%%)"
    )
    parser.add_argument(
        "--export-coeff", type=float, default=0.8,
        help="Export coefficient (0-1)"
    )
    args = parser.parse_args()

    cfg = load_config()
    hourly = load_data(cfg)

    # Determine date range
    if args.start:
        start = pd.Timestamp(args.start, tz="UTC")
    else:
        start = hourly.index.min().normalize()
    if args.end:
        end = pd.Timestamp(args.end, tz="UTC")
    else:
        end = hourly.index.max().normalize() + pd.Timedelta(days=1)

    total_days = (end - start).days

    # Generate all configs
    configs = generate_dyness_configs(args.max_towers) + generate_pylontech_configs(args.max_towers)

    print(f"\n=== Battery Hardware ROI Comparison ===")
    print(f"Period: {start.strftime('%Y-%m-%d')} to {end.strftime('%Y-%m-%d')} ({total_days} days)")
    print(f"Inverter: Deye {INVERTER_MAX_POWER_W}W ({INVERTER_COST_PLN} PLN, shared)")
    print(f"Configurations: {len(configs)} total\n")

    # Run all configs
    results = []
    for i, hw in enumerate(configs):
        print(f"  [{i+1}/{len(configs)}] {hw.label} ({hw.hardware_cost_pln:,.0f} PLN)...", end="", flush=True)
        r = run_config(
            hourly, hw, start, end,
            soc_min_pct=args.soc_min,
            soc_max_pct=args.soc_max,
            export_coeff=args.export_coeff,
        )
        results.append(r)
        print(f" savings {r['annual_savings']:,.0f} PLN/yr, payback {r['payback_years']:.1f}y")

    # Sort by ROI
    results.sort(key=lambda r: r["roi_pct"], reverse=True)
    best = results[0]

    # Print table
    print(f"\n{'Brand':<10} {'Capacity':>9} {'Modules':>8} {'Cost':>10} {'Savings/yr':>11} {'PLN/kWh/yr':>11} {'Payback':>9} {'ROI':>7}")
    print("-" * 85)

    for r in results:
        hw = r["config"]
        mod_str = f"{hw.total_modules}"
        if hw.towers > 1:
            mod_str += f" ({hw.towers}x{hw.modules_per_tower})"
        marker = " <-- best" if r is best else ""
        print(
            f"{hw.brand:<10} {hw.capacity_kwh:>7.1f}kWh {mod_str:>8} "
            f"{hw.hardware_cost_pln:>9,.0f} {r['annual_savings']:>10,.0f} "
            f"{r['pln_per_kwh_yr']:>11.1f} {r['payback_years']:>8.1f}y "
            f"{r['roi_pct']:>6.1f}%{marker}"
        )

    print(f"\nBest ROI: {best['config'].label}")
    print(
        f"  Cost: {best['hardware_cost']:,.0f} PLN | "
        f"Savings: {best['annual_savings']:,.0f} PLN/yr | "
        f"Payback: {best['payback_years']:.1f} years"
    )

    _plot_comparison(results)


def _plot_comparison(results: list[dict]) -> None:
    """Generate comparison plot with payback years and ROI."""
    try:
        import matplotlib.pyplot as plt
    except ImportError:
        print("matplotlib not available, skipping plot")
        return

    plt.style.use("ggplot")
    fig, (ax1, ax2) = plt.subplots(1, 2, figsize=(16, 7))

    # Separate by brand
    dyness = [r for r in results if r["config"].brand == "Dyness"]
    pylontech = [r for r in results if r["config"].brand == "Pylontech"]

    # --- Left: Payback years vs capacity ---
    for brand_results, color, marker in [
        (dyness, "#5bb88a", "o"),
        (pylontech, "#4a6fa5", "s"),
    ]:
        caps = [r["capacity_kwh"] for r in brand_results]
        paybacks = [min(r["payback_years"], 30) for r in brand_results]
        costs = [r["hardware_cost"] for r in brand_results]
        brand = brand_results[0]["config"].brand if brand_results else ""

        ax1.scatter(caps, paybacks, c=color, marker=marker, s=80, label=brand, zorder=3)
        for r in brand_results:
            ax1.annotate(
                f"{r['hardware_cost']/1000:.0f}k",
                (r["capacity_kwh"], min(r["payback_years"], 30)),
                textcoords="offset points", xytext=(5, 5),
                fontsize=7, color=color,
            )

    ax1.set_xlabel("Battery Capacity (kWh)")
    ax1.set_ylabel("Payback (years)")
    ax1.set_title("Payback Period vs Battery Size")
    ax1.legend()
    ax1.grid(True, alpha=0.3)

    # --- Right: Annual savings vs hardware cost ---
    for brand_results, color, marker in [
        (dyness, "#5bb88a", "o"),
        (pylontech, "#4a6fa5", "s"),
    ]:
        costs = [r["hardware_cost"] for r in brand_results]
        savings = [r["annual_savings"] for r in brand_results]
        brand = brand_results[0]["config"].brand if brand_results else ""

        ax2.scatter(costs, savings, c=color, marker=marker, s=80, label=brand, zorder=3)
        for r in brand_results:
            ax2.annotate(
                f"{r['capacity_kwh']:.0f}kWh",
                (r["hardware_cost"], r["annual_savings"]),
                textcoords="offset points", xytext=(5, 5),
                fontsize=7, color=color,
            )

    # Add break-even lines
    max_cost = max(r["hardware_cost"] for r in results) * 1.1
    for years in [5, 10, 15, 20]:
        x = np.linspace(0, max_cost, 100)
        y = x / years
        ax2.plot(x, y, "--", color="gray", alpha=0.3, linewidth=0.8)
        ax2.annotate(
            f"{years}y payback", (max_cost * 0.9, max_cost * 0.9 / years),
            fontsize=7, color="gray",
        )

    ax2.set_xlabel("Hardware Cost (PLN)")
    ax2.set_ylabel("Annual Savings (PLN)")
    ax2.set_title("Annual Savings vs Investment")
    ax2.legend()
    ax2.grid(True, alpha=0.3)

    fig.suptitle("Battery Hardware ROI: Dyness vs Pylontech", fontsize=14, fontweight="bold")
    plt.tight_layout()

    out_path = project_root() / "docs" / "forecast" / "battery_hw_roi.png"
    out_path.parent.mkdir(parents=True, exist_ok=True)
    plt.savefig(out_path, dpi=150)
    print(f"\nSaved plot to {out_path}")
    plt.close()


if __name__ == "__main__":
    main()
