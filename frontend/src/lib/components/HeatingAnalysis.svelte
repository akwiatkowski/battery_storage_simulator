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
</style>
