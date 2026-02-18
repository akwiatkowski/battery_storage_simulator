"""Compare predicted vs actual daily HP energy usage.

Usage: mise exec -- python -m analysis.python.src.compare_daily
"""

import numpy as np
import pandas as pd

from .config import load_config, python_root
from .features import prepare_heat_pump_dataset
from .models.lightgbm_model import LightGBMModel


def main():
    config = load_config()
    model_path = str(python_root() / "models" / "heat_pump_model")
    model = LightGBMModel.load(model_path)

    print("Loading dataset...")
    X, y, timestamps = prepare_heat_pump_dataset(config)

    # Filter to January 2026
    start = pd.Timestamp("2026-01-01", tz="UTC")
    end = pd.Timestamp("2026-01-31", tz="UTC")
    mask = (timestamps >= start) & (timestamps < end)

    X_jan = X[mask]
    y_jan = y[mask]
    ts_jan = timestamps[mask]

    if len(X_jan) == 0:
        print("No data found for January 2026!")
        return

    print(f"January 2026: {len(X_jan)} samples at 6h resolution\n")

    # Predict
    y_pred = model.predict(X_jan)

    # Build a DataFrame with actual and predicted, indexed by timestamp
    df = pd.DataFrame({
        "actual_w": y_jan.values,
        "predicted_w": y_pred,
    }, index=ts_jan)

    # Aggregate to daily: mean power (W) -> daily energy (kWh) = mean_W * 24 / 1000
    daily = df.resample("D").mean()
    daily["actual_kwh"] = daily["actual_w"] * 24 / 1000
    daily["predicted_kwh"] = daily["predicted_w"] * 24 / 1000
    daily["error_kwh"] = daily["predicted_kwh"] - daily["actual_kwh"]
    daily["error_pct"] = np.where(
        daily["actual_kwh"] > 0.1,
        daily["error_kwh"] / daily["actual_kwh"] * 100,
        0,
    )

    # Print comparison table
    print(f"{'Date':<12} {'Actual':>10} {'Predicted':>10} {'Error':>10} {'Error%':>8}")
    print(f"{'':.<12} {'(kWh)':>10} {'(kWh)':>10} {'(kWh)':>10} {'':>8}")
    print("-" * 52)

    for date, row in daily.iterrows():
        date_str = date.strftime("%Y-%m-%d")
        print(f"{date_str:<12} {row['actual_kwh']:>10.2f} {row['predicted_kwh']:>10.2f} "
              f"{row['error_kwh']:>+10.2f} {row['error_pct']:>+7.1f}%")

    print("-" * 52)

    # Summary stats
    total_actual = daily["actual_kwh"].sum()
    total_predicted = daily["predicted_kwh"].sum()
    mae_kwh = daily["error_kwh"].abs().mean()
    total_error = total_predicted - total_actual

    print(f"{'TOTAL':<12} {total_actual:>10.1f} {total_predicted:>10.1f} "
          f"{total_error:>+10.1f} {total_error / total_actual * 100:>+7.1f}%")
    print(f"\nDaily MAE: {mae_kwh:.2f} kWh")
    print(f"Daily MAPE: {daily['error_pct'].abs().mean():.1f}%")


if __name__ == "__main__":
    main()
