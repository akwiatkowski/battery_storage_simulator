"""Feature engineering: cyclical encoding, solar position, clear-sky index."""

import math
from datetime import datetime, date

import numpy as np
import pandas as pd

from .config import load_config, project_root
from .data_loading import load_sensor_data, load_spot_prices
from .holidays import is_holiday
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


# ---------------------------------------------------------------------------
# Shared feature helpers
# ---------------------------------------------------------------------------

def _add_time_features(df: pd.DataFrame, config: dict) -> None:
    """Add hour/month/day_of_year cyclical features (in-place).

    Reused by all models. Requires df to be indexed by UTC timestamps.
    """
    local_idx = df.index.tz_convert(config["location"]["timezone"])
    df["hour_sin"], df["hour_cos"] = _cyclical_encode(
        pd.Series(local_idx.hour + local_idx.minute / 60.0, index=df.index), 24.0
    )
    df["month_sin"], df["month_cos"] = _cyclical_encode(
        pd.Series(local_idx.month, index=df.index), 12.0
    )
    df["day_of_year_sin"], df["day_of_year_cos"] = _cyclical_encode(
        pd.Series(local_idx.dayofyear, index=df.index), 365.25
    )


def _add_behavioral_time_features(df: pd.DataFrame, config: dict) -> None:
    """Add day_of_week sin/cos, is_weekend, is_holiday (in-place).

    For models where human behavior matters (consumption, DHW, spot price).
    """
    local_idx = df.index.tz_convert(config["location"]["timezone"])
    df["day_of_week_sin"], df["day_of_week_cos"] = _cyclical_encode(
        pd.Series(local_idx.dayofweek, index=df.index), 7.0
    )
    df["is_weekend"] = pd.Series(local_idx.dayofweek, index=df.index).isin([5, 6]).astype(int)
    df["is_holiday"] = pd.Series(
        [is_holiday(d.date()) for d in local_idx], index=df.index
    ).astype(int)


def _add_weather_features(df: pd.DataFrame) -> None:
    """Add temperature, wind_speed, cloud_cover, humidity from weather columns (in-place).

    Expects raw weather column names (temperature_2m, wind_speed_10m, cloud_cover,
    relative_humidity_2m).
    """
    df["temperature"] = df["temperature_2m"].ffill()
    df["wind_speed"] = df["wind_speed_10m"].fillna(0)
    df["cloud_cover"] = df["cloud_cover"].fillna(50)
    df["humidity"] = df["relative_humidity_2m"].fillna(50)


def _load_weather(config: dict, start_date: date, end_date: date) -> pd.DataFrame:
    """Load historical weather data for a date range."""
    root = project_root()
    cache_dir = root / "analysis" / "python" / "data" / "weather"
    loc = config["location"]
    print(f"  Loading weather data: {start_date} to {end_date}")
    weather_df = fetch_historical(loc["latitude"], loc["longitude"], start_date, end_date, cache_dir)
    if weather_df.empty:
        raise ValueError("No weather data available")
    return weather_df


# ---------------------------------------------------------------------------
# PV model
# ---------------------------------------------------------------------------

