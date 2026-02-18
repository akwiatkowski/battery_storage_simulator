# Home Energy Analysis

A data-driven analysis of a single-family home with 10 kWp PV, air-source heat
pump, and 37 sensor channels — covering solar sizing, heat pump diagnostics,
building envelope, indoor climate, and energy economics.

## Executive Summary

This report analyzes ~15 months of real home energy data at hourly resolution,
supplemented by high-resolution (60-second) recent data. The analysis covers
seven areas: solar system sizing, energy economics, heat pump performance, hot
water efficiency, building thermal quality, indoor comfort, and grid power quality.

**Key findings:**
- Peak power is 1.5-3x hourly averages — sizing to averages undersizes everything
- Self-sufficiency reaches 47% in summer with PV alone, 3% in December
- Defrost cycles consume 14-17% of winter HP energy (302 kWh/year)
- DHW tank standby loss is 55W continuous (~384 PLN/year)
- Rooms run too warm (21% of hours >25°C) — heating curve is over-steep
- Workshop battery storage is feasible with minimal insulation + waste heat
- All living spaces have zero mold risk; workshop (garage) has 64%

## Table of Contents

1. [Solar & Grid Sizing](01_solar_sizing.md) — peaks vs averages, hidden PV, duration curves, export clipping, seasonal variation
2. [Solar Economy](02_solar_economy.md) — spot prices, self-consumption, self-sufficiency, appliance timing, battery feasibility, baseload
3. [Heat Pump Performance](03_heat_pump.md) — COP analysis, temperature lift, compressor diagnostics, cycling, defrost, wind chill
4. [Domestic Hot Water](04_domestic_hot_water.md) — DHW timing optimization, tank standby loss
5. [Building Envelope & Heating](05_building_envelope.md) — heating curve audit, floor temperature differences, curve impact analysis, room temperatures, thermal response, savings projections
6. [Indoor Climate](06_indoor_climate.md) — ASHRAE comfort, mold risk, air quality
7. [Grid Infrastructure](07_grid_infrastructure.md) — voltage, power quality, PV curtailment

## Conclusions

**Solar sizing:**
1. Hourly averages undersize everything. Peak power is 1.5-3x higher, and
   winter drives the sizing requirement.
2. Hidden PV exists in ~30% of "import" hours. Brief export bursts are
   invisible in averaged data.
3. Diminishing returns are real — the economically optimal battery/inverter
   size is well below the technical maximum.

**Energy economics:**
4. Spot price timing matters. Clipping costs depend on *when*, not just how
   much. PV self-consumption is the cheapest optimization lever.
5. Self-sufficiency peaks at 47% in June with PV alone. A battery would push
   shoulder months (March-April, September-October) significantly higher.
6. Workshop battery storage is feasible with light insulation + waste heat,
   losing only 12 no-charge days per year.

**Heat pump:**
7. Defrost cycles consume 14-17% of winter HP energy (302 kWh/year), doubling
   below -5°C.
8. DHW is the efficiency outlier — temperature lifts of 40-50°C vs 5-10°C for
   space heating.
9. Wind increases heating cost beyond temperature alone — measurable COP
   reduction from evaporator wind chill.
10. Short cycling at mild temperatures wastes energy. The HP may be oversized
    for moderate-weather loads.

**Building & comfort:**
11. The heating curve is over-steep — rooms run warm (21% of hours >25°C). Each
    unnecessary degree of water temperature costs ~2-3% COP.
12. Room thermal quality varies 2-3x. Targeting the worst rooms gives the
    highest insulation ROI.
13. Each 1°C indoor reduction saves ~4% of heating energy. Enforcing 21°C saves
    ~7% (~61 PLN/season).

**DHW:**
14. DHW tank standby loss is 55W (τ ≈ 105h), costing ~384 PLN/year. A case for
    better insulation or on-demand DHW.
15. Shifting DHW heating to warmest hours improves COP with zero hardware changes.

---

*Generated from `analysis/scripts/01-38`. Run `make -C analysis` to reproduce all charts.*
