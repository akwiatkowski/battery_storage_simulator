<script lang="ts">
	import { simulation } from '$lib/stores/simulation.svelte';

	function formatDuration(seconds: number): string {
		if (seconds < 60) return `${seconds.toFixed(0)}s`;
		if (seconds < 3600) return `${(seconds / 60).toFixed(0)}m`;
		return `${(seconds / 3600).toFixed(1)}h`;
	}

	let powerEntries = $derived.by(() => {
		const data = simulation.batteryTimeAtPowerSec;
		if (!data || Object.keys(data).length === 0) return [];
		const entries = Object.entries(data)
			.map(([k, v]) => ({ bucket: Number(k), seconds: v }))
			.sort((a, b) => a.bucket - b.bucket);
		return entries;
	});

	let powerMax = $derived(
		powerEntries.length > 0 ? Math.max(...powerEntries.map((e) => e.seconds)) : 1
	);

	let socEntries = $derived.by(() => {
		const data = simulation.batteryTimeAtSoCPctSec;
		if (!data || Object.keys(data).length === 0) return [];
		const entries = Object.entries(data)
			.map(([k, v]) => ({ bucket: Number(k), seconds: v }))
			.sort((a, b) => a.bucket - b.bucket);
		return entries;
	});

	let socMax = $derived(
		socEntries.length > 0 ? Math.max(...socEntries.map((e) => e.seconds)) : 1
	);
</script>

{#if simulation.batteryEnabled}
	<div class="battery-stats">
		<div class="stat-row">
			<span class="stat-label">Cycles</span>
			<span class="stat-value">{simulation.batteryCycles.toFixed(2)}</span>
		</div>

		{#if powerEntries.length > 0}
			<div class="histogram">
				<div class="histogram-title">Time at Power</div>
				{#each powerEntries as entry}
					<div class="bar-row">
						<span class="bar-label">{entry.bucket} kW</span>
						<div class="bar-track">
							<div
								class="bar-fill"
								class:bar-charge={entry.bucket < 0}
								class:bar-discharge={entry.bucket > 0}
								style="width: {(entry.seconds / powerMax) * 100}%"
							></div>
						</div>
						<span class="bar-value">{formatDuration(entry.seconds)}</span>
					</div>
				{/each}
			</div>
		{/if}

		{#if socEntries.length > 0}
			<div class="histogram">
				<div class="histogram-title">Time at SoC</div>
				{#each socEntries as entry}
					<div class="bar-row">
						<span class="bar-label">{entry.bucket}%</span>
						<div class="bar-track">
							<div
								class="bar-fill bar-soc"
								style="width: {(entry.seconds / socMax) * 100}%"
							></div>
						</div>
						<span class="bar-value">{formatDuration(entry.seconds)}</span>
					</div>
				{/each}
			</div>
		{/if}
	</div>
{/if}

<style>
	.battery-stats {
		background: #fafbfc;
		border: 1px solid #e5e7eb;
		border-radius: 12px;
		padding: 16px;
		display: flex;
		flex-direction: column;
		gap: 16px;
	}

	.stat-row {
		display: flex;
		justify-content: space-between;
		align-items: center;
	}

	.stat-label {
		font-size: 13px;
		font-weight: 500;
		color: #64748b;
		text-transform: uppercase;
		letter-spacing: 0.04em;
	}

	.stat-value {
		font-size: 18px;
		font-weight: 700;
		font-family: 'SF Mono', 'Cascadia Code', 'Fira Code', monospace;
		color: #334155;
	}

	.histogram {
		display: flex;
		flex-direction: column;
		gap: 4px;
	}

	.histogram-title {
		font-size: 12px;
		font-weight: 600;
		color: #475569;
		text-transform: uppercase;
		letter-spacing: 0.04em;
		margin-bottom: 4px;
	}

	.bar-row {
		display: flex;
		align-items: center;
		gap: 8px;
	}

	.bar-label {
		width: 48px;
		text-align: right;
		font-size: 12px;
		font-family: 'SF Mono', 'Cascadia Code', 'Fira Code', monospace;
		color: #64748b;
		flex-shrink: 0;
	}

	.bar-track {
		flex: 1;
		height: 14px;
		background: #f1f5f9;
		border-radius: 4px;
		overflow: hidden;
	}

	.bar-fill {
		height: 100%;
		border-radius: 4px;
		background: #94a3b8;
		min-width: 2px;
		transition: width 0.3s ease;
	}

	.bar-fill.bar-charge {
		background: #3b82f6;
	}

	.bar-fill.bar-discharge {
		background: #f59e0b;
	}

	.bar-fill.bar-soc {
		background: #22c55e;
	}

	.bar-value {
		width: 40px;
		text-align: right;
		font-size: 11px;
		font-family: 'SF Mono', 'Cascadia Code', 'Fira Code', monospace;
		color: #94a3b8;
		flex-shrink: 0;
	}
</style>
