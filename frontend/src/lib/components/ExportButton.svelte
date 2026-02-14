<script lang="ts">
	import { simulation, type DailyRecord } from '$lib/stores/simulation.svelte';
	import type { ArbitrageDayRecord } from '$lib/ws/messages';

	function formatTimeRange(start: string, end: string): string {
		if (!start) return '-';
		if (start === end) return start;
		return `${start}-${end}`;
	}

	function formatGap(minutes: number, hasCharge: boolean, hasDischarge: boolean): string {
		if (!hasCharge || !hasDischarge) return '-';
		const h = Math.floor(minutes / 60);
		const m = minutes % 60;
		return h > 0 ? `${h}h${m > 0 ? String(m).padStart(2, '0') + 'm' : ''}` : `${m}m`;
	}

	function esc(s: string | number): string {
		return String(s).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
	}

	function buildArbitrageTable(records: ArbitrageDayRecord[]): string {
		if (records.length === 0) return '';

		// Group by month
		const groups = new Map<string, ArbitrageDayRecord[]>();
		for (const r of records) {
			const month = r.date.slice(0, 7);
			if (!groups.has(month)) groups.set(month, []);
			groups.get(month)!.push(r);
		}

		const sorted = Array.from(groups.entries()).sort((a, b) => a[0].localeCompare(b[0]));

		let rows = '';
		let grandCharge = 0, grandDischarge = 0, grandCycles = 0, grandEarnings = 0;

		for (const [month, recs] of sorted) {
			let mCharge = 0, mDischarge = 0, mCycles = 0, mEarnings = 0;
			for (const r of recs) {
				rows += `<tr>
					<td class="mono">${esc(r.date)}</td>
					<td class="mono">${esc(formatTimeRange(r.charge_start_time, r.charge_end_time))}</td>
					<td class="mono r">${r.charge_kwh.toFixed(1)}</td>
					<td class="mono r">${formatGap(r.gap_minutes, !!r.charge_start_time, !!r.discharge_start_time)}</td>
					<td class="mono">${esc(formatTimeRange(r.discharge_start_time, r.discharge_end_time))}</td>
					<td class="mono r">${r.discharge_kwh.toFixed(1)}</td>
					<td class="mono r">${r.cycles_delta.toFixed(2)}</td>
					<td class="mono r ${r.earnings_pln > 0 ? 'pos' : r.earnings_pln < 0 ? 'neg' : ''}">${r.earnings_pln.toFixed(2)}</td>
				</tr>`;
				mCharge += r.charge_kwh;
				mDischarge += r.discharge_kwh;
				mCycles += r.cycles_delta;
				mEarnings += r.earnings_pln;
			}
			rows += `<tr class="subtotal">
				<td colspan="2">${esc(month)} subtotal</td>
				<td class="mono r">${mCharge.toFixed(1)}</td>
				<td></td>
				<td></td>
				<td class="mono r">${mDischarge.toFixed(1)}</td>
				<td class="mono r">${mCycles.toFixed(2)}</td>
				<td class="mono r ${mEarnings > 0 ? 'pos' : ''}">${mEarnings.toFixed(2)}</td>
			</tr>`;
			grandCharge += mCharge;
			grandDischarge += mDischarge;
			grandCycles += mCycles;
			grandEarnings += mEarnings;
		}

		return `<h2>Arbitrage Day Log</h2>
		<table>
			<thead><tr>
				<th>Date</th><th>Charge window</th><th class="r">Charge kWh</th><th class="r">Gap</th>
				<th>Discharge window</th><th class="r">Discharge kWh</th>
				<th class="r">Cycles</th><th class="r">Earned PLN</th>
			</tr></thead>
			<tbody>${rows}</tbody>
			<tfoot><tr>
				<td colspan="2"><strong>Grand total</strong></td>
				<td class="mono r"><strong>${grandCharge.toFixed(1)}</strong></td>
				<td></td>
				<td></td>
				<td class="mono r"><strong>${grandDischarge.toFixed(1)}</strong></td>
				<td class="mono r"><strong>${grandCycles.toFixed(2)}</strong></td>
				<td class="mono r ${grandEarnings > 0 ? 'pos' : ''}"><strong>${grandEarnings.toFixed(2)}</strong></td>
			</tr></tfoot>
		</table>`;
	}

	function buildDailyRecordsTable(records: DailyRecord[]): string {
		if (records.length === 0) return '';

		let rows = '';
		for (const r of records) {
			rows += `<tr>
				<td class="mono">${esc(r.date)}</td>
				<td class="mono r">${r.gridImportKWh.toFixed(2)}</td>
				<td class="mono r">${r.selfConsumptionKWh.toFixed(2)}</td>
				<td class="mono r">${r.batterySavingsKWh.toFixed(2)}</td>
				<td class="mono r">${r.homeDemandKWh.toFixed(2)}</td>
				<td class="mono r">${r.offGridPct.toFixed(1)}%</td>
			</tr>`;
		}

		return `<h2>Daily Records</h2>
		<table>
			<thead><tr>
				<th>Date</th><th class="r">Grid Import kWh</th><th class="r">Self-Cons. kWh</th>
				<th class="r">Battery Savings kWh</th><th class="r">Home Demand kWh</th><th class="r">Off-Grid %</th>
			</tr></thead>
			<tbody>${rows}</tbody>
		</table>`;
	}

	function exportReport(): void {
		const s = simulation;
		const now = new Date().toISOString().slice(0, 19).replace('T', ' ');

		const hasCosts = s.rawNetCostPLN !== 0 || s.netCostPLN !== 0 || s.arbNetCostPLN !== 0;

		const costSection = hasCosts
			? `<h2>Cost Summary</h2>
			<table>
				<thead><tr><th></th><th class="r">No Battery</th><th class="r">Self-Consumption</th><th class="r">Arbitrage</th></tr></thead>
				<tbody>
					<tr><td>Import cost</td><td class="mono r">${s.rawGridImportCostPLN.toFixed(2)}</td><td class="mono r">${s.gridImportCostPLN.toFixed(2)}</td><td class="mono r">-</td></tr>
					<tr><td>Export revenue</td><td class="mono r">${s.rawGridExportRevenuePLN.toFixed(2)}</td><td class="mono r">${s.gridExportRevenuePLN.toFixed(2)}</td><td class="mono r">-</td></tr>
					<tr><td><strong>Net cost</strong></td><td class="mono r"><strong>${s.rawNetCostPLN.toFixed(2)}</strong></td><td class="mono r"><strong>${s.netCostPLN.toFixed(2)}</strong></td><td class="mono r"><strong>${s.arbNetCostPLN.toFixed(2)}</strong></td></tr>
					<tr><td>Savings</td><td class="mono r">-</td><td class="mono r pos">${s.batterySavingsPLN.toFixed(2)}</td><td class="mono r pos">${s.arbBatterySavingsPLN.toFixed(2)}</td></tr>
				</tbody>
			</table>`
			: '';

		const batterySection = s.batteryEnabled
			? `<h2>Battery Config</h2>
			<table>
				<tbody>
					<tr><td>Capacity</td><td class="mono r">${s.batteryCapacityKWh} kWh</td></tr>
					<tr><td>Max power</td><td class="mono r">${s.batteryMaxPowerKW} kW</td></tr>
					<tr><td>SoC range</td><td class="mono r">${s.batteryDischargeToPercent}% - ${s.batteryChargeToPercent}%</td></tr>
					<tr><td>Total cycles</td><td class="mono r">${s.batteryCycles.toFixed(1)}</td></tr>
				</tbody>
			</table>`
			: '';

		const html = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Energy Simulation Report</title>
<style>
	body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; max-width: 960px; margin: 0 auto; padding: 24px; color: #1e293b; background: #f7f9fc; }
	h1 { font-size: 22px; margin-bottom: 4px; }
	h2 { font-size: 16px; margin-top: 32px; margin-bottom: 8px; color: #334155; border-bottom: 2px solid #e8ecf1; padding-bottom: 4px; }
	.meta { font-size: 13px; color: #64748b; margin-bottom: 24px; }
	table { width: 100%; border-collapse: collapse; margin-bottom: 16px; font-size: 13px; }
	th { text-align: left; font-size: 11px; font-weight: 600; color: #64748b; text-transform: uppercase; letter-spacing: 0.04em; padding: 6px 10px; border-bottom: 2px solid #e8ecf1; background: #eef2f6; }
	td { padding: 5px 10px; border-bottom: 1px solid #e8ecf1; }
	th.r, td.r { text-align: right; }
	.mono { font-family: 'SF Mono', 'Cascadia Code', 'Fira Code', monospace; font-size: 12px; }
	.pos { color: #5bb88a; }
	.neg { color: #e87c6c; }
	tbody tr:nth-child(even) { background: #f8fafb; }
	tbody tr:nth-child(odd) { background: #fff; }
	tr.subtotal { background: #eef2f6 !important; font-weight: 600; }
	tfoot td { font-weight: 700; border-top: 2px solid #d0d8e0; border-bottom: none; background: #eef2f6; }
</style>
</head>
<body>
<h1>Energy Simulation Report</h1>
<p class="meta">Generated: ${esc(now)} &middot; Period: ${esc(s.timeRangeStart.slice(0, 10))} &rarr; ${esc(s.simTime.slice(0, 10))}</p>

<h2>Energy Summary</h2>
<table>
	<tbody>
		<tr><td>Grid import</td><td class="mono r">${s.gridImportKWh.toFixed(2)} kWh</td></tr>
		<tr><td>Grid export</td><td class="mono r">${s.gridExportKWh.toFixed(2)} kWh</td></tr>
		<tr><td>PV production</td><td class="mono r">${s.pvProductionKWh.toFixed(2)} kWh</td></tr>
		<tr><td>Self-consumption</td><td class="mono r">${s.selfConsumptionKWh.toFixed(2)} kWh</td></tr>
		<tr><td>Heat pump consumption</td><td class="mono r">${s.heatPumpKWh.toFixed(2)} kWh</td></tr>
		<tr><td>Heat pump production</td><td class="mono r">${s.heatPumpProdKWh.toFixed(2)} kWh</td></tr>
		<tr><td>Home demand</td><td class="mono r">${s.homeDemandKWh.toFixed(2)} kWh</td></tr>
		<tr><td>Battery savings</td><td class="mono r">${s.batterySavingsKWh.toFixed(2)} kWh</td></tr>
	</tbody>
</table>

${costSection}
${batterySection}
${buildArbitrageTable(s.arbitrageDayRecords)}
${buildDailyRecordsTable(s.dailyRecords)}

</body>
</html>`;

		const blob = new Blob([html], { type: 'text/html' });
		const url = URL.createObjectURL(blob);
		const a = document.createElement('a');
		a.href = url;
		a.download = `energy-report-${s.simTime.slice(0, 10)}.html`;
		a.click();
		URL.revokeObjectURL(url);
	}
</script>

<button class="export-btn" onclick={(e) => { e.stopPropagation(); exportReport(); }}>
	Export
</button>

<style>
	.export-btn {
		font-size: 11px;
		font-weight: 600;
		color: #475569;
		background: #fff;
		border: 1px solid #cbd5e1;
		border-radius: 6px;
		padding: 2px 10px;
		cursor: pointer;
		transition: all 0.15s;
	}

	.export-btn:hover {
		background: #f1f5f9;
		border-color: #94a3b8;
	}
</style>
