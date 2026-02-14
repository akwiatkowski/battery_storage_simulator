<script lang="ts">
	import { simulation } from '$lib/stores/simulation.svelte';

	function formatKWh(value: number): string {
		if (value >= 1000) {
			return (value / 1000).toFixed(2) + ' MWh';
		}
		return value.toFixed(1) + ' kWh';
	}

	let hasPV = $derived(simulation.pvProductionKWh > 0);
	let hasHeatPump = $derived(simulation.heatPumpKWh > 0);
	let hasBattery = $derived(simulation.batteryEnabled);

	// Self-consumption percentage
	let selfConsumptionPct = $derived(
		simulation.pvProductionKWh > 0
			? ((simulation.selfConsumptionKWh / simulation.pvProductionKWh) * 100).toFixed(0)
			: '0'
	);

	// COP = thermal output / electrical input
	let cop = $derived(
		simulation.heatPumpKWh > 0
			? (simulation.heatPumpProdKWh / simulation.heatPumpKWh).toFixed(1)
			: '0.0'
	);

	let hasHeatPumpCost = $derived(simulation.heatPumpCostPLN > 0);
	let heatPumpAvgPrice = $derived(
		simulation.heatPumpKWh > 0
			? simulation.heatPumpCostPLN / simulation.heatPumpKWh
			: 0
	);

	// Appliance consumption = demand - heat pump
	let applianceKWh = $derived(
		Math.max(0, simulation.homeDemandKWh - simulation.heatPumpKWh)
	);

	// Battery comparison values
	let withoutBattery = $derived(simulation.gridImportKWh + simulation.batterySavingsKWh);
	let withBattery = $derived(simulation.gridImportKWh);

	// Savings per kWh of battery capacity
	let savingsPerKWh = $derived(
		simulation.batteryCapacityKWh > 0
			? simulation.batterySavingsKWh / simulation.batteryCapacityKWh
			: 0
	);

	// Off-grid coverage percentage
	let offGridPct = $derived(
		simulation.homeDemandKWh > 0
			? Math.min(100, ((simulation.selfConsumptionKWh + simulation.batterySavingsKWh) / simulation.homeDemandKWh) * 100)
			: 0
	);
</script>

