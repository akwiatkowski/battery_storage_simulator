# analysis/r/Makefile — Run all R analysis scripts
#
# Usage:
#   make            # run all 18 scripts
#   make 03         # run just script 03
#   make clean      # remove all generated output
#
# Scripts are run from the project root so that relative paths like
# "input/grid_power.csv" and "analysis/r/R/load_data.R" resolve correctly.

# Project root is two levels up from analysis/r/
ROOT := $(shell cd ../.. && pwd)

# All scripts, discovered automatically
SCRIPTS := $(sort $(wildcard scripts/*.R))

# Sentinel files track which scripts have been run.
# scripts/03_peak_vs_average.R → scripts/03_peak_vs_average.done
SENTINELS := $(SCRIPTS:.R=.done)

# Shared library files — if any change, all scripts re-run
SHARED := R/load_data.R R/theme.R R/helpers.R

.PHONY: all clean 01 02 03 04 05 06 07 08 09 10 11 12 13 14 15 16 17 18 19 20 21 22 23 24 25 26 27 28 29 30 31 32 33 34 35 36 37

all: $(SENTINELS)

# Pattern rule: run a script when it or any shared file changes.
# The sentinel (.done) file is "touched" on success so make knows
# not to re-run it unless inputs change.
scripts/%.done: scripts/%.R $(SHARED)
	cd $(ROOT) && Rscript analysis/r/$<
	@touch $@

# Convenience targets by number: `make 03` runs script 03
01: scripts/01_cop_analysis.done
02: scripts/02_grid_heatmap.done
03: scripts/03_peak_vs_average.done
04: scripts/04_self_sufficiency.done
05: scripts/05_export_clipping.done
06: scripts/06_hidden_pv.done
07: scripts/07_power_duration.done
08: scripts/08_seasonal_inverter.done
09: scripts/09_cost_of_clipping.done
10: scripts/10_spot_price_patterns.done
11: scripts/11_hp_delta_t.done
12: scripts/12_pv_self_consumption.done
13: scripts/13_baseload.done
14: scripts/14_load_shifting.done
15: scripts/15_indoor_temperature.done
16: scripts/16_hp_compressor.done
17: scripts/17_air_quality.done
18: scripts/18_voltage_power_quality.done
19: scripts/19_workshop_thermal.done
20: scripts/20_heating_curve.done
21: scripts/21_room_thermal_response.done
22: scripts/22_hp_cycling.done
23: scripts/23_dhw_optimization.done
24: scripts/24_wind_chill.done
25: scripts/25_heating_savings.done
26: scripts/26_battery_temperature.done
27: scripts/27_mold_risk.done
28: scripts/28_ventilation.done
29: scripts/29_thermal_comfort.done
30: scripts/30_auxiliary_heater.done
31: scripts/31_defrost_analysis.done
32: scripts/32_dhw_tank.done
33: scripts/33_rain_pv.done
34: scripts/34_pressure_heating.done
35: scripts/35_appliance_pv.done
36: scripts/36_noise_occupancy.done
37: scripts/37_self_sufficiency.done

clean:
	rm -rf output/ scripts/*.done
