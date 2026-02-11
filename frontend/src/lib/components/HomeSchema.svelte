<script lang="ts">
	import { simulation } from '$lib/stores/simulation.svelte';

	function formatPower(value: number): string {
		const abs = Math.abs(value);
		if (abs >= 1000) {
			return (abs / 1000).toFixed(2) + ' kW';
		}
		return abs.toFixed(0) + ' W';
	}

	let consuming = $derived(simulation.currentPower >= 0);
	let hasPower = $derived(simulation.currentPowerTimestamp !== '');
</script>

<div class="schema-card">
	<svg viewBox="0 0 700 260" xmlns="http://www.w3.org/2000/svg">
		<defs>
			<linearGradient id="wireGrad" x1="0%" y1="0%" x2="100%" y2="0%">
				<stop offset="0%" stop-color={consuming ? '#ef4444' : '#22c55e'} stop-opacity="0.15" />
				<stop offset="50%" stop-color={consuming ? '#ef4444' : '#22c55e'} stop-opacity="0.4" />
				<stop offset="100%" stop-color={consuming ? '#ef4444' : '#22c55e'} stop-opacity="0.15" />
			</linearGradient>
			<filter id="glow">
				<feGaussianBlur stdDeviation="2" result="blur" />
				<feMerge>
					<feMergeNode in="blur" />
					<feMergeNode in="SourceGraphic" />
				</feMerge>
			</filter>
		</defs>

		<!-- Grid icon group -->
		<g transform="translate(70, 30)">
			<!-- Pylon -->
			<line x1="0" y1="20" x2="0" y2="140" stroke="#94a3b8" stroke-width="4" />
			<line x1="0" y1="140" x2="-22" y2="180" stroke="#94a3b8" stroke-width="3" />
			<line x1="0" y1="140" x2="22" y2="180" stroke="#94a3b8" stroke-width="3" />
			<!-- Cross arms -->
			<line x1="-35" y1="40" x2="35" y2="40" stroke="#94a3b8" stroke-width="3.5" />
			<line x1="-25" y1="70" x2="25" y2="70" stroke="#94a3b8" stroke-width="3" />
			<line x1="-18" y1="100" x2="18" y2="100" stroke="#94a3b8" stroke-width="2.5" />
			<!-- Struts -->
			<line x1="-12" y1="120" x2="12" y2="120" stroke="#94a3b8" stroke-width="2" />
			<line x1="-17" y1="155" x2="17" y2="155" stroke="#94a3b8" stroke-width="1.5" />
			<!-- Insulators -->
			<circle cx="-35" cy="45" r="3" fill="#cbd5e1" />
			<circle cx="35" cy="45" r="3" fill="#cbd5e1" />
			<circle cx="-25" cy="75" r="2.5" fill="#cbd5e1" />
			<circle cx="25" cy="75" r="2.5" fill="#cbd5e1" />
			<!-- Wires out -->
			<path d="M-35,45 Q-55,25 -60,10" fill="none" stroke="#94a3b8" stroke-width="1.5" />
			<path d="M35,45 Q55,25 60,10" fill="none" stroke="#94a3b8" stroke-width="1.5" />
		</g>

		<!-- Label: Grid -->
		<text x="70" y="228" text-anchor="middle" class="node-label">Grid</text>

		<!-- Connection wire -->
		<g>
			<!-- Wire background glow -->
			{#if hasPower}
				<line x1="130" y1="148" x2="480" y2="148" stroke="url(#wireGrad)" stroke-width="18" stroke-linecap="round" />
			{/if}
			<!-- Wire core -->
			<line x1="130" y1="148" x2="480" y2="148" stroke="#d1d5db" stroke-width="3" stroke-linecap="round" />
		</g>

		<!-- Power flow dots -->
		{#if hasPower}
			{#if consuming}
				<circle r="5" fill="#ef4444" filter="url(#glow)" class="flow-dot dot1">
					<animateMotion dur="2s" repeatCount="indefinite" path="M130,148 L480,148" />
				</circle>
				<circle r="5" fill="#ef4444" filter="url(#glow)" class="flow-dot dot2">
					<animateMotion dur="2s" repeatCount="indefinite" begin="0.66s" path="M130,148 L480,148" />
				</circle>
				<circle r="5" fill="#ef4444" filter="url(#glow)" class="flow-dot dot3">
					<animateMotion dur="2s" repeatCount="indefinite" begin="1.33s" path="M130,148 L480,148" />
				</circle>
			{:else}
				<circle r="5" fill="#22c55e" filter="url(#glow)" class="flow-dot dot1">
					<animateMotion dur="2s" repeatCount="indefinite" path="M480,148 L130,148" />
				</circle>
				<circle r="5" fill="#22c55e" filter="url(#glow)" class="flow-dot dot2">
					<animateMotion dur="2s" repeatCount="indefinite" begin="0.66s" path="M480,148 L130,148" />
				</circle>
				<circle r="5" fill="#22c55e" filter="url(#glow)" class="flow-dot dot3">
					<animateMotion dur="2s" repeatCount="indefinite" begin="1.33s" path="M480,148 L130,148" />
				</circle>
			{/if}
		{/if}

		<!-- Smart meter -->
		<g transform="translate(195, 128)">
			<rect x="0" y="0" width="24" height="40" rx="4" fill="white" stroke="#94a3b8" stroke-width="1.5" />
			<rect x="4" y="5" width="16" height="10" rx="2" fill="#e2e8f0" />
			<circle cx="12" cy="27" r="3" fill="none" stroke="#94a3b8" stroke-width="1" />
			<line x1="12" y1="24" x2="12" y2="27" stroke="#94a3b8" stroke-width="1" />
		</g>

		<!-- Power reading badge -->
		<g transform="translate(305, 100)">
			<rect x="-60" y="-20" width="120" height="52" rx="10"
				fill="white" stroke={consuming ? '#fca5a5' : '#86efac'} stroke-width="1.5" />
			<text x="0" y="-2" text-anchor="middle" class="power-label">
				{consuming ? 'Consuming' : 'Exporting'}
			</text>
			<text x="0" y="22" text-anchor="middle" class="power-value" class:exporting={!consuming}>
				{hasPower ? formatPower(simulation.currentPower) : '-- W'}
			</text>
		</g>

		<!-- House group -->
		<g transform="translate(520, 30)">
			<!-- Roof -->
			<path d="M-10,115 L70,55 L150,115" fill="none" stroke="#475569" stroke-width="3.5" stroke-linejoin="round" stroke-linecap="round" />
			<!-- Chimney -->
			<rect x="100" y="66" width="14" height="35" fill="#f8fafc" stroke="#475569" stroke-width="2" rx="1" />
			<!-- Walls -->
			<rect x="8" y="115" width="124" height="80" fill="#f8fafc" stroke="#475569" stroke-width="3" rx="2" />
			<!-- Door -->
			<rect x="55" y="148" width="30" height="47" fill="#f1f5f9" stroke="#475569" stroke-width="2" rx="3" />
			<circle cx="78" cy="174" r="2" fill="#94a3b8" />
			<!-- Window left -->
			<rect x="22" y="130" width="22" height="22" fill="#e0f2fe" stroke="#475569" stroke-width="1.5" rx="2" />
			<line x1="33" y1="130" x2="33" y2="152" stroke="#475569" stroke-width="0.8" />
			<line x1="22" y1="141" x2="44" y2="141" stroke="#475569" stroke-width="0.8" />
			<!-- Window right -->
			<rect x="96" y="130" width="22" height="22" fill="#e0f2fe" stroke="#475569" stroke-width="1.5" rx="2" />
			<line x1="107" y1="130" x2="107" y2="152" stroke="#475569" stroke-width="0.8" />
			<line x1="96" y1="141" x2="118" y2="141" stroke="#475569" stroke-width="0.8" />
		</g>

		<!-- Label: Home -->
		<text x="590" y="228" text-anchor="middle" class="node-label">Home</text>
	</svg>
</div>

<style>
	.schema-card {
		width: 100%;
		max-width: 750px;
		margin: 0 auto;
		background: #fafbfc;
		border: 1px solid #e5e7eb;
		border-radius: 12px;
		padding: 20px 16px 12px;
	}

	svg {
		width: 100%;
		height: auto;
		display: block;
	}

	.node-label {
		font-size: 15px;
		font-weight: 600;
		fill: #64748b;
		letter-spacing: 0.03em;
	}

	.power-label {
		font-size: 11px;
		fill: #94a3b8;
		font-weight: 500;
		text-transform: uppercase;
		letter-spacing: 0.06em;
	}

	.power-value {
		font-size: 18px;
		font-weight: 700;
		font-family: 'SF Mono', 'Cascadia Code', 'Fira Code', monospace;
		fill: #ef4444;
	}

	.power-value.exporting {
		fill: #22c55e;
	}

	.flow-dot {
		opacity: 0.85;
	}
</style>
