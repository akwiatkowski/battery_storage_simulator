"""Tests for feature engineering module."""

import math
from datetime import datetime, timezone

import numpy as np
import pandas as pd
import pytest

from analysis.python.src.features import (
    build_pv_features,
    build_consumption_features,
    build_heating_features,
    build_dhw_features,
    build_spot_price_features,
    solar_elevation,
)


def _make_config():
    return {
        "location": {
            "latitude": 52.4064,
            "longitude": 16.9252,
            "timezone": "Europe/Warsaw",
        },
        "pv_system": {"capacity_kwp": 6.5, "azimuth": 90, "tilt": 40},
    }


def _make_weather_df(n_hours=48, start="2024-07-01"):
    """Create a synthetic weather DataFrame."""
    timestamps = pd.date_range(start, periods=n_hours, freq="h", tz="UTC")
    return pd.DataFrame({
        "timestamp": timestamps,
        "temperature_2m": np.random.uniform(15, 30, n_hours),
        "cloud_cover": np.random.uniform(0, 100, n_hours),
        "direct_radiation": np.maximum(0, np.random.uniform(-50, 800, n_hours)),
        "diffuse_radiation": np.maximum(0, np.random.uniform(0, 200, n_hours)),
        "sunshine_duration": np.random.uniform(0, 3600, n_hours),
        "wind_speed_10m": np.random.uniform(0, 15, n_hours),
        "precipitation": np.random.uniform(0, 5, n_hours),
        "relative_humidity_2m": np.random.uniform(30, 95, n_hours),
    })


class TestSolarElevation:
    """Test solar elevation computation."""

    def test_noon_summer_poznan(self):
        """Sun should be high at solar noon in summer at 52N."""
        dt = datetime(2024, 6, 21, 11, 0, tzinfo=timezone.utc)
        elev = solar_elevation(52.41, 16.93, dt)
        assert 50 < elev < 65, f"Expected ~61 at summer solstice noon, got {elev:.1f}"

    def test_midnight_is_negative(self):
        """Sun should be below horizon at midnight."""
        dt = datetime(2024, 6, 21, 0, 0, tzinfo=timezone.utc)
        elev = solar_elevation(52.41, 16.93, dt)
        assert elev < 0, f"Expected negative elevation at midnight, got {elev:.1f}"

    def test_winter_low(self):
        """Sun should be low at winter solstice noon."""
        dt = datetime(2024, 12, 21, 11, 0, tzinfo=timezone.utc)
        elev = solar_elevation(52.41, 16.93, dt)
        assert 5 < elev < 20, f"Expected ~15 at winter solstice noon, got {elev:.1f}"

    def test_equinox_elevation(self):
        """Sun should be at ~38 (90-52) at equinox noon."""
        dt = datetime(2024, 3, 20, 11, 0, tzinfo=timezone.utc)
        elev = solar_elevation(52.41, 16.93, dt)
        assert 30 < elev < 45, f"Expected ~38 at equinox noon, got {elev:.1f}"

    def test_elevation_range(self):
        """Elevation should always be between -90 and 90."""
        for month in range(1, 13):
            for hour in range(0, 24):
                dt = datetime(2024, month, 15, hour, 0, tzinfo=timezone.utc)
                elev = solar_elevation(52.41, 16.93, dt)
                assert -90 <= elev <= 90


class TestBuildPVFeatures:
    """Test PV feature building."""

    def test_output_columns(self):
        """Should produce all expected feature columns."""
        features = build_pv_features(_make_weather_df(), _make_config())
        expected = [
            "hour_sin", "hour_cos", "month_sin", "month_cos",
            "day_of_year_sin", "day_of_year_cos",
            "direct_radiation", "diffuse_radiation",
            "cloud_cover", "temperature", "wind_speed",
            "solar_elevation", "is_daylight", "clear_sky_index",
        ]
        assert list(features.columns) == expected

    def test_output_length(self):
        """Output should have same length as input."""
        features = build_pv_features(_make_weather_df(n_hours=72), _make_config())
        assert len(features) == 72

    def test_cyclical_features_bounded(self):
        """Sin/cos features should be in [-1, 1]."""
        features = build_pv_features(_make_weather_df(n_hours=365 * 24), _make_config())
        for col in ["hour_sin", "hour_cos", "month_sin", "month_cos",
                     "day_of_year_sin", "day_of_year_cos"]:
            assert features[col].min() >= -1.0 - 1e-10
            assert features[col].max() <= 1.0 + 1e-10

    def test_is_daylight_binary(self):
        """is_daylight should be 0 or 1."""
        features = build_pv_features(_make_weather_df(), _make_config())
        assert set(features["is_daylight"].unique()).issubset({0, 1})

    def test_no_nans(self):
        """Features should not contain NaN values."""
        features = build_pv_features(_make_weather_df(), _make_config())
        assert not features.isna().any().any(), f"NaN columns: {features.columns[features.isna().any()].tolist()}"

    def test_clear_sky_index_non_negative(self):
        """Clear-sky index should be >= 0."""
        features = build_pv_features(_make_weather_df(), _make_config())
        assert (features["clear_sky_index"] >= 0).all()


