<script lang="ts">
	import { simulation, type DailyRecord } from '$lib/stores/simulation.svelte';

	const dayLabels = ['', 'Mon', '', 'Wed', '', 'Fri', ''];
	const MAX_AUTONOMY = 48; // clip color at 48h

	// Red (0h) → Blue (48h) via hue interpolation: hue 0 (red) → 240 (blue)
	function cellColor(hours: number): string {
		if (hours <= 0) return '#ebedf0';
		const clamped = Math.min(hours, MAX_AUTONOMY);
		const ratio = clamped / MAX_AUTONOMY; // 0..1
		const hue = ratio * 240; // 0=red, 120=green, 240=blue
		return `hsl(${hue}, 70%, 50%)`;
	}

	function formatDate(dateStr: string): string {
		const d = new Date(dateStr + 'T00:00:00');
		return d.toLocaleDateString('en-GB', { day: 'numeric', month: 'short', year: 'numeric' });
	}

	// Compute grid layout from daily records
	let grid = $derived.by(() => {
		const records = simulation.dailyRecords;
		if (records.length === 0) return { weeks: 0, lookup: new Map<string, DailyRecord>(), startSunday: '' };

		const lookup = new Map<string, DailyRecord>();
		for (const r of records) {
			lookup.set(r.date, r);
		}

		// Find the Sunday on or before the first record
		const firstDate = new Date(records[0].date + 'T00:00:00');
		const firstDow = firstDate.getDay(); // 0=Sun
		const startSunday = new Date(firstDate);
		startSunday.setDate(startSunday.getDate() - firstDow);

		// Find the Saturday on or after the last record
		const lastDate = new Date(records[records.length - 1].date + 'T00:00:00');
		const lastDow = lastDate.getDay();
		const endSaturday = new Date(lastDate);
		endSaturday.setDate(endSaturday.getDate() + (6 - lastDow));

		const totalDays = Math.round((endSaturday.getTime() - startSunday.getTime()) / 86400000) + 1;
		const weeks = Math.ceil(totalDays / 7);

		const yyyy = startSunday.getFullYear();
		const mm = String(startSunday.getMonth() + 1).padStart(2, '0');
		const dd = String(startSunday.getDate()).padStart(2, '0');

		return { weeks, lookup, startSunday: `${yyyy}-${mm}-${dd}` };
	});

	// Compute date string for a grid cell
	function getCellDate(weekIdx: number, dayOfWeek: number): string {
		if (!grid.startSunday) return '';
		const d = new Date(grid.startSunday + 'T00:00:00');
		d.setDate(d.getDate() + weekIdx * 7 + dayOfWeek);
		const yyyy = d.getFullYear();
		const mm = String(d.getMonth() + 1).padStart(2, '0');
		const dd = String(d.getDate()).padStart(2, '0');
		return `${yyyy}-${mm}-${dd}`;
	}

	// Month labels positioned at the first week column of each month
	let monthLabels = $derived.by(() => {
		if (grid.weeks === 0) return [];
		const labels: { col: number; label: string }[] = [];
		const monthNames = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];
		let lastMonth = -1;
		for (let w = 0; w < grid.weeks; w++) {
			// Use the Sunday of each week to determine month
			const dateStr = getCellDate(w, 0);
			const month = parseInt(dateStr.slice(5, 7)) - 1;
			if (month !== lastMonth) {
				labels.push({ col: w, label: monthNames[month] });
				lastMonth = month;
			}
		}
		return labels;
	});
</script>

{#if simulation.batteryEnabled && simulation.dailyRecords.length > 0}
	<div class="heatmap-card">
		<div class="heatmap-title">Battery Autonomy</div>
		<div class="heatmap-container">
			<div class="heatmap-grid" style="grid-template-columns: 28px repeat({grid.weeks}, 1fr)">
				<!-- Month labels row -->
				<div class="corner"></div>
				{#each { length: grid.weeks } as _, w}
					{@const label = monthLabels.find(m => m.col === w)}
					<div class="col-label">{label ? label.label : ''}</div>
				{/each}

				<!-- Data rows: 7 days (Sun=0 through Sat=6) -->
				{#each { length: 7 } as _, day}
					<div class="row-label">{dayLabels[day]}</div>
					{#each { length: grid.weeks } as _, w}
						{@const dateStr = getCellDate(w, day)}
						{@const record = grid.lookup.get(dateStr)}
						{#if record}
							<div
								class="cell"
								style="background: {cellColor(record.batteryAutonomyHours)}"
								title="{formatDate(dateStr)} · {Math.min(record.batteryAutonomyHours, MAX_AUTONOMY).toFixed(1)}h autonomy · Demand {record.homeDemandKWh.toFixed(1)} kWh · Import {record.gridImportKWh.toFixed(1)} kWh"
							></div>
						{:else}
							<div class="cell empty"></div>
						{/if}
					{/each}
				{/each}
			</div>

			<!-- Legend -->
			<div class="legend">
				<span class="legend-label">0h</span>
				<div class="legend-cell" style="background: #ebedf0" title="0 hours"></div>
				<div class="legend-cell" style="background: hsl(0, 70%, 50%)" title="~0h"></div>
				<div class="legend-cell" style="background: hsl(60, 70%, 50%)" title="~12h"></div>
				<div class="legend-cell" style="background: hsl(120, 70%, 50%)" title="~24h"></div>
				<div class="legend-cell" style="background: hsl(180, 70%, 50%)" title="~36h"></div>
				<div class="legend-cell" style="background: hsl(240, 70%, 50%)" title="48h"></div>
				<span class="legend-label">48h</span>
			</div>
		</div>
	</div>
{/if}

<style>
	.heatmap-card {
		background: #fafbfc;
		border: 1px solid #e5e7eb;
		border-radius: 12px;
		padding: 16px;
	}

	.heatmap-title {
		font-size: 12px;
		font-weight: 600;
		color: #475569;
		text-transform: uppercase;
		letter-spacing: 0.04em;
		margin-bottom: 12px;
	}

	.heatmap-container {
		overflow-x: auto;
	}

	.heatmap-grid {
		display: grid;
		gap: 2px;
		min-width: fit-content;
	}

	.corner {
		min-height: 1px;
	}

	.col-label {
		font-size: 10px;
		font-family: 'SF Mono', 'Cascadia Code', 'Fira Code', monospace;
		color: #64748b;
		text-align: left;
		padding-bottom: 4px;
		white-space: nowrap;
	}

	.row-label {
		font-size: 10px;
		font-family: 'SF Mono', 'Cascadia Code', 'Fira Code', monospace;
		color: #64748b;
		text-align: right;
		padding-right: 4px;
		display: flex;
		align-items: center;
		justify-content: flex-end;
	}

	.cell {
		min-width: 12px;
		aspect-ratio: 1;
		border-radius: 2px;
		cursor: default;
		transition: opacity 0.15s;
	}

	.cell:not(.empty):hover {
		opacity: 0.8;
		outline: 1px solid #94a3b8;
	}

	.cell.empty {
		background: transparent;
	}

	.legend {
		display: flex;
		align-items: center;
		gap: 3px;
		margin-top: 8px;
		justify-content: flex-end;
	}

	.legend-label {
		font-size: 10px;
		color: #64748b;
		font-family: 'SF Mono', 'Cascadia Code', 'Fira Code', monospace;
		padding: 0 2px;
	}

	.legend-cell {
		width: 12px;
		height: 12px;
		border-radius: 2px;
	}
</style>
