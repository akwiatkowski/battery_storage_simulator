<script lang="ts">
	import { Chart, Svg, Spline, Area, Axis, Grid, Rule } from 'layerchart';
	import { scaleTime, scaleLinear } from 'd3-scale';
	import { bisector } from 'd3-array';
	import { curveMonotoneX } from 'd3-shape';
	import { simulation, type TimeSeriesPoint } from '$lib/stores/simulation.svelte';
	import HelpTip from './HelpTip.svelte';

	const WINDOWS: { label: string; key: string; ms: number }[] = [
		{ label: '1h', key: '1h', ms: 3600_000 },
		{ label: '6h', key: '6h', ms: 6 * 3600_000 },
		{ label: '24h', key: '24h', ms: 24 * 3600_000 },
		{ label: '7d', key: '7d', ms: 7 * 24 * 3600_000 }
	];

	let expanded = $state(false);

	// Series toggle state — only Grid and Battery on by default
	let showGrid = $state(true);
	let showPV = $state(false);
	let showBattery = $state(true);
	let showHP = $state(false);
	let showPrice = $state(false);
	let showSoC = $state(false);
	let showTempActual = $state(false);
	let showTempPredicted = $state(false);

	type SeriesKey = 'grid' | 'pv' | 'battery' | 'hp' | 'price' | 'soc' | 'tempActual' | 'tempPredicted';

	const SERIES: { key: SeriesKey; label: string; color: string; unit: string }[] = [
		{ key: 'grid', label: 'Grid', color: '#e87c6c', unit: 'W' },
		{ key: 'pv', label: 'PV', color: '#e8b830', unit: 'W' },
		{ key: 'battery', label: 'Battery', color: '#64b5f6', unit: 'W' },
		{ key: 'hp', label: 'Heat Pump', color: '#e8884c', unit: 'W' },
		{ key: 'price', label: 'Price', color: '#94a3b8', unit: 'PLN' },
		{ key: 'soc', label: 'SoC', color: '#5bb88a', unit: '%' },
		{ key: 'tempActual', label: 'Temp', color: '#64748b', unit: '°C' },
		{ key: 'tempPredicted', label: 'Predicted', color: '#9b8fd8', unit: '°C' }
	];

	function isVisible(key: SeriesKey): boolean {
		switch (key) {
			case 'grid': return showGrid;
			case 'pv': return showPV;
			case 'battery': return showBattery;
			case 'hp': return showHP;
			case 'price': return showPrice;
			case 'soc': return showSoC;
			case 'tempActual': return showTempActual;
			case 'tempPredicted': return showTempPredicted;
		}
	}

	function toggle(key: SeriesKey): void {
		switch (key) {
			case 'grid': showGrid = !showGrid; break;
			case 'pv': showPV = !showPV; break;
			case 'battery': showBattery = !showBattery; break;
			case 'hp': showHP = !showHP; break;
			case 'price': showPrice = !showPrice; break;
			case 'soc': showSoC = !showSoC; break;
			case 'tempActual': showTempActual = !showTempActual; break;
			case 'tempPredicted': showTempPredicted = !showTempPredicted; break;
		}
	}

	// Window-filtered data using binary search
	const dataBisector = bisector<TimeSeriesPoint, Date>((d) => d.timestamp);

	let windowMs = $derived(WINDOWS.find((w) => w.key === simulation.chartWindow)?.ms ?? 6 * 3600_000);

	let windowedData = $derived.by(() => {
		const data = simulation.timeSeriesData;
		if (data.length === 0) return [];
		const latest = data[data.length - 1].timestamp.getTime();
		const cutoff = new Date(latest - windowMs);
		const idx = dataBisector.left(data, cutoff);
		return data.slice(idx);
	});

	let hasData = $derived(windowedData.length > 1);

	// X domain
	let xDomain = $derived<[Date, Date]>(
		hasData
			? [windowedData[0].timestamp, windowedData[windowedData.length - 1].timestamp]
			: [new Date(), new Date()]
	);

	// Which panel groups have visible series?
	let showPowerPanel = $derived(showGrid || showPV || showBattery || showHP);
	let showPricePanel = $derived(showPrice);
	let showSoCPanel = $derived(showSoC);
	let showTempPanel = $derived(showTempActual || showTempPredicted);

	// Power Y domain (W)
	let powerYDomain = $derived.by(() => {
		if (!hasData) return [-1000, 1000];
		let min = 0;
		let max = 100;
		for (const d of windowedData) {
			if (showGrid) { min = Math.min(min, d.gridPowerW); max = Math.max(max, d.gridPowerW); }
			if (showPV) { min = Math.min(min, d.pvPowerW); max = Math.max(max, d.pvPowerW); }
			if (showBattery) { min = Math.min(min, d.batteryPowerW); max = Math.max(max, d.batteryPowerW); }
			if (showHP) { min = Math.min(min, d.heatPumpPowerW); max = Math.max(max, d.heatPumpPowerW); }
		}
		const pad = (max - min) * 0.05 || 100;
		return [min - pad, max + pad];
	});

	// Price Y domain (PLN/kWh)
	let priceYDomain = $derived.by(() => {
		if (!hasData) return [0, 1];
		let min = Infinity;
		let max = -Infinity;
		for (const d of windowedData) {
			min = Math.min(min, d.spotPrice);
			max = Math.max(max, d.spotPrice);
		}
		if (min === Infinity) { min = 0; max = 1; }
		const pad = (max - min) * 0.1 || 0.1;
		return [min - pad, max + pad];
	});

	// Temperature Y domain (°C)
	let tempYDomain = $derived.by(() => {
		if (!hasData) return [-10, 30];
		let min = Infinity;
		let max = -Infinity;
		for (const d of windowedData) {
			if (showTempActual && d.tempActualC !== null) { min = Math.min(min, d.tempActualC); max = Math.max(max, d.tempActualC); }
			if (showTempPredicted && d.tempPredictedC !== null) { min = Math.min(min, d.tempPredictedC); max = Math.max(max, d.tempPredictedC); }
		}
		if (min === Infinity) { min = -10; max = 30; }
		const pad = (max - min) * 0.1 || 2;
		return [min - pad, max + pad];
	});

	// Tooltip bisect
	let hoveredIdx = $state<number | null>(null);

	function bisectIndex(data: TimeSeriesPoint[], x: Date): number {
		const idx = dataBisector.left(data, x);
		if (idx === 0) return 0;
		if (idx >= data.length) return data.length - 1;
		const d0 = data[idx - 1];
		const d1 = data[idx];
		return x.getTime() - d0.timestamp.getTime() < d1.timestamp.getTime() - x.getTime() ? idx - 1 : idx;
	}

	let hoveredPoint = $derived(
		hoveredIdx !== null && windowedData[hoveredIdx]
			? windowedData[hoveredIdx]
			: null
	);

	// Shared hover handler for all panels
	function handlePointerMove(e: PointerEvent) {
		const rect = (e.currentTarget as HTMLElement).closest('.charts-area')!.getBoundingClientRect();
		const xPct = (e.clientX - rect.left) / rect.width;
		const tMin = xDomain[0].getTime();
		const tMax = xDomain[1].getTime();
		const t = new Date(tMin + xPct * (tMax - tMin));
		hoveredIdx = bisectIndex(windowedData, t);
	}

	function handlePointerLeave() {
		hoveredIdx = null;
	}

	function crosshairPct(pt: TimeSeriesPoint): number {
		return ((pt.timestamp.getTime() - xDomain[0].getTime()) / (xDomain[1].getTime() - xDomain[0].getTime())) * 100;
	}

	function formatW(v: number): string {
		const abs = Math.abs(v);
		if (abs >= 1000) return (v / 1000).toFixed(1) + ' kW';
		return v.toFixed(0) + ' W';
	}

	function formatAxisW(v: number): string {
		const abs = Math.abs(v);
		if (abs >= 1000) return (v / 1000).toFixed(1) + 'k';
		return String(Math.round(v));
	}

	function formatTime(d: Date | number): string {
		const date = d instanceof Date ? d : new Date(d);
		return (
			date.getUTCHours().toString().padStart(2, '0') +
			':' +
			date.getUTCMinutes().toString().padStart(2, '0')
		);
	}

	function formatTimeFull(d: Date): string {
		const month = (d.getUTCMonth() + 1).toString().padStart(2, '0');
		const day = d.getUTCDate().toString().padStart(2, '0');
		const hours = d.getUTCHours().toString().padStart(2, '0');
		const mins = d.getUTCMinutes().toString().padStart(2, '0');
		return `${month}-${day} ${hours}:${mins}`;
	}
