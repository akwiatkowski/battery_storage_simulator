"""Evaluation CLI: generate accuracy plots for a trained model."""

import argparse

import matplotlib
matplotlib.use("Agg")
import matplotlib.pyplot as plt
import numpy as np
import pandas as pd
from sklearn.metrics import mean_absolute_error, mean_squared_error, r2_score

from .config import load_config, python_root
from .features import prepare_pv_dataset
from .models.lightgbm_model import LightGBMModel


PREPARE_FUNCTIONS = {
    "pv": prepare_pv_dataset,
}


def evaluate_model(model_name: str, config: dict) -> None:
    """Load trained model, compute metrics, and generate evaluation plots."""
    model_cfg = config["models"][model_name]
    model_dir = python_root() / "models"
    out_dir = python_root() / "output"
    out_dir.mkdir(parents=True, exist_ok=True)

    print(f"=== Evaluating {model_name} model ===")

    # Load model
    model_path = str(model_dir / f"{model_name}_model")
    model = LightGBMModel.load(model_path)
    print(f"  Loaded: {model_path}.joblib")

    # Prepare dataset (same as training)
    prepare_fn = PREPARE_FUNCTIONS[model_name]
    X, y, timestamps = prepare_fn(config)

    # Temporal split
    test_days = model_cfg.get("test_days", 30)
    split_date = timestamps.max() - pd.Timedelta(days=test_days)
    test_mask = timestamps > split_date
    X_test, y_test = X[test_mask], y[test_mask]
    ts_test = timestamps[test_mask]

    y_pred = model.predict(X_test)

    # Local timestamps for plotting
    local_tz = config["location"]["timezone"]
    ts_local = ts_test.tz_convert(local_tz)

    # Overall metrics
    mae = mean_absolute_error(y_test, y_pred)
    rmse = np.sqrt(mean_squared_error(y_test, y_pred))
    r2 = r2_score(y_test, y_pred)
    print(f"  Test — MAE: {mae:.2f}  RMSE: {rmse:.2f}  R²: {r2:.4f}")

    # Scale to actual watts for more meaningful display
    cap = config["pv_system"]["capacity_kwp"]

    # 1. Actual vs predicted scatter
    _plot_scatter(y_test * cap, y_pred * cap, ts_local, out_dir / f"{model_name}_scatter.png")

    # 2. Daily energy comparison
    _plot_daily_energy(y_test * cap, y_pred * cap, ts_local, out_dir / f"{model_name}_daily_energy.png")

    # 3. Error by hour
    _plot_error_by_hour(y_test * cap, y_pred * cap, ts_local, out_dir / f"{model_name}_error_by_hour.png")

    # 4. Error by month
    _plot_error_by_month(y_test * cap, y_pred * cap, ts_local, out_dir / f"{model_name}_error_by_month.png")

    # 5. Time series overlay (2-week window)
    _plot_timeseries(y_test * cap, y_pred * cap, ts_local, out_dir / f"{model_name}_timeseries.png")


def _plot_scatter(y_true, y_pred, timestamps, path):
    fig, ax = plt.subplots(figsize=(8, 8))
    months = timestamps.month
    scatter = ax.scatter(y_true, y_pred, c=months, cmap="twilight", alpha=0.5, s=10)
    max_val = max(y_true.max(), y_pred.max()) * 1.1
    ax.plot([0, max_val], [0, max_val], "k--", alpha=0.3, label="Perfect")
    ax.set_xlabel("Actual (W)")
    ax.set_ylabel("Predicted (W)")
    ax.set_title("PV Production: Actual vs Predicted")
    ax.set_xlim(0, max_val)
    ax.set_ylim(0, max_val)
    ax.set_aspect("equal")
    plt.colorbar(scatter, ax=ax, label="Month")
    fig.tight_layout()
    fig.savefig(path, dpi=150)
    plt.close(fig)
    print(f"  Plot: {path}")


