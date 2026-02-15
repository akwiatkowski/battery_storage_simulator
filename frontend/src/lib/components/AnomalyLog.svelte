<script lang="ts">
	import { simulation } from '$lib/stores/simulation.svelte';
	import HelpTip from './HelpTip.svelte';

	let expanded = $state(false);

	// Filter to significant anomalies (|deviation| > 20%) and sort by |deviation| desc
	let significantAnomalies = $derived.by(() => {
		return simulation.anomalyDayRecords
			.filter((r) => Math.abs(r.deviation_pct) > 20)
			.sort((a, b) => Math.abs(b.deviation_pct) - Math.abs(a.deviation_pct));
	});

	function deviationClass(pct: number): string {
		if (pct > 0) return 'deviation-high';
		return 'deviation-low';
	}

	function formatDay(date: string): string {
		const d = new Date(date + 'T00:00:00');
		return d.toLocaleDateString('en-GB', { day: 'numeric', month: 'short' });
	}
</script>

{#if significantAnomalies.length > 0}
	<div class="anomaly-log">
		<button class="header" onclick={() => (expanded = !expanded)}>
			<span class="arrow" class:open={expanded}>&#9654;</span>
			<span class="title">Consumption Anomalies</span>
			<HelpTip key="anomalyLog" />
			<span class="badge">{significantAnomalies.length} days</span>
		</button>

		{#if expanded}
			<div class="content">
				<p class="subtitle">Days where actual grid import deviated &gt;20% from NN prediction <HelpTip key="anomalyDeviation" /></p>
				<div class="table-wrap">
					<table>
						<thead>
							<tr>
								<th>Date</th>
								<th class="num">Actual</th>
								<th class="num">Predicted</th>
								<th class="num">Deviation</th>
								<th class="num">Avg Temp</th>
							</tr>
						</thead>
						<tbody>
							{#each significantAnomalies as r}
								<tr>
									<td class="mono">{formatDay(r.date)}</td>
									<td class="mono num">{r.actual_kwh.toFixed(1)} kWh</td>
									<td class="mono num">{r.predicted_kwh.toFixed(1)} kWh</td>
									<td class="mono num {deviationClass(r.deviation_pct)}">
										{r.deviation_pct > 0 ? '+' : ''}{r.deviation_pct.toFixed(0)}%
									</td>
									<td class="mono num">{r.avg_temp_c.toFixed(1)} °C</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>
			</div>
		{/if}
	</div>
{/if}

<style>
	.anomaly-log {
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

	.subtitle {
		font-size: 12px;
		color: #64748b;
		margin: 0 0 12px;
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

	.deviation-high {
		color: #e87c6c;
	}

	.deviation-low {
		color: #64b5f6;
	}
</style>
