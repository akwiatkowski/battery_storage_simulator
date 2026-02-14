<script lang="ts">
	import { simulation } from '$lib/stores/simulation.svelte';
	import HelpTip from './HelpTip.svelte';

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

	let hasNMData = $derived(simulation.nmNetCostPLN > 0);
	let hasNBData = $derived(simulation.nbNetCostPLN > 0 || simulation.nbDepositPLN > 0);
	let hasFullComparison = $derived(hasArbData && (hasNMData || hasNBData));

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

	// ROI calculations
	let investment = $derived(simulation.batteryCapacityKWh * simulation.batteryCostPerKWh);
	let simDays = $derived.by(() => {
		if (!simulation.timeRangeStart || !simulation.simTime) return 0;
		const start = new Date(simulation.timeRangeStart).getTime();
		const now = new Date(simulation.simTime).getTime();
		return Math.max(0, (now - start) / 86400000);
	});
	let annualSavings = $derived(
		simDays > 0 ? (simulation.batterySavingsPLN / simDays) * 365 : 0
	);
	let simplePaybackYears = $derived(
		annualSavings > 0 ? investment / annualSavings : 0
	);
	let savingsPerCycle = $derived(
		simulation.batteryCycles > 0
			? simulation.batterySavingsPLN / simulation.batteryCycles
			: 0
	);
	let hasROI = $derived(
		simulation.batteryEnabled && simulation.batterySavingsPLN > 0 && simDays > 0
	);
</script>

