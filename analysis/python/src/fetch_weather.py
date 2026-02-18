"""CLI entry point: fetch historical weather data from Open-Meteo."""

import argparse
from datetime import date

from .config import load_config, python_root
from .weather import fetch_historical


def main():
    parser = argparse.ArgumentParser(description="Fetch historical weather data from Open-Meteo")
    parser.add_argument(
        "--start", type=date.fromisoformat, default="2024-07-01",
        help="Start date (YYYY-MM-DD)",
    )
    parser.add_argument(
        "--end", type=date.fromisoformat, default=None,
        help="End date (YYYY-MM-DD, default: today)",
    )
    args = parser.parse_args()

    cfg = load_config()
    loc = cfg["location"]
    end = args.end or date.today()
    cache_dir = python_root() / "data" / "weather"

    print(f"Fetching weather for {loc['latitude']}°N {loc['longitude']}°E")
    print(f"  Range: {args.start} to {end}")
    print(f"  Cache: {cache_dir}")

    df = fetch_historical(loc["latitude"], loc["longitude"], args.start, end, cache_dir)
    print(f"  Total rows: {len(df)}")
    if not df.empty:
        print(f"  Date range: {df['timestamp'].min()} to {df['timestamp'].max()}")


if __name__ == "__main__":
    main()
