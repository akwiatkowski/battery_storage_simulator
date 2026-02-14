<script lang="ts">
	import { simulation } from '$lib/stores/simulation.svelte';

	let powerMAE = $derived(
		simulation.predPowerErrors.length > 0
			? simulation.predPowerErrors.reduce((a, b) => a + b, 0) / simulation.predPowerErrors.length
			: 0
	);

	let tempMAE = $derived(
		simulation.predTempErrors.length > 0
			? simulation.predTempErrors.reduce((a, b) => a + b, 0) / simulation.predTempErrors.length
			: 0
	);

	function formatW(v: number): string {
		if (Math.abs(v) >= 1000) return (v / 1000).toFixed(1) + ' kW';
		return v.toFixed(0) + ' W';
	}
</script>

{#if simulation.predHasData && !simulation.predictionEnabled}
	<div class="comparison-panel">
		<div class="section-title">NN Prediction vs Actual</div>
		<div class="comparison-grid">
			<div class="comp-section">
				<div class="comp-header">Grid Power</div>
				<div class="comp-row">
					<div class="comp-item">
						<span class="comp-label">Actual</span>
						<span class="comp-value">{formatW(simulation.predActualPowerW)}</span>
					</div>
					<div class="comp-item">
						<span class="comp-label">Predicted</span>
						<span class="comp-value predicted">{formatW(simulation.predPredictedPowerW)}</span>
					</div>
					<div class="comp-item">
						<span class="comp-label">MAE</span>
						<span class="comp-value mae">{formatW(powerMAE)}</span>
					</div>
				</div>
			</div>
			{#if simulation.predHasActualTemp}
				<div class="comp-section">
					<div class="comp-header">Temperature</div>
					<div class="comp-row">
						<div class="comp-item">
							<span class="comp-label">Actual</span>
							<span class="comp-value">{simulation.predActualTempC.toFixed(1)} C</span>
						</div>
						<div class="comp-item">
							<span class="comp-label">Predicted</span>
							<span class="comp-value predicted">{simulation.predPredictedTempC.toFixed(1)} C</span>
						</div>
						<div class="comp-item">
							<span class="comp-label">MAE</span>
							<span class="comp-value mae">{tempMAE.toFixed(1)} C</span>
						</div>
					</div>
				</div>
			{/if}
		</div>
		<div class="sample-count">{simulation.predPowerErrors.length} samples</div>
	</div>
{/if}

<style>
	.comparison-panel {
		background: #fff;
		border: 1px solid #e5e7eb;
		border-radius: 12px;
		padding: 16px 20px;
		border-left: 3px solid #8b5cf6;
		box-shadow: 0 1px 3px rgba(0, 0, 0, 0.04);
	}

	.section-title {
		font-size: 11px;
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.06em;
		color: #94a3b8;
		margin-bottom: 12px;
	}

	.comparison-grid {
		display: flex;
		flex-direction: column;
		gap: 12px;
	}

	.comp-header {
		font-size: 12px;
		font-weight: 600;
		color: #475569;
		margin-bottom: 6px;
	}

	.comp-row {
		display: grid;
		grid-template-columns: repeat(3, 1fr);
		gap: 12px;
	}

	.comp-item {
		text-align: center;
	}

	.comp-label {
		display: block;
		color: #888;
		font-size: 11px;
		text-transform: uppercase;
		letter-spacing: 0.5px;
		margin-bottom: 4px;
	}

	.comp-value {
		display: block;
		font-size: 18px;
		font-weight: 600;
		color: #222;
		font-family: 'SF Mono', 'Cascadia Code', 'Fira Code', monospace;
	}

	.comp-value.predicted {
		color: #8b5cf6;
	}

	.comp-value.mae {
		color: #f59e0b;
	}

	.sample-count {
		margin-top: 8px;
		font-size: 11px;
		color: #94a3b8;
		text-align: right;
	}
</style>
