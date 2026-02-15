<script lang="ts">
	import { simulation } from '$lib/stores/simulation.svelte';
	import HelpTip from './HelpTip.svelte';

	let expanded = $state(false);

	let stats = $derived(simulation.heatingMonthStats);

	let totals = $derived.by(() => {
		let consumption = 0, production = 0, cost = 0, tempSum = 0, tempCount = 0;
		for (const s of stats) {
			consumption += s.consumption_kwh;
			production += s.production_kwh;
			cost += s.cost_pln;
			if (s.avg_temp_c !== 0 || consumption > 0) {
				tempSum += s.avg_temp_c;
				tempCount++;
			}
		}
		const cop = consumption > 0 ? production / consumption : 0;
		const avgTemp = tempCount > 0 ? tempSum / tempCount : 0;
		return { consumption, production, cop, cost, avgTemp };
	});

	interface Season {
		startMonth: string;
		endMonth: string;
		months: number;
		consumption: number;
		production: number;
		cop: number;
		cost: number;
		avgTemp: number;
	}

	let seasons = $derived.by((): Season[] => {
		if (stats.length === 0) return [];
		const threshold = 5; // kWh minimum to count as a heating month
		const result: Season[] = [];
		let current: { months: typeof stats; startIdx: number } | null = null;

		for (let i = 0; i < stats.length; i++) {
			const s = stats[i];
			if (s.consumption_kwh >= threshold) {
				if (!current) {
					current = { months: [s], startIdx: i };
				} else {
					current.months.push(s);
				}
			} else {
				if (current && current.months.length >= 2) {
					result.push(buildSeason(current.months));
				}
				current = null;
			}
		}
		if (current && current.months.length >= 2) {
			result.push(buildSeason(current.months));
		}
		return result;
	});

	function buildSeason(months: typeof stats): Season {
		let consumption = 0, production = 0, cost = 0, tempSum = 0, tempCount = 0;
		for (const m of months) {
			consumption += m.consumption_kwh;
			production += m.production_kwh;
			cost += m.cost_pln;
			tempSum += m.avg_temp_c;
			tempCount++;
		}
		return {
			startMonth: months[0].month,
			endMonth: months[months.length - 1].month,
			months: months.length,
			consumption,
			production,
			cop: consumption > 0 ? production / consumption : 0,
			cost,
			avgTemp: tempCount > 0 ? tempSum / tempCount : 0
		};
	}

	let heatingCostFraction = $derived.by(() => {
		const hpCost = simulation.heatPumpCostPLN;
		const totalCost = simulation.gridImportCostPLN;
		if (totalCost <= 0 || hpCost <= 0) return null;
		return { hpCost, totalCost, pct: (hpCost / totalCost) * 100 };
	});

	let yoyComparison = $derived.by(() => {
		if (seasons.length < 2) return null;
		const prev = seasons[seasons.length - 2];
		const curr = seasons[seasons.length - 1];
		const costDelta = curr.cost - prev.cost;
		const costDeltaPct = prev.cost > 0 ? (costDelta / prev.cost) * 100 : 0;
		const consDelta = curr.consumption - prev.consumption;
		const consDeltaPct = prev.consumption > 0 ? (consDelta / prev.consumption) * 100 : 0;
		const copDelta = curr.cop - prev.cop;
		const tempDelta = curr.avgTemp - prev.avgTemp;
		return { prev, curr, costDelta, costDeltaPct, consDelta, consDeltaPct, copDelta, tempDelta };
	});

	function copClass(cop: number): string {
		if (cop >= 3.5) return 'cop-good';
		if (cop >= 2.5) return 'cop-ok';
		return 'cop-bad';
	}

	function formatMonth(month: string): string {
		const [year, m] = month.split('-');
		const d = new Date(Number(year), Number(m) - 1);
		return d.toLocaleDateString('en-GB', { month: 'short', year: 'numeric' });
	}

	function signPrefix(v: number): string {
		return v >= 0 ? '+' : '';
	}
