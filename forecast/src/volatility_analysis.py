"""Electricity price volatility analysis: future projections and battery ROI impact.

Projects how increasing renewable penetration will affect electricity price
spreads, and how that changes battery storage payback time. Based on academic
research linking RES share to intra-day price volatility.

Generates two plots:
  1. Historical + projected daily price spread (3 scenarios)
  2. Battery payback years under each volatility scenario for multiple configs

Academic references (embedded in PAPERS dict below).

Usage:
    mise exec -- python -m forecast.src.volatility_analysis
    mise exec -- python -m forecast.src.volatility_analysis --max-towers 1
"""

import argparse

import numpy as np
import pandas as pd

from .backtest import load_data
from .battery_hw_roi import (
    INVERTER_MAX_POWER_W,
    HardwareConfig,
    generate_dyness_configs,
    generate_pylontech_configs,
    run_config,
)
from .config import load_config, project_root
from .data_loading import load_spot_prices

# ── Academic references ─────────────────────────────────────────────────────

PAPERS = [
    {
        "id": "IMF2025",
        "title": "Shocked: Electricity Price Volatility Spillovers in Europe",
        "authors": "IMF Working Paper",
        "year": 2025,
        "url": "https://www.imf.org/en/Publications/WP/Issues/2025/01/11/Shocked-Electricity-Price-Volatility-Spillovers-in-Europe-559701",
        "finding": (
            "Wind/solar fluctuations create price shocks that spill across "
            "borders within 48h. German wind volatility directly impacts "
            "Polish, Czech, and Slovak prices. Volatility spillovers increased "
            "significantly after 2022."
        ),
    },
    {
        "id": "MDPI2025",
        "title": (
            "Renewable Energy and Price Stability: An Analysis of Volatility "
            "and Market Shifts in the European Electricity Sector (2015-2025)"
        ),
        "authors": "Applied Sciences (MDPI)",
        "year": 2025,
        "url": "https://www.mdpi.com/2076-3417/15/12/6397",
        "finding": (
            "Significant volatility increase after 2021. Once RES share "
            "exceeds a threshold, extreme price volatility rises sharply. "
            "Negative pricing is now a systemic feature across European markets."
        ),
    },
    {
        "id": "Adaptive2025",
        "title": (
            "Adaptive Optimal Operation of Grid-Connected Battery Systems "
            "Under Varying Electricity Market Volatility"
        ),
        "authors": "Energy Conversion and Management (ScienceDirect)",
        "year": 2025,
        "url": "https://www.sciencedirect.com/science/article/pii/S0196890425011203",
        "finding": (
            "Under high-volatility scenarios, adaptive strategies increase "
            "annual battery profit by 80% and raise NPV by 50%, while "
            "shortening payback time by 35% vs low-volatility cases."
        ),
    },
    {
        "id": "NatureEnergy2025",
        "title": "Power Price Stability and the Insurance Value of Renewable Technologies",
        "authors": "Nature Energy",
        "year": 2025,
        "url": "https://www.nature.com/articles/s41560-025-01704-0",
        "finding": (
            "Renewables reduce average prices but increase short-term "
            "volatility. The 'insurance value' of price stability decreases "
            "as VRE share grows, creating arbitrage opportunities."
        ),
    },
    {
        "id": "Germany2024",
        "title": (
            "High Electricity Price Despite Expansion in Renewables: "
            "How Market Trends Shape Germany's Power Market"
        ),
        "authors": "Energy Policy (ScienceDirect)",
        "year": 2024,
        "url": "https://www.sciencedirect.com/science/article/pii/S0301421524004683",
        "finding": (
            "More VRE penetration leads to lower average wholesale prices "
            "but increased frequency of very low and very high price hours, "
            "widening the arbitrage window for storage."
        ),
    },
    {
        "id": "Stanford",
        "title": "Economics of Grid-Scale Energy Storage in Wholesale Electricity Markets",
        "authors": "Stanford GSB (Karaduman)",
        "year": 2022,
        "url": "https://gsb-faculty.stanford.edu/omer-karaduman/files/2022/09/Economics-of-Grid-Scale-Energy-Storage.pdf",
        "finding": (
            "Battery returns are maximized by trading intra-day price "
            "fluctuations. Storage reduces peak prices by 6-18% but the "
            "arbitrage value persists as renewable penetration grows."
        ),
    },
    {
        "id": "Rystad2025",
        "title": "European BESS ROI with 15-Minute Trading",
        "authors": "Rystad Energy",
        "year": 2025,
        "url": "https://www.energy-storage.news/battery-storage-assets-in-europe-could-see-3-roi-uplift-with-15-minute-trading-rystad-says/",
        "finding": (
            "EU switch to 15-min settlement increased arbitrage potential "
            "by 14%. Energy arbitrage grew from 9% to 23% of European BESS "
            "revenues between 2020 and 2024."
        ),
    },
]


