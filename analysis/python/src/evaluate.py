"""Evaluation CLI: generate accuracy plots for a trained model."""

import argparse

import matplotlib
matplotlib.use("Agg")
import matplotlib.pyplot as plt
import numpy as np
import pandas as pd
from sklearn.metrics import mean_absolute_error, mean_squared_error, r2_score

from .config import load_config, python_root
from .features import (
    prepare_pv_dataset,
    prepare_consumption_dataset,
    prepare_heat_pump_dataset,
    prepare_dhw_dataset,
    prepare_spot_price_dataset,
)
from .models.lightgbm_model import LightGBMModel


PREPARE_FUNCTIONS = {
    "pv": prepare_pv_dataset,
    "consumption": prepare_consumption_dataset,
    "heat_pump": prepare_heat_pump_dataset,
    "dhw": prepare_dhw_dataset,
    "spot_price": prepare_spot_price_dataset,
}

MODEL_META = {
    "pv":          {"unit": "W",       "title": "PV Production"},
    "consumption": {"unit": "W",       "title": "Base Consumption"},
    "heat_pump":   {"unit": "W",       "title": "Heat Pump Heating"},
    "dhw":         {"unit": "W",       "title": "DHW Hot Water"},
    "spot_price":  {"unit": "PLN/kWh", "title": "Spot Price"},
}


def evaluate_model(model_name: str, config: dict) -> None:
    """Load trained model, compute metrics, and generate evaluation plots."""
    model_cfg = config["models"][model_name]
    model_dir = python_root() / "models"
    out_dir = python_root() / "output"
    out_dir.mkdir(parents=True, exist_ok=True)

    meta = MODEL_META[model_name]
    unit = meta["unit"]
    title = meta["title"]

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

    # Scale PV from W/kWp to W for display
    if model_name == "pv":
        scale = config["pv_system"]["capacity_kwp"]
        y_display = y_test * scale
        y_pred_display = y_pred * scale
    else:
        y_display = y_test
        y_pred_display = y_pred

    # Overall metrics (on display scale)
    mae = mean_absolute_error(y_display, y_pred_display)
    rmse = np.sqrt(mean_squared_error(y_display, y_pred_display))
    r2 = r2_score(y_display, y_pred_display)
    print(f"  Test — MAE: {mae:.2f} {unit}  RMSE: {rmse:.2f}  R²: {r2:.4f}")

    # 1. Actual vs predicted scatter
    _plot_scatter(y_display, y_pred_display, ts_local, unit, title,
                  out_dir / f"{model_name}_scatter.png")

    # 2. Daily energy/value comparison
    _plot_daily(y_display, y_pred_display, ts_local, unit, title,
                out_dir / f"{model_name}_daily.png")

    # 3. Error by hour
    _plot_error_by_hour(y_display, y_pred_display, ts_local, unit, title,
                        out_dir / f"{model_name}_error_by_hour.png")

    # 4. Error by month
    _plot_error_by_month(y_display, y_pred_display, ts_local, unit, title,
                         out_dir / f"{model_name}_error_by_month.png")

    # 5. Time series overlay (2-week window)
    _plot_timeseries(y_display, y_pred_display, ts_local, unit, title,
                     out_dir / f"{model_name}_timeseries.png")


def _plot_scatter(y_true, y_pred, timestamps, unit, title, path):
    fig, ax = plt.subplots(figsize=(8, 8))
    months = timestamps.month
    scatter = ax.scatter(y_true, y_pred, c=months, cmap="twilight", alpha=0.5, s=10)
    max_val = max(y_true.max(), y_pred.max()) * 1.1
    min_val = min(y_true.min(), y_pred.min(), 0)
    ax.plot([min_val, max_val], [min_val, max_val], "k--", alpha=0.3, label="Perfect")
    ax.set_xlabel(f"Actual ({unit})")
    ax.set_ylabel(f"Predicted ({unit})")
    ax.set_title(f"{title}: Actual vs Predicted")
    ax.set_xlim(min_val, max_val)
    ax.set_ylim(min_val, max_val)
    ax.set_aspect("equal")
    plt.colorbar(scatter, ax=ax, label="Month")
    fig.tight_layout()
    fig.savefig(path, dpi=150)
    plt.close(fig)
    print(f"  Plot: {path}")


def _plot_daily(y_true, y_pred, timestamps, unit, title, path):
    """Daily aggregate bar chart."""
    df = pd.DataFrame({
        "actual": y_true.values,
        "predicted": y_pred,
    }, index=timestamps)
    # For power models: convert W to kWh (hourly samples)
    if unit == "W":
        daily = df.resample("D").sum() / 1000.0
        ylabel = "Energy (kWh/day)"
    else:
        daily = df.resample("D").mean()
        ylabel = f"Mean ({unit})"
    daily = daily[daily["actual"].abs() > 0]

    fig, ax = plt.subplots(figsize=(14, 5))
    x = range(len(daily))
    width = 0.4
    ax.bar([i - width / 2 for i in x], daily["actual"], width, label="Actual", color="#e8b830", alpha=0.8)
    ax.bar([i + width / 2 for i in x], daily["predicted"], width, label="Predicted", color="#9b8fd8", alpha=0.8)
    ax.set_xlabel("Day")
    ax.set_ylabel(ylabel)
    ax.set_title(f"Daily {title}: Actual vs Predicted")
    ax.legend()
    tick_positions = list(range(0, len(daily), 7))
    ax.set_xticks(tick_positions)
    ax.set_xticklabels([daily.index[i].strftime("%m-%d") for i in tick_positions], rotation=45)
    fig.tight_layout()
    fig.savefig(path, dpi=150)
    plt.close(fig)
    print(f"  Plot: {path}")


def _plot_error_by_hour(y_true, y_pred, timestamps, unit, title, path):
    """MAE per hour of day."""
    df = pd.DataFrame({
        "error": np.abs(y_true.values - y_pred),
        "hour": timestamps.hour,
    })
    hourly_mae = df.groupby("hour")["error"].mean()

    fig, ax = plt.subplots(figsize=(10, 5))
    hourly_mae.plot(kind="bar", ax=ax, color="#e87c6c", alpha=0.8)
    ax.set_xlabel("Hour of Day")
    ax.set_ylabel(f"MAE ({unit})")
    ax.set_title(f"{title}: Error by Hour")
    fig.tight_layout()
    fig.savefig(path, dpi=150)
    plt.close(fig)
    print(f"  Plot: {path}")


def _plot_error_by_month(y_true, y_pred, timestamps, unit, title, path):
    """MAE per month."""
    df = pd.DataFrame({
        "error": np.abs(y_true.values - y_pred),
        "month": timestamps.month,
    })
    monthly_mae = df.groupby("month")["error"].mean()

    fig, ax = plt.subplots(figsize=(10, 5))
    monthly_mae.plot(kind="bar", ax=ax, color="#64b5f6", alpha=0.8)
    ax.set_xlabel("Month")
    ax.set_ylabel(f"MAE ({unit})")
    ax.set_title(f"{title}: Error by Month")
    fig.tight_layout()
    fig.savefig(path, dpi=150)
    plt.close(fig)
    print(f"  Plot: {path}")


def _plot_timeseries(y_true, y_pred, timestamps, unit, title, path):
    """2-week time series overlay."""
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
    ax.set_ylabel(f"Value ({unit})")
    ax.set_title(f"{title}: 2-Week Test Window")
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
