# Sample Input Data

Example CSV files demonstrating all supported data formats with realistic sample data.

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

- The sample data values are fictional but the entity IDs match the backend's `HAEntityToSensorType` mapping, so these files will load correctly.
- Stats and recent formats require entity IDs present in the mapping. Unknown entity IDs are silently skipped.
- See `input/README.md` for full format documentation.
