# Why Hourly Averages Are Misleading for Energy System Sizing

A data-driven analysis of real home energy data showing why peak power — not
average power — should drive inverter, battery, and PV system design.

## Executive Summary

Hourly average power readings are the standard unit in energy monitoring
systems like Home Assistant. But within each hour, actual power fluctuates
wildly: peak demand is typically **1.5-3x** the hourly average, and brief
solar export bursts are invisible in averaged data. This analysis quantifies
the gap and its financial consequences.

**Key findings:**
- Median peak-to-average ratio is ~1.8x across all hours
- ~30% of "import" hours actually contain hidden PV export moments
- Sizing an inverter to hourly averages leaves 20-40% of peaks uncovered
- The annual cost of undersizing can reach hundreds of PLN
- Winter drives inverter sizing requirements more than summer
- Battery self-sufficiency shows strong diminishing returns above 5-10 kWh

---

## 1. The Data

We use two data sources:
- **Legacy Home Assistant exports** (hourly readings, ~1 year)
- **High-resolution recent data** (~60-second intervals, recent weeks)

The high-res data lets us compute true min/max within each hour, revealing
information that hourly averages discard.

### Heat Pump Performance

The heat pump's COP (Coefficient of Performance) depends heavily on outdoor
temperature. Below 0°C, efficiency drops sharply.

![COP vs Temperature](output/01_cop_vs_temp.png)

COP varies by month (seasonal temperature ranges) and time of day:

![COP by Month](output/01_cop_by_month.png)

![COP by Time of Day](output/01_cop_by_time.png)

---

## 2. The Grid Power Pattern

Average grid power follows a predictable pattern: import in the morning and
evening, export during solar hours midday.

![Grid Heatmap](output/02_grid_heatmap.png)

But averages hide the variance within each hour.

---

## 3. The Problem: Peaks vs Averages

### 3.1 The Scatter

Every hour plotted as (average, peak). Points above the 1:1 line — all of
them — show where peak exceeded average. Many hours have peaks 2-3x the
average.

![Peak vs Average Scatter](output/03_peak_vs_avg_scatter.png)

### 3.2 The Ratio Distribution

The histogram of peak/average ratios shows the typical multiplier is ~1.8x:

![Ratio Histogram](output/03_ratio_histogram.png)

### 3.3 Inverter Sizing Implications

If you size your inverter using hourly averages (green curve), you think a
small inverter suffices. But to cover 95% of actual peak demand (red curve),
you need significantly more:

![Inverter Sizing Curve](output/03_inverter_sizing.png)

### 3.4 When Is the Gap Worst?

The gap between average and peak power varies by hour of day. Midday
(solar fluctuations) and evening (cooking + heating) show the biggest
discrepancies:

![Hourly Gap](output/03_hourly_gap.png)

---

## 4. Hidden PV Generation

Some hours appear as "pure import" (positive average) but actually contained
moments of solar export (negative instantaneous power). Hourly averaging
cancels out these brief export bursts.

![Hidden PV Scatter](output/06_hidden_pv_scatter.png)

Hidden PV is concentrated in morning and late afternoon hours — the edges
of the solar window:

![Hidden PV Hourly](output/06_hidden_pv_hourly.png)

A single day's data shows just how much the average (black line) conceals
compared to the actual min-max range (red band):

![One Day Range](output/06_one_day_range.png)

---

## 5. Power Duration Curves

Classic power engineering visualization: sort all peaks highest-to-lowest
and plot. The curve shows that extreme peaks (>5 kW) occur only a few
percent of the time:

![Import Duration](output/07_import_duration.png)

![Export Duration](output/07_export_duration.png)

Overlaying both shows whether import or export drives the inverter size:

![Combined Duration](output/07_combined_duration.png)

---

## 6. Export Clipping

When the inverter caps export power, excess PV generation is wasted. The
clipping loss curve shows diminishing returns as inverter size increases:

![Export Clipping](output/05_export_clipping.png)

The distribution of peak export power shows where most hours fall:

![Export Distribution](output/05_export_distribution.png)

