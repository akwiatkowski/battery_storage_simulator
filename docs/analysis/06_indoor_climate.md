# Part VI — Indoor Climate

Comfort, moisture, and air quality across all monitored rooms.

## Thermal Comfort (ASHRAE)

ASHRAE Standard 55 comfort zone: 19-25°C, 30-70% RH. The dominant discomfort
mode is overheating (21% of room-hours), not cold (3%):

![Comfort Scatter](29_comfort_scatter.png)

![Comfort Score](29_comfort_score.png)

![Comfort Daily](29_comfort_daily.png)

## Mold Risk

Dewpoint proximity analysis using the Magnus formula. All living spaces show
zero risk. The workshop (unheated garage, 82% RH) has 64% of hours at risk —
relevant for stored electronics and tools:

![Mold Risk Ranking](27_mold_risk_ranking.png)

![Mold Risk Heatmap](27_mold_risk_heatmap.png)

![Mold Risk Daily](27_mold_risk_daily.png)

![Dewpoint vs Outdoor](27_dewpoint_vs_outdoor.png)

## Air Quality

CO2 by room reveals occupancy — bedroom peaks at night, living room in evening:

![CO2 Daily Pattern](17_co2_daily_pattern.png)

Noise levels track daily activity patterns:

![Noise Pattern](17_noise_pattern.png)
