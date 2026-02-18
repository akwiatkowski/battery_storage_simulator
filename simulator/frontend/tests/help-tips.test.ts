import { describe, it, expect } from 'vitest';
import { helpTexts } from '$lib/help-texts';

describe('helpTexts: chart entries', () => {
	const chartKeys = ['chartPower', 'chartPrice', 'chartSoC', 'chartTemp'];

	it.each(chartKeys)('has entry for %s', (key) => {
		const entry = helpTexts[key];
		expect(entry).toBeTruthy();
		expect(entry.title).toBeTruthy();
		expect(entry.description).toBeTruthy();
	});

	it('chartPower has correct title', () => {
		expect(helpTexts['chartPower'].title).toBe('Power Chart');
	});

	it('chartPrice has correct title', () => {
		expect(helpTexts['chartPrice'].title).toBe('Spot Price Chart');
	});

	it('chartSoC has correct title', () => {
		expect(helpTexts['chartSoC'].title).toBe('Battery State of Charge');
	});

	it('chartTemp has correct title', () => {
		expect(helpTexts['chartTemp'].title).toBe('Temperature Chart');
	});
});

describe('helpTexts: coverage for all component groups', () => {
	it('has EnergySummary entries', () => {
		expect(helpTexts['gridImportToday']).toBeTruthy();
		expect(helpTexts['pvProduction']).toBeTruthy();
		expect(helpTexts['selfConsumption']).toBeTruthy();
		expect(helpTexts['gridExport']).toBeTruthy();
		expect(helpTexts['homeDemand']).toBeTruthy();
		expect(helpTexts['heatPump']).toBeTruthy();
		expect(helpTexts['batterySaved']).toBeTruthy();
	});

	it('has CostSummary entries', () => {
		expect(helpTexts['importCost']).toBeTruthy();
		expect(helpTexts['exportRevenue']).toBeTruthy();
		expect(helpTexts['netCost']).toBeTruthy();
		expect(helpTexts['noBattery']).toBeTruthy();
		expect(helpTexts['selfConsumptionStrategy']).toBeTruthy();
		expect(helpTexts['arbitrageStrategy']).toBeTruthy();
		expect(helpTexts['netMetering']).toBeTruthy();
		expect(helpTexts['netBilling']).toBeTruthy();
	});

	it('has BatteryConfig entries', () => {
		expect(helpTexts['capacity']).toBeTruthy();
		expect(helpTexts['maxPower']).toBeTruthy();
		expect(helpTexts['dischargeTo']).toBeTruthy();
		expect(helpTexts['chargeTo']).toBeTruthy();
		expect(helpTexts['degradationCycles']).toBeTruthy();
	});

	it('has BatteryStats entries', () => {
		expect(helpTexts['cycles']).toBeTruthy();
		expect(helpTexts['effectiveCapacity']).toBeTruthy();
		expect(helpTexts['degradation']).toBeTruthy();
		expect(helpTexts['timeAtPower']).toBeTruthy();
		expect(helpTexts['timeAtSoC']).toBeTruthy();
	});

	it('has SimConfig entries', () => {
		expect(helpTexts['exportCoefficient']).toBeTruthy();
		expect(helpTexts['fixedTariff']).toBeTruthy();
		expect(helpTexts['distributionFee']).toBeTruthy();
		expect(helpTexts['netMeteringRatio']).toBeTruthy();
		expect(helpTexts['batteryCost']).toBeTruthy();
	});

	it('has PredictionComparison entries', () => {
		expect(helpTexts['nnPrediction']).toBeTruthy();
		expect(helpTexts['mae']).toBeTruthy();
	});

	it('has TimeSeriesChart entries', () => {
		expect(helpTexts['chartPower']).toBeTruthy();
		expect(helpTexts['chartPrice']).toBeTruthy();
		expect(helpTexts['chartSoC']).toBeTruthy();
		expect(helpTexts['chartTemp']).toBeTruthy();
	});

	it('has HeatingAnalysis entries', () => {
		expect(helpTexts['heatingAnalysis']).toBeTruthy();
		expect(helpTexts['heatingCOP']).toBeTruthy();
	});

	it('has AnomalyLog entries', () => {
		expect(helpTexts['anomalyLog']).toBeTruthy();
		expect(helpTexts['anomalyDeviation']).toBeTruthy();
	});
});

describe('helpTexts: data integrity', () => {
	it('all entries have required title and description', () => {
		for (const [key, entry] of Object.entries(helpTexts)) {
			expect(entry.title, `${key} missing title`).toBeTruthy();
			expect(entry.description, `${key} missing description`).toBeTruthy();
		}
	});

	it('optional fields are strings when present', () => {
		for (const [key, entry] of Object.entries(helpTexts)) {
			if (entry.formula !== undefined) {
				expect(typeof entry.formula, `${key}.formula not a string`).toBe('string');
			}
			if (entry.example !== undefined) {
				expect(typeof entry.example, `${key}.example not a string`).toBe('string');
			}
			if (entry.insight !== undefined) {
				expect(typeof entry.insight, `${key}.insight not a string`).toBe('string');
			}
		}
	});
});
