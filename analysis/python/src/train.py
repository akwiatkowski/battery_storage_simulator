"""Unified training CLI: python -m analysis.python.src.train --model pv"""

import argparse
import json
from pathlib import Path

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


def _load_baseline_metrics(model_path: str) -> dict | None:
    """Load test metrics from existing model JSON, if available."""
    meta_path = Path(f"{model_path}.json")
    if not meta_path.exists():
        return None
    try:
        with open(meta_path) as f:
            meta = json.load(f)
        return meta.get("test_metrics")
    except (json.JSONDecodeError, OSError):
        return None


def _print_comparison(model_name: str, old: dict, new: dict) -> None:
    """Print before/after comparison table."""
    print(f"\n  === {model_name}: Before vs After ===")
    print(f"  {'Metric':<10} {'Old':>10} {'New':>10} {'Delta':>20}")
    print(f"  {'-' * 52}")
    for key, fmt in [("mae", ".2f"), ("rmse", ".2f"), ("r2", ".3f")]:
        old_val = old.get(key, 0)
        new_val = new.get(key, 0)
        delta = new_val - old_val
        if key == "r2":
            delta_str = f"{delta:+.3f}"
        else:
            pct = (delta / old_val * 100) if old_val != 0 else 0
            delta_str = f"{delta:+{fmt}}  ({pct:+.1f}%)"
        print(f"  {key.upper():<10} {old_val:>{fmt}} {new_val:>{fmt}} {delta_str:>20}")


def train_model(model_name: str, config: dict) -> None:
    """Train a single model by name."""
    if model_name not in PREPARE_FUNCTIONS:
        raise ValueError(f"Unknown model: {model_name}. Available: {list(PREPARE_FUNCTIONS)}")

    model_cfg = config["models"][model_name]
    out_dir = python_root() / "models"
    out_dir.mkdir(parents=True, exist_ok=True)
    plot_dir = python_root() / "output"
    plot_dir.mkdir(parents=True, exist_ok=True)

    print(f"=== Training {model_name} model ===")

    # Load baseline metrics from existing model (before overwriting)
    model_path = str(out_dir / f"{model_name}_model")
    baseline_metrics = _load_baseline_metrics(model_path)

    # Prepare dataset
    prepare_fn = PREPARE_FUNCTIONS[model_name]
    X, y, timestamps = prepare_fn(config)

    # Temporal split: last N days as test
    test_days = model_cfg.get("test_days", 30)
    split_date = timestamps.max() - pd.Timedelta(days=test_days)
    train_mask = timestamps <= split_date
    test_mask = timestamps > split_date

    X_train, y_train = X[train_mask], y[train_mask]
    X_test, y_test = X[test_mask], y[test_mask]

    print(f"  Train: {len(X_train)} samples ({timestamps[train_mask].min()} to {timestamps[train_mask].max()})")
    print(f"  Test:  {len(X_test)} samples ({timestamps[test_mask].min()} to {timestamps[test_mask].max()})")

    # Compute sample weights for heating models: weight cold samples more
    sample_weight = None
    if "heating_degree_hour" in X_train.columns:
        hdh = X_train["heating_degree_hour"].values
        # Weight proportional to heating severity: 1 + (hdh/10)^2
        # At HDH=30 (-12°C): weight=10, at HDH=15 (3°C): weight=3.25, at HDH=5 (13°C): weight=1.25
        sample_weight = 1.0 + (hdh / 10.0) ** 2
        print(f"  Sample weights: min={sample_weight.min():.1f}, max={sample_weight.max():.1f}, mean={sample_weight.mean():.1f}")

    # Instantiate and train
    model = LightGBMModel(params=model_cfg.get("params", {}))
    train_metrics = model.fit(X_train, y_train, eval_set=(X_test, y_test),
                              sample_weight=sample_weight)

    # Test metrics
    y_pred = model.predict(X_test)
    test_metrics = {
        "mae": float(mean_absolute_error(y_test, y_pred)),
        "rmse": float(np.sqrt(mean_squared_error(y_test, y_pred))),
        "r2": float(r2_score(y_test, y_pred)),
    }

    print(f"\n  Train — MAE: {train_metrics['mae']:.2f}  RMSE: {train_metrics['rmse']:.2f}  R²: {train_metrics['r2']:.4f}")
    print(f"  Test  — MAE: {test_metrics['mae']:.2f}  RMSE: {test_metrics['rmse']:.2f}  R²: {test_metrics['r2']:.4f}")

    # Print comparison if baseline exists
    if baseline_metrics:
        _print_comparison(model_name, baseline_metrics, test_metrics)
    else:
        print("\n  (No previous model found — skipping comparison)")

    # Save model
    model.save(model_path)

    # Append test_metrics to saved JSON
    meta_path = Path(f"{model_path}.json")
    with open(meta_path) as f:
        meta = json.load(f)
    meta["test_metrics"] = test_metrics
    with open(meta_path, "w") as f:
        json.dump(meta, f, indent=2)

    print(f"\n  Model saved: {model_path}.joblib")

    # Feature importance plot
    _save_feature_importance(model, plot_dir / f"{model_name}_feature_importance.png")


def _save_feature_importance(model: LightGBMModel, path: Path) -> None:
    """Save feature importance bar chart."""
    import matplotlib
    matplotlib.use("Agg")
    import matplotlib.pyplot as plt

    importance = model.feature_importance()

    fig, ax = plt.subplots(figsize=(10, 6))
    importance.plot(kind="barh", ax=ax)
    ax.set_xlabel("Importance (split count)")
    ax.set_title("Feature Importance")
    ax.invert_yaxis()
    fig.tight_layout()
    fig.savefig(path, dpi=150)
    plt.close(fig)
    print(f"  Feature importance plot: {path}")


def main():
    parser = argparse.ArgumentParser(description="Train ML model")
    parser.add_argument("--model", required=True, choices=list(PREPARE_FUNCTIONS.keys()),
                        help="Model to train")
    args = parser.parse_args()

    config = load_config()
    train_model(args.model, config)


if __name__ == "__main__":
    main()
