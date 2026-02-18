# Part V — Building Envelope & Heating

Understanding the thermal performance of the building: which rooms hold heat,
where the heating curve is wrong, and what temperature changes would save.

## Heating Curve Audit

The HP uses a weather-compensating curve: outdoor temperature → target water
supply temperature. A wrong curve wastes energy.

![Heating Curve](20_heating_curve.png)

Room temperatures by outdoor bin reveal over- or under-heating:

![Curve vs Comfort](20_curve_vs_comfort.png)

Where is the curve wrong? Red = overheating, blue = insufficient:

![Overheating Map](20_overheating_map.png)

COP penalty for each unnecessary degree of water temperature:

![Curve Efficiency](20_curve_efficiency.png)

## Indoor Temperature Stability

Room-by-room comparison — which rooms are warmer, cooler, more stable:

![Room Temperature Comparison](15_room_temp_comparison.png)

Daily temperature swing per room:

![Daily Temperature Range](15_daily_temp_range.png)

## Room Thermal Response

Cooling rate when the HP cycles off — a direct measure of insulation and
thermal mass per room:

![Cooling Rates](21_cooling_rates.png)

Thermal inertia ranking — hours to lose 1°C:

![Thermal Inertia](21_thermal_inertia.png)

Overnight temperature drop (22:00 → 06:00):

![Night Drop](21_night_drop.png)

Temperature uniformity across rooms — wide spread means zoning problems:

![Uniformity](21_uniformity.png)

## Floor Temperature Differences

Ground floor (living room, kitchen, Olek, Beata) vs first floor/attic
(bathroom, bedrooms). First floor averages 0.5°C cooler:

![Room Temperatures by Floor](39_room_by_floor.png)

Monthly temperature difference between floors:

![Monthly Floor Difference](39_monthly_floor_diff.png)

Hourly floor temperature profiles — ground floor runs consistently warmer:

![Floor Temperature Profiles](39_floor_temp_profiles.png)

Hourly difference pattern (first floor − ground floor):

![Hourly Floor Difference](39_hourly_floor_diff.png)

## Heating Curve Impact on Floor Temperatures

How sensitive is each room to supply water temperature changes? Linear model
controls for outdoor temperature:

![Curve Sensitivity](40_curve_sensitivity.png)

Predicted temperature drop at different curve reductions, by floor:

![Curve Reduction Impact](40_curve_reduction.png)

Floor-level comparison — which floor hits comfort limits first:

![Floor Sensitivity](40_floor_sensitivity.png)

Cold risk — percentage of heating hours below 20°C at each reduction:

![Cold Risk](40_cold_risk.png)

## Workshop Thermal Response

The unheated workshop follows outdoor conditions closely:

![Workshop Time Series](19_workshop_timeseries.png)

![Workshop Scatter](19_workshop_scatter.png)

Thermal lag — hours for outdoor changes to propagate:

![Workshop Lag](19_workshop_lag.png)

## Heating Savings & Cooling Projections

Energy contributed by each outdoor temperature bin:

![Heating Energy Curve](25_heating_energy_curve.png)

Weekly savings from reducing indoor temperature (-1°C, -2°C, -3°C, max 21°C):

![Weekly Savings kWh](25_weekly_savings_kwh.png)

![Weekly Savings PLN](25_weekly_savings_pln.png)

Estimated cooling energy for summer. Right panel: impact of +4°C European warming:

![Cooling Energy](25_cooling_energy.png)

![Cooling Monthly](25_cooling_monthly.png)
