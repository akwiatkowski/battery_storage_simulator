<script lang="ts">
	import { simulation } from '$lib/stores/simulation.svelte';
	import HelpTip from './HelpTip.svelte';

	let pq = $derived(simulation.powerQuality);
	let hasData = $derived(pq !== null && pq.voltage_v > 0);

	let voltageStatus = $derived.by(() => {
		if (!pq) return 'normal';
		if (pq.voltage_v >= 253) return 'danger';
		if (pq.voltage_v >= 245) return 'elevated';
		if (pq.voltage_v < 210) return 'low';
		return 'normal';
	});

	let pfStatus = $derived.by(() => {
		if (!pq) return 'normal';
		if (pq.power_factor_pct < 80) return 'danger';
		if (pq.power_factor_pct < 90) return 'warning';
		return 'normal';
	});
</script>

{#if hasData && pq}
<div class="card">
	<h3>Power Quality <HelpTip key="power_quality" /></h3>

	<div class="pq-grid">
		<div class="pq-item">
			<span class="pq-label">Grid Voltage</span>
			<span class="pq-value {voltageStatus}">
				{pq.voltage_v.toFixed(1)} V
			</span>
			{#if voltageStatus === 'danger'}
				<span class="pq-badge danger">Curtailment risk</span>
			{:else if voltageStatus === 'elevated'}
				<span class="pq-badge elevated">Elevated</span>
			{/if}
		</div>

		<div class="pq-item">
			<span class="pq-label">Power Factor</span>
			<span class="pq-value {pfStatus}">
				{pq.power_factor_pct.toFixed(1)}%
			</span>
			{#if pfStatus === 'danger'}
				<span class="pq-badge danger">Poor</span>
			{:else if pfStatus === 'warning'}
				<span class="pq-badge warning">Below target</span>
			{/if}
		</div>

		<div class="pq-item">
			<span class="pq-label">Reactive Power</span>
			<span class="pq-value">{pq.reactive_power_var.toFixed(0)} VAR</span>
		</div>
	</div>
</div>
{/if}

<style>
	.card {
		background: #fff;
		border: 1px solid #e8ecf1;
		border-radius: 14px;
		padding: 20px;
	}

	h3 {
		margin: 0 0 12px;
		font-size: 15px;
		font-weight: 600;
		color: #334155;
		display: flex;
		align-items: center;
		gap: 6px;
	}

	.pq-grid {
		display: flex;
		gap: 24px;
	}

	.pq-item {
		display: flex;
		flex-direction: column;
		gap: 2px;
	}

	.pq-label {
		font-size: 12px;
		color: #94a3b8;
		font-weight: 500;
	}

	.pq-value {
		font-family: 'SF Mono', 'Cascadia Code', 'Fira Code', monospace;
		font-size: 16px;
		font-weight: 600;
		color: #334155;
	}

	.pq-value.danger {
		color: #e87c6c;
	}

	.pq-value.elevated {
		color: #e0a040;
	}

	.pq-value.warning {
		color: #e0a040;
	}

	.pq-value.low {
		color: #e87c6c;
	}

	.pq-badge {
		font-size: 10px;
		font-weight: 600;
		padding: 1px 6px;
		border-radius: 4px;
		width: fit-content;
	}

	.pq-badge.danger {
		background: #fef2f2;
		color: #e87c6c;
	}

	.pq-badge.elevated {
		background: #fffbeb;
		color: #d97706;
	}

	.pq-badge.warning {
		background: #fffbeb;
		color: #d97706;
	}
</style>