# ── Volatility projection model ────────────────────────────────────────────

# Poland RES share trajectory (% of electricity generation)
# Sources: NECP, EU RED III targets, IEA projections
RES_TRAJECTORY = {
    2018: 15, 2019: 16, 2020: 17, 2021: 19, 2022: 22, 2023: 25,
    2024: 27, 2025: 30, 2026: 33, 2027: 36, 2028: 39, 2029: 42,
    2030: 45, 2031: 48, 2032: 51, 2033: 54, 2034: 57, 2035: 60,
    2036: 62, 2037: 64, 2038: 66, 2039: 68, 2040: 70,
}

# Spread multiplier scenarios relative to 2024-2025 baseline (1.0x)
# Based on literature: each 10pp RES increase → ~30-60% spread increase,
# but partially offset by storage deployment and market coupling.
SCENARIOS = {
    "conservative": {
        "label": "Conservative (storage dampens volatility)",
        "color": "#5bb88a",
        # Storage + interconnectors limit spread growth
        "spread_factor": lambda res: 1.0 + 0.012 * max(0, res - 28),
    },
    "moderate": {
        "label": "Moderate (literature consensus)",
        "color": "#e8b830",
        # Follows observed RES-volatility relationship
        "spread_factor": lambda res: 1.0 + 0.020 * max(0, res - 28),
    },
    "aggressive": {
        "label": "Aggressive (fast RES, slow storage)",
        "color": "#e87c6c",
        # Limited storage buildout, more extreme weather events
        "spread_factor": lambda res: 1.0 + 0.030 * max(0, res - 28),
    },
}


def compute_historical_spreads(prices_path) -> pd.DataFrame:
    """Load spot prices and compute yearly average daily spread."""
    price_df = load_spot_prices(prices_path)
    df = price_df.set_index("timestamp").sort_index()
    df.index = df.index.tz_convert("UTC")
    hourly = df["value"].resample("h").mean().dropna()

    daily = hourly.resample("D").agg(["mean", "std", "min", "max"])
    daily["spread"] = daily["max"] - daily["min"]

    yearly = daily.resample("YE").agg(
        mean_spread=("spread", "mean"),
        mean_price=("mean", "mean"),
        mean_std=("std", "mean"),
    )
    yearly.index = yearly.index.year
    return yearly


def project_spreads(historical: pd.DataFrame) -> pd.DataFrame:
    """Build future spread projections for each scenario.

    Returns DataFrame indexed by year with columns for each scenario.
    """
    # Use 2024-2025 average as baseline spread
    recent = historical.loc[historical.index.isin([2024, 2025])]
    if recent.empty:
        recent = historical.tail(2)
    baseline_spread = recent["mean_spread"].mean()

    rows = []
    for year in range(2018, 2041):
        res = RES_TRAJECTORY.get(year, 70)
        row = {"year": year, "res_share": res}

        if year in historical.index:
            row["historical"] = historical.loc[year, "mean_spread"]

        for name, scenario in SCENARIOS.items():
            factor = scenario["spread_factor"](res)
            row[name] = baseline_spread * factor

        rows.append(row)

    return pd.DataFrame(rows).set_index("year")


def scale_prices(hourly: pd.DataFrame, spread_factor: float) -> pd.DataFrame:
    """Scale intra-day price deviations by spread_factor.

    Preserves daily mean prices but amplifies/dampens the spread.
    """
    result = hourly.copy()
    daily_mean = result["price_pln_kwh"].resample("D").transform("mean")
    deviation = result["price_pln_kwh"] - daily_mean
    result["price_pln_kwh"] = daily_mean + deviation * spread_factor
    return result