---

## 7. Seasonal Variation

### 7.1 Sizing by Season

Different seasons have different peak power profiles. Winter typically
drives the inverter size requirement:

![Seasonal Peak Sizing](output/08_seasonal_peak_sizing.png)

### 7.2 The Average-Peak Gap by Season

![Seasonal Gap](output/08_seasonal_gap.png)

### 7.3 Daily Profiles by Season

![Seasonal Profiles](output/08_seasonal_profiles.png)

---

## 8. Financial Impact

### 8.1 Annual Cost of Undersizing

Combining peak power data with spot electricity prices gives the actual
monetary cost of clipping at each inverter size:

![Clipping Cost](output/09_clipping_cost.png)

### 8.2 When Clipping Costs Most

The hourly breakdown (at 3 kW) shows that high-price evening peaks are
the most expensive to clip:

![Clipping by Hour](output/09_clipping_by_hour.png)

### 8.3 Marginal Value

Each additional kW of inverter capacity saves less money than the previous
one — classic diminishing returns:

![Marginal Value](output/09_marginal_value.png)

---

## 9. Battery Self-Sufficiency

A simple battery simulation shows what % of hours can avoid grid import
at each capacity. The curve rises steeply at first (high value per kWh),
then flattens:

![Self Sufficiency](output/04_self_sufficiency.png)

---

## 10. Electricity Market Structure

Eight years of spot price data (2018-2026) reveal the underlying price
patterns that drive every optimization decision.

![Price Heatmap](output/10_price_heatmap.png)

The year-over-year evolution shows structural changes — the 2022 energy
crisis is clearly visible:

![Price YoY](output/10_price_yoy.png)

Daily price volatility (max/min ratio) determines the potential for
arbitrage and load shifting:

![Price Volatility](output/10_price_volatility.png)

---

## 11. Heat Pump Temperature Lift

The heat pump's water temperature lift (outlet - inlet) directly determines
efficiency. Higher lifts mean harder work for the compressor:

![COP vs Delta-T](output/11_cop_vs_delta_t.png)

DHW (hot water) cycles require much higher temperature lifts than space
heating, making them significantly less efficient:

![Heating vs DHW](output/11_heating_vs_dhw.png)

---

## 12. PV Self-Consumption

How much solar generation is actually used by the house vs exported?

![Self-Consumption Hourly](output/12_self_consumption_hourly.png)

![PV Utilization](output/12_pv_utilization.png)

---

## 13. Baseload

The always-on power floor — fridge, network equipment, standby devices:

![Baseload Hourly](output/13_baseload_hourly.png)

![Baseload Cost](output/13_baseload_cost.png)

---

## 14. Appliance Load Shifting

Washing machine, drier, and oven usage overlaid with spot prices reveals
shifting potential:

![Cycle Times](output/14_cycle_times.png)

![Shifting Savings](output/14_shifting_savings.png)

![Best Hours](output/14_best_hours.png)

---

## 15. Indoor Temperature Stability

Room-by-room temperature comparison reveals which rooms are warmer or
cooler, and how stable each room maintains temperature:

![Room Temperature Comparison](output/15_room_temp_comparison.png)

Daily temperature swing (max - min) per room shows stability — smaller
swing means better thermal inertia or more consistent heating:

![Daily Temperature Range](output/15_daily_temp_range.png)

Indoor vs outdoor correlation shows thermal lag — how quickly the
building responds to outdoor temperature changes:

![Thermal Response](output/15_thermal_response.png)

---

## 16. Heat Pump Compressor Diagnostics

Compressor speed vs COP demonstrates part-load efficiency — lower
compressor speeds achieve higher COP:

![Compressor vs COP](output/16_compressor_vs_cop.png)

True thermal power calculated from flow × ΔT compared with the
reported production sensor reveals measurement accuracy:

![Thermal Power](output/16_thermal_power.png)

The refrigerant cycle (discharge temperature vs high pressure, colored
by COP) shows how hard the compressor is working:

![Refrigerant Cycle](output/16_refrigerant_cycle.png)

