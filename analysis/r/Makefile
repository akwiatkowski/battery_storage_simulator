# analysis/r/Makefile — Run all R analysis scripts
#
# Usage:
#   make            # run all 14 scripts
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

.PHONY: all clean 01 02 03 04 05 06 07 08 09 10 11 12 13 14

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

clean:
	rm -rf output/ scripts/*.done
