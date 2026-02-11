<script lang="ts">
	import { simulation } from '$lib/stores/simulation';

	const speedOptions = [1, 2, 5, 10, 50, 100, 500, 1000];

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
		if (!iso) return '—';
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
			{simulation.running ? '⏸ Pause' : '▶ Play'}
		</button>

		<label class="speed-control">
			Speed:
			<select value={String(simulation.speed)} onchange={handleSpeedChange}>
				{#each speedOptions as s}
					<option value={String(s)}>{s}x</option>
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
		background: #1a1a2e;
		border: 1px solid #2a2a4a;
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
		background: #4a9eff;
		color: white;
		border: none;
		border-radius: 6px;
		padding: 8px 20px;
		font-size: 14px;
		font-weight: 600;
		cursor: pointer;
		min-width: 100px;
	}

	.play-btn:hover {
		background: #3a8eef;
	}

	.speed-control {
		display: flex;
		align-items: center;
		gap: 8px;
		color: #aaa;
		font-size: 13px;
	}

	select {
		background: #0f0f23;
		color: white;
		border: 1px solid #2a2a4a;
		border-radius: 4px;
		padding: 6px 8px;
		font-size: 13px;
	}

	.sim-time {
		color: #4a9eff;
		font-size: 14px;
		font-family: monospace;
		margin-left: auto;
	}

	.connection-status {
		font-size: 12px;
		color: #ff4444;
		padding: 4px 8px;
		border-radius: 4px;
		background: rgba(255, 68, 68, 0.1);
	}

	.connection-status.connected {
		color: #44ff44;
		background: rgba(68, 255, 68, 0.1);
	}

	.seek-control {
		display: flex;
		align-items: center;
		gap: 8px;
		color: #aaa;
		font-size: 13px;
	}

	input[type='datetime-local'] {
		background: #0f0f23;
		color: white;
		border: 1px solid #2a2a4a;
		border-radius: 4px;
		padding: 6px 8px;
		font-size: 13px;
	}
</style>
