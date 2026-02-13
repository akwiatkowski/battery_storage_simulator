# Sample Input Data

Example CSV files demonstrating all supported data formats. These use fictional sensor entity IDs and won't match a real Home Assistant installation.

## Usage

Copy this directory to `input/` to get started:

```bash
cp -r input.sample/ input/
```

Then replace the sample files with your own Home Assistant data exports.

## What's Included

### Legacy per-sensor files (`input.sample/*.csv`)

One file per sensor, loaded from the root `input/` directory:

- `grid_power.csv` — smart meter power (W), positive = import, negative = export
- `pv_power.csv` — solar panel power output (W)
- `pump_ext_temp.csv` — outdoor temperature (°C)

### Multi-sensor statistics (`input.sample/stats/`)

- `sample.csv` — hourly aggregated stats (avg/min/max) for multiple sensors

### Multi-sensor recent readings (`input.sample/recent/`)

- `sample.csv` — recent sensor readings for multiple sensors
- `energy_prices.csv` — hourly spot energy prices (PLN/kWh)

## Notes

- The entity IDs in these samples (e.g. `sensor.example_grid_power`) are fictional. For real data, use your actual Home Assistant entity IDs.
- Stats and recent formats require entity IDs that match the `HAEntityToSensorType` mapping in the backend. Unknown entity IDs are silently skipped.
- See `input/README.md` for full format documentation.
