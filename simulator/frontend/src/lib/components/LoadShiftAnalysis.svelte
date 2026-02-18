<script lang="ts">
	import { simulation } from '$lib/stores/simulation.svelte';

	let collapsed = $state(false);
	let stats = $derived(simulation.loadShiftStats);
	let hasData = $derived(stats !== null && stats.avg_hp_price > 0);

	const dayLabels = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'];

	function cellColor(avgPrice: number, minPrice: number, maxPrice: number): string {
		if (maxPrice <= minPrice) return '#f0f0f0';
		const t = (avgPrice - minPrice) / (maxPrice - minPrice);
		// Green (#5bb88a) to coral (#e87c6c)
		const r = Math.round(91 + t * (232 - 91));
		const g = Math.round(184 - t * (184 - 124));
		const b = Math.round(138 - t * (138 - 108));
		return `rgb(${r}, ${g}, ${b})`;
	}

	let priceRange = $derived.by(() => {
		if (!stats) return { min: 0, max: 1 };
		let min = Infinity, max = -Infinity;
		for (let dow = 0; dow < 7; dow++) {
			for (let h = 0; h < 24; h++) {
				const cell = stats.heatmap[dow][h];
				if (cell.kwh > 0 && cell.avg_price > 0) {
					if (cell.avg_price < min) min = cell.avg_price;
					if (cell.avg_price > max) max = cell.avg_price;
				}
			}
		}
		if (min === Infinity) return { min: 0, max: 1 };
		return { min, max };
	});

	let hpTimingPct = $derived.by(() => {
		if (!stats || stats.overall_avg_price <= 0) return 0;
		return ((stats.overall_avg_price - stats.avg_hp_price) / stats.overall_avg_price) * 100;
	});

	let shiftSavingsPct = $derived.by(() => {
		if (!stats || stats.shift_current_pln <= 0) return 0;
		return (stats.shift_savings_pln / stats.shift_current_pln) * 100;
	});
</script>

{#if hasData}
	<div class="load-shift-card">
		<button class="collapse-header" onclick={() => (collapsed = !collapsed)}>
			<span class="section-title">Load Shifting Analysis</span>
			<span class="collapse-icon">{collapsed ? '+' : '-'}</span>
		</button>

		{#if !collapsed}
			<div class="content">
				<!-- HP Timing Efficiency -->
				<div class="timing-row">
					<span class="timing-label">HP avg price:</span>
					<span class="timing-value">{stats?.avg_hp_price.toFixed(2)} PLN/kWh</span>
					<span class="timing-vs">vs overall:</span>
					<span class="timing-value">{stats?.overall_avg_price.toFixed(2)} PLN/kWh</span>
					{#if hpTimingPct > 0}
						<span class="timing-badge good">({hpTimingPct.toFixed(0)}% better)</span>
					{:else if hpTimingPct < 0}
						<span class="timing-badge bad">({Math.abs(hpTimingPct).toFixed(0)}% worse)</span>
					{/if}
				</div>

				<!-- Shift Potential -->
				{#if stats && stats.shift_savings_pln > 0}
					<div class="shift-row">
						<span class="shift-label">&plusmn;{stats.shift_window_h}h shifting could save</span>
						<span class="shift-value">{stats.shift_savings_pln.toFixed(2)} PLN</span>
						<span class="shift-pct">({shiftSavingsPct.toFixed(1)}%)</span>
					</div>
				{/if}

				<!-- Heatmap -->
				<div class="heatmap-container">
					<div class="heatmap-header">
						<div class="heatmap-label"></div>
						{#each Array(24) as _, h}
							<div class="heatmap-hour">{h}</div>
						{/each}
					</div>
					{#each dayLabels as day, dow}
						<div class="heatmap-row">
							<div class="heatmap-label">{day}</div>
							{#each Array(24) as _, h}
								{@const cell = stats?.heatmap[dow][h]}
								<div
									class="heatmap-cell"
									style="background: {cell && cell.kwh > 0.01 ? cellColor(cell.avg_price, priceRange.min, priceRange.max) : '#f5f5f5'}"
									title="{day} {h}:00 — {cell?.kwh.toFixed(2)} kWh @ {cell?.avg_price.toFixed(2)} PLN/kWh"
								></div>
							{/each}
						</div>
					{/each}
					<div class="heatmap-legend">
						<span class="legend-low">Cheap</span>
						<div class="legend-gradient"></div>
						<span class="legend-high">Expensive</span>
					</div>
				</div>
			</div>
		{/if}
	</div>
{/if}

<style>
	.load-shift-card {
		background: #fff;
		border: 1px solid #e8ecf1;
		border-radius: 14px;
		box-shadow: 0 1px 4px rgba(0, 0, 0, 0.03);
		overflow: hidden;
	}

	.collapse-header {
		display: flex;
		justify-content: space-between;
		align-items: center;
		width: 100%;
		padding: 14px 20px;
		background: none;
		border: none;
		cursor: pointer;
		font-family: inherit;
	}

	.section-title {
		font-size: 11px;
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.06em;
		color: #94a3b8;
	}

	.collapse-icon {
		font-size: 16px;
		color: #94a3b8;
		font-weight: 600;
	}

	.content {
		padding: 0 20px 14px;
		display: flex;
		flex-direction: column;
		gap: 12px;
	}

	.timing-row, .shift-row {
		display: flex;
		align-items: center;
		gap: 6px;
		font-size: 13px;
		flex-wrap: wrap;
	}

	.timing-label, .shift-label {
		color: #64748b;
	}

	.timing-value, .shift-value {
		font-family: 'SF Mono', 'Cascadia Code', 'Fira Code', monospace;
		font-weight: 600;
		color: #222;
	}

	.timing-vs {
		color: #94a3b8;
	}

	.timing-badge {
		font-size: 12px;
		font-weight: 600;
		padding: 1px 6px;
		border-radius: 4px;
	}

	.timing-badge.good {
		color: #5bb88a;
		background: #edf8f2;
	}

	.timing-badge.bad {
		color: #e87c6c;
		background: #fef2f0;
	}

	.shift-pct {
		color: #5bb88a;
		font-weight: 600;
	}

	.heatmap-container {
		display: flex;
		flex-direction: column;
		gap: 1px;
	}

	.heatmap-header, .heatmap-row {
		display: grid;
		grid-template-columns: 36px repeat(24, 1fr);
		gap: 1px;
	}

	.heatmap-hour {
		font-size: 9px;
		color: #94a3b8;
		text-align: center;
	}

	.heatmap-label {
		font-size: 10px;
		color: #64748b;
		text-align: right;
		padding-right: 4px;
		line-height: 16px;
	}

	.heatmap-cell {
		height: 16px;
		border-radius: 2px;
		cursor: default;
	}

	.heatmap-legend {
		display: flex;
		align-items: center;
		gap: 8px;
		margin-top: 6px;
		font-size: 10px;
		color: #94a3b8;
		justify-content: center;
	}

	.legend-gradient {
		width: 80px;
		height: 8px;
		border-radius: 4px;
		background: linear-gradient(to right, #5bb88a, #e87c6c);
	}
</style>