class TestBuildConsumptionFeatures:
    """Test consumption feature building."""

    def test_output_columns(self):
        """Should produce all expected feature columns."""
        features = build_consumption_features(_make_weather_df(), _make_config())
        expected = [
            "hour_sin", "hour_cos",
            "month_sin", "month_cos",
            "day_of_year_sin", "day_of_year_cos",
            "day_of_week_sin", "day_of_week_cos",
            "is_weekend", "is_holiday",
            "temperature", "wind_speed", "cloud_cover",
        ]
        assert list(features.columns) == expected

    def test_output_length(self):
        features = build_consumption_features(_make_weather_df(n_hours=72), _make_config())
        assert len(features) == 72

    def test_behavioral_features(self):
        """Should include is_weekend and is_holiday."""
        features = build_consumption_features(_make_weather_df(), _make_config())
        assert "is_weekend" in features.columns
        assert "is_holiday" in features.columns
        assert set(features["is_weekend"].unique()).issubset({0, 1})
        assert set(features["is_holiday"].unique()).issubset({0, 1})

    def test_no_nans(self):
        features = build_consumption_features(_make_weather_df(), _make_config())
        assert not features.isna().any().any(), f"NaN columns: {features.columns[features.isna().any()].tolist()}"

    def test_cyclical_features_bounded(self):
        features = build_consumption_features(_make_weather_df(n_hours=365 * 24), _make_config())
        for col in ["hour_sin", "hour_cos", "month_sin", "month_cos",
                     "day_of_week_sin", "day_of_week_cos"]:
            assert features[col].min() >= -1.0 - 1e-10
            assert features[col].max() <= 1.0 + 1e-10


class TestBuildHeatingFeatures:
    """Test heating feature building."""

    def test_output_columns(self):
        features = build_heating_features(_make_weather_df(), _make_config())
        expected = [
            "hour_sin", "hour_cos",
            "month_sin", "month_cos",
            "day_of_year_sin", "day_of_year_cos",
            "temperature", "wind_speed", "cloud_cover",
            "temp_derivative", "is_daylight",
        ]
        assert list(features.columns) == expected

    def test_temp_derivative_computed(self):
        """temp_derivative should be computed from temperature."""
        weather = _make_weather_df(n_hours=48)
        # Set known temperatures
        weather["temperature_2m"] = np.arange(48, dtype=float)
        features = build_heating_features(weather, _make_config())
        # After 3h, derivative should be 3.0 (each hour increases by 1)
        assert features["temp_derivative"].iloc[3] == pytest.approx(3.0)

    def test_is_daylight_binary(self):
        features = build_heating_features(_make_weather_df(), _make_config())
        assert set(features["is_daylight"].unique()).issubset({0, 1})

    def test_no_nans(self):
        features = build_heating_features(_make_weather_df(), _make_config())
        assert not features.isna().any().any(), f"NaN columns: {features.columns[features.isna().any()].tolist()}"

    def test_output_length(self):
        features = build_heating_features(_make_weather_df(n_hours=72), _make_config())
        assert len(features) == 72


class TestBuildDHWFeatures:
    """Test DHW feature building."""

    def test_output_columns(self):
        """Should produce time-only features without weather."""
        timestamps = pd.date_range("2024-07-01", periods=48, freq="h", tz="UTC")
        features = build_dhw_features(timestamps, _make_config())
        expected = [
            "hour_sin", "hour_cos",
            "month_sin", "month_cos",
            "day_of_week_sin", "day_of_week_cos",
            "is_weekend", "is_holiday",
        ]
        assert list(features.columns) == expected

    def test_works_without_weather(self):
        """Should work with just timestamps, no weather DataFrame."""
        timestamps = pd.date_range("2024-07-01", periods=168, freq="h", tz="UTC")
        features = build_dhw_features(timestamps, _make_config())
        assert len(features) == 168
        assert not features.isna().any().any()

    def test_weekend_detection(self):
        """Should correctly identify weekends."""
        # 2024-07-06 is Saturday, 2024-07-07 is Sunday
        timestamps = pd.date_range("2024-07-05", periods=72, freq="h", tz="UTC")
        features = build_dhw_features(timestamps, _make_config())
        # Check Saturday noon (index ~36, depends on local time)
        local_idx = features.index.tz_convert("Europe/Warsaw")
        weekend_mask = local_idx.dayofweek.isin([5, 6])
        assert features["is_weekend"][weekend_mask].all() == 1

    def test_no_nans(self):
        timestamps = pd.date_range("2024-01-01", periods=365 * 24, freq="h", tz="UTC")
        features = build_dhw_features(timestamps, _make_config())
        assert not features.isna().any().any()


class TestBuildSpotPriceFeatures:
    """Test spot price feature building."""

    def test_output_columns(self):
        features = build_spot_price_features(_make_weather_df(), _make_config())
        expected = [
            "hour_sin", "hour_cos",
            "month_sin", "month_cos",
            "day_of_week_sin", "day_of_week_cos",
            "is_weekend", "is_holiday",
            "temperature", "wind_speed",
        ]
        assert list(features.columns) == expected

    def test_behavioral_features(self):
        features = build_spot_price_features(_make_weather_df(), _make_config())
        assert "is_weekend" in features.columns
        assert "is_holiday" in features.columns

    def test_no_cloud_cover(self):
        """Spot price should not include cloud_cover (unlike consumption)."""
        features = build_spot_price_features(_make_weather_df(), _make_config())
        assert "cloud_cover" not in features.columns

    def test_no_nans(self):
        features = build_spot_price_features(_make_weather_df(), _make_config())
        assert not features.isna().any().any()

    def test_output_length(self):
        features = build_spot_price_features(_make_weather_df(n_hours=100), _make_config())
        assert len(features) == 100
