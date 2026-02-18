# Part II — Solar Economy

How solar generation, spot prices, and load patterns interact to determine
the economic value of PV and battery systems.

## Spot Price Patterns

Eight years of spot price data (2018-2026) — the 2022 energy crisis is
clearly visible:

![Price Heatmap](10_price_heatmap.png)

![Price YoY](10_price_yoy.png)

Daily price volatility (max/min ratio) determines arbitrage and load-shifting
potential:

![Price Volatility](10_price_volatility.png)

## PV Self-Consumption

How much solar generation is used directly vs exported:

![Self-Consumption Hourly](12_self_consumption_hourly.png)

![PV Utilization](12_pv_utilization.png)

## Self-Sufficiency

A self-sufficient hour draws no grid power. Overall: 22.8% of hours, peaking
at 47% in June:

![Self-Sufficiency Heatmap](37_self_sufficiency_heatmap.png)

![Self-Sufficiency Monthly](37_self_sufficiency_monthly.png)

Daily self-sufficiency rate (daytime hours, 6-20h):

![Self-Sufficiency Calendar](37_self_sufficiency_calendar.png)

## Appliance vs PV Timing

Do major appliances run during solar hours? Washing machine already 37%
solar-covered, oven 21%:

![Appliance Timing vs PV](35_appliance_timing_vs_pv.png)

![Appliance Solar Coverage](35_appliance_solar_coverage.png)

Shifting all runs to PV peak hours (10-14h) would save ~35 PLN/year:

![Appliance Shift Savings](35_appliance_shift_savings.png)

## Battery Self-Sufficiency

Simulated battery at each capacity. Steep initial rise, then diminishing returns:

![Self Sufficiency](04_self_sufficiency.png)

## Battery Temperature Feasibility

Can LFP batteries survive year-round in an unheated workshop (metal garage)?
Charge limit: 0°C, discharge limit: -10°C.

With light insulation + battery waste heat (300W) + 50W heating pad, no-charge
days drop from 108 to 12 — just 2% of the year:

![Battery Feasibility](26_battery_feasibility.png)

![Battery Days Lost](26_battery_days_lost.png)

PV surplus lost because the battery is too cold to charge — negligible with
insulation:

![Lost PV Surplus](26_lost_pv_surplus.png)

## Baseload

The always-on power floor — fridge, network, standby devices:

![Baseload Hourly](13_baseload_hourly.png)

![Baseload Cost](13_baseload_cost.png)

## Appliance Load Shifting

Washing machine, drier, and oven usage overlaid with spot prices:

![Cycle Times](14_cycle_times.png)

![Shifting Savings](14_shifting_savings.png)

![Best Hours](14_best_hours.png)
