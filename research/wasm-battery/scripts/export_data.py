"""Export hourly energy data to JSON for the WASM battery simulator.

Run from project root:
    mise exec -- python research/wasm-battery/scripts/export_data.py
"""

import json
import sys
from pathlib import Path

# Add project root to path so we can import from analysis.python
project_root = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(project_root))

from analysis.python.src.backtest import load_data
from analysis.python.src.config import load_config


def main():
    config = load_config()
    hourly = load_data(config)

    # Group by day
    days = []
    for date, group in hourly.groupby(hourly.index.date):
        if len(group) < 20:
            continue
        days.append({
            "date": str(date),
            "net_load_w": [round(v, 1) for v in group["net_load_w"].tolist()],
            "price_pln_kwh": [round(v, 4) for v in group["price_pln_kwh"].tolist()],
        })

    out_path = Path(__file__).parent.parent / "data" / "hourly.json"
    out_path.parent.mkdir(parents=True, exist_ok=True)

    with open(out_path, "w") as f:
        json.dump(days, f, separators=(",", ":"))

    size_kb = out_path.stat().st_size / 1024
    print(f"Exported {len(days)} days to {out_path} ({size_kb:.0f} KB)")


if __name__ == "__main__":
    main()
