"""LightGBM model implementation."""

import json
from pathlib import Path

import joblib
import numpy as np
import pandas as pd
from lightgbm import LGBMRegressor
from sklearn.metrics import mean_absolute_error, mean_squared_error, r2_score

from .base import BaseModel


class LightGBMModel(BaseModel):
    def __init__(self, params: dict | None = None):
        self.params = params or {}
        self.model: LGBMRegressor | None = None
        self.feature_names: list[str] = []
        self.train_metrics: dict = {}

    def fit(self, X: pd.DataFrame, y: pd.Series, eval_set=None) -> dict:
        """Train LightGBM model with optional eval set for early stopping."""
        self.feature_names = list(X.columns)

        callbacks = []
        fit_params = {}
        if eval_set is not None:
            fit_params["eval_set"] = [eval_set]
            fit_params["eval_metric"] = "mae"
            # Early stopping via callback
            from lightgbm import early_stopping, log_evaluation
            callbacks.append(early_stopping(50, verbose=True))
            callbacks.append(log_evaluation(100))

        self.model = LGBMRegressor(**self.params, verbose=-1)
        self.model.fit(X, y, callbacks=callbacks, **fit_params)

        # Compute training metrics
        y_pred = self.model.predict(X)
        self.train_metrics = {
            "mae": float(mean_absolute_error(y, y_pred)),
            "rmse": float(np.sqrt(mean_squared_error(y, y_pred))),
            "r2": float(r2_score(y, y_pred)),
        }
        return self.train_metrics

    def predict(self, X: pd.DataFrame) -> np.ndarray:
        """Predict using trained model."""
        if self.model is None:
            raise RuntimeError("Model not trained or loaded")
        return self.model.predict(X)

    def save(self, path: str) -> None:
        """Save model (.joblib) and metadata (.json)."""
        if self.model is None:
            raise RuntimeError("No model to save")

        joblib.dump(self.model, f"{path}.joblib")

        meta = {
            "type": "lightgbm",
            "feature_names": self.feature_names,
            "train_metrics": self.train_metrics,
            "params": self.params,
            "n_estimators_actual": self.model.n_estimators_,
        }
        with open(f"{path}.json", "w") as f:
            json.dump(meta, f, indent=2)

    @classmethod
    def load(cls, path: str) -> "LightGBMModel":
        """Load model from .joblib + .json files."""
        model = cls()
        model.model = joblib.load(f"{path}.joblib")

        meta_path = Path(f"{path}.json")
        if meta_path.exists():
            with open(meta_path) as f:
                meta = json.load(f)
            model.feature_names = meta.get("feature_names", [])
            model.train_metrics = meta.get("train_metrics", {})
            model.params = meta.get("params", {})

        return model

    def feature_importance(self) -> pd.Series:
        """Return feature importance (gain-based)."""
        if self.model is None:
            raise RuntimeError("Model not trained or loaded")
        return pd.Series(
            self.model.feature_importances_,
            index=self.feature_names,
        ).sort_values(ascending=False)

    def export_c(self, path: str) -> None:
        """Export model to C code using m2cgen."""
        try:
            import m2cgen as m2c
        except ImportError:
            raise ImportError("Install m2cgen: pip install m2cgen")

        if self.model is None:
            raise RuntimeError("No model to export")

        code = m2c.export_to_c(self.model)
        with open(path, "w") as f:
            f.write(code)
        print(f"  Exported C code to {path}")
