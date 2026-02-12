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
	<!-- Row 1: Grid Import -->
	<div class="section">
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

	<!-- Row 2: Energy Sources (when PV data exists) -->
	{#if hasPV}
		<div class="section">
			<div class="section-title">Energy Sources</div>
			<div class="summary-row">
				<div class="summary-item">
					<span class="label">PV Production</span>
					<span class="value pv">{formatKWh(simulation.pvProductionKWh)}</span>
				</div>
				<div class="summary-item">
					<span class="label">Self-Consumption</span>
					<span class="value pv">{formatKWh(simulation.selfConsumptionKWh)} <small>({selfConsumptionPct}%)</small></span>
				</div>
				<div class="summary-item">
					<span class="label">Grid Export</span>
					<span class="value export">{formatKWh(simulation.gridExportKWh)}</span>
				</div>
			</div>
		</div>
	{/if}

	<!-- Row 3: Home (when heat pump data exists) -->
	{#if hasHeatPump}
		<div class="section">
			<div class="section-title">Home</div>
			<div class="summary-row">
				<div class="summary-item">
					<span class="label">Demand</span>
					<span class="value">{formatKWh(simulation.homeDemandKWh)}</span>
				</div>
				<div class="summary-item">
					<span class="label">Heat Pump <small>(COP {cop})</small></span>
					<span class="value heat-pump">{formatKWh(simulation.heatPumpKWh)}</span>
				</div>
				<div class="summary-item">
					<span class="label">Appliances</span>
					<span class="value">{formatKWh(applianceKWh)}</span>
				</div>
			</div>
		</div>
	{/if}

	<!-- Row 4: Battery Savings (when battery enabled) -->
	{#if hasBattery && simulation.batterySavingsKWh > 0}
		<div class="section">
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
					<span class="value savings">{formatKWh(simulation.batterySavingsKWh)}</span>
				</div>
			</div>
			<div class="summary-row" style="margin-top: 8px">
				<div class="summary-item">
					<span class="label">Savings/kWh</span>
					<span class="value">{savingsPerKWh.toFixed(1)} kWh</span>
				</div>
				<div class="summary-item">
					<span class="label">Off-Grid <span class="help-icon" title="Percentage of home energy demand covered by PV self-consumption and battery, without relying on grid import. Formula: (Self-Consumption + Battery Savings) / Home Demand &times; 100">?</span></span>
					<span class="value savings">{offGridPct.toFixed(1)}%</span>
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
		background: #f8f8f8;
		border: 1px solid #ddd;
		border-radius: 8px;
		padding: 12px 16px;
	}

	.section-title {
		font-size: 11px;
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.06em;
		color: #94a3b8;
		margin-bottom: 8px;
	}

	.summary-row {
		display: grid;
		grid-template-columns: repeat(3, 1fr);
		gap: 12px;
	}

	.summary-item {
		text-align: center;
	}

	.label {
		display: block;
		color: #888;
		font-size: 12px;
		text-transform: uppercase;
		letter-spacing: 1px;
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
		font-size: 20px;
		font-weight: 600;
		color: #222;
		font-family: monospace;
	}

	.value small {
		font-size: 13px;
		font-weight: 500;
		color: #64748b;
	}

	.value.pv {
		color: #ca8a04;
	}

	.value.export {
		color: #22c55e;
	}

	.value.heat-pump {
		color: #ea580c;
	}

	.value.savings {
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