<div class="summary-sections">
	<!-- Grid Import -->
	<div class="section grid-import">
		<div class="section-title">Grid Import</div>
		<div class="summary-row">
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
	</div>

	<!-- Energy Sources -->
	{#if hasPV}
		<div class="section pv">
			<div class="section-title">Energy Sources</div>
			<div class="summary-row">
				<div class="summary-item">
					<span class="label">PV Production</span>
					<span class="value accent-pv">{formatKWh(simulation.pvProductionKWh)}</span>
				</div>
				<div class="summary-item">
					<span class="label">Self-Consumption</span>
					<span class="value accent-pv">{formatKWh(simulation.selfConsumptionKWh)} <small>({selfConsumptionPct}%)</small></span>
				</div>
				<div class="summary-item">
					<span class="label">Grid Export</span>
					<span class="value accent-export">{formatKWh(simulation.gridExportKWh)}</span>
				</div>
			</div>
		</div>
	{/if}

	<!-- Home -->
	{#if hasHeatPump}
		<div class="section home">
			<div class="section-title">Home</div>
			<div class="summary-row">
				<div class="summary-item">
					<span class="label">Demand</span>
					<span class="value">{formatKWh(simulation.homeDemandKWh)}</span>
				</div>
				<div class="summary-item">
					<span class="label">Heat Pump <small>(COP {cop})</small></span>
					<span class="value accent-pump">{formatKWh(simulation.heatPumpKWh)}</span>
				</div>
				<div class="summary-item">
					<span class="label">Appliances</span>
					<span class="value">{formatKWh(applianceKWh)}</span>
				</div>
			</div>
			{#if hasHeatPumpCost}
				<div class="summary-row secondary">
					<div class="summary-item">
						<span class="label">HP Cost</span>
						<span class="value small accent-pump">{simulation.heatPumpCostPLN.toFixed(2)} PLN</span>
					</div>
					<div class="summary-item">
						<span class="label">Avg HP Price</span>
						<span class="value small">{heatPumpAvgPrice.toFixed(2)} PLN/kWh</span>
					</div>
					<div class="summary-item"></div>
				</div>
			{/if}
		</div>
	{/if}

	<!-- Battery Savings -->
	{#if hasBattery && simulation.batterySavingsKWh > 0}
		<div class="section battery">
			<div class="section-title">Battery Savings</div>
			<div class="summary-row">
				<div class="summary-item">
					<span class="label">Without Battery</span>
					<span class="value muted">{formatKWh(withoutBattery)}</span>
				</div>
				<div class="summary-item">
					<span class="label">With Battery</span>
					<span class="value">{formatKWh(withBattery)}</span>
				</div>
				<div class="summary-item">
					<span class="label">Saved</span>
					<span class="value accent-savings">{formatKWh(simulation.batterySavingsKWh)}</span>
				</div>
			</div>
			<div class="summary-row secondary">
				<div class="summary-item">
					<span class="label">Savings/kWh</span>
					<span class="value small">{savingsPerKWh.toFixed(1)} kWh</span>
				</div>
				<div class="summary-item">
					<span class="label">Off-Grid <span class="help-icon" title="Percentage of home energy demand covered by PV self-consumption and battery, without relying on grid import. Formula: (Self-Consumption + Battery Savings) / Home Demand &times; 100">?</span></span>
					<span class="value accent-savings small">{offGridPct.toFixed(1)}%</span>
				</div>
				<div class="summary-item"></div>
			</div>
		</div>
	{/if}
</div>

<style>
	.summary-sections {
		display: flex;
		flex-direction: column;
		gap: 12px;
	}

	.section {
		background: #fff;
		border: 1px solid #e5e7eb;
		border-radius: 12px;
		padding: 14px 20px;
		border-left: 3px solid transparent;
		box-shadow: 0 1px 3px rgba(0, 0, 0, 0.04);
	}

	.section.grid-import {
		border-left-color: #ef4444;
	}

	.section.pv {
		border-left-color: #eab308;
	}

	.section.home {
		border-left-color: #3b82f6;
	}

	.section.battery {
		border-left-color: #22c55e;
	}

	.section-title {
		font-size: 11px;
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.06em;
		color: #94a3b8;
		margin-bottom: 10px;
	}

	.summary-row {
		display: grid;
		grid-template-columns: repeat(3, 1fr);
		gap: 12px;
	}

	.summary-row.secondary {
		margin-top: 10px;
		padding-top: 10px;
		border-top: 1px solid #f1f5f9;
	}

	.summary-item {
		text-align: center;
	}

	.label {
		display: block;
		color: #888;
		font-size: 11px;
		text-transform: uppercase;
		letter-spacing: 0.5px;
		margin-bottom: 4px;
	}

	.label small {
		text-transform: none;
		letter-spacing: 0;
		font-size: 11px;
		color: #64748b;
	}

	.value {
		display: block;
		font-size: 24px;
		font-weight: 600;
		color: #222;
		font-family: 'SF Mono', 'Cascadia Code', 'Fira Code', monospace;
	}

	.value.small {
		font-size: 18px;
	}

	.value small {
		font-size: 13px;
		font-weight: 500;
		color: #64748b;
	}

	.value.accent-pv {
		color: #ca8a04;
	}

	.value.accent-export {
		color: #22c55e;
	}

	.value.accent-pump {
		color: #ea580c;
	}

	.value.accent-savings {
		color: #16a34a;
	}

	.value.muted {
		color: #94a3b8;
	}

	.help-icon {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 14px;
		height: 14px;
		border-radius: 50%;
		border: 1px solid #cbd5e1;
		font-size: 10px;
		font-family: 'SF Mono', 'Cascadia Code', 'Fira Code', monospace;
		color: #94a3b8;
		cursor: help;
		vertical-align: middle;
		line-height: 1;
		text-transform: none;
		letter-spacing: 0;
	}
</style>
