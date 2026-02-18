<script lang="ts">
	import { simulation } from '$lib/stores/simulation.svelte';
	import HelpTip from './HelpTip.svelte';

	let diag = $derived(simulation.hpDiagnostics);
	let hasData = $derived(diag !== null && (diag.cop > 0 || diag.compressor_speed_rpm > 0));

	function fmt(v: number, decimals = 1): string {
		return v.toFixed(decimals);
	}
</script>

{#if hasData && diag}
<div class="card">
	<h3>Heat Pump Diagnostics <HelpTip key="hp_diagnostics" /></h3>

	<div class="diag-grid">
		<div class="diag-section">
			<h4>Performance</h4>
			<div class="diag-row">
				<span class="label">COP</span>
				<span class="value cop" class:good={diag.cop >= 3} class:warning={diag.cop > 0 && diag.cop < 2}>
					{fmt(diag.cop, 2)}
				</span>
			</div>
			<div class="diag-row">
				<span class="label">Thermal Power</span>
				<span class="value">{fmt(diag.thermal_power_w, 0)} W</span>
			</div>
			<div class="diag-row">
				<span class="label">Pump Flow</span>
				<span class="value">{fmt(diag.pump_flow_lmin)} L/min</span>
			</div>
		</div>

		<div class="diag-section">
			<h4>Temperatures</h4>
			<div class="diag-row">
				<span class="label">Inlet</span>
				<span class="value">{fmt(diag.inlet_temp_c)}&deg;C</span>
			</div>
			<div class="diag-row">
				<span class="label">Outlet</span>
				<span class="value">{fmt(diag.outlet_temp_c)}&deg;C</span>
			</div>
			<div class="diag-row">
				<span class="label">&Delta;T</span>
				<span class="value">{fmt(diag.outlet_temp_c - diag.inlet_temp_c)}&deg;C</span>
			</div>
			<div class="diag-row">
				<span class="label">DHW Tank</span>
				<span class="value">{fmt(diag.dhw_temp_c)}&deg;C</span>
			</div>
			<div class="diag-row">
				<span class="label">Z1 Target</span>
				<span class="value">{fmt(diag.z1_target_temp_c)}&deg;C</span>
			</div>
		</div>

		<div class="diag-section">
			<h4>Compressor</h4>
			<div class="diag-row">
				<span class="label">Speed</span>
				<span class="value">{fmt(diag.compressor_speed_rpm, 0)} RPM</span>
			</div>
			<div class="diag-row">
				<span class="label">Fan Speed</span>
				<span class="value">{fmt(diag.fan_speed_rpm, 0)} RPM</span>
			</div>
			<div class="diag-row">
				<span class="label">Discharge</span>
				<span class="value">{fmt(diag.discharge_temp_c)}&deg;C</span>
			</div>
			<div class="diag-row">
				<span class="label">High Pressure</span>
				<span class="value">{fmt(diag.high_pressure)} Kgf/cm&sup2;</span>
			</div>
		</div>

		<div class="diag-section">
			<h4>Refrigerant</h4>
			<div class="diag-row">
				<span class="label">Inside Pipe</span>
				<span class="value">{fmt(diag.inside_pipe_temp_c)}&deg;C</span>
			</div>
			<div class="diag-row">
				<span class="label">Outside Pipe</span>
				<span class="value">{fmt(diag.outside_pipe_temp_c)}&deg;C</span>
			</div>
		</div>
	</div>
</div>
{/if}

<style>
	.card {
		background: #fff;
		border: 1px solid #e8ecf1;
		border-radius: 14px;
		padding: 20px;
	}

	h3 {
		margin: 0 0 16px;
		font-size: 15px;
		font-weight: 600;
		color: #334155;
		display: flex;
		align-items: center;
		gap: 6px;
	}

	.diag-grid {
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: 16px;
	}

	.diag-section h4 {
		margin: 0 0 8px;
		font-size: 12px;
		font-weight: 600;
		color: #94a3b8;
		text-transform: uppercase;
		letter-spacing: 0.5px;
	}

	.diag-row {
		display: flex;
		justify-content: space-between;
		align-items: center;
		padding: 3px 0;
		font-size: 13px;
	}

	.label {
		color: #64748b;
	}

	.value {
		font-family: 'SF Mono', 'Cascadia Code', 'Fira Code', monospace;
		font-size: 13px;
		font-weight: 500;
		color: #334155;
	}

	.cop {
		font-size: 15px;
		font-weight: 700;
	}

	.cop.good {
		color: #5bb88a;
	}

	.cop.warning {
		color: #e87c6c;
	}
</style>
