<script lang="ts">
	import { simulation } from '$lib/stores/simulation.svelte';

	function handleToggle() {
		simulation.setPVConfig();
	}

	function handleArrayChange() {
		if (simulation.pvCustomEnabled) {
			simulation.setPVConfig();
		}
	}

	function handleArrayToggle(index: number) {
		simulation.pvArrays[index].enabled = !simulation.pvArrays[index].enabled;
		simulation.pvArrays = [...simulation.pvArrays];
		handleArrayChange();
	}

	function handlePeakChange(index: number, e: Event) {
		const target = e.target as HTMLInputElement;
		simulation.pvArrays[index].peak_wp = Number(target.value);
		simulation.pvArrays = [...simulation.pvArrays];
		handleArrayChange();
	}

	function handleTiltChange(index: number, e: Event) {
		const target = e.target as HTMLInputElement;
		simulation.pvArrays[index].tilt = Number(target.value);
		simulation.pvArrays = [...simulation.pvArrays];
		handleArrayChange();
	}

	const azimuthLabels: Record<number, string> = {
		90: 'East',
		180: 'South',
		270: 'West'
	};
</script>

<div class="pv-config-card">
	<div class="header-row">
		<span class="section-title">Custom PV Configuration</span>
		<label class="toggle-label">
			<input
				type="checkbox"
				bind:checked={simulation.pvCustomEnabled}
				onchange={handleToggle}
			/>
			<span class="toggle-text">{simulation.pvCustomEnabled ? 'On' : 'Off'}</span>
		</label>
	</div>

	{#if simulation.pvCustomEnabled}
		<div class="arrays-grid">
			<div class="grid-header">
				<span></span>
				<span>Peak (W)</span>
				<span>Azimuth</span>
				<span>Tilt (&deg;)</span>
				<span>Active</span>
			</div>
			{#each simulation.pvArrays as arr, i}
				<div class="grid-row" class:disabled={!arr.enabled}>
					<span class="array-name">{arr.name}</span>
					<input
						type="number"
						min="0"
						max="50000"
						step="100"
						value={arr.peak_wp}
						onchange={(e) => handlePeakChange(i, e)}
						disabled={!arr.enabled}
					/>
					<span class="azimuth-hint">{azimuthLabels[arr.azimuth] ?? arr.azimuth + '°'} ({arr.azimuth}°)</span>
					<input
						type="number"
						min="0"
						max="90"
						step="5"
						value={arr.tilt}
						onchange={(e) => handleTiltChange(i, e)}
						disabled={!arr.enabled}
					/>
					<label class="array-toggle">
						<input
							type="checkbox"
							checked={arr.enabled}
							onchange={() => handleArrayToggle(i)}
						/>
					</label>
				</div>
			{/each}
		</div>
	{/if}
</div>

<style>
	.pv-config-card {
		background: #fff;
		border: 1px solid #e8ecf1;
		border-radius: 14px;
		padding: 14px 20px;
		box-shadow: 0 1px 4px rgba(0, 0, 0, 0.03);
	}

	.header-row {
		display: flex;
		justify-content: space-between;
		align-items: center;
		margin-bottom: 10px;
	}

	.section-title {
		font-size: 11px;
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.06em;
		color: #94a3b8;
	}

	.toggle-label {
		display: flex;
		align-items: center;
		gap: 6px;
		cursor: pointer;
		font-size: 12px;
	}

	.toggle-text {
		color: #64748b;
		font-weight: 500;
	}

	.arrays-grid {
		display: flex;
		flex-direction: column;
		gap: 6px;
	}

	.grid-header {
		display: grid;
		grid-template-columns: 60px 1fr 1fr 80px 50px;
		gap: 8px;
		font-size: 10px;
		color: #94a3b8;
		text-transform: uppercase;
		letter-spacing: 0.5px;
	}

	.grid-row {
		display: grid;
		grid-template-columns: 60px 1fr 1fr 80px 50px;
		gap: 8px;
		align-items: center;
	}

	.grid-row.disabled {
		opacity: 0.5;
	}

	.array-name {
		font-size: 12px;
		font-weight: 600;
		color: #475569;
	}

	.azimuth-hint {
		font-size: 11px;
		color: #64748b;
	}

	input[type='number'] {
		background: #f8fafc;
		color: #222;
		border: 1px solid #d1d5db;
		border-radius: 6px;
		padding: 4px 8px;
		font-size: 12px;
		font-family: 'SF Mono', 'Cascadia Code', 'Fira Code', monospace;
		width: 100%;
		box-sizing: border-box;
	}

	input[type='number']:disabled {
		background: #f1f5f9;
		color: #94a3b8;
	}

	.array-toggle {
		display: flex;
		justify-content: center;
		cursor: pointer;
	}
</style>