def build_pv_features(weather_df: pd.DataFrame, config: dict) -> pd.DataFrame:
    """Build feature DataFrame for PV model from weather data.

    Input weather_df must have a UTC-aware 'timestamp' column.
    Returns DataFrame indexed by timestamp with all PV features.
    """
    loc = config["location"]
    df = weather_df.copy()
    df = df.set_index("timestamp").sort_index()

    _add_time_features(df, config)
    _add_weather_features(df)

    # PV-specific: radiation and solar features
    df["direct_radiation"] = df["direct_radiation"].fillna(0)
    df["diffuse_radiation"] = df["diffuse_radiation"].fillna(0)

    # Solar elevation
    df["solar_elevation"] = [
        solar_elevation(loc["latitude"], loc["longitude"], ts) for ts in df.index
    ]
    df["is_daylight"] = (df["solar_elevation"] > 0).astype(int)

    # Global horizontal irradiance (approximate)
    ghi = df["direct_radiation"] + df["diffuse_radiation"]

    # Clear-sky GHI estimate (simplified: extraterrestrial x atmospheric transmission)
    solar_elev_rad = np.radians(df["solar_elevation"].clip(lower=0))
    ext_ghi = 1361.0 * np.sin(solar_elev_rad)  # W/m2
    clear_sky_ghi = ext_ghi * 0.75
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

    Supports optional rolling window smoothing on target via
    config["models"]["pv"]["smoothing_window_h"].

    Returns (X, y, timestamps) where:
    - X: feature DataFrame
    - y: target Series (W per kWp)
    - timestamps: DatetimeIndex for temporal splitting
    """
    root = project_root()
    sensor_id = config["sensors"]["pv_power"]
    capacity = config["pv_system"]["capacity_kwp"]
    model_cfg = config["models"].get("pv", {})
    smoothing_window = model_cfg.get("smoothing_window_h", 1)

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
    weather_df = _load_weather(config, start_date, end_date)

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

    # Apply rolling window smoothing to target
    if smoothing_window > 1:
        combined["pv_per_kwp"] = combined["pv_per_kwp"].rolling(
            smoothing_window, center=True, min_periods=1).mean()
        print(f"  Smoothing window: {smoothing_window}h")

    X = combined.drop(columns=["pv_per_kwp"])
    y = combined["pv_per_kwp"]
    timestamps = combined.index

    print(f"  Training samples (daytime): {len(X)}")
    print(f"  Date range: {timestamps.min()} to {timestamps.max()}")

    return X, y, timestamps


# ---------------------------------------------------------------------------
# Base Consumption model
# ---------------------------------------------------------------------------

def build_consumption_features(weather_df: pd.DataFrame, config: dict) -> pd.DataFrame:
    """Build features for base consumption model.

    Features: time cyclicals, behavioral (day_of_week, weekend, holiday),
    weather (temperature, wind_speed, cloud_cover), solar radiation.
    """
    df = weather_df.copy()
    df = df.set_index("timestamp").sort_index()

    _add_time_features(df, config)
    _add_behavioral_time_features(df, config)
    _add_weather_features(df)

    # Solar radiation: affects lighting usage and occupant behavior
    df["solar_radiation"] = (
        df["direct_radiation"].fillna(0) + df["diffuse_radiation"].fillna(0)
    )

    feature_cols = [
        "hour_sin", "hour_cos",
        "month_sin", "month_cos",
        "day_of_year_sin", "day_of_year_cos",
        "day_of_week_sin", "day_of_week_cos",
        "is_weekend", "is_holiday",
        "temperature", "wind_speed", "cloud_cover",
        "humidity", "solar_radiation",
    ]
    return df[feature_cols]


def prepare_consumption_dataset(config: dict) -> tuple[pd.DataFrame, pd.Series, pd.DatetimeIndex]:
    """Prepare base consumption dataset.

    base_load = grid_power + pv_power - hp_heat - hp_dhw (household excl. HP).
    Supports configurable resolution via config["models"]["consumption"]["resolution"].
    """
    root = project_root()
    sensors = config["sensors"]
    model_cfg = config["models"].get("consumption", {})
    resolution = model_cfg.get("resolution", "1h")

    # Load 4 sensors
    sensor_defs = {
        "grid": (sensors["grid_power"], "grid_power.csv"),
        "pv": (sensors["pv_power"], "pv_power.csv"),
        "hp_heat": (sensors["hp_heating"], "pump_heat_power_consumed.csv"),
        "hp_dhw": (sensors["hp_dhw"], "pump_cwu_power_consumed.csv"),
    }

    hourly = {}
    for name, (sensor_id, legacy_file) in sensor_defs.items():
        df = load_sensor_data(
            sensor_id=sensor_id,
            legacy_path=root / "input" / legacy_file,
            recent_dir=root / "input" / "recent",
        )
        if df.empty and name in ("grid", "pv"):
            raise ValueError(f"No data found for required sensor {sensor_id}")
        if df.empty:
            print(f"  Warning: no data for {name} ({sensor_id}), will fill with 0")
            continue

        print(f"  {name} readings: {len(df)}")
        df = df.set_index("timestamp")
        df.index = df.index.tz_convert("UTC")
        hourly[name] = df["value"].resample("h").mean()

    # Inner-join grid + pv (required), left-join HP sensors
    combined = pd.DataFrame({"grid": hourly["grid"], "pv": hourly["pv"]}).dropna()
    for name in ("hp_heat", "hp_dhw"):
        if name in hourly:
            combined[name] = hourly[name].reindex(combined.index).fillna(0)
        else:
            combined[name] = 0.0

    # Compute base load
    combined["base_load"] = (
        combined["grid"] + combined["pv"] - combined["hp_heat"] - combined["hp_dhw"]
    ).clip(lower=0)

    print(f"  Hourly consumption samples: {len(combined)}")
    print(f"  Resolution: {resolution}")

    # Load weather
    start_date = combined.index.min().date()
    end_date = combined.index.max().date()
    weather_df = _load_weather(config, start_date, end_date)

    features = build_consumption_features(weather_df, config)

    # Join at hourly resolution
    target = combined["base_load"]
    target.name = "base_load"
    joined = features.join(target, how="inner").dropna(subset=["base_load"])

    # Aggregate to desired resolution
    if resolution != "1h":
        joined = joined.resample(resolution).mean()

        # Drop features that lose meaning at coarser resolutions
        drop_cols = []
        if resolution in ("6h", "12h", "24h", "D"):
            drop_cols += ["hour_sin", "hour_cos"]
        if resolution in ("24h", "D"):
            drop_cols += ["month_sin", "month_cos"]
        if drop_cols:
            joined = joined.drop(columns=list(set(drop_cols)), errors="ignore")

        joined = joined.dropna()
        print(f"  Samples at {resolution}: {len(joined)}")

    # Lagged target features (capture consumption momentum/patterns)
    # Use shift(1) to avoid target leakage — only past data
    periods_per_day = {"1h": 24, "6h": 4, "12h": 2, "24h": 1, "D": 1}
    ppd = periods_per_day.get(resolution, 24)
    shifted_load = joined["base_load"].shift(1)
    joined["load_lag_1"] = shifted_load                                        # previous period
    joined["load_lag_1d"] = joined["base_load"].shift(ppd)                     # same period yesterday
    joined["load_rolling_6h"] = shifted_load.rolling(
        max(ppd // 4, 1), min_periods=1).mean()                                # short-term rolling avg (past only)
    joined["load_rolling_1d"] = shifted_load.rolling(
        ppd, min_periods=1).mean()                                              # 24h rolling avg (past only)
    joined = joined.dropna()

    X = joined.drop(columns=["base_load"])
    y = joined["base_load"]
    timestamps = joined.index

    print(f"  Training samples: {len(X)}")
    print(f"  Date range: {timestamps.min()} to {timestamps.max()}")

    return X, y, timestamps


# ---------------------------------------------------------------------------
# Heat Pump Heating model
# ---------------------------------------------------------------------------

def build_heating_features(weather_df: pd.DataFrame, config: dict) -> pd.DataFrame:
    """Build features for heat pump heating model.

    Features: time cyclicals, weather, temp_derivative, is_daylight.
    """
    loc = config["location"]
    df = weather_df.copy()
    df = df.set_index("timestamp").sort_index()

    _add_time_features(df, config)
    _add_weather_features(df)

    # Temperature derivative: change over previous 3 hours
    df["temp_derivative"] = df["temperature"].diff(3).fillna(0)

    # Wind chill interaction: wind amplifies heat loss from building
    df["wind_chill"] = df["temperature"] * df["wind_speed"]

    # Heating degree hour: standard proxy for heating demand
    df["heating_degree_hour"] = (18.0 - df["temperature"]).clip(lower=0)

    # Squared heating degree: nonlinear heat loss at extreme cold
    df["heating_degree_sq"] = df["heating_degree_hour"] ** 2

    # Wind-driven heat loss: combines cold severity with wind (always positive)
    df["wind_heat_loss"] = df["heating_degree_hour"] * df["wind_speed"]

    # Longer temperature history
    df["temp_lag_6h"] = df["temperature"].shift(6).ffill().bfill()
    df["temp_lag_12h"] = df["temperature"].shift(12).ffill().bfill()

    # Temperature change over 24h (cold front detection)
    df["temp_change_24h"] = df["temperature"].diff(24).fillna(0)

    # Precipitation: frontal weather indicator
    df["precipitation"] = df["precipitation"].fillna(0)

    # Solar radiation: passive solar gains reduce heating demand
    df["solar_radiation"] = (
        df["direct_radiation"].fillna(0) + df["diffuse_radiation"].fillna(0)
    )

    # Solar elevation for is_daylight
    df["is_daylight"] = pd.Series(
        [1 if solar_elevation(loc["latitude"], loc["longitude"], ts) > 0 else 0
         for ts in df.index],
        index=df.index,
    )

    feature_cols = [
        "hour_sin", "hour_cos",
        "month_sin", "month_cos",
        "day_of_year_sin", "day_of_year_cos",
        "temperature", "wind_speed", "cloud_cover",
        "humidity", "wind_chill", "heating_degree_hour", "heating_degree_sq",
        "wind_heat_loss",
        "temp_derivative", "temp_lag_6h", "temp_lag_12h", "temp_change_24h",
        "precipitation",
        "solar_radiation", "is_daylight",
    ]
    return df[feature_cols]


def prepare_heat_pump_dataset(config: dict) -> tuple[pd.DataFrame, pd.Series, pd.DatetimeIndex]:
    """Prepare heat pump heating dataset (weather-only, no lag features).

    Supports configurable resolution via config["models"]["heat_pump"]["resolution"].
    Values: "1h" (default), "6h", "12h", "24h". Coarser resolutions smooth out
    HP cycling noise and focus on underlying heating demand.
    """
    root = project_root()
    sensor_id = config["sensors"]["hp_heating"]
    model_cfg = config["models"].get("heat_pump", {})
    resolution = model_cfg.get("resolution", "1h")

    hp_df = load_sensor_data(
        sensor_id=sensor_id,
        legacy_path=root / "input" / "pump_heat_power_consumed.csv",
        recent_dir=root / "input" / "recent",
    )
    if hp_df.empty:
        raise ValueError(f"No HP heating data found for sensor {sensor_id}")

    print(f"  HP heating readings: {len(hp_df)}")
    print(f"  Resolution: {resolution}")

    hp_df = hp_df.set_index("timestamp")
    hp_df.index = hp_df.index.tz_convert("UTC")

    # Resample to hourly, fill NaN with 0 (HP off = no heating)
    hp_hourly = hp_df["value"].resample("h").mean().fillna(0).clip(lower=0)
    print(f"  Hourly HP samples: {len(hp_hourly)}")

    # Load weather
    start_date = hp_hourly.index.min().date()
    end_date = hp_hourly.index.max().date()
    weather_df = _load_weather(config, start_date, end_date)

    features = build_heating_features(weather_df, config)

    # Join at hourly resolution
    target = hp_hourly
    target.name = "hp_heat_w"
    joined = features.join(target, how="inner")
    joined["hp_heat_w"] = joined["hp_heat_w"].fillna(0)

    # Aggregate to desired resolution
    if resolution != "1h":
        # Save temperature for min aggregation (min temp drives peak demand)
        temp_hourly = joined["temperature"]

        joined = joined.resample(resolution).mean()
        joined["temp_min"] = temp_hourly.resample(resolution).min()

        # Drop features that lose meaning at coarser resolutions
        drop_cols = []
        if resolution in ("24h", "D"):
            drop_cols += ["hour_sin", "hour_cos", "is_daylight",
                          "temp_derivative", "temp_lag_6h", "temp_lag_12h",
                          "temp_change_24h", "month_sin", "month_cos"]
        if resolution in ("6h", "12h", "24h", "D"):
            drop_cols += ["hour_sin", "hour_cos", "is_daylight",
                          "month_sin", "month_cos"]
        if drop_cols:
            joined = joined.drop(columns=list(set(drop_cols)), errors="ignore")

        joined = joined.dropna()
        print(f"  Samples at {resolution}: {len(joined)}")

    # Filter to heating conditions only (removes summer with HP=0)
    heating_threshold = model_cfg.get("heating_threshold_c")
    if heating_threshold is not None:
        before = len(joined)
        joined = joined[joined["temperature"] <= heating_threshold]
        print(f"  Filtered to temp <= {heating_threshold}°C: {len(joined)} (dropped {before - len(joined)} warm samples)")

    X = joined.drop(columns=["hp_heat_w"])
    y = joined["hp_heat_w"]
    timestamps = joined.index

    print(f"  Training samples: {len(X)}")
    print(f"  Date range: {timestamps.min()} to {timestamps.max()}")

    return X, y, timestamps



# ---------------------------------------------------------------------------
# DHW (hot water) model
# ---------------------------------------------------------------------------

def build_dhw_features(timestamps_index: pd.DatetimeIndex, config: dict,
                       weather_df: pd.DataFrame | None = None) -> pd.DataFrame:
    """Build features for DHW model.

    Features: hour/month cyclicals, day_of_week, is_weekend, is_holiday,
    plus temperature (affects cold water inlet temp → energy needed).
    """
    df = pd.DataFrame(index=timestamps_index)

    _add_time_features(df, config)
    _add_behavioral_time_features(df, config)

    feature_cols = [
        "hour_sin", "hour_cos",
        "month_sin", "month_cos",
        "day_of_week_sin", "day_of_week_cos",
        "is_weekend", "is_holiday",
    ]

    if weather_df is not None:
        wdf = weather_df.copy().set_index("timestamp").sort_index()
        df["temperature"] = wdf["temperature_2m"].reindex(df.index, method="nearest")
        feature_cols.append("temperature")

    return df[feature_cols]


def prepare_dhw_dataset(config: dict) -> tuple[pd.DataFrame, pd.Series, pd.DatetimeIndex]:
    """Prepare DHW (hot water) dataset.

    Uses rolling window smoothing on target to predict daily-scale DHW demand
    while keeping all hourly samples for training. No lag features needed.
    """
    root = project_root()
    sensor_id = config["sensors"]["hp_dhw"]
    model_cfg = config["models"].get("dhw", {})
    smoothing_window = model_cfg.get("smoothing_window_h", 24)

    dhw_df = load_sensor_data(
        sensor_id=sensor_id,
        legacy_path=root / "input" / "pump_cwu_power_consumed.csv",
        recent_dir=root / "input" / "recent",
    )
    if dhw_df.empty:
        raise ValueError(f"No DHW data found for sensor {sensor_id}")

    print(f"  DHW readings: {len(dhw_df)}")

    dhw_df = dhw_df.set_index("timestamp")
    dhw_df.index = dhw_df.index.tz_convert("UTC")

    # Resample to hourly, fill NaN with 0 (no DHW = 0W, that IS the signal)
    dhw_hourly = dhw_df["value"].resample("h").mean().fillna(0).clip(lower=0)
    print(f"  Hourly DHW samples: {len(dhw_hourly)}")

    # Load weather for temperature feature
    start_date = dhw_hourly.index.min().date()
    end_date = dhw_hourly.index.max().date()
    weather_df = _load_weather(config, start_date, end_date)

    # Build features with weather
    features = build_dhw_features(dhw_hourly.index, config, weather_df=weather_df)

    # Join features with target
    joined = features.copy()
    joined["dhw_w"] = dhw_hourly.reindex(features.index).fillna(0)

    # Apply rolling window smoothing to target
    if smoothing_window > 1:
        joined["dhw_w"] = joined["dhw_w"].rolling(
            smoothing_window, center=True, min_periods=1).mean()
        print(f"  Smoothing window: {smoothing_window}h")

    joined = joined.dropna()

    X = joined.drop(columns=["dhw_w"])
    y = joined["dhw_w"]
    timestamps = joined.index

    print(f"  Training samples: {len(X)}")
    print(f"  Date range: {timestamps.min()} to {timestamps.max()}")

    return X, y, timestamps


# ---------------------------------------------------------------------------
# Spot Price model
# ---------------------------------------------------------------------------

def build_spot_price_features(weather_df: pd.DataFrame, config: dict) -> pd.DataFrame:
    """Build features for spot price model.

    Features: time cyclicals, behavioral (day_of_week, weekend, holiday),
    temperature, wind_speed.
    """
    df = weather_df.copy()
    df = df.set_index("timestamp").sort_index()

    _add_time_features(df, config)
    _add_behavioral_time_features(df, config)
    _add_weather_features(df)

    feature_cols = [
        "hour_sin", "hour_cos",
        "month_sin", "month_cos",
        "day_of_week_sin", "day_of_week_cos",
        "is_weekend", "is_holiday",
        "temperature", "wind_speed",
    ]
    return df[feature_cols]


def prepare_spot_price_dataset(config: dict) -> tuple[pd.DataFrame, pd.Series, pd.DatetimeIndex]:
    """Prepare spot price dataset."""
    root = project_root()

    # Load historic spot prices
    prices_path = root / "input" / "recent" / "historic_spot_prices.csv"
    price_df = load_spot_prices(prices_path)
    if price_df.empty:
        raise ValueError(f"No spot price data found at {prices_path}")

    print(f"  Spot price readings: {len(price_df)}")

    price_df = price_df.set_index("timestamp").sort_index()
    # Already hourly, but resample to ensure alignment
    price_hourly = price_df["value"].resample("h").mean().dropna()
    print(f"  Hourly price samples: {len(price_hourly)}")

    # Load weather (limit start to 2024-07-01 to match weather cache)
    weather_start = date(2024, 7, 1)
    start_date = max(price_hourly.index.min().date(), weather_start)
    end_date = price_hourly.index.max().date()
    weather_df = _load_weather(config, start_date, end_date)

    features = build_spot_price_features(weather_df, config)

    # Join
    target = price_hourly
    target.name = "spot_price"
    joined = features.join(target, how="inner").dropna(subset=["spot_price"])

    # Add lagged features (past only — shift to avoid leakage)
    shifted_price = joined["spot_price"].shift(1)
    joined["price_lag_1h"] = shifted_price
    joined["price_lag_24h"] = joined["spot_price"].shift(24)
    joined["price_rolling_24h_mean"] = shifted_price.rolling(24, min_periods=1).mean()

    # Drop rows with NaN from lags (first 24h)
    joined = joined.dropna()

    X = joined.drop(columns=["spot_price"])
    y = joined["spot_price"]
    timestamps = joined.index

    print(f"  Training samples: {len(X)}")
    print(f"  Date range: {timestamps.min()} to {timestamps.max()}")

    return X, y, timestamps
