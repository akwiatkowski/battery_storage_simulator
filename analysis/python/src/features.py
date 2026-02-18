"""Feature engineering: cyclical encoding, solar position, clear-sky index."""

import math
from datetime import datetime, date

import numpy as np
import pandas as pd

from .config import load_config, project_root
from .data_loading import load_sensor_data
from .weather import fetch_historical


def solar_elevation(lat: float, lon: float, dt: datetime) -> float:
    """Compute solar elevation angle in degrees.

    Uses simplified astronomical formula — accurate to ~1° for energy modeling.
    """
    # Day of year and fractional hour (UTC)
    utc_dt = dt.astimezone(tz=None) if dt.tzinfo else dt
    doy = utc_dt.timetuple().tm_yday
    hour_utc = utc_dt.hour + utc_dt.minute / 60.0

    # Solar declination (Spencer, 1971)
    gamma = 2 * math.pi * (doy - 1) / 365.0
    decl = (
        0.006918
        - 0.399912 * math.cos(gamma)
        + 0.070257 * math.sin(gamma)
        - 0.006758 * math.cos(2 * gamma)
        + 0.000907 * math.sin(2 * gamma)
        - 0.002697 * math.cos(3 * gamma)
        + 0.00148 * math.sin(3 * gamma)
    )

    # Equation of time (minutes)
    eqtime = 229.18 * (
        0.000075
        + 0.001868 * math.cos(gamma)
        - 0.032077 * math.sin(gamma)
        - 0.014615 * math.cos(2 * gamma)
        - 0.04089 * math.sin(2 * gamma)
    )

    # Solar hour angle
    time_offset = eqtime + 4 * lon  # minutes
    tst = hour_utc * 60 + time_offset  # true solar time in minutes
    hour_angle = math.radians((tst / 4.0) - 180.0)

    # Solar elevation
    lat_rad = math.radians(lat)
    sin_elev = (
        math.sin(lat_rad) * math.sin(decl)
        + math.cos(lat_rad) * math.cos(decl) * math.cos(hour_angle)
    )
    return math.degrees(math.asin(max(-1.0, min(1.0, sin_elev))))


def _cyclical_encode(series: pd.Series, period: float) -> tuple[pd.Series, pd.Series]:
    """Encode a numeric series as sin/cos with given period."""
    angle = 2 * np.pi * series / period
    return np.sin(angle), np.cos(angle)


def build_pv_features(weather_df: pd.DataFrame, config: dict) -> pd.DataFrame:
    """Build feature DataFrame for PV model from weather data.

    Input weather_df must have a UTC-aware 'timestamp' column.
    Returns DataFrame indexed by timestamp with all PV features.
    """
    loc = config["location"]
    df = weather_df.copy()
    df = df.set_index("timestamp").sort_index()

    # Convert to local time for cyclical features
    local_idx = df.index.tz_convert(loc["timezone"])

    # Cyclical time features
    df["hour_sin"], df["hour_cos"] = _cyclical_encode(
        pd.Series(local_idx.hour + local_idx.minute / 60.0, index=df.index), 24.0
    )
    df["month_sin"], df["month_cos"] = _cyclical_encode(
        pd.Series(local_idx.month, index=df.index), 12.0
    )
    df["day_of_year_sin"], df["day_of_year_cos"] = _cyclical_encode(
        pd.Series(local_idx.dayofyear, index=df.index), 365.25
    )

    # Weather features (already present, just rename for clarity)
    df["direct_radiation"] = df["direct_radiation"].fillna(0)
    df["diffuse_radiation"] = df["diffuse_radiation"].fillna(0)
    df["cloud_cover"] = df["cloud_cover"].fillna(50)
    df["temperature"] = df["temperature_2m"].ffill()
    df["wind_speed"] = df["wind_speed_10m"].fillna(0)

    # Solar elevation
    df["solar_elevation"] = [
        solar_elevation(loc["latitude"], loc["longitude"], ts) for ts in df.index
    ]
    df["is_daylight"] = (df["solar_elevation"] > 0).astype(int)

    # Global horizontal irradiance (approximate)
    ghi = df["direct_radiation"] + df["diffuse_radiation"]

    # Clear-sky GHI estimate (simplified: extraterrestrial × atmospheric transmission)
    solar_elev_rad = np.radians(df["solar_elevation"].clip(lower=0))
    # Extraterrestrial irradiance on horizontal surface
    ext_ghi = 1361.0 * np.sin(solar_elev_rad)  # W/m²
    # Simple clear-sky model: ~75% transmission at sea level
    clear_sky_ghi = ext_ghi * 0.75
    # Clear-sky index: ratio of actual to theoretical
    df["clear_sky_index"] = np.where(
        clear_sky_ghi > 50, (ghi / clear_sky_ghi).clip(0, 1.5), 0.0
    )

    feature_cols = [
        "hour_sin", "hour_cos",
        "month_sin", "month_cos",
        "day_of_year_sin", "day_of_year_cos",
        "direct_radiation", "diffuse_radiation",
        "cloud_cover", "temperature", "wind_speed",
        "solar_elevation", "is_daylight", "clear_sky_index",
    ]
    return df[feature_cols]


