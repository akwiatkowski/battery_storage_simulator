"""Tests for weather module (parsing only, no API calls)."""

import pandas as pd

from forecast.src.weather import _parse_hourly_response


class TestParseHourlyResponse:
    def test_parses_valid_response(self):
        data = {
            "hourly": {
                "time": ["2024-07-01T00:00", "2024-07-01T01:00", "2024-07-01T02:00"],
                "temperature_2m": [18.5, 17.8, 17.2],
                "cloud_cover": [20, 30, 50],
                "direct_radiation": [0, 0, 0],
                "diffuse_radiation": [0, 0, 0],
                "sunshine_duration": [0, 0, 0],
                "wind_speed_10m": [3.5, 3.2, 2.8],
                "precipitation": [0, 0, 0],
                "relative_humidity_2m": [75, 78, 80],
            }
        }
        df = _parse_hourly_response(data)
        assert len(df) == 3
        assert "timestamp" in df.columns
        assert "temperature_2m" in df.columns
        assert "cloud_cover" in df.columns
        assert "direct_radiation" in df.columns
        assert df["temperature_2m"].iloc[0] == 18.5

    def test_handles_missing_fields(self):
        """Fields not in response should be None."""
        data = {
            "hourly": {
                "time": ["2024-07-01T00:00"],
                "temperature_2m": [18.5],
                # All other fields missing
            }
        }
        df = _parse_hourly_response(data)
        assert len(df) == 1
        assert df["temperature_2m"].iloc[0] == 18.5
        assert df["cloud_cover"].iloc[0] is None

    def test_timestamps_are_utc(self):
        data = {
            "hourly": {
                "time": ["2024-07-01T12:00"],
                "temperature_2m": [25.0],
            }
        }
        df = _parse_hourly_response(data)
        assert df["timestamp"].dt.tz is not None
