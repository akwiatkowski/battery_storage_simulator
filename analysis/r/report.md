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

---

# Part I — Solar & Grid Sizing

The original question: are hourly averages sufficient for sizing inverters,
batteries, and wiring? No. Peak power within each hour is systematically
higher, and the gap has financial consequences.

## Grid Power Pattern

Average grid power follows a predictable pattern: import in the morning and
evening, export during midday solar hours.

![Grid Heatmap](output/02_grid_heatmap.png)

But averages hide the variance within each hour.

## Peaks vs Averages

Every hour plotted as (average, peak). All points above the 1:1 line — nearly
all of them — show where peak exceeded average:

![Peak vs Average Scatter](output/03_peak_vs_avg_scatter.png)

The typical peak/average ratio is ~1.8x:

![Ratio Histogram](output/03_ratio_histogram.png)

Sizing an inverter to hourly averages (green) leaves peaks uncovered. Covering
95% of actual demand (red) requires significantly more:

![Inverter Sizing Curve](output/03_inverter_sizing.png)

The gap between average and peak is worst at midday (solar fluctuations) and
evening (cooking + heating):

![Hourly Gap](output/03_hourly_gap.png)

## Hidden PV Generation

~30% of "import" hours actually contain moments of solar export. Hourly
averaging cancels out these brief bursts:

![Hidden PV Scatter](output/06_hidden_pv_scatter.png)

![Hidden PV Hourly](output/06_hidden_pv_hourly.png)

A single day shows how much the average conceals vs the actual min-max range:

![One Day Range](output/06_one_day_range.png)

## Power Duration Curves

Extreme peaks (>5 kW) occur only a few percent of the time. Import vs export
overlaid shows which drives inverter sizing:

![Combined Duration](output/07_combined_duration.png)

## Export Clipping

When the inverter caps export power, excess PV is wasted. Diminishing returns
as inverter size increases:

![Export Clipping](output/05_export_clipping.png)

## Seasonal Variation

Winter drives inverter sizing more than summer — high heating demand creates
the largest peaks:

![Seasonal Peak Sizing](output/08_seasonal_peak_sizing.png)

![Seasonal Gap](output/08_seasonal_gap.png)

![Seasonal Profiles](output/08_seasonal_profiles.png)

## Financial Impact of Undersizing

Monetary cost of clipping at each inverter size, using real spot prices:

![Clipping Cost](output/09_clipping_cost.png)

High-price evening peaks are the most expensive to clip:

![Clipping by Hour](output/09_clipping_by_hour.png)

Each additional kW of inverter capacity saves less than the previous one:

![Marginal Value](output/09_marginal_value.png)

---

# Part II — Solar Economy

How solar generation, spot prices, and load patterns interact to determine
the economic value of PV and battery systems.

## Spot Price Patterns

Eight years of spot price data (2018-2026) — the 2022 energy crisis is
clearly visible:

![Price Heatmap](output/10_price_heatmap.png)

![Price YoY](output/10_price_yoy.png)

Daily price volatility (max/min ratio) determines arbitrage and load-shifting
potential:

![Price Volatility](output/10_price_volatility.png)

## PV Self-Consumption

How much solar generation is used directly vs exported:

![Self-Consumption Hourly](output/12_self_consumption_hourly.png)

![PV Utilization](output/12_pv_utilization.png)

## Self-Sufficiency

A self-sufficient hour draws no grid power. Overall: 22.8% of hours, peaking
at 47% in June:

![Self-Sufficiency Heatmap](output/37_self_sufficiency_heatmap.png)

![Self-Sufficiency Monthly](output/37_self_sufficiency_monthly.png)

Daily self-sufficiency rate (daytime hours, 6-20h):

![Self-Sufficiency Calendar](output/37_self_sufficiency_calendar.png)

## Appliance vs PV Timing

Do major appliances run during solar hours? Washing machine already 37%
solar-covered, oven 21%:

![Appliance Timing vs PV](output/35_appliance_timing_vs_pv.png)

![Appliance Solar Coverage](output/35_appliance_solar_coverage.png)

Shifting all runs to PV peak hours (10-14h) would save ~35 PLN/year:

![Appliance Shift Savings](output/35_appliance_shift_savings.png)

## Battery Self-Sufficiency

Simulated battery at each capacity. Steep initial rise, then diminishing returns:

![Self Sufficiency](output/04_self_sufficiency.png)

## Battery Temperature Feasibility

Can LFP batteries survive year-round in an unheated workshop (metal garage)?
Charge limit: 0°C, discharge limit: -10°C.

