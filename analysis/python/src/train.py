"""Unified training CLI: python -m analysis.python.src.train --model pv"""

import argparse
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

    # Instantiate and train
    model = LightGBMModel(params=model_cfg.get("params", {}))
    train_metrics = model.fit(X_train, y_train, eval_set=(X_test, y_test))

    # Test metrics
    y_pred = model.predict(X_test)
    test_metrics = {
        "mae": float(mean_absolute_error(y_test, y_pred)),
        "rmse": float(np.sqrt(mean_squared_error(y_test, y_pred))),
        "r2": float(r2_score(y_test, y_pred)),
    }

    print(f"\n  Train — MAE: {train_metrics['mae']:.2f}  RMSE: {train_metrics['rmse']:.2f}  R²: {train_metrics['r2']:.4f}")
    print(f"  Test  — MAE: {test_metrics['mae']:.2f}  RMSE: {test_metrics['rmse']:.2f}  R²: {test_metrics['r2']:.4f}")

    # Save model
    model_path = str(out_dir / f"{model_name}_model")
    model.save(model_path)
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