---

## 17. Indoor Air Quality

CO2 levels by room reveal occupancy patterns — bedroom peaks at night,
living room in the evening:

![CO2 Daily Pattern](output/17_co2_daily_pattern.png)

Noise levels from the Netatmo sensor track daily activity:

![Noise Pattern](output/17_noise_pattern.png)

Atmospheric pressure trend correlates with weather fronts:

![Pressure Trend](output/17_pressure_trend.png)

---

## 18. Voltage & Power Quality

Grid voltage varies by time of day — higher at night (low demand),
dipping during peak hours. The 253V curtailment threshold matters for PV:

![Voltage Profile](output/18_voltage_profile.png)

Per-circuit voltage comparison shows wiring voltage drop from the meter
to each outlet:

![Circuit Voltage](output/18_circuit_voltage.png)

Power factor below 90% indicates significant reactive power, common with
heat pump compressor operation:

![Power Factor](output/18_power_factor.png)

Voltage rises during PV export — a clear indicator of grid saturation:

![Voltage vs Export](output/18_voltage_vs_export.png)

---

## 19. Workshop Thermal Response

The unheated workshop's temperature follows outdoor conditions closely,
revealing building envelope characteristics:

![Workshop Time Series](output/19_workshop_timeseries.png)

Thermal coupling analysis: slope < 1 means the building envelope provides
some damping, but R² close to 1 means minimal independent temperature control:

![Workshop Scatter](output/19_workshop_scatter.png)

Cross-correlation reveals the thermal lag — how many hours for outdoor
changes to propagate indoors:

![Workshop Lag](output/19_workshop_lag.png)

---

## 20. Heating Curve Audit

The heat pump uses a weather-compensating curve: outdoor temperature
determines the target water supply temperature. A wrong curve wastes energy.

![Heating Curve](output/20_heating_curve.png)

Does the configured curve actually deliver indoor comfort? Room temperatures
by outdoor temperature bin reveal over- or under-heating:

![Curve vs Comfort](output/20_curve_vs_comfort.png)

When and where is the curve wrong? The overheating map shows excess heat
(red) or insufficient heat (blue) by hour and outdoor temperature:

![Overheating Map](output/20_overheating_map.png)

Higher water temperatures cost efficiency. The COP penalty for each degree
of water temperature increase:

![Curve Efficiency](output/20_curve_efficiency.png)

---

## 21. Room-by-Room Thermal Response

How fast each room loses heat when the HP cycles off — a direct measure
of insulation quality and thermal mass:

![Cooling Rates](output/21_cooling_rates.png)

Thermal inertia ranking — hours to lose 1°C. Higher = room holds heat
longer and needs less frequent HP operation:

![Thermal Inertia](output/21_thermal_inertia.png)

Overnight temperature drop (22:00 → 06:00) reveals which rooms need the
most heating recovery in the morning:

![Night Drop](output/21_night_drop.png)

Temperature uniformity across rooms — a wide spread means some rooms are
over-heated while others are under-heated:

![Uniformity](output/21_uniformity.png)

---

## 22. Heat Pump Cycling & Modulation

Compressor speed distribution reveals whether the HP modulates smoothly
or spends too much time at extremes:

![Modulation Histogram](output/22_modulation_histogram.png)

Short cycling detection — too many on/off transitions per day reduce COP
and increase compressor wear:

![Cycling Detection](output/22_cycling_detection.png)

Defrost cycles identified by outside pipe temperature depression below
outdoor air temperature:

![Defrost Detection](output/22_defrost_detection.png)

The part-load sweet spot — COP vs compressor speed at different outdoor
temperatures reveals the optimal operating range:

![Part-Load Sweet Spot](output/22_partload_sweetspot.png)

---

## 23. DHW Timing Optimization

When does domestic hot water heating happen vs when COP is highest?
Misalignment = wasted energy:

![DHW Timing](output/23_dhw_timing.png)

Tank heat loss rate during idle periods — steeper slope means the tank
loses heat faster and needs more frequent reheating:

![DHW Tank Loss](output/23_dhw_tank_loss.png)

COP improvement potential: actual DHW timing vs optimal scheduling at
best-COP hours of each day:

![DHW COP Potential](output/23_dhw_cop_potential.png)

DHW heating cost by hour and month — dark cells are the most expensive
times to heat water:

![DHW Cost Heatmap](output/23_dhw_cost_heatmap.png)

---

## 24. Wind Chill & Heat Loss

Wind strips heat from the HP's outdoor evaporator coil, reducing COP.
The effect is measurable even at moderate wind speeds:

![Wind vs COP](output/24_wind_vs_cop.png)

Wind direction matters — certain directions expose more building surface
or the HP outdoor unit directly:

![Wind Rose Heating](output/24_wind_rose_heating.png)

Wind-driven indoor temperature deviation despite HP running:

![Wind Indoor Effect](output/24_wind_indoor_effect.png)

HP works harder in windy conditions — power consumption rises with wind
speed even at the same outdoor temperature:

![Wind Power Demand](output/24_wind_power_demand.png)

---

## Conclusions

1. **Hourly averages are not sufficient for system sizing.** Peak power is
   typically 1.5-3x higher than the hourly average, and this matters for
   inverter, battery, and wiring specifications.

2. **Hidden PV generation exists in ~30% of "import" hours.** Brief solar
   export moments are invisible in averaged data but represent real energy
   flow that affects battery and inverter requirements.

3. **Winter drives inverter sizing.** High heating demand creates the largest
   peaks, even though summer has more total energy flow from PV.

4. **Diminishing returns are real.** Whether for battery capacity or inverter
   power rating, the marginal value of each additional unit drops quickly.
   The economically optimal size is usually well below the technical maximum.

5. **Spot price timing matters.** The financial impact of clipping depends
   not just on how much power is clipped, but *when* — expensive hours
   amplify the cost.

6. **DHW cycles are the efficiency outlier.** Hot water heating requires
   much higher temperature lifts (40-50°C vs 5-10°C for space heating),
   resulting in significantly lower COP. Pre-heating DHW during solar
   hours could save meaningful energy.

7. **PV self-consumption is the first lever.** Before adding batteries or
   selling export, maximizing direct use of solar generation — by shifting
   loads to midday — is the cheapest optimization.

8. **Baseload is a fixed cost.** The always-on power floor (fridge, network,
   standby) runs 24/7 regardless of price. Reducing it by even 50W saves
   ~440 kWh/year.

9. **Indoor temperature stability varies by room.** Well-insulated rooms
   maintain tighter temperature ranges. Rooms with more external walls
   show greater daily swings and faster response to outdoor changes.

10. **Grid voltage constrains PV export.** During midday export peaks,
    voltage can exceed 253V, triggering inverter curtailment. This is a
    real-world PV clipping mechanism beyond inverter capacity limits.

11. **Heating curve tuning is a free efficiency gain.** An over-steep heating
    curve produces water temperatures higher than needed, directly reducing
    COP. Each unnecessary degree of water temperature costs ~2-3% efficiency.

12. **Rooms vary dramatically in thermal quality.** Cooling rates when the HP
    cycles off differ by 2-3x between rooms, revealing insulation weak spots.
    Targeting improvements at the worst rooms gives the highest ROI.

13. **Short cycling at mild temperatures wastes energy.** When outdoor temps
    are moderate, the HP may be oversized for the load, causing frequent
    start/stop cycles that reduce COP and increase compressor wear.

14. **DHW timing is a low-hanging optimization.** Shifting hot water heating
    to the warmest hours of the day improves COP without any hardware changes.
    The tank's heat loss rate determines how far ahead DHW can be pre-heated.

15. **Wind increases heating cost beyond temperature alone.** Wind chill on
    the evaporator coil reduces COP, and wind-driven infiltration increases
    heating demand. Wind direction matters — exposed facades lose more heat.

---

*Generated from `analysis/r/scripts/01-24`. Run `make -C analysis/r` to
reproduce all charts.*