def _plot_daily_energy(y_true, y_pred, timestamps, path):
    """Daily energy bar chart: predicted vs actual kWh/day."""
    df = pd.DataFrame({
        "actual": y_true.values,
        "predicted": y_pred,
    }, index=timestamps)
    # Convert W to kWh (hourly samples → W × 1h / 1000 = kWh)
    daily = df.resample("D").sum() / 1000.0
    daily = daily[daily["actual"] > 0]

    fig, ax = plt.subplots(figsize=(14, 5))
    x = range(len(daily))
    width = 0.4
    ax.bar([i - width / 2 for i in x], daily["actual"], width, label="Actual", color="#e8b830", alpha=0.8)
    ax.bar([i + width / 2 for i in x], daily["predicted"], width, label="Predicted", color="#9b8fd8", alpha=0.8)
    ax.set_xlabel("Day")
    ax.set_ylabel("Energy (kWh/day)")
    ax.set_title("Daily PV Energy: Actual vs Predicted")
    ax.legend()
    # Show date labels for every 7th day
    tick_positions = list(range(0, len(daily), 7))
    ax.set_xticks(tick_positions)
    ax.set_xticklabels([daily.index[i].strftime("%m-%d") for i in tick_positions], rotation=45)
    fig.tight_layout()
    fig.savefig(path, dpi=150)
    plt.close(fig)
    print(f"  Plot: {path}")


def _plot_error_by_hour(y_true, y_pred, timestamps, path):
    """MAE per hour of day."""
    df = pd.DataFrame({
        "error": np.abs(y_true.values - y_pred),
        "hour": timestamps.hour,
    })
    hourly_mae = df.groupby("hour")["error"].mean()

    fig, ax = plt.subplots(figsize=(10, 5))
    hourly_mae.plot(kind="bar", ax=ax, color="#e87c6c", alpha=0.8)
    ax.set_xlabel("Hour of Day")
    ax.set_ylabel("MAE (W)")
    ax.set_title("Prediction Error by Hour")
    fig.tight_layout()
    fig.savefig(path, dpi=150)
    plt.close(fig)
    print(f"  Plot: {path}")


def _plot_error_by_month(y_true, y_pred, timestamps, path):
    """MAE per month."""
    df = pd.DataFrame({
        "error": np.abs(y_true.values - y_pred),
        "month": timestamps.month,
    })
    monthly_mae = df.groupby("month")["error"].mean()

    fig, ax = plt.subplots(figsize=(10, 5))
    monthly_mae.plot(kind="bar", ax=ax, color="#64b5f6", alpha=0.8)
    ax.set_xlabel("Month")
    ax.set_ylabel("MAE (W)")
    ax.set_title("Prediction Error by Month")
    fig.tight_layout()
    fig.savefig(path, dpi=150)
    plt.close(fig)
    print(f"  Plot: {path}")


def _plot_timeseries(y_true, y_pred, timestamps, path):
    """2-week time series overlay."""
    # Take the last 14 days
    end = timestamps.max()
    start = end - pd.Timedelta(days=14)
    mask = timestamps >= start

    ts = timestamps[mask]
    actual = y_true.values[mask]
    predicted = y_pred[mask]

    fig, ax = plt.subplots(figsize=(16, 5))
    ax.plot(ts, actual, label="Actual", color="#e8b830", alpha=0.8, linewidth=1)
    ax.plot(ts, predicted, label="Predicted", color="#9b8fd8", alpha=0.8, linewidth=1)
    ax.fill_between(ts, actual, predicted, alpha=0.15, color="#e87c6c")
    ax.set_xlabel("Time")
    ax.set_ylabel("Power (W)")
    ax.set_title("PV Production: 2-Week Test Window")
    ax.legend()
    fig.autofmt_xdate()
    fig.tight_layout()
    fig.savefig(path, dpi=150)
    plt.close(fig)
    print(f"  Plot: {path}")


def main():
    parser = argparse.ArgumentParser(description="Evaluate trained ML model")
    parser.add_argument("--model", required=True, choices=list(PREPARE_FUNCTIONS.keys()),
                        help="Model to evaluate")
    args = parser.parse_args()

    config = load_config()
    evaluate_model(args.model, config)


if __name__ == "__main__":
    main()