def prepare_pv_dataset(config: dict) -> tuple[pd.DataFrame, pd.Series, pd.DatetimeIndex]:
    """Prepare full PV training dataset: load sensor data, weather, build features.

    Returns (X, y, timestamps) where:
    - X: feature DataFrame
    - y: target Series (W per kWp)
    - timestamps: DatetimeIndex for temporal splitting
    """
    root = project_root()
    sensor_id = config["sensors"]["pv_power"]
    capacity = config["pv_system"]["capacity_kwp"]

    # Load PV sensor data from all sources
    pv_df = load_sensor_data(
        sensor_id=sensor_id,
        legacy_path=root / "input" / "pv_power.csv",
        recent_dir=root / "input" / "recent",
        stats_path=None,  # Stats format has avg/min/max, not point values
    )
    if pv_df.empty:
        raise ValueError(f"No PV data found for sensor {sensor_id}")

    print(f"  PV readings: {len(pv_df)} ({pv_df['timestamp'].min()} to {pv_df['timestamp'].max()})")

    # Convert to UTC for resampling
    pv_df = pv_df.set_index("timestamp")
    pv_df.index = pv_df.index.tz_convert("UTC")

    # Resample to hourly means
    pv_hourly = pv_df["value"].resample("h").mean().dropna()
    print(f"  Hourly PV samples: {len(pv_hourly)}")

    # Determine date range for weather
    start_date = pv_hourly.index.min().date()
    end_date = pv_hourly.index.max().date()

    # Fetch/load weather data
    cache_dir = root / "analysis" / "python" / "data" / "weather"
    loc = config["location"]
    print(f"  Loading weather data: {start_date} to {end_date}")
    weather_df = fetch_historical(loc["latitude"], loc["longitude"], start_date, end_date, cache_dir)

    if weather_df.empty:
        raise ValueError("No weather data available")

    # Build features from weather
    features = build_pv_features(weather_df, config)

    # Join PV hourly data with features on hourly timestamp
    # Both are UTC-indexed
    pv_per_kwp = pv_hourly / capacity
    pv_per_kwp.name = "pv_per_kwp"

    # Align: inner join on common hourly timestamps
    combined = features.join(pv_per_kwp, how="inner")
    combined = combined.dropna(subset=["pv_per_kwp"])

    # Drop nighttime hours (no signal to learn)
    combined = combined[combined["solar_elevation"] > 0]

    # Clip negative PV values (sensor noise)
    combined["pv_per_kwp"] = combined["pv_per_kwp"].clip(lower=0)

    X = combined.drop(columns=["pv_per_kwp"])
    y = combined["pv_per_kwp"]
    timestamps = combined.index

    print(f"  Training samples (daytime): {len(X)}")
    print(f"  Date range: {timestamps.min()} to {timestamps.max()}")

    return X, y, timestamps