With light insulation + battery waste heat (300W) + 50W heating pad, no-charge
days drop from 108 to 12 — just 2% of the year:

![Battery Feasibility](output/26_battery_feasibility.png)

![Battery Days Lost](output/26_battery_days_lost.png)

PV surplus lost because the battery is too cold to charge — negligible with
insulation:

![Lost PV Surplus](output/26_lost_pv_surplus.png)

## Baseload

The always-on power floor — fridge, network, standby devices:

![Baseload Hourly](output/13_baseload_hourly.png)

![Baseload Cost](output/13_baseload_cost.png)

## Appliance Load Shifting

Washing machine, drier, and oven usage overlaid with spot prices:

![Cycle Times](output/14_cycle_times.png)

![Shifting Savings](output/14_shifting_savings.png)

![Best Hours](output/14_best_hours.png)

---

# Part III — Heat Pump Performance

Diagnosing the air-source heat pump: COP drivers, compressor behavior,
defrost overhead, and weather effects.

## COP vs Outdoor Temperature

COP depends heavily on outdoor temperature. Below 0°C, efficiency drops sharply:

![COP vs Temperature](output/01_cop_vs_temp.png)

COP varies by time of day (morning defrost periods vs afternoon):

![COP by Time of Day](output/01_cop_by_time.png)

## Temperature Lift

The water temperature lift (outlet - inlet) directly determines efficiency.
DHW cycles require much higher lifts (40-50°C vs 5-10°C for heating):

![COP vs Delta-T](output/11_cop_vs_delta_t.png)

![Heating vs DHW](output/11_heating_vs_dhw.png)

## Compressor Diagnostics

Lower compressor speeds achieve higher COP — part-load efficiency matters:

![Compressor vs COP](output/16_compressor_vs_cop.png)

True thermal power (flow × ΔT) vs reported sensor — measurement accuracy check:

![Thermal Power](output/16_thermal_power.png)

Refrigerant cycle (discharge temperature vs high pressure, colored by COP):

![Refrigerant Cycle](output/16_refrigerant_cycle.png)

## Cycling & Modulation

Compressor speed distribution — smooth modulation or excessive on/off?

![Modulation Histogram](output/22_modulation_histogram.png)

Short cycling detection — too many transitions per day waste energy:

![Cycling Detection](output/22_cycling_detection.png)

The part-load sweet spot — COP vs compressor speed at different outdoor temps:

![Part-Load Sweet Spot](output/22_partload_sweetspot.png)

## Defrost Energy Budget

Defrost cycles reverse the refrigerant to melt ice from the outdoor evaporator.
They produce zero useful heat — pure overhead.

Frequency increases sharply below -5°C, consuming 14-17% of winter HP energy:

![Defrost by Temperature](output/31_defrost_by_temp.png)

![Defrost Duration](output/31_defrost_duration.png)

Monthly defrost energy as fraction of total HP consumption:

![Defrost Monthly Energy](output/31_defrost_monthly_energy.png)

Pipe temperature during a defrost event:

![Defrost Pipe Example](output/31_defrost_pipe_example.png)

## Wind Chill Effect

Wind strips heat from the evaporator coil, reducing COP measurably:

![Wind vs COP](output/24_wind_vs_cop.png)

Certain wind directions expose more building surface or the outdoor unit:

![Wind Rose Heating](output/24_wind_rose_heating.png)

HP works harder in windy conditions at the same outdoor temperature:

![Wind Power Demand](output/24_wind_power_demand.png)

---

# Part IV — Domestic Hot Water

DHW heating is the least efficient HP mode (high temperature lift) and the
tank loses heat continuously. Two opportunities: better timing and less loss.

## DHW Timing Optimization

When does DHW heating happen vs when COP is highest? Misalignment wastes energy:

![DHW Timing](output/23_dhw_timing.png)

COP improvement potential — actual timing vs optimal scheduling:

![DHW COP Potential](output/23_dhw_cop_potential.png)

Cost by hour and month — dark cells are the most expensive times:

![DHW Cost Heatmap](output/23_dhw_cost_heatmap.png)

## Tank Standby Loss

Exponential decay model fit to idle cooling periods: τ ≈ 105 hours, steady
standby loss ≈ 55W. Annual cost: ~384 PLN.

![DHW Tank Profile](output/32_dhw_tank_profile.png)

![DHW Cooling Rate](output/32_dhw_cooling_rate.png)

How long until the tank reaches 40°C from different starting temperatures:

