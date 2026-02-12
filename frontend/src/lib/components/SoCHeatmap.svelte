<script lang="ts">
	import { simulation } from '$lib/stores/simulation.svelte';

	const socBuckets = [0, 10, 20, 30, 40, 50, 60, 70, 80, 90, 100];

	let months = $derived.by(() => {
		const data = simulation.batteryMonthSoCSeconds;
		if (!data || Object.keys(data).length === 0) return [];
		return Object.keys(data).sort();
	});

	let maxHours = $derived.by(() => {
		const data = simulation.batteryMonthSoCSeconds;
		let max = 0;
		for (const monthData of Object.values(data)) {
			for (const sec of Object.values(monthData)) {
				const h = sec / 3600;
				if (h > max) max = h;
			}
		}
		return max || 1;
	});

	function getHours(month: string, bucket: number): number {
		const data = simulation.batteryMonthSoCSeconds;
		return (data[month]?.[String(bucket)] ?? 0) / 3600;
	}

	function cellColor(hours: number): string {
		if (hours === 0) return '#f1f5f9';
		const intensity = Math.min(1, hours / maxHours);
		// Green gradient: light to dark
		const r = Math.round(220 - intensity * 186);
		const g = Math.round(240 - intensity * 43);
		const b = Math.round(220 - intensity * 186);
		return `rgb(${r}, ${g}, ${b})`;
	}

	function formatMonth(month: string): string {
		const [y, m] = month.split('-');
		const months = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];
		return months[parseInt(m) - 1] + ' ' + y.slice(2);
	}
</script>

{#if simulation.batteryEnabled && months.length > 0}
	<div class="heatmap-card">
		<div class="heatmap-title">SoC Distribution by Month</div>
		<div class="heatmap-container">
			<div class="heatmap-grid" style="grid-template-columns: 48px repeat({months.length}, 1fr)">
				<!-- Header row -->
				<div class="corner"></div>
				{#each months as month}
					<div class="col-label">{formatMonth(month)}</div>
				{/each}

				<!-- Data rows (top = 100%, bottom = 0%) -->
				{#each [...socBuckets].reverse() as bucket}
					<div class="row-label">{bucket}%</div>
					{#each months as month}
						{@const hours = getHours(month, bucket)}
						<div
							class="cell"
							style="background: {cellColor(hours)}"
							title="{formatMonth(month)} · SoC {bucket}-{bucket + 10}% · {hours.toFixed(1)}h"
						></div>
					{/each}
				{/each}
			</div>
		</div>
	</div>
{/if}

<style>
	.heatmap-card {
		background: #fafbfc;
		border: 1px solid #e5e7eb;
		border-radius: 12px;
		padding: 16px;
	}

	.heatmap-title {
		font-size: 12px;
		font-weight: 600;
		color: #475569;
		text-transform: uppercase;
		letter-spacing: 0.04em;
		margin-bottom: 12px;
	}

	.heatmap-container {
		overflow-x: auto;
	}

	.heatmap-grid {
		display: grid;
		gap: 2px;
		min-width: fit-content;
	}

	.corner {
		min-height: 1px;
	}

	.col-label {
		font-size: 10px;
		font-family: 'SF Mono', 'Cascadia Code', 'Fira Code', monospace;
		color: #64748b;
		text-align: center;
		padding-bottom: 4px;
		white-space: nowrap;
	}

	.row-label {
		font-size: 10px;
		font-family: 'SF Mono', 'Cascadia Code', 'Fira Code', monospace;
		color: #64748b;
		text-align: right;
		padding-right: 6px;
		display: flex;
		align-items: center;
		justify-content: flex-end;
	}

	.cell {
		width: 100%;
		min-width: 28px;
		height: 18px;
		border-radius: 3px;
		cursor: default;
		transition: opacity 0.15s;
	}

	.cell:hover {
		opacity: 0.8;
		outline: 1px solid #94a3b8;
	}
</style>
