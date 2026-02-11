<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { simulation } from '$lib/stores/simulation';
	import SimControls from '$lib/components/SimControls.svelte';
	import SensorCard from '$lib/components/SensorCard.svelte';
	import EnergySummary from '$lib/components/EnergySummary.svelte';
	import PowerChart from '$lib/components/PowerChart.svelte';

	onMount(() => {
		simulation.init();
	});

	onDestroy(() => {
		simulation.destroy();
	});
</script>

<svelte:head>
	<title>Energy Simulator</title>
</svelte:head>

<div class="dashboard">
	<header>
		<h1>Energy Simulator</h1>
	</header>

	<SimControls />

	<div class="grid-top">
		<SensorCard />
		<EnergySummary />
	</div>

	<PowerChart />
</div>

<style>
	:global(body) {
		margin: 0;
		background: #0f0f23;
		color: #e0e0e0;
		font-family:
			-apple-system,
			BlinkMacSystemFont,
			'Segoe UI',
			Roboto,
			sans-serif;
	}

	.dashboard {
		max-width: 1200px;
		margin: 0 auto;
		padding: 20px;
		display: flex;
		flex-direction: column;
		gap: 16px;
	}

	header h1 {
		margin: 0;
		font-size: 22px;
		font-weight: 600;
		color: #e0e0e0;
	}

	.grid-top {
		display: grid;
		grid-template-columns: 1fr 2fr;
		gap: 16px;
	}

	@media (max-width: 768px) {
		.grid-top {
			grid-template-columns: 1fr;
		}
	}
</style>