{#if hasCostData}
	<div class="cost-sections">
		<div class="section-title">Energy Costs</div>
		<div class="cost-cards">
			<div class="cost-card import">
				<span class="cost-label">Import Cost <HelpTip key="importCost" /></span>
				<span class="cost-value">{formatPLN(simulation.gridImportCostPLN)}</span>
			</div>
			<div class="cost-card export">
				<span class="cost-label">Export Revenue <HelpTip key="exportRevenue" /></span>
				<span class="cost-value">{formatPLN(simulation.gridExportRevenuePLN)}</span>
			</div>
			<div class="cost-card net">
				<span class="cost-label">Net Cost <HelpTip key="netCost" /></span>
				<span class="cost-value">{formatPLN(simulation.netCostPLN)}</span>
			</div>
		</div>

		{#if hasFullComparison}
			<div class="battery-comparison">
				<div class="comparison-title">Strategy Comparison</div>
				<div class="comparison-row five-col">
					<div class="comparison-item">
						<span class="comp-label">No Battery <HelpTip key="noBattery" /></span>
						<span class="comp-value muted">{formatPLN(simulation.rawNetCostPLN)}</span>
					</div>
					<div class="comparison-item">
						<span class="comp-label">Self-Consump. <HelpTip key="selfConsumptionStrategy" /></span>
						<span class="comp-value">{formatPLN(simulation.netCostPLN)}</span>
						<span class="comp-saved">saved {formatPLN(simulation.batterySavingsPLN)}</span>
					</div>
					<div class="comparison-item">
						<span class="comp-label">Arbitrage <HelpTip key="arbitrageStrategy" /></span>
						<span class="comp-value">{formatPLN(simulation.arbNetCostPLN)}</span>
						<span class="comp-saved">saved {formatPLN(simulation.arbBatterySavingsPLN)}</span>
					</div>
					{#if hasNMData}
						<div class="comparison-item">
							<span class="comp-label">Net Metering <HelpTip key="netMetering" /></span>
							<span class="comp-value">{formatPLN(simulation.nmNetCostPLN)}</span>
							<span class="comp-detail">{simulation.nmCreditBankKWh.toFixed(1)} kWh credits</span>
						</div>
					{/if}
					{#if hasNBData}
						<div class="comparison-item">
							<span class="comp-label">Net Billing <HelpTip key="netBilling" /></span>
							<span class="comp-value">{formatPLN(simulation.nbNetCostPLN)}</span>
							<span class="comp-detail">{formatPLN(simulation.nbDepositPLN)} deposit</span>
						</div>
					{/if}
				</div>
			</div>
		{:else if hasArbData}
			<div class="battery-comparison">
				<div class="comparison-title">Strategy Comparison</div>
				<div class="comparison-row three-col">
					<div class="comparison-item">
						<span class="comp-label">No Battery <HelpTip key="noBattery" /></span>
						<span class="comp-value muted">{formatPLN(simulation.rawNetCostPLN)}</span>
					</div>
					<div class="comparison-item">
						<span class="comp-label">Self-Consumption <HelpTip key="selfConsumptionStrategy" /></span>
						<span class="comp-value">{formatPLN(simulation.netCostPLN)}</span>
						<span class="comp-saved">saved {formatPLN(simulation.batterySavingsPLN)}</span>
					</div>
					<div class="comparison-item">
						<span class="comp-label">Arbitrage <HelpTip key="arbitrageStrategy" /></span>
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
						<span class="comp-label">Without Battery <HelpTip key="noBattery" /></span>
						<span class="comp-value muted">{formatPLN(simulation.rawNetCostPLN)}</span>
					</div>
					<div class="comparison-item">
						<span class="comp-label">With Battery <HelpTip key="selfConsumptionStrategy" /></span>
						<span class="comp-value">{formatPLN(simulation.netCostPLN)}</span>
					</div>
					<div class="comparison-item">
						<span class="comp-label">Saved <HelpTip key="selfConsumptionStrategy" /></span>
						<span class="comp-value saved">{formatPLN(simulation.batterySavingsPLN)}</span>
					</div>
				</div>
			</div>
		{/if}

		{#if !hasFullComparison && (hasNMData || hasNBData)}
			<div class="battery-comparison">
				<div class="comparison-title">Tariff Comparison</div>
				<div class="comparison-row">
					{#if hasNMData}
						<div class="comparison-item">
							<span class="comp-label">Net Metering <HelpTip key="netMetering" /></span>
							<span class="comp-value">{formatPLN(simulation.nmNetCostPLN)}</span>
							<span class="comp-detail">{simulation.nmCreditBankKWh.toFixed(1)} kWh credits</span>
						</div>
					{/if}
					{#if hasNBData}
						<div class="comparison-item">
							<span class="comp-label">Net Billing <HelpTip key="netBilling" /></span>
							<span class="comp-value">{formatPLN(simulation.nbNetCostPLN)}</span>
							<span class="comp-detail">{formatPLN(simulation.nbDepositPLN)} deposit</span>
						</div>
					{/if}
				</div>
			</div>
		{/if}

		{#if hasROI}
			<div class="battery-comparison">
				<div class="comparison-title">Battery ROI</div>
				<div class="comparison-row">
					<div class="comparison-item">
						<span class="comp-label">Investment <HelpTip key="investment" /></span>
						<span class="comp-value">{formatPLN(investment)}</span>
					</div>
					<div class="comparison-item">
						<span class="comp-label">Annual Savings <HelpTip key="annualSavings" /></span>
						<span class="comp-value saved">{formatPLN(annualSavings)}</span>
					</div>
					<div class="comparison-item">
						<span class="comp-label">Payback <HelpTip key="payback" /></span>
						<span class="comp-value">{simplePaybackYears.toFixed(1)} yrs</span>
					</div>
					<div class="comparison-item">
						<span class="comp-label">Savings/Cycle <HelpTip key="savingsPerCycle" /></span>
						<span class="comp-value">{formatPLN(savingsPerCycle)}</span>
					</div>
				</div>
			</div>
		{/if}

		{#if hasCheapExport}
			<div class="battery-comparison">
				<div class="comparison-title">Cheap Export</div>
				<div class="comparison-row">
					<div class="comparison-item">
						<span class="comp-label">Energy <HelpTip key="cheapExportEnergy" /></span>
						<span class="comp-value warning">{simulation.cheapExportKWh.toFixed(1)} kWh</span>
					</div>
					<div class="comparison-item">
						<span class="comp-label">Revenue <HelpTip key="cheapExportRevenue" /></span>
						<span class="comp-value warning">{formatPLN(simulation.cheapExportRevPLN)}</span>
					</div>
					<div class="comparison-item">
						<span class="comp-label">% of Export <HelpTip key="cheapExportPct" /></span>
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
						<span class="comp-label">Avg Import Price <HelpTip key="avgImportPrice" /></span>
						<span class="comp-value">{avgImportPrice.toFixed(2)} PLN/kWh</span>
					</div>
					<div class="comparison-item">
						<span class="comp-label">Cost/100km <HelpTip key="evCost100km" /></span>
						<span class="comp-value">{formatPLN(evCostPer100km)}</span>
					</div>
					<div class="comparison-item">
						<span class="comp-label">km from Export <HelpTip key="evKmFromExport" /></span>
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
		border: 1px solid #e8ecf1;
		border-radius: 14px;
		padding: 16px 20px;
		box-shadow: 0 1px 4px rgba(0, 0, 0, 0.03);
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
		background: #f8fafb;
		border-left: 3px solid transparent;
	}

	.cost-card.import {
		border-left-color: #e87c6c;
	}

	.cost-card.export {
		border-left-color: #5bb88a;
	}

	.cost-card.net {
		border-left-color: #64b5f6;
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
		border-top: 1px solid #eef2f6;
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

	.comparison-row.five-col {
		grid-template-columns: repeat(auto-fit, minmax(100px, 1fr));
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
		color: #5bb88a;
	}

	.comp-value.warning {
		color: #e0a040;
	}

	.comp-saved {
		display: block;
		font-size: 11px;
		color: #5bb88a;
		font-family: 'SF Mono', 'Cascadia Code', 'Fira Code', monospace;
		margin-top: 2px;
	}

	.comp-detail {
		display: block;
		font-size: 11px;
		color: #64748b;
		font-family: 'SF Mono', 'Cascadia Code', 'Fira Code', monospace;
		margin-top: 2px;
	}
</style>
