<script lang="ts">
	import { simulation } from '$lib/stores/simulation.svelte';

	function formatPower(value: number): string {
		const abs = Math.abs(value);
		if (abs >= 1000) {
			return (abs / 1000).toFixed(2) + ' kW';
		}
		return abs.toFixed(0) + ' W';
	}

	let hasPower = $derived(simulation.currentPowerTimestamp !== '');
	let batteryOn = $derived(simulation.batteryEnabled);

	// Grid power values
	let gridPower = $derived(batteryOn ? simulation.adjustedGridW : simulation.currentPower);
	let gridConsuming = $derived(gridPower >= 0);

	// Battery values
	let batteryPower = $derived(simulation.batteryPowerW);
	let batteryDischarging = $derived(batteryPower > 0);
	let batterySoC = $derived(simulation.batterySoCPercent);
	let batteryStoredKWh = $derived((batterySoC / 100) * simulation.batteryCapacityKWh);

	// Home demand (raw grid power = what home actually uses/produces)
	let homeDemand = $derived(simulation.currentPower);
	let homeConsuming = $derived(homeDemand >= 0);

	// PV
	let pvPower = $derived(simulation.currentPVPower);
	let pvProducing = $derived(pvPower > 10);
	let pvHighPower = $derived(pvPower > 1000);

	// Heat pump
	let heatPumpPower = $derived(simulation.currentHeatPumpPower);

	// Derived: appliance power = home demand - heat pump
	let appliancePower = $derived(Math.max(0, homeDemand - heatPumpPower));

	// Grid wire color
	let gridColor = $derived(gridConsuming ? '#ef4444' : '#22c55e');
	// Battery wire color: orange for discharge, blue for charge
	let batteryColor = $derived(batteryDischarging ? '#f59e0b' : '#3b82f6');

	// Battery fill color based on SoC
	let batteryFill = $derived(
		batterySoC > 60 ? '#22c55e' : batterySoC > 20 ? '#f59e0b' : '#ef4444'
	);

	// Battery fill height for SVG
	let batteryFillHeight = $derived((Math.max(0, Math.min(100, batterySoC)) / 100) * 108);

	// Show extra dots for high power
	let gridHighPower = $derived(Math.abs(gridPower) > 1000);
	let batteryHighPower = $derived(Math.abs(batteryPower) > 1000);

	// PV color
	const pvColor = '#eab308';
</script>

