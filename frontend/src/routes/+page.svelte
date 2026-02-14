<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { simulation } from '$lib/stores/simulation.svelte';
	import SimControls from '$lib/components/SimControls.svelte';
	import EnergySummary from '$lib/components/EnergySummary.svelte';
	import CostSummary from '$lib/components/CostSummary.svelte';
	import HomeSchema from '$lib/components/HomeSchema.svelte';
	import BatteryConfig from '$lib/components/BatteryConfig.svelte';
	import BatteryStats from '$lib/components/BatteryStats.svelte';
	import SoCHeatmap from '$lib/components/SoCHeatmap.svelte';
	import OffGridHeatmap from '$lib/components/OffGridHeatmap.svelte';
	import ArbitrageLog from '$lib/components/ArbitrageLog.svelte';
	import SimConfig from '$lib/components/SimConfig.svelte';
	import PredictionComparison from '$lib/components/PredictionComparison.svelte';

	onMount(() => {
		simulation.init();
	});

	onDestroy(() => {
		simulation.destroy();
	});

	// Date display
	let simDate = $derived.by(() => {
		if (!simulation.simTime) return '';
		const d = new Date(simulation.simTime);
		return d.toLocaleDateString('en-GB', { day: 'numeric', month: 'short', year: 'numeric' });
	});

	// Day counter
	let dayInfo = $derived.by(() => {
		if (!simulation.simTime || !simulation.timeRangeStart || !simulation.timeRangeEnd) return '';
		const current = new Date(simulation.simTime).getTime();
		const start = new Date(simulation.timeRangeStart).getTime();
		const end = new Date(simulation.timeRangeEnd).getTime();
		const elapsed = Math.floor((current - start) / 86400000) + 1;
		const total = Math.floor((end - start) / 86400000) + 1;
		return `Day ${elapsed} of ${total}`;
	});

	let hasCostData = $derived(
		simulation.gridImportCostPLN > 0 || simulation.gridExportRevenuePLN > 0
	);

	function handleReset() {
		simulation.reset();
	}
</script>

<svelte:head>
	<title>Energy Simulator</title>
</svelte:head>

<div class="dashboard">
	<header>
		<h1>Energy Simulator</h1>
		{#if simDate}
			<div class="sim-info">
				<span class="sim-date">{simDate}</span>
				{#if dayInfo}
					<span class="sim-separator">&middot;</span>
					<span class="sim-day">{dayInfo}</span>
				{/if}
				<button class="reset-btn" onclick={handleReset} title="Reset to start">Reset</button>
			</div>
		{/if}
	</header>

	<SimControls />
	<HomeSchema />

	<div class="summary-grid" class:two-col={hasCostData}>
		<EnergySummary />
		{#if hasCostData}
			<CostSummary />
		{/if}
	</div>

	<div class="bottom-row">
		<div class="left-col">
			<BatteryConfig />
			<SimConfig />
		</div>
		<div class="right-col">
			<BatteryStats />
			<SoCHeatmap />
		</div>
	</div>

	<PredictionComparison />
	<OffGridHeatmap />
	<ArbitrageLog />
</div>

<style>
	:global(body) {
		margin: 0;
		background: #f8fafc;
		color: #222;
		font-family:
			-apple-system,
			BlinkMacSystemFont,
			'Segoe UI',
			Roboto,
			sans-serif;
	}

	.dashboard {
		max-width: 1100px;
		margin: 0 auto;
		padding: 24px;
		display: flex;
		flex-direction: column;
		gap: 16px;
	}

	header {
		display: flex;
		align-items: baseline;
		gap: 16px;
		flex-wrap: wrap;
	}

	header h1 {
		margin: 0;
		font-size: 22px;
		font-weight: 600;
		color: #222;
	}

	.sim-info {
		display: flex;
		align-items: center;
		gap: 8px;
		font-size: 14px;
		font-family: 'SF Mono', 'Cascadia Code', 'Fira Code', monospace;
		color: #64748b;
	}

	.sim-date {
		font-weight: 600;
		color: #475569;
	}

	.sim-separator {
		color: #cbd5e1;
	}

	.sim-day {
		color: #94a3b8;
	}

	.reset-btn {
		background: none;
		border: 1px solid #cbd5e1;
		border-radius: 6px;
		padding: 2px 10px;
		font-size: 12px;
		font-family: inherit;
		color: #64748b;
		cursor: pointer;
		transition: all 0.15s;
	}

	.reset-btn:hover {
		background: #f1f5f9;
		border-color: #94a3b8;
		color: #475569;
	}

	.summary-grid {
		display: grid;
		grid-template-columns: 1fr;
		gap: 16px;
	}

	.summary-grid.two-col {
		grid-template-columns: 1fr 1fr;
	}

	.bottom-row {
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: 16px;
	}

	.left-col, .right-col {
		display: flex;
		flex-direction: column;
		gap: 16px;
	}
</style>
