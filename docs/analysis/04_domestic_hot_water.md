# Part IV — Domestic Hot Water

DHW heating is the least efficient HP mode (high temperature lift) and the
tank loses heat continuously. Two opportunities: better timing and less loss.

## DHW Timing Optimization

When does DHW heating happen vs when COP is highest? Misalignment wastes energy:

![DHW Timing](23_dhw_timing.png)

COP improvement potential — actual timing vs optimal scheduling:

![DHW COP Potential](23_dhw_cop_potential.png)

Cost by hour and month — dark cells are the most expensive times:

![DHW Cost Heatmap](23_dhw_cost_heatmap.png)

## Tank Standby Loss

Exponential decay model fit to idle cooling periods: τ ≈ 105 hours, steady
standby loss ≈ 55W. Annual cost: ~384 PLN.

![DHW Tank Profile](32_dhw_tank_profile.png)

![DHW Cooling Rate](32_dhw_cooling_rate.png)

How long until the tank reaches 40°C from different starting temperatures:

![DHW Reheat Schedule](32_dhw_reheat_schedule.png)