def run_sensitivity(
    hourly: pd.DataFrame,
    configs: list[HardwareConfig],
    spread_factors: list[float],
    start: pd.Timestamp,
    end: pd.Timestamp,
    soc_min_pct: float,
    soc_max_pct: float,
    export_coeff: float,
) -> pd.DataFrame:
    """Run LP optimization for each config x spread_factor combination."""
    rows = []
    total = len(configs) * len(spread_factors)
    done = 0

    for hw in configs:
        for sf in spread_factors:
            done += 1
            print(
                f"  [{done}/{total}] {hw.short_label} @ {sf:.1f}x spread...",
                end="",
                flush=True,
            )
            scaled = scale_prices(hourly, sf)
            r = run_config(
                scaled, hw, start, end,
                soc_min_pct=soc_min_pct,
                soc_max_pct=soc_max_pct,
                export_coeff=export_coeff,
            )
            rows.append({
                "config": hw,
                "label": hw.label,
                "short_label": hw.short_label,
                "spread_factor": sf,
                "capacity_kwh": hw.capacity_kwh,
                "hardware_cost": hw.hardware_cost_pln,
                "annual_savings": r["annual_savings"],
                "payback_years": r["payback_years"],
                "roi_pct": r["roi_pct"],
            })
            print(f" payback {r['payback_years']:.1f}y")

    return pd.DataFrame(rows)


# ── Plotting ────────────────────────────────────────────────────────────────

def plot_results(
    projections: pd.DataFrame,
    sensitivity: pd.DataFrame,
    configs: list[HardwareConfig],
) -> None:
    try:
        import matplotlib.pyplot as plt
        from matplotlib.lines import Line2D
    except ImportError:
        print("matplotlib not available, skipping plots")
        return

    plt.style.use("ggplot")
    fig, (ax1, ax2) = plt.subplots(1, 2, figsize=(18, 7))

    # ── Graph 1: Price spread projection ────────────────────────────────
    # Historical data points
    hist = projections["historical"].dropna()
    ax1.scatter(
        hist.index, hist.values, color="#333", s=50, zorder=5, label="Historical"
    )

    # Scenario lines (only future)
    for name, scenario in SCENARIOS.items():
        data = projections[name]
        # Draw dashed for future, solid overlap with historical
        future_mask = projections.index >= 2025
        ax1.plot(
            projections.index[future_mask],
            data[future_mask],
            color=scenario["color"],
            linewidth=2.5,
            label=scenario["label"],
        )

    # RES share on secondary axis
    ax1_res = ax1.twinx()
    res_data = projections["res_share"]
    ax1_res.fill_between(
        projections.index, 0, res_data, alpha=0.08, color="#4a6fa5"
    )
    ax1_res.plot(
        projections.index, res_data,
        color="#4a6fa5", linewidth=1, linestyle=":", alpha=0.5,
    )
    ax1_res.set_ylabel("RES Share (%)", color="#4a6fa5")
    ax1_res.set_ylim(0, 100)
    ax1_res.tick_params(axis="y", labelcolor="#4a6fa5")

    ax1.set_xlabel("Year")
    ax1.set_ylabel("Avg Daily Price Spread (PLN/kWh)")
    ax1.set_title("Electricity Price Volatility: Historical + Projected")
    ax1.legend(loc="upper left", fontsize=9)
    ax1.set_xlim(2018, 2040)
    ax1.grid(True, alpha=0.3)

    # Add paper citation annotations
    ax1.annotate(
        "Sources: IMF 2025, MDPI 2025,\n"
        "Nature Energy 2025, ScienceDirect 2024",
        xy=(0.02, 0.02),
        xycoords="axes fraction",
        fontsize=7,
        color="gray",
        style="italic",
    )

    # ── Graph 2: Payback sensitivity ────────────────────────────────────
    # Map spread factors to approximate future years using moderate scenario
    mod_func = SCENARIOS["moderate"]["spread_factor"]
    factor_to_year = {}
    for year, res in RES_TRAJECTORY.items():
        if year >= 2025:
            f = round(mod_func(res), 1)
            if f not in factor_to_year:
                factor_to_year[f] = year

    # Brand colors and markers
    brand_styles = {
        "Dyness": {"color": "#5bb88a", "marker": "o"},
        "Pylontech": {"color": "#4a6fa5", "marker": "s"},
    }

    for hw in configs:
        cfg_data = sensitivity[sensitivity["label"] == hw.label]
        if cfg_data.empty:
            continue
        style = brand_styles.get(hw.brand, {"color": "gray", "marker": "x"})
        ax2.plot(
            cfg_data["spread_factor"],
            cfg_data["payback_years"].clip(upper=30),
            color=style["color"],
            marker=style["marker"],
            markersize=6,
            linewidth=2,
            label=f"{hw.label} ({hw.hardware_cost_pln/1000:.0f}k PLN)",
        )

    # Add secondary x-axis labels with approximate years
    ax2_year = ax2.twiny()
    year_labels = {2025: 1.0, 2030: mod_func(RES_TRAJECTORY[2030]),
                   2035: mod_func(RES_TRAJECTORY[2035]),
                   2040: mod_func(RES_TRAJECTORY[2040])}
    ax2_year.set_xlim(ax2.get_xlim())
    ax2_year.set_xticks(list(year_labels.values()))
    ax2_year.set_xticklabels(
        [f"~{y}" for y in year_labels.keys()], fontsize=9
    )
    ax2_year.set_xlabel("Approximate Year (moderate scenario)", fontsize=9)

    ax2.set_xlabel("Price Spread Multiplier (1.0 = current)")
    ax2.set_ylabel("Payback Period (years)")
    ax2.set_title("Battery Payback vs Price Volatility")
    ax2.legend(fontsize=8, loc="upper right")
    ax2.grid(True, alpha=0.3)
    ax2.axhline(y=10, color="gray", linestyle="--", alpha=0.3)
    ax2.annotate(
        "10-year target", xy=(ax2.get_xlim()[1], 10),
        fontsize=8, color="gray", ha="right", va="bottom",
    )

    # Annotate key finding from Adaptive2025 paper
    ax2.annotate(
        '"High volatility shortens\n payback by 35%"\n— ScienceDirect 2025',
        xy=(0.98, 0.02),
        xycoords="axes fraction",
        fontsize=7,
        color="gray",
        style="italic",
        ha="right",
    )

    fig.suptitle(
        "Impact of Renewable-Driven Price Volatility on Battery Storage ROI",
        fontsize=14,
        fontweight="bold",
    )
    plt.tight_layout()

    out_path = (
        project_root() / "docs" / "forecast" / "volatility_roi.png"
    )
    out_path.parent.mkdir(parents=True, exist_ok=True)
    plt.savefig(out_path, dpi=150)
    print(f"\nSaved plot to {out_path}")
    plt.close()


