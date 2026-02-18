"""Abstract base class for ML models."""

from abc import ABC, abstractmethod

import numpy as np
import pandas as pd


class BaseModel(ABC):
    @abstractmethod
    def fit(self, X: pd.DataFrame, y: pd.Series, eval_set=None) -> dict:
        """Train model. Returns metrics dict."""

    @abstractmethod
    def predict(self, X: pd.DataFrame) -> np.ndarray:
        """Predict from features."""

    @abstractmethod
    def save(self, path: str) -> None:
        """Save trained model to path (without extension)."""

    @classmethod
    @abstractmethod
    def load(cls, path: str) -> "BaseModel":
        """Load trained model from path (without extension)."""

    @abstractmethod
    def feature_importance(self) -> pd.Series:
        """Return feature importance scores as a Series."""

    def export_c(self, path: str) -> None:
        """Export to C code for microcontroller. Optional."""
        raise NotImplementedError(f"{self.__class__.__name__} does not support C export")
