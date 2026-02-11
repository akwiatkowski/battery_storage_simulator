<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { simulation } from '$lib/stores/simulation.svelte';
	import SimControls from '$lib/components/SimControls.svelte';
	import EnergySummary from '$lib/components/EnergySummary.svelte';
	import HomeSchema from '$lib/components/HomeSchema.svelte';

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

	<HomeSchema />
	<EnergySummary />
	<SimControls />
</div>

<style>
	:global(body) {
		margin: 0;
		background: #fff;
		color: #222;
		font-family:
			-apple-system,
			BlinkMacSystemFont,
			'Segoe UI',
			Roboto,
			sans-serif;
	}

	.dashboard {
		max-width: 900px;
		margin: 0 auto;
		padding: 24px;
		display: flex;
		flex-direction: column;
		gap: 20px;
	}

	header h1 {
		margin: 0;
		font-size: 22px;
		font-weight: 600;
		color: #222;
	}
</style>
