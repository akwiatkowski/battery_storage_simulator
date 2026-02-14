<script lang="ts">
	import { simulation } from '$lib/stores/simulation.svelte';

	const EV_KWH_PER_100KM = 18;

	function formatPLN(value: number): string {
		return value.toFixed(2) + ' PLN';
	}

	let hasCostData = $derived(
		simulation.gridImportCostPLN > 0 || simulation.gridExportRevenuePLN > 0
	);

	let hasBatterySavings = $derived(
		simulation.batteryEnabled && simulation.batterySavingsPLN > 0
	);

	let hasArbData = $derived(
		simulation.batteryEnabled && simulation.arbBatterySavingsPLN > 0
	);

	let hasCheapExport = $derived(simulation.cheapExportKWh > 0);

	let cheapExportPct = $derived(
		simulation.gridExportKWh > 0
			? ((simulation.cheapExportKWh / simulation.gridExportKWh) * 100).toFixed(0)
			: '0'
	);

	let avgImportPrice = $derived(
		simulation.gridImportKWh > 0
			? simulation.gridImportCostPLN / simulation.gridImportKWh
			: 0
	);

	let evCostPer100km = $derived(avgImportPrice * EV_KWH_PER_100KM);
	let evKmFromExport = $derived(
		simulation.gridExportKWh > 0
			? (simulation.gridExportKWh / EV_KWH_PER_100KM) * 100
			: 0
	);
</script>

{#if hasCostData}
	<div class="cost-sections">
		<div class="section-title">Energy Costs</div>
		<div class="cost-cards">
			<div class="cost-card import">
				<span class="cost-label">Import Cost</span>
				<span class="cost-value">{formatPLN(simulation.gridImportCostPLN)}</span>
			</div>
			<div class="cost-card export">
				<span class="cost-label">Export Revenue</span>
				<span class="cost-value">{formatPLN(simulation.gridExportRevenuePLN)}</span>
			</div>
			<div class="cost-card net">
				<span class="cost-label">Net Cost</span>
				<span class="cost-value">{formatPLN(simulation.netCostPLN)}</span>
			</div>
		</div>

		{#if hasArbData}
			<div class="battery-comparison">
				<div class="comparison-title">Strategy Comparison</div>
				<div class="comparison-row three-col">
					<div class="comparison-item">
						<span class="comp-label">No Battery</span>
						<span class="comp-value muted">{formatPLN(simulation.rawNetCostPLN)}</span>
					</div>
					<div class="comparison-item">
						<span class="comp-label">Self-Consumption</span>
						<span class="comp-value">{formatPLN(simulation.netCostPLN)}</span>
						<span class="comp-saved">saved {formatPLN(simulation.batterySavingsPLN)}</span>
					</div>
					<div class="comparison-item">
						<span class="comp-label">Arbitrage</span>
						<span class="comp-value">{formatPLN(simulation.arbNetCostPLN)}</span>
						<span class="comp-saved"
							>saved {formatPLN(simulation.arbBatterySavingsPLN)}</span
						>
					</div>
				</div>
			</div>
		{:else if hasBatterySavings}
			<div class="battery-comparison">
				<div class="comparison-title">Battery Cost Impact</div>
				<div class="comparison-row">
					<div class="comparison-item">
						<span class="comp-label">Without Battery</span>
						<span class="comp-value muted">{formatPLN(simulation.rawNetCostPLN)}</span>
					</div>
					<div class="comparison-item">
						<span class="comp-label">With Battery</span>
						<span class="comp-value">{formatPLN(simulation.netCostPLN)}</span>
					</div>
					<div class="comparison-item">
						<span class="comp-label">Saved</span>
						<span class="comp-value saved">{formatPLN(simulation.batterySavingsPLN)}</span>
					</div>
				</div>
			</div>
		{/if}

		{#if hasCheapExport}
			<div class="battery-comparison">
				<div class="comparison-title">Cheap Export</div>
				<div class="comparison-row">
					<div class="comparison-item">
						<span class="comp-label">Energy</span>
						<span class="comp-value warning">{simulation.cheapExportKWh.toFixed(1)} kWh</span>
					</div>
					<div class="comparison-item">
						<span class="comp-label">Revenue</span>
						<span class="comp-value warning">{formatPLN(simulation.cheapExportRevPLN)}</span>
					</div>
					<div class="comparison-item">
						<span class="comp-label">% of Export</span>
						<span class="comp-value warning">{cheapExportPct}%</span>
					</div>
				</div>
			</div>
		{/if}

		{#if simulation.gridImportKWh > 0}
			<div class="battery-comparison">
				<div class="comparison-title">EV Range (18 kWh/100km)</div>
				<div class="comparison-row">
					<div class="comparison-item">
						<span class="comp-label">Avg Import Price</span>
						<span class="comp-value">{avgImportPrice.toFixed(2)} PLN/kWh</span>
					</div>
					<div class="comparison-item">
						<span class="comp-label">Cost/100km</span>
						<span class="comp-value">{formatPLN(evCostPer100km)}</span>
					</div>
					<div class="comparison-item">
						<span class="comp-label">km from Export</span>
						<span class="comp-value">{evKmFromExport.toFixed(0)} km</span>
					</div>
				</div>
			</div>
		{/if}
	</div>
{/if}

<style>
	.cost-sections {
		background: #fff;
		border: 1px solid #e5e7eb;
		border-radius: 12px;
		padding: 16px 20px;
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

	.cost-cards {
		display: grid;
		grid-template-columns: repeat(3, 1fr);
		gap: 12px;
	}

	.cost-card {
		text-align: center;
		padding: 12px 8px;
		border-radius: 8px;
		background: #f8fafc;
		border-left: 3px solid transparent;
	}

	.cost-card.import {
		border-left-color: #ef4444;
	}

	.cost-card.export {
		border-left-color: #22c55e;
	}

	.cost-card.net {
		border-left-color: #3b82f6;
	}

	.cost-label {
		display: block;
		color: #888;
		font-size: 11px;
		text-transform: uppercase;
		letter-spacing: 0.5px;
		margin-bottom: 4px;
	}

	.cost-value {
		display: block;
		font-size: 20px;
		font-weight: 600;
		color: #222;
		font-family: 'SF Mono', 'Cascadia Code', 'Fira Code', monospace;
	}

	.battery-comparison {
		margin-top: 16px;
		padding-top: 12px;
		border-top: 1px solid #f1f5f9;
	}

	.comparison-title {
		font-size: 11px;
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.06em;
		color: #94a3b8;
		margin-bottom: 8px;
	}

	.comparison-row {
		display: grid;
		grid-template-columns: repeat(3, 1fr);
		gap: 12px;
	}

	.comparison-item {
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

	.comp-value.muted {
		color: #94a3b8;
	}

	.comp-value.saved {
		color: #16a34a;
	}

	.comp-value.warning {
		color: #d97706;
	}

	.comp-saved {
		display: block;
		font-size: 11px;
		color: #16a34a;
		font-family: 'SF Mono', 'Cascadia Code', 'Fira Code', monospace;
		margin-top: 2px;
	}
</style>
