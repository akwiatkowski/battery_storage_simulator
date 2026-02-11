<script lang="ts">
	import { Chart, Svg, Area, Axis, Highlight, LinearGradient, Grid } from 'layerchart';
	import { scaleTime, scaleLinear } from 'd3-scale';
	import { extent } from 'd3-array';
	import { simulation, type ChartPoint } from '$lib/stores/simulation';

	let data: ChartPoint[] = $derived(simulation.chartData);

	let xDomain = $derived(
		data.length > 1
			? (extent(data, (d: ChartPoint) => d.timestamp) as [Date, Date])
			: [new Date(), new Date()]
	);

	let yMin = $derived(data.length > 0 ? Math.min(0, ...data.map((d) => d.value)) : 0);
	let yMax = $derived(data.length > 0 ? Math.max(1, ...data.map((d) => d.value)) : 1000);
</script>

<div class="chart-container">
	<div class="chart-header">Grid Power</div>
	{#if data.length > 1}
		<div class="chart-wrapper">
			<Chart
				{data}
				x="timestamp"
				xScale={scaleTime()}
				{xDomain}
				y="value"
				yScale={scaleLinear()}
				yDomain={[yMin, yMax * 1.1]}
				yNice
				padding={{ left: 60, bottom: 36, right: 16, top: 16 }}
			>
				<Svg>
					<Grid horizontal class="stroke-white/10" />
					<Axis
						placement="left"
						format={(v) => (Math.abs(v) >= 1000 ? (v / 1000).toFixed(1) + 'k' : String(v))}
						class="text-xs"
					/>
					<Axis
						placement="bottom"
						format={(d) => {
							const date = d instanceof Date ? d : new Date(d);
							return (
								date.getUTCHours().toString().padStart(2, '0') +
								':' +
								date.getUTCMinutes().toString().padStart(2, '0')
							);
						}}
						class="text-xs"
					/>
					<LinearGradient id="power-gradient" from="#4a9eff" to="#4a9eff" fromOpacity={0.4} toOpacity={0.05} vertical />
					<Area fill="url(#power-gradient)" line={{ class: 'stroke-[#4a9eff] stroke-[1.5]' }} />
					<Highlight area lines />
				</Svg>
			</Chart>
		</div>
	{:else}
		<div class="chart-placeholder">
			Waiting for data... Press Play to start the simulation.
		</div>
	{/if}
</div>

<style>
	.chart-container {
		background: #1a1a2e;
		border: 1px solid #2a2a4a;
		border-radius: 8px;
		padding: 16px;
	}

	.chart-header {
		color: #888;
		font-size: 13px;
		text-transform: uppercase;
		letter-spacing: 1px;
		margin-bottom: 12px;
	}

	.chart-wrapper {
		height: 300px;
	}

	.chart-placeholder {
		height: 300px;
		display: flex;
		align-items: center;
		justify-content: center;
		color: #555;
		font-size: 14px;
	}

	:global(.chart-wrapper .tick text) {
		fill: #888;
	}

	:global(.chart-wrapper .tick line) {
		stroke: #333;
	}
</style>
