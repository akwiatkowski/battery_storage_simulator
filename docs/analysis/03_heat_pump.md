# Part III — Heat Pump Performance

Diagnosing the air-source heat pump: COP drivers, compressor behavior,
defrost overhead, and weather effects.

## COP vs Outdoor Temperature

COP depends heavily on outdoor temperature. Below 0°C, efficiency drops sharply:

![COP vs Temperature](01_cop_vs_temp.png)

COP varies by time of day (morning defrost periods vs afternoon):

![COP by Time of Day](01_cop_by_time.png)

## Temperature Lift

The water temperature lift (outlet - inlet) directly determines efficiency.
DHW cycles require much higher lifts (40-50°C vs 5-10°C for heating):

![COP vs Delta-T](11_cop_vs_delta_t.png)

![Heating vs DHW](11_heating_vs_dhw.png)

## Compressor Diagnostics

Lower compressor speeds achieve higher COP — part-load efficiency matters:

![Compressor vs COP](16_compressor_vs_cop.png)

True thermal power (flow × ΔT) vs reported sensor — measurement accuracy check:

![Thermal Power](16_thermal_power.png)

Refrigerant cycle (discharge temperature vs high pressure, colored by COP):

![Refrigerant Cycle](16_refrigerant_cycle.png)

## Cycling & Modulation

Compressor speed distribution — smooth modulation or excessive on/off?

![Modulation Histogram](22_modulation_histogram.png)

Short cycling detection — too many transitions per day waste energy:

![Cycling Detection](22_cycling_detection.png)

The part-load sweet spot — COP vs compressor speed at different outdoor temps:

![Part-Load Sweet Spot](22_partload_sweetspot.png)

## Defrost Energy Budget

Defrost cycles reverse the refrigerant to melt ice from the outdoor evaporator.
They produce zero useful heat — pure overhead.

Frequency increases sharply below -5°C, consuming 14-17% of winter HP energy:

![Defrost by Temperature](31_defrost_by_temp.png)

![Defrost Duration](31_defrost_duration.png)

Monthly defrost energy as fraction of total HP consumption:

![Defrost Monthly Energy](31_defrost_monthly_energy.png)

Pipe temperature during a defrost event:

![Defrost Pipe Example](31_defrost_pipe_example.png)

## Wind Chill Effect

Wind strips heat from the evaporator coil, reducing COP measurably:

![Wind vs COP](24_wind_vs_cop.png)

Certain wind directions expose more building surface or the outdoor unit:

![Wind Rose Heating](24_wind_rose_heating.png)

HP works harder in windy conditions at the same outdoor temperature:

![Wind Power Demand](24_wind_power_demand.png)
