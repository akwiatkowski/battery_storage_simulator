<script lang="ts">
	import { simulation } from '$lib/stores/simulation';

	function formatKWh(value: number): string {
		if (value >= 1000) {
			return (value / 1000).toFixed(2) + ' MWh';
		}
		return value.toFixed(1) + ' kWh';
	}
</script>

<div class="summary">
	<div class="summary-item">
		<span class="label">Today</span>
		<span class="value">{formatKWh(simulation.todayKWh)}</span>
	</div>
	<div class="summary-item">
		<span class="label">This Month</span>
		<span class="value">{formatKWh(simulation.monthKWh)}</span>
	</div>
	<div class="summary-item">
		<span class="label">Total</span>
		<span class="value">{formatKWh(simulation.totalKWh)}</span>
	</div>
</div>

<style>
	.summary {
		display: grid;
		grid-template-columns: repeat(3, 1fr);
		gap: 12px;
	}

	.summary-item {
		background: #1a1a2e;
		border: 1px solid #2a2a4a;
		border-radius: 8px;
		padding: 16px;
		text-align: center;
	}

	.label {
		display: block;
		color: #888;
		font-size: 12px;
		text-transform: uppercase;
		letter-spacing: 1px;
		margin-bottom: 8px;
	}

	.value {
		display: block;
		font-size: 22px;
		font-weight: 600;
		color: #e0e0e0;
		font-family: monospace;
	}
</style>
