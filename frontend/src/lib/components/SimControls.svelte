<script lang="ts">
	import { simulation } from '$lib/stores/simulation.svelte';

	const speedOptions = [
		{ value: 3600, label: '1 h/s' },
		{ value: 7200, label: '2 h/s' },
		{ value: 14400, label: '4 h/s' },
		{ value: 28800, label: '8 h/s' },
		{ value: 86400, label: '1 d/s' },
		{ value: 604800, label: '1 w/s' }
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
	<div class="controls-row">
		<button class="play-btn" onclick={togglePlayPause}>
			{simulation.running ? 'Pause' : 'Play'}
		</button>

		<label class="speed-control">
			Speed:
			<select onchange={handleSpeedChange}>
				{#each speedOptions as opt}
					<option value={String(opt.value)} selected={simulation.speed === opt.value}>{opt.label}</option>
				{/each}
			</select>
		</label>

		<span class="sim-time">
			{formatSimTime(simulation.simTime)}
		</span>

		<span class="connection-status" class:connected={simulation.connected}>
			{simulation.connected ? 'Connected' : 'Disconnected'}
		</span>
	</div>

	<div class="controls-row">
		<label class="seek-control">
			Seek:
			<input
				type="datetime-local"
				min={toDatetimeLocal(simulation.timeRangeStart)}
				max={toDatetimeLocal(simulation.timeRangeEnd)}
				onchange={handleSeek}
			/>
		</label>
	</div>
</div>

<style>
	.controls {
		background: #f8f8f8;
		border: 1px solid #ddd;
		border-radius: 8px;
		padding: 16px;
		display: flex;
		flex-direction: column;
		gap: 12px;
	}

	.controls-row {
		display: flex;
		align-items: center;
		gap: 16px;
		flex-wrap: wrap;
	}

	.play-btn {
		background: #333;
		color: white;
		border: none;
		border-radius: 6px;
		padding: 8px 24px;
		font-size: 14px;
		font-weight: 600;
		cursor: pointer;
		min-width: 90px;
	}

	.play-btn:hover {
		background: #555;
	}

	.speed-control {
		display: flex;
		align-items: center;
		gap: 8px;
		color: #666;
		font-size: 13px;
	}

	select {
		background: #fff;
		color: #222;
		border: 1px solid #ccc;
		border-radius: 4px;
		padding: 6px 8px;
		font-size: 13px;
	}

	.sim-time {
		color: #333;
		font-size: 14px;
		font-family: monospace;
		margin-left: auto;
	}

	.connection-status {
		font-size: 12px;
		color: #c0392b;
		padding: 4px 8px;
		border-radius: 4px;
		background: #fdecea;
	}

	.connection-status.connected {
		color: #27ae60;
		background: #eafaf1;
	}

	.seek-control {
		display: flex;
		align-items: center;
		gap: 8px;
		color: #666;
		font-size: 13px;
	}

	input[type='datetime-local'] {
		background: #fff;
		color: #222;
		border: 1px solid #ccc;
		border-radius: 4px;
		padding: 6px 8px;
		font-size: 13px;
	}
</style>