</script>

{#if stats.length > 0}
	<div class="heating-analysis">
		<button class="header" onclick={() => (expanded = !expanded)}>
			<span class="arrow" class:open={expanded}>&#9654;</span>
			<span class="title">Heating Analysis</span>
			<HelpTip key="heatingAnalysis" />
			<span class="badge">{stats.length} months</span>
		</button>

		{#if expanded}
			<div class="content">
				<div class="table-wrap">
					<table>
						<thead>
							<tr>
								<th>Month</th>
								<th class="num">Consumption</th>
								<th class="num">Production</th>
								<th class="num">COP <HelpTip key="heatingCOP" /></th>
								<th class="num">Cost</th>
								<th class="num">Avg Temp</th>
							</tr>
						</thead>
						<tbody>
							{#each stats as s}
								<tr>
									<td class="mono">{formatMonth(s.month)}</td>
									<td class="mono num">{s.consumption_kwh.toFixed(1)} kWh</td>
									<td class="mono num">{s.production_kwh.toFixed(1)} kWh</td>
									<td class="mono num {copClass(s.cop)}">{s.cop.toFixed(2)}</td>
									<td class="mono num">{s.cost_pln.toFixed(2)} PLN</td>
									<td class="mono num">{s.avg_temp_c.toFixed(1)} °C</td>
								</tr>
							{/each}
						</tbody>
						<tfoot>
							<tr>
								<td>Total</td>
								<td class="mono num">{totals.consumption.toFixed(1)} kWh</td>
								<td class="mono num">{totals.production.toFixed(1)} kWh</td>
								<td class="mono num {copClass(totals.cop)}">{totals.cop.toFixed(2)}</td>
								<td class="mono num">{totals.cost.toFixed(2)} PLN</td>
								<td class="mono num">{totals.avgTemp.toFixed(1)} °C</td>
							</tr>
						</tfoot>
					</table>
				</div>

				{#if seasons.length > 0}
					<div class="section-divider"></div>
					<div class="seasons">
						<div class="section-title">Heating Seasons</div>
						{#each seasons as season}
							<div class="season-row">
								<span class="season-label">{formatMonth(season.startMonth)} → {formatMonth(season.endMonth)}</span>
								<span class="season-detail">({season.months} months)</span>
								<div class="season-stats mono">
									{season.consumption.toFixed(1)} kWh consumed |
									{season.production.toFixed(1)} kWh produced |
									COP <span class={copClass(season.cop)}>{season.cop.toFixed(2)}</span> |
									{season.cost.toFixed(2)} PLN |
									Avg {season.avgTemp.toFixed(1)} °C
								</div>
							</div>
						{/each}
					</div>
				{/if}

				{#if heatingCostFraction}
					<div class="section-divider"></div>
					<div class="cost-fraction">
						<div class="section-title">Heating Cost Share</div>
						<div class="fraction-row mono">
							{heatingCostFraction.hpCost.toFixed(2)} PLN of {heatingCostFraction.totalCost.toFixed(2)} PLN total
							<span class="fraction-pct">({heatingCostFraction.pct.toFixed(1)}%)</span>
						</div>
						<div class="fraction-bar">
							<div class="fraction-fill" style="width: {Math.min(100, heatingCostFraction.pct)}%"></div>
						</div>
					</div>
				{/if}

				{#if yoyComparison}
					<div class="section-divider"></div>
					<div class="yoy">
						<div class="section-title">Year-over-Year Comparison</div>
						<div class="yoy-row">
							<span class="yoy-label">Previous:</span>
							<span class="mono">{formatMonth(yoyComparison.prev.startMonth)} → {formatMonth(yoyComparison.prev.endMonth)}</span>
						</div>
						<div class="yoy-row">
							<span class="yoy-label">Current:</span>
							<span class="mono">{formatMonth(yoyComparison.curr.startMonth)} → {formatMonth(yoyComparison.curr.endMonth)}</span>
						</div>
						<div class="yoy-deltas">
							<div class="yoy-delta">
								<span class="delta-label">Cost</span>
								<span class="mono {yoyComparison.costDelta <= 0 ? 'delta-good' : 'delta-bad'}">
									{signPrefix(yoyComparison.costDelta)}{yoyComparison.costDelta.toFixed(2)} PLN ({signPrefix(yoyComparison.costDeltaPct)}{yoyComparison.costDeltaPct.toFixed(1)}%)
								</span>
							</div>
							<div class="yoy-delta">
								<span class="delta-label">Consumption</span>
								<span class="mono {yoyComparison.consDelta <= 0 ? 'delta-good' : 'delta-bad'}">
									{signPrefix(yoyComparison.consDelta)}{yoyComparison.consDelta.toFixed(1)} kWh ({signPrefix(yoyComparison.consDeltaPct)}{yoyComparison.consDeltaPct.toFixed(1)}%)
								</span>
							</div>
							<div class="yoy-delta">
								<span class="delta-label">COP</span>
								<span class="mono {yoyComparison.copDelta >= 0 ? 'delta-good' : 'delta-bad'}">
									{signPrefix(yoyComparison.copDelta)}{yoyComparison.copDelta.toFixed(2)}
								</span>
							</div>
							<div class="yoy-delta">
								<span class="delta-label">Avg Temp</span>
								<span class="mono">
									{signPrefix(yoyComparison.tempDelta)}{yoyComparison.tempDelta.toFixed(1)} °C
								</span>
							</div>
						</div>
					</div>
				{/if}
			</div>
		{/if}
	</div>
{/if}

<style>
	.heating-analysis {
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

	:global(.cop-good) {
		color: #5bb88a;
	}

	:global(.cop-ok) {
		color: #e0a040;
	}

	:global(.cop-bad) {
		color: #e87c6c;
	}

	tfoot td {
		font-weight: 700;
		border-top: 2px solid #e8ecf1;
		border-bottom: none;
		padding-top: 8px;
	}

	.section-divider {
		border-top: 1px solid #eef2f6;
		margin: 12px 0;
	}

	.section-title {
		font-size: 12px;
		font-weight: 600;
		color: #64748b;
		text-transform: uppercase;
		letter-spacing: 0.04em;
		margin-bottom: 8px;
	}

	.season-row {
		margin-bottom: 8px;
	}

	.season-label {
		font-weight: 600;
		font-size: 13px;
		color: #334155;
	}

	.season-detail {
		font-size: 12px;
		color: #94a3b8;
		margin-left: 4px;
	}

	.season-stats {
		color: #64748b;
		margin-top: 2px;
		line-height: 1.5;
	}

	.cost-fraction {
		margin-bottom: 4px;
	}

	.fraction-row {
		color: #334155;
		margin-bottom: 6px;
	}

	.fraction-pct {
		font-weight: 600;
		color: #e8884c;
	}

	.fraction-bar {
		height: 6px;
		background: #eef2f6;
		border-radius: 3px;
		overflow: hidden;
	}

	.fraction-fill {
		height: 100%;
		background: #e8884c;
		border-radius: 3px;
		transition: width 0.3s;
	}

	.yoy-row {
		font-size: 13px;
		margin-bottom: 4px;
		color: #334155;
	}

	.yoy-label {
		font-weight: 600;
		color: #64748b;
		display: inline-block;
		width: 70px;
	}

	.yoy-deltas {
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: 6px 16px;
		margin-top: 8px;
	}

	.yoy-delta {
		display: flex;
		flex-direction: column;
		gap: 2px;
	}

	.delta-label {
		font-size: 11px;
		font-weight: 600;
		color: #94a3b8;
		text-transform: uppercase;
		letter-spacing: 0.03em;
	}

	:global(.delta-good) {
		color: #5bb88a;
	}

	:global(.delta-bad) {
		color: #e87c6c;
	}
</style>
