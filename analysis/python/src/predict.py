"""Forecast next 48h PV production using trained model + weather forecast."""

import argparse
from datetime import datetime

import pandas as pd

from .config import load_config, python_root
from .features import build_pv_features
from .models.lightgbm_model import LightGBMModel
from .weather import fetch_forecast


def predict_pv(config: dict, hours: int = 48) -> pd.DataFrame:
    """Generate PV production forecast."""
    loc = config["location"]
    capacity = config["pv_system"]["capacity_kwp"]

    # Load model
    model_dir = python_root() / "models"
    model = LightGBMModel.load(str(model_dir / "pv_model"))

    # Fetch weather forecast
    print(f"  Fetching {hours}h weather forecast...")
    weather_df = fetch_forecast(loc["latitude"], loc["longitude"], hours)

    # Build features
    features = build_pv_features(weather_df, config)

    # Predict (W per kWp)
    pred_per_kwp = model.predict(features)

    # Scale to system capacity and clip negatives
    pred_w = (pred_per_kwp * capacity).clip(min=0)

    # Build result table
    local_tz = loc["timezone"]
    result = pd.DataFrame({
        "timestamp": features.index.tz_convert(local_tz),
        "predicted_w": pred_w.round(0),
        "temperature": weather_df.set_index("timestamp")["temperature_2m"].reindex(features.index).values,
        "cloud_cover": weather_df.set_index("timestamp")["cloud_cover"].reindex(features.index).values,
        "direct_radiation": weather_df.set_index("timestamp")["direct_radiation"].reindex(features.index).values,
    })

    # Only show daytime hours
    result = result[features["solar_elevation"].values > 0]

    return result


def main():
    parser = argparse.ArgumentParser(description="Forecast PV production")
    parser.add_argument("--hours", type=int, default=48, help="Forecast horizon (hours)")
    parser.add_argument("--csv", type=str, default=None, help="Output CSV path")
    args = parser.parse_args()

    config = load_config()
    print(f"=== PV Forecast ({args.hours}h) ===")

    result = predict_pv(config, args.hours)

    if result.empty:
        print("  No daytime hours in forecast window")
        return

    # Print table
    print(f"\n{'Hour':<20} {'PV (W)':>8} {'Temp':>6} {'Cloud':>6} {'Rad':>6}")
    print("-" * 50)
    for _, row in result.iterrows():
        print(
            f"{row['timestamp'].strftime('%Y-%m-%d %H:%M'):<20} "
            f"{row['predicted_w']:>8.0f} "
            f"{row['temperature']:>5.1f}° "
            f"{row['cloud_cover']:>5.0f}% "
            f"{row['direct_radiation']:>5.0f}"
        )

    # Daily totals
    result_indexed = result.set_index("timestamp")
    daily = result_indexed["predicted_w"].resample("D").sum() / 1000.0
    print(f"\nDaily totals (kWh):")
    for day, kwh in daily.items():
        print(f"  {day.strftime('%Y-%m-%d')}: {kwh:.1f} kWh")

    if args.csv:
        result.to_csv(args.csv, index=False)
        print(f"\n  Saved to {args.csv}")


if __name__ == "__main__":
    main()
