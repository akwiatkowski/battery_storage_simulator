<script lang="ts">
	import { simulation } from '$lib/stores/simulation.svelte';

	function handleChange() {
		simulation.setBatteryConfig();
	}
</script>

<div class="battery-config">
	<div class="config-header">
		<label class="toggle-label">
			<input
				type="checkbox"
				bind:checked={simulation.batteryEnabled}
				onchange={handleChange}
			/>
			<span>Battery Storage</span>
		</label>
	</div>

	{#if simulation.batteryEnabled}
		<div class="config-fields">
			<label class="field">
				<span class="field-label">Capacity</span>
				<div class="field-input">
					<input
						type="number"
						min="1"
						max="100"
						step="1"
						bind:value={simulation.batteryCapacityKWh}
						onchange={handleChange}
					/>
					<span class="field-unit">kWh</span>
				</div>
			</label>

			<label class="field">
				<span class="field-label">Max Power</span>
				<div class="field-input">
					<input
						type="number"
						min="0.5"
						max="50"
						step="0.5"
						bind:value={simulation.batteryMaxPowerKW}
						onchange={handleChange}
					/>
					<span class="field-unit">kW</span>
				</div>
			</label>

			<label class="field">
				<span class="field-label">Discharge to</span>
				<div class="field-input">
					<input
						type="number"
						min="0"
						max="100"
						step="5"
						bind:value={simulation.batteryDischargeToPercent}
						onchange={handleChange}
					/>
					<span class="field-unit">%</span>
				</div>
			</label>

			<label class="field">
				<span class="field-label">Charge to</span>
				<div class="field-input">
					<input
						type="number"
						min="0"
						max="100"
						step="5"
						bind:value={simulation.batteryChargeToPercent}
						onchange={handleChange}
					/>
					<span class="field-unit">%</span>
				</div>
			</label>

			<label class="field">
				<span class="field-label">Degradation (to 80%)</span>
				<div class="field-input">
					<input
						type="number"
						min="0"
						max="20000"
						step="100"
						bind:value={simulation.batteryDegradationCycles}
						onchange={handleChange}
					/>
					<span class="field-unit">cycles</span>
				</div>
			</label>
		</div>
	{/if}
</div>

<style>
	.battery-config {
		background: #f8fafb;
		border: 1px solid #e8ecf1;
		border-radius: 14px;
		padding: 16px;
	}

	.config-header {
		margin-bottom: 4px;
	}

	.toggle-label {
		display: flex;
		align-items: center;
		gap: 8px;
		cursor: pointer;
		font-size: 15px;
		font-weight: 600;
		color: #334155;
	}

	.toggle-label input {
		width: 18px;
		height: 18px;
		accent-color: #64b5f6;
	}

	.config-fields {
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: 12px;
		margin-top: 12px;
	}

	.field {
		display: flex;
		flex-direction: column;
		gap: 4px;
	}

	.field-label {
		font-size: 12px;
		font-weight: 500;
		color: #64748b;
		text-transform: uppercase;
		letter-spacing: 0.04em;
	}

	.field-input {
		display: flex;
		align-items: center;
		gap: 6px;
	}

	.field-input input {
		width: 80px;
		padding: 6px 8px;
		border: 1px solid #d1d5db;
		border-radius: 6px;
		font-size: 14px;
		font-family: 'SF Mono', 'Cascadia Code', 'Fira Code', monospace;
		text-align: right;
	}

	.field-input input:focus {
		outline: none;
		border-color: #64b5f6;
		box-shadow: 0 0 0 2px rgba(100, 181, 246, 0.15);
	}

	.field-unit {
		font-size: 13px;
		color: #94a3b8;
		font-weight: 500;
	}
</style>
