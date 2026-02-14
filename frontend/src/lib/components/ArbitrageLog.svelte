<script lang="ts">
	import { simulation } from '$lib/stores/simulation.svelte';
	import type { ArbitrageDayRecord } from '$lib/ws/messages';
	import ExportButton from './ExportButton.svelte';

	let expanded = $state(false);
	let monthOffset = $state(0); // 0 = latest month

	// Group records by month ("2024-11")
	let monthGroups = $derived.by(() => {
		const records = simulation.arbitrageDayRecords;
		if (records.length === 0) return [];

		const groups = new Map<string, ArbitrageDayRecord[]>();
		for (const r of records) {
			const month = r.date.slice(0, 7);
			if (!groups.has(month)) groups.set(month, []);
			groups.get(month)!.push(r);
		}

		return Array.from(groups.entries())
			.sort((a, b) => a[0].localeCompare(b[0]))
			.map(([month, recs]) => ({ month, records: recs }));
	});

	let currentMonthIndex = $derived.by(() => {
		if (monthGroups.length === 0) return 0;
		const idx = monthGroups.length - 1 + monthOffset;
		return Math.max(0, Math.min(monthGroups.length - 1, idx));
	});

	let currentGroup = $derived(monthGroups[currentMonthIndex]);

	let monthLabel = $derived.by(() => {
		if (!currentGroup) return '';
		const [year, month] = currentGroup.month.split('-');
		const d = new Date(Number(year), Number(month) - 1);
		return d.toLocaleDateString('en-GB', { month: 'long', year: 'numeric' });
	});

	let totals = $derived.by(() => {
		if (!currentGroup) return { chargeKWh: 0, dischargeKWh: 0, cycles: 0, earnings: 0 };
		let chargeKWh = 0, dischargeKWh = 0, cycles = 0, earnings = 0;
		for (const r of currentGroup.records) {
			chargeKWh += r.charge_kwh;
			dischargeKWh += r.discharge_kwh;
			cycles += r.cycles_delta;
			earnings += r.earnings_pln;
		}
		return { chargeKWh, dischargeKWh, cycles, earnings };
	});

	function prevMonth() {
		if (currentMonthIndex > 0) monthOffset--;
	}

	function nextMonth() {
		if (currentMonthIndex < monthGroups.length - 1) monthOffset++;
	}

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

	function formatDay(date: string): string {
		return date.slice(5); // "MM-DD"
	}
</script>

