# Part I — Solar & Grid Sizing

The original question: are hourly averages sufficient for sizing inverters,
batteries, and wiring? No. Peak power within each hour is systematically
higher, and the gap has financial consequences.

## Grid Power Pattern

Average grid power follows a predictable pattern: import in the morning and
evening, export during midday solar hours.

![Grid Heatmap](02_grid_heatmap.png)

But averages hide the variance within each hour.

## Peaks vs Averages

Every hour plotted as (average, peak). All points above the 1:1 line — nearly
all of them — show where peak exceeded average:

![Peak vs Average Scatter](03_peak_vs_avg_scatter.png)

The typical peak/average ratio is ~1.8x:

![Ratio Histogram](03_ratio_histogram.png)

Sizing an inverter to hourly averages (green) leaves peaks uncovered. Covering
95% of actual demand (red) requires significantly more:

![Inverter Sizing Curve](03_inverter_sizing.png)

The gap between average and peak is worst at midday (solar fluctuations) and
evening (cooking + heating):

![Hourly Gap](03_hourly_gap.png)

## Hidden PV Generation

~30% of "import" hours actually contain moments of solar export. Hourly
averaging cancels out these brief bursts:

![Hidden PV Scatter](06_hidden_pv_scatter.png)

![Hidden PV Hourly](06_hidden_pv_hourly.png)

A single day shows how much the average conceals vs the actual min-max range:

![One Day Range](06_one_day_range.png)

## Power Duration Curves

Extreme peaks (>5 kW) occur only a few percent of the time. Import vs export
overlaid shows which drives inverter sizing:

![Combined Duration](07_combined_duration.png)

## Export Clipping

When the inverter caps export power, excess PV is wasted. Diminishing returns
as inverter size increases:

![Export Clipping](05_export_clipping.png)

## Seasonal Variation

Winter drives inverter sizing more than summer — high heating demand creates
the largest peaks:

![Seasonal Peak Sizing](08_seasonal_peak_sizing.png)

![Seasonal Gap](08_seasonal_gap.png)

![Seasonal Profiles](08_seasonal_profiles.png)

## Financial Impact of Undersizing

Monetary cost of clipping at each inverter size, using real spot prices:

![Clipping Cost](09_clipping_cost.png)

High-price evening peaks are the most expensive to clip:

![Clipping by Hour](09_clipping_by_hour.png)

Each additional kW of inverter capacity saves less than the previous one:

![Marginal Value](09_marginal_value.png)
