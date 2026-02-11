<script lang="ts">
	import { simulation } from '$lib/stores/simulation.svelte';

	function formatPower(value: number): string {
		if (Math.abs(value) >= 1000) {
			return (value / 1000).toFixed(2) + ' kW';
		}
		return value.toFixed(0) + ' W';
	}
</script>

<div class="card">
	<div class="card-header">Grid Power</div>
	<div class="card-value" class:exporting={simulation.currentPower < 0}>
		{formatPower(simulation.currentPower)}
	</div>
	<div class="card-label">
		{#if simulation.currentPower < 0}
			Exporting to grid
		{:else}
			Consuming from grid
		{/if}
	</div>
</div>

<style>
	.card {
		background: #1a1a2e;
		border: 1px solid #2a2a4a;
		border-radius: 8px;
		padding: 20px;
		text-align: center;
	}

	.card-header {
		color: #888;
		font-size: 13px;
		text-transform: uppercase;
		letter-spacing: 1px;
		margin-bottom: 8px;
	}

	.card-value {
		font-size: 36px;
		font-weight: 700;
		color: #ff6b35;
		font-family: monospace;
	}

	.card-value.exporting {
		color: #44cc44;
	}

	.card-label {
		color: #666;
		font-size: 12px;
		margin-top: 4px;
	}
</style>
