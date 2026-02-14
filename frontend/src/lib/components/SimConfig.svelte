<script lang="ts">
	import { simulation } from '$lib/stores/simulation.svelte';

	function handleExportCoeffChange(e: Event) {
		const target = e.target as HTMLInputElement;
		simulation.exportCoefficient = Number(target.value);
		simulation.sendConfig();
	}

	function handlePriceThresholdChange(e: Event) {
		const target = e.target as HTMLInputElement;
		simulation.priceThresholdPLN = Number(target.value);
		simulation.sendConfig();
	}

	function handleTempOffsetChange(e: Event) {
		const target = e.target as HTMLInputElement;
		simulation.tempOffsetC = Number(target.value);
		simulation.sendConfig();
	}
</script>

<div class="config-panel">
	<div class="section-title">Simulation Config</div>
	<div class="config-row">
		<label class="config-item">
			<span class="config-label">Export Coefficient</span>
			<input
				type="number"
				min="0"
				max="1"
				step="0.05"
				value={simulation.exportCoefficient}
				onchange={handleExportCoeffChange}
			/>
		</label>
		<label class="config-item">
			<span class="config-label">Cheap Export Threshold (PLN)</span>
			<input
				type="number"
				min="-1"
				max="2"
				step="0.05"
				value={simulation.priceThresholdPLN}
				onchange={handlePriceThresholdChange}
			/>
		</label>
		<label class="config-item">
			<span class="config-label">Temp Offset (C)</span>
			<input
				type="number"
				min="-10"
				max="10"
				step="0.5"
				value={simulation.tempOffsetC}
				onchange={handleTempOffsetChange}
			/>
		</label>
	</div>
</div>

<style>
	.config-panel {
		background: #fff;
		border: 1px solid #e5e7eb;
		border-radius: 12px;
		padding: 14px 20px;
		box-shadow: 0 1px 3px rgba(0, 0, 0, 0.04);
	}

	.section-title {
		font-size: 11px;
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.06em;
		color: #94a3b8;
		margin-bottom: 10px;
	}

	.config-row {
		display: grid;
		grid-template-columns: repeat(3, 1fr);
		gap: 12px;
	}

	.config-item {
		display: flex;
		flex-direction: column;
		gap: 4px;
	}

	.config-label {
		font-size: 11px;
		color: #64748b;
		text-transform: uppercase;
		letter-spacing: 0.5px;
	}

	input[type='number'] {
		background: #f8fafc;
		color: #222;
		border: 1px solid #d1d5db;
		border-radius: 6px;
		padding: 6px 10px;
		font-size: 13px;
		font-family: 'SF Mono', 'Cascadia Code', 'Fira Code', monospace;
		width: 100%;
		box-sizing: border-box;
	}
</style>