![DHW Reheat Schedule](output/32_dhw_reheat_schedule.png)

---

# Part V — Building Envelope & Heating

Understanding the thermal performance of the building: which rooms hold heat,
where the heating curve is wrong, and what temperature changes would save.

## Heating Curve Audit

The HP uses a weather-compensating curve: outdoor temperature → target water
supply temperature. A wrong curve wastes energy.

![Heating Curve](output/20_heating_curve.png)

Room temperatures by outdoor bin reveal over- or under-heating:

![Curve vs Comfort](output/20_curve_vs_comfort.png)

Where is the curve wrong? Red = overheating, blue = insufficient:

![Overheating Map](output/20_overheating_map.png)

COP penalty for each unnecessary degree of water temperature:

![Curve Efficiency](output/20_curve_efficiency.png)

## Indoor Temperature Stability

Room-by-room comparison — which rooms are warmer, cooler, more stable:

![Room Temperature Comparison](output/15_room_temp_comparison.png)

Daily temperature swing per room:

![Daily Temperature Range](output/15_daily_temp_range.png)

## Room Thermal Response

Cooling rate when the HP cycles off — a direct measure of insulation and
thermal mass per room:

![Cooling Rates](output/21_cooling_rates.png)

Thermal inertia ranking — hours to lose 1°C:

![Thermal Inertia](output/21_thermal_inertia.png)

Overnight temperature drop (22:00 → 06:00):

![Night Drop](output/21_night_drop.png)

Temperature uniformity across rooms — wide spread means zoning problems:

![Uniformity](output/21_uniformity.png)

## Workshop Thermal Response

The unheated workshop follows outdoor conditions closely:

![Workshop Time Series](output/19_workshop_timeseries.png)

![Workshop Scatter](output/19_workshop_scatter.png)

Thermal lag — hours for outdoor changes to propagate:

![Workshop Lag](output/19_workshop_lag.png)

## Heating Savings & Cooling Projections

Energy contributed by each outdoor temperature bin:

![Heating Energy Curve](output/25_heating_energy_curve.png)

Weekly savings from reducing indoor temperature (-1°C, -2°C, -3°C, max 21°C):

![Weekly Savings kWh](output/25_weekly_savings_kwh.png)

![Weekly Savings PLN](output/25_weekly_savings_pln.png)

Estimated cooling energy for summer. Right panel: impact of +4°C European warming:

![Cooling Energy](output/25_cooling_energy.png)

![Cooling Monthly](output/25_cooling_monthly.png)

---

# Part VI — Indoor Climate

Comfort, moisture, and air quality across all monitored rooms.

## Thermal Comfort (ASHRAE)

ASHRAE Standard 55 comfort zone: 19-25°C, 30-70% RH. The dominant discomfort
mode is overheating (21% of room-hours), not cold (3%):

![Comfort Scatter](output/29_comfort_scatter.png)

![Comfort Score](output/29_comfort_score.png)

![Comfort Daily](output/29_comfort_daily.png)

## Mold Risk

Dewpoint proximity analysis using the Magnus formula. All living spaces show
zero risk. The workshop (unheated garage, 82% RH) has 64% of hours at risk —
relevant for stored electronics and tools:

![Mold Risk Ranking](output/27_mold_risk_ranking.png)

![Mold Risk Heatmap](output/27_mold_risk_heatmap.png)

![Mold Risk Daily](output/27_mold_risk_daily.png)

![Dewpoint vs Outdoor](output/27_dewpoint_vs_outdoor.png)

## Air Quality

CO2 by room reveals occupancy — bedroom peaks at night, living room in evening:

![CO2 Daily Pattern](output/17_co2_daily_pattern.png)

Noise levels track daily activity patterns:

![Noise Pattern](output/17_noise_pattern.png)

---

# Part VII — Grid Infrastructure

## Voltage & Power Quality

Grid voltage varies by time of day. The 253V curtailment threshold matters
for PV export:

![Voltage Profile](output/18_voltage_profile.png)

Per-circuit voltage comparison shows wiring voltage drop:

![Circuit Voltage](output/18_circuit_voltage.png)

Power factor below 90% indicates reactive power from HP compressor:

![Power Factor](output/18_power_factor.png)

Voltage rises during PV export — grid saturation indicator:

![Voltage vs Export](output/18_voltage_vs_export.png)

---

# Conclusions

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

*Generated from `analysis/r/scripts/01-37`. Run `make -C analysis/r` to
reproduce all charts.*