</script>

<div class="chart-card">
	<button class="card-header" onclick={() => (expanded = !expanded)}>
		<span class="arrow" class:open={expanded}>&#9654;</span>
		<span class="card-title">Time Series</span>
	</button>

	{#if expanded}
	<div class="chart-toolbar">
		<div class="window-buttons">
			{#each WINDOWS as w}
				<button
					class="window-btn"
					class:active={simulation.chartWindow === w.key}
					onclick={() => simulation.setChartWindow(w.key)}
				>{w.label}</button>
			{/each}
		</div>
	</div>

	{#if hasData}
		<!-- svelte-ignore a11y_no_static_element_interactions -->
		<div class="charts-area"
			onpointermove={handlePointerMove}
			onpointerleave={handlePointerLeave}
		>
			{#if showPowerPanel}
				<div class="panel-label">Power <span class="panel-unit">(W)</span> <HelpTip key="chartPower" /></div>
				<div class="chart-wrapper power-chart">
					<Chart
						data={windowedData}
						x="timestamp"
						xScale={scaleTime()}
						{xDomain}
						y="gridPowerW"
						yScale={scaleLinear()}
						yDomain={powerYDomain}
						padding={{ left: 56, bottom: 24, right: 16, top: 8 }}
					>
						<Svg>
							<Grid y class="chart-grid" />
							<Axis
								placement="left"
								format={formatAxisW}
								ticks={5}
								classes={{ tickLabel: 'axis-label' }}
							/>
							<Axis
								placement="bottom"
								format={formatTime}
								ticks={6}
								classes={{ tickLabel: 'axis-label' }}
							/>
							<Rule y={0} class="zero-line" />
							{#if showGrid}
								<Spline data={windowedData} y={(d) => d.gridPowerW} stroke="#e87c6c" fill="none" strokeWidth={1.5} curve={curveMonotoneX} />
							{/if}
							{#if showPV}
								<Spline data={windowedData} y={(d) => d.pvPowerW} stroke="#e8b830" fill="none" strokeWidth={1.5} curve={curveMonotoneX} />
							{/if}
							{#if showBattery}
								<Spline data={windowedData} y={(d) => d.batteryPowerW} stroke="#64b5f6" fill="none" strokeWidth={1.5} curve={curveMonotoneX} />
							{/if}
							{#if showHP}
								<Spline data={windowedData} y={(d) => d.heatPumpPowerW} stroke="#e8884c" fill="none" strokeWidth={1.5} curve={curveMonotoneX} />
							{/if}
						</Svg>
					</Chart>
					{#if hoveredPoint}
						<div class="crosshair" style:left="{crosshairPct(hoveredPoint)}%"></div>
						<div class="tooltip-card" style:left="{Math.min(75, Math.max(5, crosshairPct(hoveredPoint)))}%">
							<div class="tooltip-time">{formatTimeFull(hoveredPoint.timestamp)}</div>
							{#if showGrid}<div class="tooltip-row"><span class="tooltip-dot" style:background="#e87c6c"></span> Grid: {formatW(hoveredPoint.gridPowerW)}</div>{/if}
							{#if showPV}<div class="tooltip-row"><span class="tooltip-dot" style:background="#e8b830"></span> PV: {formatW(hoveredPoint.pvPowerW)}</div>{/if}
							{#if showBattery}<div class="tooltip-row"><span class="tooltip-dot" style:background="#64b5f6"></span> Battery: {formatW(hoveredPoint.batteryPowerW)}</div>{/if}
							{#if showHP}<div class="tooltip-row"><span class="tooltip-dot" style:background="#e8884c"></span> HP: {formatW(hoveredPoint.heatPumpPowerW)}</div>{/if}
						</div>
					{/if}
				</div>
			{/if}

			{#if showPricePanel}
				<div class="panel-label">Spot Price <span class="panel-unit">(PLN/kWh)</span> <HelpTip key="chartPrice" /></div>
				<div class="chart-wrapper secondary-chart">
					<Chart
						data={windowedData}
						x="timestamp"
						xScale={scaleTime()}
						{xDomain}
						y="spotPrice"
						yScale={scaleLinear()}
						yDomain={priceYDomain}
						padding={{ left: 56, bottom: 24, right: 16, top: 8 }}
					>
						<Svg>
							<Grid y class="chart-grid" />
							<Axis
								placement="left"
								format={(v) => typeof v === 'number' ? v.toFixed(2) : String(v)}
								ticks={4}
								classes={{ tickLabel: 'axis-label' }}
							/>
							<Axis
								placement="bottom"
								format={formatTime}
								ticks={6}
								classes={{ tickLabel: 'axis-label' }}
							/>
							<Spline data={windowedData} y={(d) => d.spotPrice} stroke="#94a3b8" fill="none" strokeWidth={1.5} curve={curveMonotoneX} />
						</Svg>
					</Chart>
					{#if hoveredPoint}
						<div class="crosshair" style:left="{crosshairPct(hoveredPoint)}%"></div>
						<div class="tooltip-card" style:left="{Math.min(75, Math.max(5, crosshairPct(hoveredPoint)))}%">
							<div class="tooltip-time">{formatTimeFull(hoveredPoint.timestamp)}</div>
							<div class="tooltip-row"><span class="tooltip-dot" style:background="#94a3b8"></span> Price: {hoveredPoint.spotPrice.toFixed(3)} PLN</div>
						</div>
					{/if}
				</div>
			{/if}

			{#if showSoCPanel}
				<div class="panel-label">Battery SoC <span class="panel-unit">(%)</span> <HelpTip key="chartSoC" /></div>
				<div class="chart-wrapper secondary-chart">
					<Chart
						data={windowedData}
						x="timestamp"
						xScale={scaleTime()}
						{xDomain}
						y="batterySoCPct"
						yScale={scaleLinear()}
						yDomain={[0, 100]}
						padding={{ left: 56, bottom: 24, right: 16, top: 8 }}
					>
						<Svg>
							<Grid y class="chart-grid" />
							<Axis
								placement="left"
								format={(v) => v + '%'}
								ticks={4}
								classes={{ tickLabel: 'axis-label' }}
							/>
							<Axis
								placement="bottom"
								format={formatTime}
								ticks={6}
								classes={{ tickLabel: 'axis-label' }}
							/>
							<Area
								data={windowedData}
								y0={() => 0}
								y1={(d) => d.batterySoCPct}
								fill="#5bb88a"
								fillOpacity={0.15}
								line={{ stroke: '#5bb88a', strokeWidth: 1.5, curve: curveMonotoneX }}
								curve={curveMonotoneX}
							/>
						</Svg>
					</Chart>
					{#if hoveredPoint}
						<div class="crosshair" style:left="{crosshairPct(hoveredPoint)}%"></div>
						<div class="tooltip-card" style:left="{Math.min(75, Math.max(5, crosshairPct(hoveredPoint)))}%">
							<div class="tooltip-time">{formatTimeFull(hoveredPoint.timestamp)}</div>
							<div class="tooltip-row"><span class="tooltip-dot" style:background="#5bb88a"></span> SoC: {hoveredPoint.batterySoCPct.toFixed(1)}%</div>
						</div>
					{/if}
				</div>
			{/if}

			{#if showTempPanel}
				<div class="panel-label">Temperature <span class="panel-unit">(°C)</span> <HelpTip key="chartTemp" /></div>
				<div class="chart-wrapper secondary-chart">
					<Chart
						data={windowedData}
						x="timestamp"
						xScale={scaleTime()}
						{xDomain}
						y="tempActualC"
						yScale={scaleLinear()}
						yDomain={tempYDomain}
						padding={{ left: 56, bottom: 24, right: 16, top: 8 }}
					>
						<Svg>
							<Grid y class="chart-grid" />
							<Axis
								placement="left"
								format={(v) => typeof v === 'number' ? v.toFixed(0) + '°' : String(v)}
								ticks={4}
								classes={{ tickLabel: 'axis-label' }}
							/>
							<Axis
								placement="bottom"
								format={formatTime}
								ticks={6}
								classes={{ tickLabel: 'axis-label' }}
							/>
							{#if showTempActual}
								<Spline data={windowedData} y={(d) => d.tempActualC ?? undefined} stroke="#64748b" fill="none" strokeWidth={1.5} curve={curveMonotoneX} defined={(d) => d.tempActualC !== null} />
							{/if}
							{#if showTempPredicted}
								<Spline data={windowedData} y={(d) => d.tempPredictedC ?? undefined} stroke="#9b8fd8" fill="none" strokeWidth={1.5} curve={curveMonotoneX} defined={(d) => d.tempPredictedC !== null} />
							{/if}
						</Svg>
					</Chart>
					{#if hoveredPoint}
						<div class="crosshair" style:left="{crosshairPct(hoveredPoint)}%"></div>
						<div class="tooltip-card" style:left="{Math.min(75, Math.max(5, crosshairPct(hoveredPoint)))}%">
							<div class="tooltip-time">{formatTimeFull(hoveredPoint.timestamp)}</div>
							{#if showTempActual && hoveredPoint.tempActualC !== null}<div class="tooltip-row"><span class="tooltip-dot" style:background="#64748b"></span> Temp: {hoveredPoint.tempActualC.toFixed(1)}°C</div>{/if}
							{#if showTempPredicted && hoveredPoint.tempPredictedC !== null}<div class="tooltip-row"><span class="tooltip-dot" style:background="#9b8fd8"></span> Pred: {hoveredPoint.tempPredictedC.toFixed(1)}°C</div>{/if}
						</div>
					{/if}
				</div>
			{/if}
		</div>
	{:else}
		<div class="chart-placeholder">
			Waiting for data... Press Play to start the simulation.
		</div>
	{/if}

	<!-- Legend toggles -->
	<div class="legend">
		{#each SERIES as s}
			<button
				class="legend-item"
				class:off={!isVisible(s.key)}
				onclick={() => toggle(s.key)}
			>
				<span class="legend-dot" style:background={isVisible(s.key) ? s.color : '#ccc'}></span>
				{s.label}
			</button>
		{/each}
	</div>
	{/if}
</div>

<style>
	.chart-card {
		background: #fff;
		border: 1px solid #e8ecf1;
		border-radius: 14px;
		padding: 16px;
		box-shadow: 0 1px 3px rgba(0, 0, 0, 0.04);
	}

	.card-header {
		display: flex;
		align-items: center;
		gap: 8px;
		width: 100%;
		background: none;
		border: none;
		padding: 0;
		cursor: pointer;
		font-family: inherit;
		text-align: left;
	}

	.arrow {
		font-size: 10px;
		color: #94a3b8;
		transition: transform 0.15s;
		display: inline-block;
	}

	.arrow.open {
		transform: rotate(90deg);
	}

	.card-title {
		font-size: 14px;
		font-weight: 600;
		color: #475569;
	}

	.chart-toolbar {
		display: flex;
		justify-content: flex-start;
		margin-top: 12px;
		margin-bottom: 12px;
	}

	.window-buttons {
		display: flex;
		gap: 4px;
	}

	.window-btn {
		padding: 4px 12px;
		border: 1px solid #e8ecf1;
		border-radius: 20px;
		background: #fff;
		color: #64748b;
		font-size: 12px;
		font-weight: 500;
		cursor: pointer;
		transition: all 0.15s;
		font-family: inherit;
	}

	.window-btn:hover {
		background: #f1f5f9;
		border-color: #94a3b8;
	}

	.window-btn.active {
		background: #475569;
		color: #fff;
		border-color: #475569;
	}

	.panel-label {
		font-size: 11px;
		font-weight: 600;
		color: #94a3b8;
		text-transform: uppercase;
		letter-spacing: 0.5px;
		margin-bottom: 4px;
		margin-top: 8px;
		display: flex;
		align-items: center;
		gap: 4px;
	}

	.panel-unit {
		font-weight: 400;
		text-transform: none;
		letter-spacing: 0;
	}

	.chart-wrapper {
		position: relative;
	}

	.power-chart {
		height: 220px;
	}

	.secondary-chart {
		height: 140px;
	}

	.chart-placeholder {
		height: 200px;
		display: flex;
		align-items: center;
		justify-content: center;
		color: #94a3b8;
		font-size: 14px;
	}

	.crosshair {
		position: absolute;
		top: 0;
		bottom: 24px;
		width: 1px;
		background: #94a3b8;
		opacity: 0.5;
		pointer-events: none;
	}

	.tooltip-card {
		position: absolute;
		top: 8px;
		transform: translateX(-50%);
		background: #fff;
		border: 1px solid #e8ecf1;
		border-radius: 8px;
		padding: 8px 10px;
		font-size: 11px;
		box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
		pointer-events: none;
		white-space: nowrap;
		z-index: 10;
	}

	.tooltip-time {
		font-weight: 600;
		color: #475569;
		margin-bottom: 4px;
	}

	.tooltip-row {
		display: flex;
		align-items: center;
		gap: 4px;
		color: #64748b;
		line-height: 1.6;
	}

	.tooltip-dot {
		width: 8px;
		height: 8px;
		border-radius: 50%;
		display: inline-block;
		flex-shrink: 0;
	}

	.legend {
		display: flex;
		flex-wrap: wrap;
		gap: 4px;
		margin-top: 12px;
		justify-content: center;
	}

	.legend-item {
		display: flex;
		align-items: center;
		gap: 4px;
		padding: 3px 10px;
		border: 1px solid #e8ecf1;
		border-radius: 12px;
		background: #fff;
		color: #475569;
		font-size: 11px;
		cursor: pointer;
		transition: all 0.15s;
		font-family: inherit;
	}

	.legend-item:hover {
		background: #f1f5f9;
	}

	.legend-item.off {
		color: #cbd5e1;
		background: #f8fafc;
	}

	.legend-dot {
		width: 8px;
		height: 8px;
		border-radius: 50%;
		display: inline-block;
		flex-shrink: 0;
	}

	:global(.chart-card .chart-grid line) {
		stroke: #eef2f6;
	}

	:global(.chart-card .axis-label) {
		fill: #64748b;
		font-size: 11px;
	}

	:global(.chart-card .zero-line line) {
		stroke: #cbd5e1;
		stroke-dasharray: 4 3;
	}
</style>
