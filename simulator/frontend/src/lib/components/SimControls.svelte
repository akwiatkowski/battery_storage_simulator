<script lang="ts">
	import { simulation } from '$lib/stores/simulation.svelte';

	const speedOptions = [
		{ value: 3600, label: '1 h/s' },
		{ value: 7200, label: '2 h/s' },
		{ value: 14400, label: '4 h/s' },
		{ value: 28800, label: '8 h/s' },
		{ value: 86400, label: '1 d/s' },
		{ value: 604800, label: '1 w/s' },
		{ value: 2592000, label: '1 mo/s' }
	];

	function togglePlayPause() {
		if (simulation.running) {
			simulation.pause();
		} else {
			simulation.start();
		}
	}

	function handleSpeedChange(e: Event) {
		const target = e.target as HTMLSelectElement;
		simulation.setSpeed(Number(target.value));
	}

	function handleSeek(e: Event) {
		const target = e.target as HTMLInputElement;
		if (target.value) {
			simulation.seek(new Date(target.value).toISOString());
		}
	}

	const sourceOptions = [
		{ value: 'all', label: 'All' },
		{ value: 'current', label: 'Current (~2w)' },
		{ value: 'archival', label: 'Archival (~15m)' }
	];

	function handleSourceChange(e: Event) {
		const target = e.target as HTMLSelectElement;
		simulation.setDataSource(target.value);
	}

	function formatSimTime(iso: string): string {
		if (!iso) return '--';
		const d = new Date(iso);
		return d.toLocaleString('en-GB', {
			year: 'numeric',
			month: 'short',
			day: 'numeric',
			hour: '2-digit',
			minute: '2-digit',
			second: '2-digit',
			timeZone: 'UTC'
		});
	}

	function toDatetimeLocal(iso: string): string {
		if (!iso) return '';
		return iso.slice(0, 16);
	}
</script>

<div class="controls">
	<button class="play-btn" onclick={togglePlayPause}>
		{simulation.running ? 'Pause' : 'Play'}
	</button>

	<label class="control-label">
		Speed:
		<select onchange={handleSpeedChange}>
			{#each speedOptions as opt}
				<option value={String(opt.value)} selected={simulation.speed === opt.value}>{opt.label}</option>
			{/each}
		</select>
	</label>

	<span class="sim-time">{formatSimTime(simulation.simTime)}</span>

	<label class="control-label">
		Source:
		<select onchange={handleSourceChange} disabled={simulation.predictionEnabled}>
			{#each sourceOptions as opt}
				<option value={opt.value} selected={simulation.dataSource === opt.value}>{opt.label}</option>
			{/each}
		</select>
	</label>

	<label class="control-label seek">
		Seek:
		<input
			type="datetime-local"
			min={toDatetimeLocal(simulation.timeRangeStart)}
			max={toDatetimeLocal(simulation.timeRangeEnd)}
			onchange={handleSeek}
			disabled={simulation.predictionEnabled}
		/>
	</label>

	<label class="toggle-label">
		<input
			type="checkbox"
			bind:checked={simulation.predictionEnabled}
			onchange={() => simulation.setPredictionMode()}
		/>
		<span>NN Predict</span>
	</label>

	{#if simulation.currentSpotPrice !== 0}
		<span class="price-badge" class:cheap={simulation.currentSpotPrice < simulation.priceThresholdPLN && simulation.currentSpotPrice >= 0} class:negative={simulation.currentSpotPrice < 0}>
			{simulation.currentSpotPrice.toFixed(2)} PLN
			{#if simulation.currentSpotPrice < 0}
				negative!
			{:else if simulation.currentSpotPrice < simulation.priceThresholdPLN}
				cheap export!
			{/if}
		</span>
	{/if}

	<span class="connection-badge" class:connected={simulation.connected}>
		{simulation.connected ? 'Connected' : 'Disconnected'}
	</span>
</div>

<style>
	.controls {
		background: #fff;
		border: 1px solid #e8ecf1;
		border-radius: 14px;
		padding: 10px 16px;
		display: flex;
		align-items: center;
		gap: 12px;
		flex-wrap: wrap;
		box-shadow: 0 1px 4px rgba(0, 0, 0, 0.03);
	}

	.play-btn {
		background: #333;
		color: white;
		border: none;
		border-radius: 6px;
		padding: 6px 20px;
		font-size: 13px;
		font-weight: 600;
		cursor: pointer;
		min-width: 72px;
	}

	.play-btn:hover {
		background: #555;
	}

	.control-label {
		display: flex;
		align-items: center;
		gap: 6px;
		color: #666;
		font-size: 12px;
	}

	.control-label.seek {
		margin-left: auto;
	}

	select {
		background: #f8fafc;
		color: #222;
		border: 1px solid #d1d5db;
		border-radius: 6px;
		padding: 5px 8px;
		font-size: 12px;
	}

	.sim-time {
		color: #333;
		font-size: 13px;
		font-family: 'SF Mono', 'Cascadia Code', 'Fira Code', monospace;
	}

	.price-badge {
		font-size: 11px;
		color: #5bb88a;
		padding: 3px 8px;
		border-radius: 10px;
		background: #edf8f2;
		white-space: nowrap;
		font-family: 'SF Mono', 'Cascadia Code', 'Fira Code', monospace;
	}

	.price-badge.cheap {
		color: #e0a040;
		background: #fef6d8;
	}

	.price-badge.negative {
		color: #e87c6c;
		background: #fdf0ee;
	}

	.connection-badge {
		font-size: 11px;
		color: #e87c6c;
		padding: 3px 8px;
		border-radius: 10px;
		background: #fdf0ee;
		white-space: nowrap;
	}

	.connection-badge.connected {
		color: #5bb88a;
		background: #edf8f2;
	}

	.toggle-label {
		display: flex;
		align-items: center;
		gap: 5px;
		cursor: pointer;
		font-size: 12px;
		font-weight: 600;
		color: #334155;
	}

	.toggle-label input {
		width: 14px;
		height: 14px;
		accent-color: #64b5f6;
	}

	input[type='datetime-local'] {
		background: #f8fafc;
		color: #222;
		border: 1px solid #d1d5db;
		border-radius: 6px;
		padding: 5px 8px;
		font-size: 12px;
	}
</style>