<div class="schema-card">
	{#if batteryOn}
		<!-- Four-node layout: PV at top, Grid — Battery — Home horizontal -->
		<svg viewBox="0 0 900 340" xmlns="http://www.w3.org/2000/svg">
			<defs>
				<linearGradient id="wireGradGrid" x1="0%" y1="0%" x2="100%" y2="0%">
					<stop offset="0%" stop-color={gridColor} stop-opacity="0.15" />
					<stop offset="50%" stop-color={gridColor} stop-opacity="0.4" />
					<stop offset="100%" stop-color={gridColor} stop-opacity="0.15" />
				</linearGradient>
				<linearGradient id="wireGradBat" x1="0%" y1="0%" x2="100%" y2="0%">
					<stop offset="0%" stop-color={batteryColor} stop-opacity="0.15" />
					<stop offset="50%" stop-color={batteryColor} stop-opacity="0.4" />
					<stop offset="100%" stop-color={batteryColor} stop-opacity="0.15" />
				</linearGradient>
				<linearGradient id="wireGradPV" x1="0%" y1="0%" x2="0%" y2="100%">
					<stop offset="0%" stop-color={pvColor} stop-opacity="0.15" />
					<stop offset="50%" stop-color={pvColor} stop-opacity="0.4" />
					<stop offset="100%" stop-color={pvColor} stop-opacity="0.15" />
				</linearGradient>
				<filter id="glow">
					<feGaussianBlur stdDeviation="2" result="blur" />
					<feMerge>
						<feMergeNode in="blur" />
						<feMergeNode in="SourceGraphic" />
					</feMerge>
				</filter>
			</defs>

			<!-- PV Solar panel icon at top center (above wire junction) -->
			<g transform="translate(400, 10)">
				<!-- Panel body -->
				<rect x="-30" y="0" width="60" height="40" rx="4" fill="#fef9c3" stroke="#ca8a04" stroke-width="2" />
				<!-- Panel grid lines -->
				<line x1="-30" y1="20" x2="30" y2="20" stroke="#ca8a04" stroke-width="1" opacity="0.5" />
				<line x1="-10" y1="0" x2="-10" y2="40" stroke="#ca8a04" stroke-width="1" opacity="0.5" />
				<line x1="10" y1="0" x2="10" y2="40" stroke="#ca8a04" stroke-width="1" opacity="0.5" />
				<!-- Sun rays -->
				<circle cx="0" cy="-14" r="8" fill="#fbbf24" opacity="0.7" />
				<line x1="-16" y1="-14" x2="-12" y2="-14" stroke="#fbbf24" stroke-width="2" stroke-linecap="round" />
				<line x1="12" y1="-14" x2="16" y2="-14" stroke="#fbbf24" stroke-width="2" stroke-linecap="round" />
				<line x1="0" y1="-30" x2="0" y2="-26" stroke="#fbbf24" stroke-width="2" stroke-linecap="round" />
				<line x1="-10" y1="-24" x2="-8" y2="-22" stroke="#fbbf24" stroke-width="2" stroke-linecap="round" />
				<line x1="10" y1="-24" x2="8" y2="-22" stroke="#fbbf24" stroke-width="2" stroke-linecap="round" />
			</g>
			<text x="400" y="68" text-anchor="middle" class="node-label">PV</text>

			<!-- PV vertical wire down to junction point at y=148 -->
			<g>
				{#if pvProducing}
					<line x1="400" y1="75" x2="400" y2="148" stroke="url(#wireGradPV)" stroke-width="18" stroke-linecap="round" />
				{/if}
				<line x1="400" y1="75" x2="400" y2="148" stroke="#d1d5db" stroke-width="3" stroke-linecap="round" />
			</g>

			<!-- PV power badge -->
			{#if pvProducing}
				<g transform="translate(450, 95)">
					<rect x="-40" y="-14" width="80" height="32" rx="8"
						fill="white" stroke="#fcd34d" stroke-width="1.5" />
					<text x="0" y="6" text-anchor="middle" class="power-value pv-power">
						{formatPower(pvPower)}
					</text>
				</g>
			{/if}

			<!-- PV flow dots (downward) -->
			{#if pvProducing}
				<circle r="5" fill={pvColor} filter="url(#glow)" class="flow-dot">
					<animateMotion dur="1.5s" repeatCount="indefinite" path="M400,75 L400,148" />
				</circle>
				<circle r="5" fill={pvColor} filter="url(#glow)" class="flow-dot">
					<animateMotion dur="1.5s" repeatCount="indefinite" begin="0.5s" path="M400,75 L400,148" />
				</circle>
				{#if pvHighPower}
					<circle r="5" fill={pvColor} filter="url(#glow)" class="flow-dot">
						<animateMotion dur="1.5s" repeatCount="indefinite" begin="1s" path="M400,75 L400,148" />
					</circle>
				{/if}
			{/if}

			<!-- Grid icon (shifted down by 70 for PV space) -->
			<g transform="translate(70, 100)">
				<line x1="0" y1="20" x2="0" y2="140" stroke="#94a3b8" stroke-width="4" />
				<line x1="0" y1="140" x2="-22" y2="180" stroke="#94a3b8" stroke-width="3" />
				<line x1="0" y1="140" x2="22" y2="180" stroke="#94a3b8" stroke-width="3" />
				<line x1="-35" y1="40" x2="35" y2="40" stroke="#94a3b8" stroke-width="3.5" />
				<line x1="-25" y1="70" x2="25" y2="70" stroke="#94a3b8" stroke-width="3" />
				<line x1="-18" y1="100" x2="18" y2="100" stroke="#94a3b8" stroke-width="2.5" />
				<line x1="-12" y1="120" x2="12" y2="120" stroke="#94a3b8" stroke-width="2" />
				<line x1="-17" y1="155" x2="17" y2="155" stroke="#94a3b8" stroke-width="1.5" />
				<circle cx="-35" cy="45" r="3" fill="#cbd5e1" />
				<circle cx="35" cy="45" r="3" fill="#cbd5e1" />
				<circle cx="-25" cy="75" r="2.5" fill="#cbd5e1" />
				<circle cx="25" cy="75" r="2.5" fill="#cbd5e1" />
				<path d="M-35,45 Q-55,25 -60,10" fill="none" stroke="#94a3b8" stroke-width="1.5" />
				<path d="M35,45 Q55,25 60,10" fill="none" stroke="#94a3b8" stroke-width="1.5" />
			</g>
			<text x="70" y="298" text-anchor="middle" class="node-label">Grid</text>

			<!-- Grid wire (Grid → Battery junction) y=218 (100+118=218) -->
			<g>
				{#if hasPower}
					<line x1="130" y1="218" x2="340" y2="218" stroke="url(#wireGradGrid)" stroke-width="18" stroke-linecap="round" />
				{/if}
				<line x1="130" y1="218" x2="340" y2="218" stroke="#d1d5db" stroke-width="3" stroke-linecap="round" />
			</g>

			<!-- Grid direction chevron -->
			{#if hasPower && Math.abs(gridPower) > 10}
				{#if gridConsuming}
					<path d="M305,210 L315,218 L305,226" fill="none" stroke={gridColor} stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" opacity="0.7" />
				{:else}
					<path d="M165,210 L155,218 L165,226" fill="none" stroke={gridColor} stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" opacity="0.7" />
				{/if}
			{/if}

			<!-- Grid power dots -->
			{#if hasPower && Math.abs(gridPower) > 10}
				{#if gridConsuming}
					<circle r="5" fill={gridColor} filter="url(#glow)" class="flow-dot">
						<animateMotion dur="2s" repeatCount="indefinite" path="M130,218 L340,218" />
					</circle>
					<circle r="5" fill={gridColor} filter="url(#glow)" class="flow-dot">
						<animateMotion dur="2s" repeatCount="indefinite" begin="0.66s" path="M130,218 L340,218" />
					</circle>
					{#if gridHighPower}
						<circle r="5" fill={gridColor} filter="url(#glow)" class="flow-dot">
							<animateMotion dur="2s" repeatCount="indefinite" begin="1.33s" path="M130,218 L340,218" />
						</circle>
					{/if}
				{:else}
					<circle r="5" fill={gridColor} filter="url(#glow)" class="flow-dot">
						<animateMotion dur="2s" repeatCount="indefinite" path="M340,218 L130,218" />
					</circle>
					<circle r="5" fill={gridColor} filter="url(#glow)" class="flow-dot">
						<animateMotion dur="2s" repeatCount="indefinite" begin="0.66s" path="M340,218 L130,218" />
					</circle>
					{#if gridHighPower}
						<circle r="5" fill={gridColor} filter="url(#glow)" class="flow-dot">
							<animateMotion dur="2s" repeatCount="indefinite" begin="1.33s" path="M340,218 L130,218" />
						</circle>
					{/if}
				{/if}
			{/if}

			<!-- Grid power badge -->
			<g transform="translate(235, 176)">
				<rect x="-50" y="-16" width="100" height="40" rx="8"
					fill="white" stroke={gridConsuming ? '#fca5a5' : '#86efac'} stroke-width="1.5" />
				<text x="0" y="-1" text-anchor="middle" class="power-label">
					{gridConsuming ? 'Import' : 'Export'}
				</text>
				<text x="0" y="17" text-anchor="middle" class="power-value" class:exporting={!gridConsuming}>
					{hasPower ? formatPower(gridPower) : '-- W'}
				</text>
			</g>

			<!-- Battery icon -->
			<g transform="translate(370, 150)">
				<!-- Battery body -->
				<rect x="0" y="0" width="60" height="120" rx="6" fill="white" stroke="#94a3b8" stroke-width="2.5" />
				<!-- Battery terminal -->
				<rect x="18" y="-8" width="24" height="10" rx="3" fill="#94a3b8" />
				<!-- SoC fill -->
				<rect x="6" y={114 - batteryFillHeight} width="48" height={batteryFillHeight} rx="3" fill={batteryFill} opacity="0.6" />
				<!-- Text backdrop for readability -->
				<rect x="4" y="38" width="52" height="48" rx="4" fill="white" opacity="0.7" />
				<!-- SoC label -->
				<text x="30" y="58" text-anchor="middle" class="battery-soc-label">
					{batterySoC.toFixed(0)}%
				</text>
				<!-- Stored energy -->
				<text x="30" y="78" text-anchor="middle" class="battery-kwh-label">
					{batteryStoredKWh.toFixed(1)} kWh
				</text>
			</g>
			<text x="400" y="298" text-anchor="middle" class="node-label">Battery</text>

			<!-- Battery wire (Battery → Home) -->
			<g>
				{#if hasPower}
					<line x1="460" y1="218" x2="660" y2="218" stroke="url(#wireGradBat)" stroke-width="18" stroke-linecap="round" />
				{/if}
				<line x1="460" y1="218" x2="660" y2="218" stroke="#d1d5db" stroke-width="3" stroke-linecap="round" />
			</g>

			<!-- Battery direction chevron -->
			{#if hasPower && Math.abs(batteryPower) > 10}
				{#if batteryDischarging}
					<path d="M625,210 L635,218 L625,226" fill="none" stroke={batteryColor} stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" opacity="0.7" />
				{:else}
					<path d="M495,210 L485,218 L495,226" fill="none" stroke={batteryColor} stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" opacity="0.7" />
				{/if}
			{/if}

			<!-- Battery power dots -->
			{#if hasPower && Math.abs(batteryPower) > 10}
				{#if batteryDischarging}
					<circle r="5" fill={batteryColor} filter="url(#glow)" class="flow-dot">
						<animateMotion dur="2s" repeatCount="indefinite" path="M460,218 L660,218" />
					</circle>
					<circle r="5" fill={batteryColor} filter="url(#glow)" class="flow-dot">
						<animateMotion dur="2s" repeatCount="indefinite" begin="0.66s" path="M460,218 L660,218" />
					</circle>
					{#if batteryHighPower}
						<circle r="5" fill={batteryColor} filter="url(#glow)" class="flow-dot">
							<animateMotion dur="2s" repeatCount="indefinite" begin="1.33s" path="M460,218 L660,218" />
						</circle>
					{/if}
				{:else}
					<circle r="5" fill={batteryColor} filter="url(#glow)" class="flow-dot">
						<animateMotion dur="2s" repeatCount="indefinite" path="M660,218 L460,218" />
					</circle>
					<circle r="5" fill={batteryColor} filter="url(#glow)" class="flow-dot">
						<animateMotion dur="2s" repeatCount="indefinite" begin="0.66s" path="M660,218 L460,218" />
					</circle>
					{#if batteryHighPower}
						<circle r="5" fill={batteryColor} filter="url(#glow)" class="flow-dot">
							<animateMotion dur="2s" repeatCount="indefinite" begin="1.33s" path="M660,218 L460,218" />
						</circle>
					{/if}
				{/if}
			{/if}

			<!-- Battery power badge -->
			<g transform="translate(560, 176)">
				<rect x="-50" y="-16" width="100" height="40" rx="8"
					fill="white" stroke={batteryDischarging ? '#fcd34d' : '#93c5fd'} stroke-width="1.5" />
				<text x="0" y="-1" text-anchor="middle" class="power-label">
					{batteryDischarging ? 'Discharge' : 'Charge'}
				</text>
				<text x="0" y="17" text-anchor="middle" class="power-value battery-power" class:charging={!batteryDischarging}>
					{hasPower ? formatPower(batteryPower) : '-- W'}
				</text>
			</g>

			<!-- House -->
			<g transform="translate(700, 100)">
				<path d="M-10,115 L70,55 L150,115" fill="none" stroke="#475569" stroke-width="3.5" stroke-linejoin="round" stroke-linecap="round" />
				<rect x="100" y="66" width="14" height="35" fill="#f8fafc" stroke="#475569" stroke-width="2" rx="1" />
				<rect x="8" y="115" width="124" height="80" fill="#f8fafc" stroke="#475569" stroke-width="3" rx="2" />
				<rect x="55" y="148" width="30" height="47" fill="#f1f5f9" stroke="#475569" stroke-width="2" rx="3" />
				<circle cx="78" cy="174" r="2" fill="#94a3b8" />
				<rect x="22" y="130" width="22" height="22" fill="#e0f2fe" stroke="#475569" stroke-width="1.5" rx="2" />
				<line x1="33" y1="130" x2="33" y2="152" stroke="#475569" stroke-width="0.8" />
				<line x1="22" y1="141" x2="44" y2="141" stroke="#475569" stroke-width="0.8" />
				<rect x="96" y="130" width="22" height="22" fill="#e0f2fe" stroke="#475569" stroke-width="1.5" rx="2" />
				<line x1="107" y1="130" x2="107" y2="152" stroke="#475569" stroke-width="0.8" />
				<line x1="96" y1="141" x2="118" y2="141" stroke="#475569" stroke-width="0.8" />
			</g>
			<text x="770" y="298" text-anchor="middle" class="node-label">Home</text>
			<!-- Home demand sub-labels -->
			{#if hasPower}
				<text x="770" y="316" text-anchor="middle" class="demand-sublabel" class:demand-export={!homeConsuming}>
					{homeConsuming ? 'Demand' : 'Producing'} {formatPower(homeDemand)}
				</text>
				{#if heatPumpPower > 10}
					<text x="770" y="332" text-anchor="middle" class="demand-sublabel heat-pump-sublabel">
						HP {formatPower(heatPumpPower)} / App {formatPower(appliancePower)}
					</text>
				{/if}
			{/if}
		</svg>
	{:else}
		<!-- Layout with PV: Grid — Home, PV above junction -->
		<svg viewBox="0 0 700 310" xmlns="http://www.w3.org/2000/svg">
			<defs>
				<linearGradient id="wireGrad" x1="0%" y1="0%" x2="100%" y2="0%">
					<stop offset="0%" stop-color={gridConsuming ? '#ef4444' : '#22c55e'} stop-opacity="0.15" />
					<stop offset="50%" stop-color={gridConsuming ? '#ef4444' : '#22c55e'} stop-opacity="0.4" />
					<stop offset="100%" stop-color={gridConsuming ? '#ef4444' : '#22c55e'} stop-opacity="0.15" />
				</linearGradient>
				<linearGradient id="wireGradPV2" x1="0%" y1="0%" x2="0%" y2="100%">
					<stop offset="0%" stop-color={pvColor} stop-opacity="0.15" />
					<stop offset="50%" stop-color={pvColor} stop-opacity="0.4" />
					<stop offset="100%" stop-color={pvColor} stop-opacity="0.15" />
				</linearGradient>
				<filter id="glow2">
					<feGaussianBlur stdDeviation="2" result="blur" />
					<feMerge>
						<feMergeNode in="blur" />
						<feMergeNode in="SourceGraphic" />
					</feMerge>
				</filter>
			</defs>

			<!-- PV Solar panel at top center -->
			<g transform="translate(350, 10)">
				<rect x="-30" y="0" width="60" height="40" rx="4" fill="#fef9c3" stroke="#ca8a04" stroke-width="2" />
				<line x1="-30" y1="20" x2="30" y2="20" stroke="#ca8a04" stroke-width="1" opacity="0.5" />
				<line x1="-10" y1="0" x2="-10" y2="40" stroke="#ca8a04" stroke-width="1" opacity="0.5" />
				<line x1="10" y1="0" x2="10" y2="40" stroke="#ca8a04" stroke-width="1" opacity="0.5" />
				<circle cx="0" cy="-14" r="8" fill="#fbbf24" opacity="0.7" />
				<line x1="-16" y1="-14" x2="-12" y2="-14" stroke="#fbbf24" stroke-width="2" stroke-linecap="round" />
				<line x1="12" y1="-14" x2="16" y2="-14" stroke="#fbbf24" stroke-width="2" stroke-linecap="round" />
				<line x1="0" y1="-30" x2="0" y2="-26" stroke="#fbbf24" stroke-width="2" stroke-linecap="round" />
				<line x1="-10" y1="-24" x2="-8" y2="-22" stroke="#fbbf24" stroke-width="2" stroke-linecap="round" />
				<line x1="10" y1="-24" x2="8" y2="-22" stroke="#fbbf24" stroke-width="2" stroke-linecap="round" />
			</g>
			<text x="350" y="68" text-anchor="middle" class="node-label">PV</text>

			<!-- PV vertical wire -->
			<g>
				{#if pvProducing}
					<line x1="350" y1="75" x2="350" y2="198" stroke="url(#wireGradPV2)" stroke-width="18" stroke-linecap="round" />
				{/if}
				<line x1="350" y1="75" x2="350" y2="198" stroke="#d1d5db" stroke-width="3" stroke-linecap="round" />
			</g>

			<!-- PV power badge -->
			{#if pvProducing}
				<g transform="translate(400, 120)">
					<rect x="-40" y="-14" width="80" height="32" rx="8"
						fill="white" stroke="#fcd34d" stroke-width="1.5" />
					<text x="0" y="6" text-anchor="middle" class="power-value pv-power">
						{formatPower(pvPower)}
					</text>
				</g>
			{/if}

			<!-- PV flow dots -->
			{#if pvProducing}
				<circle r="5" fill={pvColor} filter="url(#glow2)" class="flow-dot">
					<animateMotion dur="1.5s" repeatCount="indefinite" path="M350,75 L350,198" />
				</circle>
				<circle r="5" fill={pvColor} filter="url(#glow2)" class="flow-dot">
					<animateMotion dur="1.5s" repeatCount="indefinite" begin="0.5s" path="M350,75 L350,198" />
				</circle>
				{#if pvHighPower}
					<circle r="5" fill={pvColor} filter="url(#glow2)" class="flow-dot">
						<animateMotion dur="1.5s" repeatCount="indefinite" begin="1s" path="M350,75 L350,198" />
					</circle>
				{/if}
			{/if}

			<!-- Grid icon group (shifted down) -->
			<g transform="translate(70, 80)">
				<line x1="0" y1="20" x2="0" y2="140" stroke="#94a3b8" stroke-width="4" />
				<line x1="0" y1="140" x2="-22" y2="180" stroke="#94a3b8" stroke-width="3" />
				<line x1="0" y1="140" x2="22" y2="180" stroke="#94a3b8" stroke-width="3" />
				<line x1="-35" y1="40" x2="35" y2="40" stroke="#94a3b8" stroke-width="3.5" />
				<line x1="-25" y1="70" x2="25" y2="70" stroke="#94a3b8" stroke-width="3" />
				<line x1="-18" y1="100" x2="18" y2="100" stroke="#94a3b8" stroke-width="2.5" />
				<line x1="-12" y1="120" x2="12" y2="120" stroke="#94a3b8" stroke-width="2" />
				<line x1="-17" y1="155" x2="17" y2="155" stroke="#94a3b8" stroke-width="1.5" />
				<circle cx="-35" cy="45" r="3" fill="#cbd5e1" />
				<circle cx="35" cy="45" r="3" fill="#cbd5e1" />
				<circle cx="-25" cy="75" r="2.5" fill="#cbd5e1" />
				<circle cx="25" cy="75" r="2.5" fill="#cbd5e1" />
				<path d="M-35,45 Q-55,25 -60,10" fill="none" stroke="#94a3b8" stroke-width="1.5" />
				<path d="M35,45 Q55,25 60,10" fill="none" stroke="#94a3b8" stroke-width="1.5" />
			</g>
			<text x="70" y="278" text-anchor="middle" class="node-label">Grid</text>

			<!-- Connection wire (y=198) -->
			<g>
				{#if hasPower}
					<line x1="130" y1="198" x2="480" y2="198" stroke="url(#wireGrad)" stroke-width="18" stroke-linecap="round" />
				{/if}
				<line x1="130" y1="198" x2="480" y2="198" stroke="#d1d5db" stroke-width="3" stroke-linecap="round" />
			</g>

			<!-- Direction chevron -->
			{#if hasPower}
				{#if gridConsuming}
					<path d="M430,190 L440,198 L430,206" fill="none" stroke={gridColor} stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" opacity="0.7" />
				{:else}
					<path d="M170,190 L160,198 L170,206" fill="none" stroke={gridColor} stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" opacity="0.7" />
				{/if}
			{/if}

			<!-- Power flow dots -->
			{#if hasPower}
				{#if gridConsuming}
					<circle r="5" fill="#ef4444" filter="url(#glow2)" class="flow-dot">
						<animateMotion dur="2s" repeatCount="indefinite" path="M130,198 L480,198" />
					</circle>
					<circle r="5" fill="#ef4444" filter="url(#glow2)" class="flow-dot">
						<animateMotion dur="2s" repeatCount="indefinite" begin="0.66s" path="M130,198 L480,198" />
					</circle>
					<circle r="5" fill="#ef4444" filter="url(#glow2)" class="flow-dot">
						<animateMotion dur="2s" repeatCount="indefinite" begin="1.33s" path="M130,198 L480,198" />
					</circle>
				{:else}
					<circle r="5" fill="#22c55e" filter="url(#glow2)" class="flow-dot">
						<animateMotion dur="2s" repeatCount="indefinite" path="M480,198 L130,198" />
					</circle>
					<circle r="5" fill="#22c55e" filter="url(#glow2)" class="flow-dot">
						<animateMotion dur="2s" repeatCount="indefinite" begin="0.66s" path="M480,198 L130,198" />
					</circle>
					<circle r="5" fill="#22c55e" filter="url(#glow2)" class="flow-dot">
						<animateMotion dur="2s" repeatCount="indefinite" begin="1.33s" path="M480,198 L130,198" />
					</circle>
				{/if}
			{/if}

			<!-- Smart meter -->
			<g transform="translate(195, 178)">
				<rect x="0" y="0" width="24" height="40" rx="4" fill="white" stroke="#94a3b8" stroke-width="1.5" />
				<rect x="4" y="5" width="16" height="10" rx="2" fill="#e2e8f0" />
				<circle cx="12" cy="27" r="3" fill="none" stroke="#94a3b8" stroke-width="1" />
				<line x1="12" y1="24" x2="12" y2="27" stroke="#94a3b8" stroke-width="1" />
			</g>

			<!-- Power reading badge -->
			<g transform="translate(305, 150)">
				<rect x="-60" y="-20" width="120" height="52" rx="10"
					fill="white" stroke={gridConsuming ? '#fca5a5' : '#86efac'} stroke-width="1.5" />
				<text x="0" y="-2" text-anchor="middle" class="power-label">
					{gridConsuming ? 'Consuming' : 'Exporting'}
				</text>
				<text x="0" y="22" text-anchor="middle" class="power-value" class:exporting={!gridConsuming}>
					{hasPower ? formatPower(simulation.currentPower) : '-- W'}
				</text>
			</g>

			<!-- House group (shifted down) -->
			<g transform="translate(520, 80)">
				<path d="M-10,115 L70,55 L150,115" fill="none" stroke="#475569" stroke-width="3.5" stroke-linejoin="round" stroke-linecap="round" />
				<rect x="100" y="66" width="14" height="35" fill="#f8fafc" stroke="#475569" stroke-width="2" rx="1" />
				<rect x="8" y="115" width="124" height="80" fill="#f8fafc" stroke="#475569" stroke-width="3" rx="2" />
				<rect x="55" y="148" width="30" height="47" fill="#f1f5f9" stroke="#475569" stroke-width="2" rx="3" />
				<circle cx="78" cy="174" r="2" fill="#94a3b8" />
				<rect x="22" y="130" width="22" height="22" fill="#e0f2fe" stroke="#475569" stroke-width="1.5" rx="2" />
				<line x1="33" y1="130" x2="33" y2="152" stroke="#475569" stroke-width="0.8" />
				<line x1="22" y1="141" x2="44" y2="141" stroke="#475569" stroke-width="0.8" />
				<rect x="96" y="130" width="22" height="22" fill="#e0f2fe" stroke="#475569" stroke-width="1.5" rx="2" />
				<line x1="107" y1="130" x2="107" y2="152" stroke="#475569" stroke-width="0.8" />
				<line x1="96" y1="141" x2="118" y2="141" stroke="#475569" stroke-width="0.8" />
			</g>
			<text x="590" y="278" text-anchor="middle" class="node-label">Home</text>
			{#if hasPower && heatPumpPower > 10}
				<text x="590" y="296" text-anchor="middle" class="demand-sublabel heat-pump-sublabel">
					HP {formatPower(heatPumpPower)} / App {formatPower(appliancePower)}
				</text>
			{/if}
		</svg>
	{/if}
</div>

<style>
	.schema-card {
		width: 100%;
		max-width: 900px;
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

	.power-value.battery-power {
		fill: #f59e0b;
	}

	.power-value.battery-power.charging {
		fill: #3b82f6;
	}

	.power-value.pv-power {
		font-size: 15px;
		fill: #ca8a04;
	}

	.battery-soc-label {
		font-size: 16px;
		font-weight: 700;
		font-family: 'SF Mono', 'Cascadia Code', 'Fira Code', monospace;
		fill: #475569;
	}

	.battery-kwh-label {
		font-size: 11px;
		font-weight: 600;
		font-family: 'SF Mono', 'Cascadia Code', 'Fira Code', monospace;
		fill: #64748b;
	}

	.demand-sublabel {
		font-size: 12px;
		font-weight: 600;
		font-family: 'SF Mono', 'Cascadia Code', 'Fira Code', monospace;
		fill: #475569;
	}

	.demand-sublabel.demand-export {
		fill: #22c55e;
	}

	.demand-sublabel.heat-pump-sublabel {
		font-size: 11px;
		fill: #64748b;
	}

	.flow-dot {
		opacity: 0.85;
	}
</style>
