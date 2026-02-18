"""Load and provide typed access to config.yaml."""

from pathlib import Path

import yaml


def load_config(path: Path | None = None) -> dict:
    """Load config.yaml and return as dict with defaults merged."""
    if path is None:
        path = Path(__file__).parent.parent / "config.yaml"
    with open(path) as f:
        cfg = yaml.safe_load(f)

    # Merge per-model configs with default_params
    defaults = cfg["models"]["default_params"]
    for model_name in ("pv", "consumption", "heat_pump", "dhw", "spot_price"):
        model_cfg = cfg["models"].get(model_name, {})
        merged = {**defaults, **model_cfg}
        # Deep-merge params dict
        if "params" in defaults and "params" in model_cfg:
            merged["params"] = {**defaults["params"], **model_cfg["params"]}
        elif "params" in defaults:
            merged["params"] = dict(defaults["params"])
        cfg["models"][model_name] = merged

    return cfg


def project_root() -> Path:
    """Return the energy_simulator project root directory."""
    return Path(__file__).parent.parent.parent.parent


def python_root() -> Path:
    """Return the analysis/python directory."""
    return Path(__file__).parent.parent
