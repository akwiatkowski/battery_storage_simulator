"""Forecast CLI: generate predictions using trained models + weather forecast."""

import argparse
from datetime import datetime

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
from .weather import fetch_forecast


FEATURE_BUILDERS = {
    "pv": build_pv_features,
    "consumption": build_consumption_features,
    "heat_pump": build_heating_features,
    "spot_price": build_spot_price_features,
}

MODEL_DISPLAY = {
    "pv":          {"col": "predicted_w",       "label": "PV (W)",       "fmt": "{:>8.0f}"},
    "consumption": {"col": "predicted_w",       "label": "Load (W)",     "fmt": "{:>8.0f}"},
    "heat_pump":   {"col": "predicted_w",       "label": "HP (W)",       "fmt": "{:>8.0f}"},
    "dhw":         {"col": "predicted_w",       "label": "DHW (W)",      "fmt": "{:>8.0f}"},
    "spot_price":  {"col": "predicted_price",   "label": "PLN/kWh",     "fmt": "{:>8.4f}"},
}


def predict_pv(config: dict, hours: int = 48) -> pd.DataFrame:
    """Generate PV production forecast."""
    loc = config["location"]
    capacity = config["pv_system"]["capacity_kwp"]

    model_dir = python_root() / "models"
    model = LightGBMModel.load(str(model_dir / "pv_model"))

    print(f"  Fetching {hours}h weather forecast...")
    weather_df = fetch_forecast(loc["latitude"], loc["longitude"], hours)
    features = build_pv_features(weather_df, config)

    pred_per_kwp = model.predict(features)
    pred_w = (pred_per_kwp * capacity).clip(min=0)

    local_tz = loc["timezone"]
    result = pd.DataFrame({
        "timestamp": features.index.tz_convert(local_tz),
        "predicted_w": pred_w.round(0),
        "temperature": weather_df.set_index("timestamp")["temperature_2m"].reindex(features.index).values,
        "cloud_cover": weather_df.set_index("timestamp")["cloud_cover"].reindex(features.index).values,
    })
    # Only show daytime hours
    result = result[features["solar_elevation"].values > 0]
    return result


def predict_weather_model(model_name: str, config: dict, hours: int = 48) -> pd.DataFrame:
    """Generate forecast for weather-dependent models (consumption, heat_pump, spot_price)."""
    loc = config["location"]

    model_dir = python_root() / "models"
    model = LightGBMModel.load(str(model_dir / f"{model_name}_model"))

    print(f"  Fetching {hours}h weather forecast...")
    weather_df = fetch_forecast(loc["latitude"], loc["longitude"], hours)
    builder = FEATURE_BUILDERS[model_name]
    features = builder(weather_df, config)

    pred = model.predict(features)

    # For spot price, add lagged features — use prediction itself as proxy
    if model_name == "spot_price":
        # Lag features aren't available for forecasts, use rolling mean of predictions
        features_with_lags = features.copy()
        features_with_lags["price_lag_1h"] = pd.Series(pred, index=features.index).shift(1).fillna(pred[0])
        features_with_lags["price_lag_24h"] = pd.Series(pred, index=features.index).shift(24).fillna(pred[0])
        features_with_lags["price_rolling_24h_mean"] = pd.Series(pred, index=features.index).rolling(24, min_periods=1).mean()
        pred = model.predict(features_with_lags)

    col = "predicted_price" if model_name == "spot_price" else "predicted_w"
    if col == "predicted_w":
        pred = pred.clip(min=0)

    local_tz = loc["timezone"]
    result = pd.DataFrame({
        "timestamp": features.index.tz_convert(local_tz),
        col: pred.round(4 if model_name == "spot_price" else 0),
        "temperature": weather_df.set_index("timestamp")["temperature_2m"].reindex(features.index).values,
    })
    return result


def predict_dhw(config: dict, hours: int = 48) -> pd.DataFrame:
    """Generate DHW forecast (no weather needed)."""
    loc = config["location"]

    model_dir = python_root() / "models"
    model = LightGBMModel.load(str(model_dir / "dhw_model"))

    # Build hourly timestamps
    now = pd.Timestamp.now(tz="UTC").floor("h")
    timestamps = pd.date_range(now, periods=hours, freq="h", tz="UTC")

    from .features import build_dhw_features
    features = build_dhw_features(timestamps, config)

    pred = model.predict(features).clip(min=0)

    local_tz = loc["timezone"]
    result = pd.DataFrame({
        "timestamp": features.index.tz_convert(local_tz),
        "predicted_w": pred.round(0),
    })
    return result


def main():
    parser = argparse.ArgumentParser(description="Generate forecast using trained model")
    parser.add_argument("--model", default="pv",
                        choices=["pv", "consumption", "heat_pump", "dhw", "spot_price"],
                        help="Model to use for prediction")
    parser.add_argument("--hours", type=int, default=48, help="Forecast horizon (hours)")
    parser.add_argument("--csv", type=str, default=None, help="Output CSV path")
    args = parser.parse_args()

    config = load_config()
    display = MODEL_DISPLAY[args.model]
    print(f"=== {args.model.replace('_', ' ').title()} Forecast ({args.hours}h) ===")

    if args.model == "pv":
        result = predict_pv(config, args.hours)
    elif args.model == "dhw":
        result = predict_dhw(config, args.hours)
    else:
        result = predict_weather_model(args.model, config, args.hours)

    if result.empty:
        print("  No data in forecast window")
        return

    # Print table
    col = display["col"]
    fmt = display["fmt"]
    header = f"{'Hour':<20} {display['label']:>8}"
    if "temperature" in result.columns:
        header += f" {'Temp':>6}"
    print(f"\n{header}")
    print("-" * len(header))
    for _, row in result.iterrows():
        line = f"{row['timestamp'].strftime('%Y-%m-%d %H:%M'):<20} {fmt.format(row[col])}"
        if "temperature" in result.columns:
            line += f" {row['temperature']:>5.1f}\u00b0"
        print(line)

    # Daily totals
    result_indexed = result.set_index("timestamp")
    if col == "predicted_w":
        daily = result_indexed[col].resample("D").sum() / 1000.0
        print(f"\nDaily totals (kWh):")
        for day, kwh in daily.items():
            print(f"  {day.strftime('%Y-%m-%d')}: {kwh:.1f} kWh")
    else:
        daily = result_indexed[col].resample("D").mean()
        print(f"\nDaily averages:")
        for day, val in daily.items():
            print(f"  {day.strftime('%Y-%m-%d')}: {val:.4f} PLN/kWh")

    if args.csv:
        result.to_csv(args.csv, index=False)
        print(f"\n  Saved to {args.csv}")


if __name__ == "__main__":
    main()
