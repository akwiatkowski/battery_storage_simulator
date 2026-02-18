"""Open-Meteo API: fetch historical weather + forecast, cache to monthly CSVs."""

from datetime import date, datetime
from pathlib import Path

import pandas as pd
import requests

HOURLY_PARAMS = (
    "temperature_2m,cloud_cover,direct_radiation,diffuse_radiation,"
    "sunshine_duration,wind_speed_10m,precipitation,relative_humidity_2m"
)


def fetch_historical(
    lat: float,
    lon: float,
    start_date: date,
    end_date: date,
    cache_dir: Path,
) -> pd.DataFrame:
    """Fetch historical hourly weather from Open-Meteo, with monthly CSV caching.

    Completed months are never re-fetched. Only the current month is updated.
    Returns merged DataFrame from all monthly cache files.
    """
    cache_dir.mkdir(parents=True, exist_ok=True)
    today = date.today()

    # Generate list of months to cover
    months = []
    d = start_date.replace(day=1)
    while d <= end_date:
        months.append((d.year, d.month))
        if d.month == 12:
            d = d.replace(year=d.year + 1, month=1)
        else:
            d = d.replace(month=d.month + 1)

    for year, month in months:
        cache_file = cache_dir / f"poznan-{year:04d}-{month:02d}.csv"
        is_current_month = year == today.year and month == today.month
        is_future = (year, month) > (today.year, today.month)

        if is_future:
            continue

        # Skip completed months that already have cached data
        if cache_file.exists() and not is_current_month:
            continue

        # Determine date range for this month
        month_start = date(year, month, 1)
        if month == 12:
            month_end = date(year + 1, 1, 1)
        else:
            month_end = date(year, month + 1, 1)
        from datetime import timedelta

        month_end = month_end - timedelta(days=1)

        # Clamp to requested range and today
        fetch_start = max(month_start, start_date)
        fetch_end = min(month_end, end_date, today)

        if fetch_start > fetch_end:
            continue

        print(f"  Fetching weather: {fetch_start} to {fetch_end}")
        df = _fetch_open_meteo_historical(lat, lon, fetch_start, fetch_end)
        df.to_csv(cache_file, index=False)

    # Load all cached files in range
    frames = []
    for year, month in months:
        cache_file = cache_dir / f"poznan-{year:04d}-{month:02d}.csv"
        if cache_file.exists():
            frames.append(pd.read_csv(cache_file, parse_dates=["timestamp"]))

    if not frames:
        return pd.DataFrame()

    result = pd.concat(frames, ignore_index=True)
    result["timestamp"] = pd.to_datetime(result["timestamp"], utc=True)
    result = result.sort_values("timestamp").drop_duplicates(subset=["timestamp"], keep="last")
    return result.reset_index(drop=True)


def fetch_forecast(lat: float, lon: float, hours: int = 48) -> pd.DataFrame:
    """Fetch weather forecast from Open-Meteo (not cached)."""
    url = "https://api.open-meteo.com/v1/forecast"
    params = {
        "latitude": lat,
        "longitude": lon,
        "hourly": HOURLY_PARAMS,
        "forecast_hours": hours,
        "timezone": "UTC",
    }
    resp = requests.get(url, params=params, timeout=30)
    resp.raise_for_status()
    data = resp.json()
    return _parse_hourly_response(data)


def _fetch_open_meteo_historical(
    lat: float, lon: float, start: date, end: date
) -> pd.DataFrame:
    """Call Open-Meteo archive API for a date range."""
    url = "https://archive-api.open-meteo.com/v1/archive"
    params = {
        "latitude": lat,
        "longitude": lon,
        "start_date": start.isoformat(),
        "end_date": end.isoformat(),
        "hourly": HOURLY_PARAMS,
        "timezone": "UTC",
    }
    resp = requests.get(url, params=params, timeout=60)
    resp.raise_for_status()
    data = resp.json()
    return _parse_hourly_response(data)


def _parse_hourly_response(data: dict) -> pd.DataFrame:
    """Parse Open-Meteo hourly JSON response into a DataFrame."""
    hourly = data["hourly"]
    df = pd.DataFrame(
        {
            "timestamp": pd.to_datetime(hourly["time"], utc=True),
            "temperature_2m": hourly.get("temperature_2m"),
            "cloud_cover": hourly.get("cloud_cover"),
            "direct_radiation": hourly.get("direct_radiation"),
            "diffuse_radiation": hourly.get("diffuse_radiation"),
            "sunshine_duration": hourly.get("sunshine_duration"),
            "wind_speed_10m": hourly.get("wind_speed_10m"),
            "precipitation": hourly.get("precipitation"),
            "relative_humidity_2m": hourly.get("relative_humidity_2m"),
        }
    )
    return df
