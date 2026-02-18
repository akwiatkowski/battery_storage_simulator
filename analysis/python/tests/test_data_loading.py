"""Tests for data loading module."""

import tempfile
from pathlib import Path

import pandas as pd
import pytest

from analysis.python.src.data_loading import (
    load_legacy_csv,
    load_recent_csv,
    load_stats_csv,
)


@pytest.fixture
def legacy_csv(tmp_path):
    """Create a sample legacy CSV file."""
    content = (
        "entity_id,state,last_changed\n"
        "sensor.pv_power,1234.5,2024-07-15T10:00:00.000Z\n"
        "sensor.pv_power,1500.0,2024-07-15T11:00:00.000Z\n"
        "sensor.pv_power,unavailable,2024-07-15T12:00:00.000Z\n"
        "sensor.other,999.0,2024-07-15T10:00:00.000Z\n"
        "sensor.pv_power,800.0,2024-07-15T13:00:00.000Z\n"
    )
    p = tmp_path / "pv_power.csv"
    p.write_text(content)
    return p


@pytest.fixture
def recent_csv(tmp_path):
    """Create a sample recent CSV file."""
    content = (
        "sensor_id,value,updated_ts\n"
        "sensor.pv_power,500.0,1721037600.0\n"
        "sensor.pv_power,600.0,1721041200.0\n"
        "sensor.other,123.0,1721037600.0\n"
        "sensor.pv_power,700.0,1721044800.0\n"
    )
    p = tmp_path / "2024-W29.csv"
    p.write_text(content)
    return p


@pytest.fixture
def stats_csv(tmp_path):
    """Create a sample stats CSV file."""
    content = (
        "sensor_id,start_time,avg,min_val,max_val\n"
        "sensor.pv_power,1721037600.0,550.0,200.0,900.0\n"
        "sensor.pv_power,1721041200.0,650.0,300.0,1000.0\n"
        "sensor.other,1721037600.0,999.0,999.0,999.0\n"
    )
    p = tmp_path / "stats.csv"
    p.write_text(content)
    return p


class TestLoadLegacyCSV:
    def test_filters_by_entity_id(self, legacy_csv):
        df = load_legacy_csv(legacy_csv, "sensor.pv_power")
        assert len(df) == 3  # 3 numeric rows, excluding 'unavailable' and 'other'

    def test_drops_non_numeric(self, legacy_csv):
        df = load_legacy_csv(legacy_csv, "sensor.pv_power")
        assert df["value"].notna().all()

    def test_timestamps_are_tz_aware(self, legacy_csv):
        df = load_legacy_csv(legacy_csv, "sensor.pv_power")
        assert df["timestamp"].dt.tz is not None
        assert str(df["timestamp"].dt.tz) == "Europe/Warsaw"

    def test_sorted_by_timestamp(self, legacy_csv):
        df = load_legacy_csv(legacy_csv, "sensor.pv_power")
        assert (df["timestamp"].diff().dropna() >= pd.Timedelta(0)).all()

    def test_columns(self, legacy_csv):
        df = load_legacy_csv(legacy_csv, "sensor.pv_power")
        assert list(df.columns) == ["timestamp", "value"]

    def test_empty_for_unknown_sensor(self, legacy_csv):
        df = load_legacy_csv(legacy_csv, "sensor.nonexistent")
        assert df.empty


class TestLoadRecentCSV:
    def test_filters_by_sensor_id(self, recent_csv):
        df = load_recent_csv([recent_csv], "sensor.pv_power")
        assert len(df) == 3

    def test_multiple_files(self, tmp_path):
        for name, val in [("w1.csv", "100.0"), ("w2.csv", "200.0")]:
            (tmp_path / name).write_text(
                f"sensor_id,value,updated_ts\nsensor.x,{val},1721037600.0\n"
            )
        df = load_recent_csv(
            [tmp_path / "w1.csv", tmp_path / "w2.csv"], "sensor.x"
        )
        # Same timestamp → deduplicated to 1 row
        assert len(df) == 1

    def test_timestamps_are_tz_aware(self, recent_csv):
        df = load_recent_csv([recent_csv], "sensor.pv_power")
        assert str(df["timestamp"].dt.tz) == "Europe/Warsaw"

    def test_empty_for_no_files(self):
        df = load_recent_csv([], "sensor.pv_power")
        assert df.empty


class TestLoadStatsCSV:
    def test_filters_by_sensor_id(self, stats_csv):
        df = load_stats_csv(stats_csv, "sensor.pv_power")
        assert len(df) == 2

    def test_has_expected_columns(self, stats_csv):
        df = load_stats_csv(stats_csv, "sensor.pv_power")
        assert list(df.columns) == ["timestamp", "avg", "min_val", "max_val"]

    def test_timestamps_are_tz_aware(self, stats_csv):
        df = load_stats_csv(stats_csv, "sensor.pv_power")
        assert str(df["timestamp"].dt.tz) == "Europe/Warsaw"