# ── Paper summary printer ───────────────────────────────────────────────────

def print_papers():
    """Print formatted summary of referenced academic papers."""
    print("\n=== Referenced Academic Papers ===\n")
    for p in PAPERS:
        print(f"[{p['id']}] {p['title']}")
        print(f"  {p['authors']}, {p['year']}")
        print(f"  Key finding: {p['finding']}")
        print(f"  URL: {p['url']}")
        print()


# ── Main ────────────────────────────────────────────────────────────────────

def main():
    parser = argparse.ArgumentParser(
        description=(
            "Analyze impact of increasing price volatility on battery ROI. "
            "Based on academic research linking renewable penetration to "
            "electricity price spread growth."
        )
    )
    parser.add_argument(
        "--max-towers", type=int, default=1,
        help="Max battery towers per brand (default: 1, keeps configs manageable)",
    )
    parser.add_argument(
        "--start", type=str, default=None, help="Start date (YYYY-MM-DD)"
    )
    parser.add_argument(
        "--end", type=str, default=None, help="End date (YYYY-MM-DD)"
    )
    parser.add_argument(
        "--soc-min", type=float, default=10, help="Min SoC (%%)"
    )
    parser.add_argument(
        "--soc-max", type=float, default=90, help="Max SoC (%%)"
    )
    parser.add_argument(
        "--export-coeff", type=float, default=0.8, help="Export coefficient"
    )
    parser.add_argument(
        "--spread-range", type=str, default="0.8,1.0,1.2,1.5,1.8,2.0,2.5",
        help="Comma-separated spread multipliers to test (default: 0.8-2.5)",
    )
    parser.add_argument(
        "--papers", action="store_true",
        help="Print referenced papers and exit",
    )
    args = parser.parse_args()

    if args.papers:
        print_papers()
        return

    spread_factors = [float(x) for x in args.spread_range.split(",")]

    # ── Load data ───────────────────────────────────────────────────────
    cfg = load_config()
    root = project_root()

    print("=== Volatility Impact on Battery ROI ===\n")
    print_papers()

    # Historical spreads
    print("─── Historical Price Volatility ───\n")
    prices_path = root / "input" / "recent" / "historic_spot_prices.csv"
    historical = compute_historical_spreads(prices_path)

    print(f"{'Year':>6} {'Spread':>8} {'Mean Price':>10} {'Std':>8}")
    for year, row in historical.iterrows():
        print(
            f"{year:>6} {row['mean_spread']:>8.4f} "
            f"{row['mean_price']:>10.4f} {row['mean_std']:>8.4f}"
        )

    # Projections
    projections = project_spreads(historical)
    print("\n─── Projected Spread by Scenario ───\n")
    print(f"{'Year':>6} {'RES%':>5} {'Conserv':>8} {'Moderate':>8} {'Aggress':>8}")
    for year in range(2025, 2041, 5):
        if year in projections.index:
            r = projections.loc[year]
            print(
                f"{year:>6} {r['res_share']:>4.0f}% "
                f"{r['conservative']:>8.3f} {r['moderate']:>8.3f} "
                f"{r['aggressive']:>8.3f}"
            )

    # ── Battery configs ─────────────────────────────────────────────────
    all_configs = (
        generate_dyness_configs(args.max_towers)
        + generate_pylontech_configs(args.max_towers)
    )

    # Pick representative configs: smallest + best-ROI + largest for each brand
    selected = _select_representative_configs(all_configs)

    print(f"\n─── Battery Payback Sensitivity ({len(selected)} configs) ───\n")
    print(f"Spread factors: {spread_factors}")
    print(f"Configs: {', '.join(c.label for c in selected)}\n")

    hourly = load_data(cfg)

    if args.start:
        start = pd.Timestamp(args.start, tz="UTC")
    else:
        start = hourly.index.min().normalize()
    if args.end:
        end = pd.Timestamp(args.end, tz="UTC")
    else:
        end = hourly.index.max().normalize() + pd.Timedelta(days=1)

    sensitivity = run_sensitivity(
        hourly, selected, spread_factors, start, end,
        soc_min_pct=args.soc_min,
        soc_max_pct=args.soc_max,
        export_coeff=args.export_coeff,
    )

    # Print summary table
    print(f"\n{'Config':<30} ", end="")
    for sf in spread_factors:
        print(f"{'x' + str(sf):>7}", end="")
    print()
    print("-" * (30 + 7 * len(spread_factors)))

    for hw in selected:
        row_data = sensitivity[sensitivity["label"] == hw.label]
        print(f"{hw.label:<30} ", end="")
        for sf in spread_factors:
            match = row_data[row_data["spread_factor"] == sf]
            if not match.empty:
                pb = match.iloc[0]["payback_years"]
                if pb > 99:
                    print(f"{'inf':>7}", end="")
                else:
                    print(f"{pb:>6.1f}y", end="")
            else:
                print(f"{'?':>7}", end="")
        print()

    # Current vs moderate 2030/2035 comparison
    mod_2030 = SCENARIOS["moderate"]["spread_factor"](RES_TRAJECTORY[2030])
    mod_2035 = SCENARIOS["moderate"]["spread_factor"](RES_TRAJECTORY[2035])
    print(f"\nModerate scenario: 2030 ≈ {mod_2030:.1f}x spread, 2035 ≈ {mod_2035:.1f}x spread")

    for hw in selected:
        row = sensitivity[sensitivity["label"] == hw.label]
        current = row[row["spread_factor"] == 1.0]
        if current.empty:
            continue
        cur_pb = current.iloc[0]["payback_years"]
        # Find closest spread factor to mod_2035
        closest_35 = row.iloc[(row["spread_factor"] - mod_2035).abs().argsort()[:1]]
        proj_pb = closest_35.iloc[0]["payback_years"]
        reduction_pct = (1 - proj_pb / cur_pb) * 100 if cur_pb > 0 else 0
        print(
            f"  {hw.label}: {cur_pb:.1f}y today → ~{proj_pb:.1f}y by 2035 "
            f"({reduction_pct:.0f}% shorter)"
        )

    # ── Plot ────────────────────────────────────────────────────────────
    plot_results(projections, sensitivity, selected)


def _select_representative_configs(
    configs: list[HardwareConfig],
) -> list[HardwareConfig]:
    """Pick representative configs: smallest and mid-size for each brand."""
    selected = []
    for brand in ("Dyness", "Pylontech"):
        brand_cfgs = [c for c in configs if c.brand == brand]
        if not brand_cfgs:
            continue
        brand_cfgs.sort(key=lambda c: c.capacity_kwh)
        # Smallest (cheapest)
        selected.append(brand_cfgs[0])
        # Middle
        mid_idx = len(brand_cfgs) // 2
        if mid_idx != 0:
            selected.append(brand_cfgs[mid_idx])
    return selected


if __name__ == "__main__":
    main()
