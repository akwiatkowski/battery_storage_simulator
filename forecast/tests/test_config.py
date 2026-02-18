"""Tests for config loading."""

from pathlib import Path

from forecast.src.config import load_config, project_root, python_root


class TestLoadConfig:
    def test_loads_successfully(self):
        cfg = load_config()
        assert "location" in cfg
        assert "pv_system" in cfg
        assert "sensors" in cfg
        assert "models" in cfg

    def test_location_fields(self):
        cfg = load_config()
        loc = cfg["location"]
        assert 50 < loc["latitude"] < 55
        assert 15 < loc["longitude"] < 20
        assert loc["timezone"] == "Europe/Warsaw"

    def test_pv_system_fields(self):
        cfg = load_config()
        pv = cfg["pv_system"]
        assert pv["capacity_kwp"] == 6.5
        assert pv["azimuth"] == 90
        assert pv["tilt"] == 40

    def test_model_defaults_merged(self):
        """Per-model configs should inherit default_params."""
        cfg = load_config()
        pv = cfg["models"]["pv"]
        assert pv["type"] == "lightgbm"
        assert pv["test_days"] == 30
        assert "params" in pv
        assert pv["params"]["n_estimators"] == 500


class TestPaths:
    def test_project_root_has_makefile(self):
        root = project_root()
        assert (root / "Makefile").exists()

    def test_python_root_has_config(self):
        pr = python_root()
        assert (pr / "config.yaml").exists()
