"""Load Home Assistant sensor data from the three CSV formats."""

from pathlib import Path
from typing import Literal

import pandas as pd


def load_legacy_csv(path: Path, entity_id: str) -> pd.DataFrame:
    """Load legacy per-sensor CSV (entity_id,state,last_changed).

    Returns DataFrame with columns: timestamp (tz-aware), value (float).
    """
    df = pd.read_csv(path, dtype={"entity_id": str, "state": str, "last_changed": str})
    df = df[df["entity_id"] == entity_id].copy()
    df["value"] = pd.to_numeric(df["state"], errors="coerce")
    df = df.dropna(subset=["value"])
    df["timestamp"] = pd.to_datetime(df["last_changed"], utc=True).dt.tz_convert(
        "Europe/Warsaw"
    )
    return df[["timestamp", "value"]].sort_values("timestamp").reset_index(drop=True)


def load_recent_csv(paths: list[Path], sensor_id: str) -> pd.DataFrame:
    """Load recent multi-sensor CSVs (sensor_id,value,updated_ts).

    Accepts multiple weekly/daily CSV paths, concatenates and deduplicates.
    Returns DataFrame with columns: timestamp (tz-aware), value (float).
    """
    frames = []
    for p in paths:
        df = pd.read_csv(p, dtype={"sensor_id": str, "value": str, "updated_ts": float})
        df = df[df["sensor_id"] == sensor_id].copy()
        df["value"] = pd.to_numeric(df["value"], errors="coerce")
        df = df.dropna(subset=["value"])
        df["timestamp"] = pd.to_datetime(df["updated_ts"], unit="s", utc=True).dt.tz_convert(
            "Europe/Warsaw"
        )
        frames.append(df[["timestamp", "value"]])

    if not frames:
        return pd.DataFrame(columns=["timestamp", "value"])

    result = pd.concat(frames, ignore_index=True)
    result = result.drop_duplicates(subset=["timestamp"]).sort_values("timestamp")
    return result.reset_index(drop=True)


def load_stats_csv(path: Path, sensor_id: str) -> pd.DataFrame:
    """Load statistics CSV (sensor_id,start_time,avg,min_val,max_val).

    Returns DataFrame with columns: timestamp (tz-aware), avg, min_val, max_val.
    """
    df = pd.read_csv(path, dtype={"sensor_id": str})
    df = df[df["sensor_id"] == sensor_id].copy()
    df["timestamp"] = pd.to_datetime(df["start_time"], unit="s", utc=True).dt.tz_convert(
        "Europe/Warsaw"
    )
    for col in ("avg", "min_val", "max_val"):
        df[col] = pd.to_numeric(df[col], errors="coerce")
    return (
        df[["timestamp", "avg", "min_val", "max_val"]]
        .dropna(subset=["avg"])
        .sort_values("timestamp")
        .reset_index(drop=True)
    )


def load_sensor_data(
    sensor_id: str,
    legacy_path: Path | None = None,
    recent_dir: Path | None = None,
    stats_path: Path | None = None,
) -> pd.DataFrame:
    """Load sensor data from all available sources, merge and deduplicate.

    Returns DataFrame with columns: timestamp (tz-aware), value (float).
    Prefers: recent > legacy > stats (by resolution).
    """
    frames = []

    if legacy_path and legacy_path.exists():
        df = load_legacy_csv(legacy_path, sensor_id)
        if not df.empty:
            frames.append(df)

    if recent_dir and recent_dir.exists():
        csv_files = sorted(recent_dir.glob("*.csv"))
        # Exclude historic_spot_prices.csv — it's a special file
        csv_files = [f for f in csv_files if "historic" not in f.name]
        if csv_files:
            df = load_recent_csv(csv_files, sensor_id)
            if not df.empty:
                frames.append(df)

    if stats_path and stats_path.exists():
        df = load_stats_csv(stats_path, sensor_id)
        if not df.empty:
            # Convert stats avg to match value column name
            df = df.rename(columns={"avg": "value"})[["timestamp", "value"]]
            frames.append(df)

    if not frames:
        return pd.DataFrame(columns=["timestamp", "value"])

    result = pd.concat(frames, ignore_index=True)
    # Deduplicate: keep first occurrence (recent data tends to be more accurate)
    result = result.sort_values("timestamp").drop_duplicates(subset=["timestamp"], keep="first")
    return result.reset_index(drop=True)


def load_spot_prices(path: Path) -> pd.DataFrame:
    """Load historic spot prices CSV (recent format: sensor_id,value,updated_ts).

    Returns DataFrame with columns: timestamp (UTC-aware), value (float, PLN/kWh).
    Already hourly data — one row per hour from 2017 onwards.
    """
    df = pd.read_csv(path, dtype={"sensor_id": str, "value": str, "updated_ts": float})
    df["value"] = pd.to_numeric(df["value"], errors="coerce")
    df = df.dropna(subset=["value"])
    df["timestamp"] = pd.to_datetime(df["updated_ts"], unit="s", utc=True)
    return df[["timestamp", "value"]].sort_values("timestamp").reset_index(drop=True)