{#if simulation.batteryEnabled && simulation.arbitrageDayRecords.length > 0}
	<div class="arb-log">
		<button class="header" onclick={() => (expanded = !expanded)}>
			<span class="arrow" class:open={expanded}>&#9654;</span>
			<span class="title">Arbitrage Day Log</span>
			<span class="badge">{simulation.arbitrageDayRecords.length} days</span>
			<ExportButton />
		</button>

		{#if expanded}
			<div class="content">
				<div class="month-nav">
					<button
						class="nav-btn"
						onclick={prevMonth}
						disabled={currentMonthIndex <= 0}
					>&laquo; prev</button>
					<span class="month-label">{monthLabel}</span>
					<button
						class="nav-btn"
						onclick={nextMonth}
						disabled={currentMonthIndex >= monthGroups.length - 1}
					>next &raquo;</button>
				</div>

				{#if currentGroup}
					<div class="table-wrap">
						<table>
							<thead>
								<tr>
									<th>Date</th>
									<th>Charge</th>
									<th class="num">kWh</th>
									<th class="num">Gap</th>
									<th>Discharge</th>
									<th class="num">kWh</th>
									<th class="num">Cycles</th>
									<th class="num">Earned</th>
								</tr>
							</thead>
							<tbody>
								{#each currentGroup.records as rec}
									<tr>
										<td class="mono">{formatDay(rec.date)}</td>
										<td class="mono">{formatTimeRange(rec.charge_start_time, rec.charge_end_time)}</td>
										<td class="mono num">{rec.charge_kwh.toFixed(1)}</td>
										<td class="mono num">{formatGap(rec.gap_minutes, !!rec.charge_start_time, !!rec.discharge_start_time)}</td>
										<td class="mono">{formatTimeRange(rec.discharge_start_time, rec.discharge_end_time)}</td>
										<td class="mono num">{rec.discharge_kwh.toFixed(1)}</td>
										<td class="mono num">{rec.cycles_delta.toFixed(2)}</td>
										<td class="mono num" class:positive={rec.earnings_pln > 0} class:negative={rec.earnings_pln < 0}>
											{rec.earnings_pln.toFixed(2)}
										</td>
									</tr>
								{/each}
							</tbody>
							<tfoot>
								<tr>
									<td>Total</td>
									<td></td>
									<td class="mono num">{totals.chargeKWh.toFixed(1)}</td>
									<td></td>
									<td></td>
									<td class="mono num">{totals.dischargeKWh.toFixed(1)}</td>
									<td class="mono num">{totals.cycles.toFixed(2)}</td>
									<td class="mono num" class:positive={totals.earnings > 0}>
										{totals.earnings.toFixed(2)}
									</td>
								</tr>
							</tfoot>
						</table>
					</div>
				{/if}
			</div>
		{/if}
	</div>
{/if}

<style>
	.arb-log {
		background: #fff;
		border: 1px solid #e8ecf1;
		border-radius: 14px;
		overflow: hidden;
	}

	.header {
		display: flex;
		align-items: center;
		gap: 8px;
		width: 100%;
		padding: 12px 16px;
		background: none;
		border: none;
		cursor: pointer;
		font-size: 14px;
		font-weight: 600;
		color: #334155;
		text-align: left;
	}

	.header:hover {
		background: #f8fafc;
	}

	.arrow {
		font-size: 10px;
		color: #94a3b8;
		transition: transform 0.15s;
		display: inline-block;
	}

	.arrow.open {
		transform: rotate(90deg);
	}

	.title {
		flex: 1;
	}

	.badge {
		font-size: 12px;
		font-weight: 500;
		color: #64748b;
		background: #eef2f6;
		padding: 2px 8px;
		border-radius: 10px;
	}

	.content {
		padding: 0 16px 16px;
	}

	.month-nav {
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 16px;
		margin-bottom: 12px;
	}

	.nav-btn {
		background: none;
		border: 1px solid #cbd5e1;
		border-radius: 6px;
		padding: 4px 12px;
		font-size: 12px;
		color: #475569;
		cursor: pointer;
		transition: all 0.15s;
	}

	.nav-btn:hover:not(:disabled) {
		background: #f1f5f9;
		border-color: #94a3b8;
	}

	.nav-btn:disabled {
		opacity: 0.3;
		cursor: default;
	}

	.month-label {
		font-size: 14px;
		font-weight: 600;
		color: #334155;
		min-width: 140px;
		text-align: center;
	}

	.table-wrap {
		overflow-x: auto;
	}

	table {
		width: 100%;
		border-collapse: collapse;
		font-size: 13px;
	}

	th {
		text-align: left;
		font-size: 11px;
		font-weight: 600;
		color: #64748b;
		text-transform: uppercase;
		letter-spacing: 0.04em;
		padding: 6px 8px;
		border-bottom: 2px solid #e8ecf1;
	}

	th.num {
		text-align: right;
	}

	td {
		padding: 5px 8px;
		border-bottom: 1px solid #eef2f6;
		color: #334155;
	}

	td.num {
		text-align: right;
	}

	.mono {
		font-family: 'SF Mono', 'Cascadia Code', 'Fira Code', monospace;
		font-size: 12px;
	}

	.positive {
		color: #5bb88a;
	}

	.negative {
		color: #e87c6c;
	}

	tfoot td {
		font-weight: 700;
		border-top: 2px solid #e8ecf1;
		border-bottom: none;
		padding-top: 8px;
	}
</style>
