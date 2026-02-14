import { describe, it, expect, beforeEach } from 'vitest';
import { simulation } from '$lib/stores/simulation.svelte';
import {
	MSG_SIM_STATE,
	MSG_SENSOR_READING,
	MSG_SUMMARY_UPDATE,
	MSG_DATA_LOADED,
	MSG_BATTERY_UPDATE,
	MSG_BATTERY_SUMMARY,
	type Envelope
} from '$lib/ws/messages';

// Access private handleMessage through bracket notation.
function handleMessage(envelope: Envelope) {
	(simulation as any).handleMessage(envelope);
}

// Reset store state between tests by sending zeroed messages.
function resetStore() {
	handleMessage({
		type: MSG_SIM_STATE,
		payload: { time: '', speed: 3600, running: false }
	});
	// Reset daily tracking
	(simulation as any).currentDayKey = '';
	(simulation as any).dayStartSnapshot = {
		gridImportKWh: 0,
		selfConsumptionKWh: 0,
		batterySavingsKWh: 0,
		homeDemandKWh: 0,
		heatPumpKWh: 0
	};
	simulation.dailyRecords = [];
	simulation.chartData = [];
}

beforeEach(() => {
	resetStore();
});

describe('handleMessage: sim:state', () => {
	it('updates simulation state', () => {
		handleMessage({
			type: MSG_SIM_STATE,
			payload: {
				time: '2024-11-21T12:00:00Z',
				speed: 7200,
				running: true
			}
		});

		expect(simulation.simTime).toBe('2024-11-21T12:00:00Z');
		expect(simulation.speed).toBe(7200);
		expect(simulation.running).toBe(true);
	});

	it('updates when paused', () => {
		handleMessage({
			type: MSG_SIM_STATE,
			payload: {
				time: '2024-11-21T14:00:00Z',
				speed: 3600,
				running: false
			}
		});

		expect(simulation.running).toBe(false);
		expect(simulation.speed).toBe(3600);
	});
});

describe('handleMessage: data:loaded', () => {
	it('resolves sensor IDs and time range', () => {
		handleMessage({
			type: MSG_DATA_LOADED,
			payload: {
				sensors: [
					{ id: 'sensor.grid', name: 'Grid Power', type: 'grid_power', unit: 'W' },
					{ id: 'sensor.pv', name: 'PV Power', type: 'pv_power', unit: 'W' },
					{ id: 'sensor.pump_c', name: 'Pump Consumption', type: 'pump_total_consumption', unit: 'W' },
					{ id: 'sensor.pump_p', name: 'Pump Production', type: 'pump_total_production', unit: 'W' }
				],
				time_range: {
					start: '2024-11-21T00:00:00Z',
					end: '2025-02-10T23:59:59Z'
				}
			}
		});

		expect(simulation.sensors).toHaveLength(4);
		expect(simulation.timeRangeStart).toBe('2024-11-21T00:00:00Z');
		expect(simulation.timeRangeEnd).toBe('2025-02-10T23:59:59Z');

		// Internal sensor ID resolution
		expect((simulation as any).gridPowerSensorId).toBe('sensor.grid');
		expect((simulation as any).pvSensorId).toBe('sensor.pv');
		expect((simulation as any).heatPumpSensorId).toBe('sensor.pump_c');
		expect((simulation as any).heatPumpProdSensorId).toBe('sensor.pump_p');
	});
});

describe('handleMessage: sensor:reading', () => {
	beforeEach(() => {
		// Set up sensor IDs first
		handleMessage({
			type: MSG_DATA_LOADED,
			payload: {
				sensors: [
					{ id: 'sensor.grid', name: 'Grid', type: 'grid_power', unit: 'W' },
					{ id: 'sensor.pv', name: 'PV', type: 'pv_power', unit: 'W' },
					{ id: 'sensor.pump_c', name: 'Pump', type: 'pump_total_consumption', unit: 'W' },
					{ id: 'sensor.pump_p', name: 'Pump Prod', type: 'pump_total_production', unit: 'W' }
				],
				time_range: { start: '2024-11-21T00:00:00Z', end: '2025-01-01T00:00:00Z' }
			}
		});
	});

	it('updates grid power and chart data', () => {
		handleMessage({
			type: MSG_SENSOR_READING,
			payload: {
				sensor_id: 'sensor.grid',
				value: 1500.5,
				unit: 'W',
				timestamp: '2024-11-21T12:00:00Z'
			}
		});

		expect(simulation.currentPower).toBe(1500.5);
		expect(simulation.currentPowerTimestamp).toBe('2024-11-21T12:00:00Z');
		expect(simulation.chartData).toHaveLength(1);
		expect(simulation.chartData[0].value).toBe(1500.5);
	});

	it('updates PV power', () => {
		handleMessage({
			type: MSG_SENSOR_READING,
			payload: {
				sensor_id: 'sensor.pv',
				value: 3000,
				unit: 'W',
				timestamp: '2024-11-21T12:00:00Z'
			}
		});

		expect(simulation.currentPVPower).toBe(3000);
		// PV reading should NOT add to chart data
		expect(simulation.chartData).toHaveLength(0);
	});

	it('updates heat pump power', () => {
		handleMessage({
			type: MSG_SENSOR_READING,
			payload: {
				sensor_id: 'sensor.pump_c',
				value: 800,
				unit: 'W',
				timestamp: '2024-11-21T12:00:00Z'
			}
		});

		expect(simulation.currentHeatPumpPower).toBe(800);
	});

	it('updates heat pump production power', () => {
		handleMessage({
			type: MSG_SENSOR_READING,
			payload: {
				sensor_id: 'sensor.pump_p',
				value: 2400,
				unit: 'W',
				timestamp: '2024-11-21T12:00:00Z'
			}
		});

		expect(simulation.currentHeatPumpProdPower).toBe(2400);
	});

	it('ignores unknown sensor when grid is known', () => {
		const prevPower = simulation.currentPower;
		handleMessage({
			type: MSG_SENSOR_READING,
			payload: {
				sensor_id: 'sensor.unknown',
				value: 999,
				unit: 'W',
				timestamp: '2024-11-21T12:00:00Z'
			}
		});

		expect(simulation.currentPower).toBe(prevPower);
		expect(simulation.chartData).toHaveLength(0);
	});

	it('caps chart data at MAX_CHART_POINTS', () => {
		for (let i = 0; i < 510; i++) {
			handleMessage({
				type: MSG_SENSOR_READING,
				payload: {
					sensor_id: 'sensor.grid',
					value: i,
					unit: 'W',
					timestamp: `2024-11-21T12:${String(i % 60).padStart(2, '0')}:00Z`
				}
			});
		}

		expect(simulation.chartData.length).toBeLessThanOrEqual(500);
		// Last value should be 509
		expect(simulation.chartData[simulation.chartData.length - 1].value).toBe(509);
	});
});

describe('handleMessage: summary:update', () => {
	it('updates all energy summary fields', () => {
		// Set simTime so trackDailyData works
		handleMessage({
			type: MSG_SIM_STATE,
			payload: { time: '2024-11-21T12:00:00Z', speed: 3600, running: true }
		});

		handleMessage({
			type: MSG_SUMMARY_UPDATE,
			payload: {
				today_kwh: 5.5,
				month_kwh: 150.0,
				total_kwh: 1200.0,
				grid_import_kwh: 1000.0,
				grid_export_kwh: 200.0,
				pv_production_kwh: 400.0,
				heat_pump_kwh: 80.0,
				heat_pump_prod_kwh: 200.0,
				self_consumption_kwh: 200.0,
				home_demand_kwh: 1200.0,
				battery_savings_kwh: 30.0
			}
		});

		expect(simulation.todayKWh).toBe(5.5);
		expect(simulation.monthKWh).toBe(150.0);
		expect(simulation.totalKWh).toBe(1200.0);
		expect(simulation.gridImportKWh).toBe(1000.0);
		expect(simulation.gridExportKWh).toBe(200.0);
		expect(simulation.pvProductionKWh).toBe(400.0);
		expect(simulation.heatPumpKWh).toBe(80.0);
		expect(simulation.heatPumpProdKWh).toBe(200.0);
		expect(simulation.selfConsumptionKWh).toBe(200.0);
		expect(simulation.homeDemandKWh).toBe(1200.0);
		expect(simulation.batterySavingsKWh).toBe(30.0);
	});
});

describe('handleMessage: battery:update', () => {
	it('updates battery state', () => {
		handleMessage({
			type: MSG_BATTERY_UPDATE,
			payload: {
				battery_power_w: 2000,
				adjusted_grid_w: -500,
				soc_percent: 75.0,
				timestamp: '2024-11-21T12:00:00Z'
			}
		});

		expect(simulation.batteryPowerW).toBe(2000);
		expect(simulation.adjustedGridW).toBe(-500);
		expect(simulation.batterySoCPercent).toBe(75.0);
	});
});

describe('handleMessage: battery:summary', () => {
	it('updates battery summary stats', () => {
		handleMessage({
			type: MSG_BATTERY_SUMMARY,
			payload: {
				soc_percent: 60.0,
				cycles: 15.3,
				time_at_power_sec: { '0': 3600, '1': 1800 },
				time_at_soc_pct_sec: { '50': 7200, '60': 3600 },
				month_soc_seconds: { '2024-11': { '50': 3600 } }
			}
		});

		expect(simulation.batterySoCPercent).toBe(60.0);
		expect(simulation.batteryCycles).toBe(15.3);
		expect(simulation.batteryTimeAtPowerSec).toEqual({ '0': 3600, '1': 1800 });
		expect(simulation.batteryTimeAtSoCPctSec).toEqual({ '50': 7200, '60': 3600 });
		expect(simulation.batteryMonthSoCSeconds).toEqual({ '2024-11': { '50': 3600 } });
	});

	it('handles null month_soc_seconds', () => {
		handleMessage({
			type: MSG_BATTERY_SUMMARY,
			payload: {
				soc_percent: 50.0,
				cycles: 1.0,
				time_at_power_sec: {},
				time_at_soc_pct_sec: {},
				month_soc_seconds: null
			}
		});

		expect(simulation.batteryMonthSoCSeconds).toEqual({});
	});
});

describe('getHoursFromMidnight', () => {
	it('returns 0 for midnight', () => {
		handleMessage({
			type: MSG_SIM_STATE,
			payload: { time: '2024-11-21T00:00:00Z', speed: 3600, running: true }
		});
		expect((simulation as any).getHoursFromMidnight()).toBe(0);
	});

	it('returns 12.5 for 12:30', () => {
		handleMessage({
			type: MSG_SIM_STATE,
			payload: { time: '2024-11-21T12:30:00Z', speed: 3600, running: true }
		});
		expect((simulation as any).getHoursFromMidnight()).toBe(12.5);
	});

	it('returns fractional hours for 23:59:59', () => {
		handleMessage({
			type: MSG_SIM_STATE,
			payload: { time: '2024-11-21T23:59:59Z', speed: 3600, running: true }
		});
		const hours = (simulation as any).getHoursFromMidnight();
		expect(hours).toBeCloseTo(23 + 59 / 60 + 59 / 3600, 2);
	});
});

describe('autonomy formulas', () => {
	function setSummaryForAutonomy(time: string, summary: Record<string, number>) {
		handleMessage({
			type: MSG_SIM_STATE,
			payload: { time, speed: 3600, running: true }
		});
		handleMessage({
			type: MSG_SUMMARY_UPDATE,
			payload: {
				today_kwh: 0, month_kwh: 0, total_kwh: 0,
				grid_import_kwh: 0, grid_export_kwh: 0, pv_production_kwh: 0,
				heat_pump_kwh: 0, heat_pump_prod_kwh: 0,
				self_consumption_kwh: 0, home_demand_kwh: 0, battery_savings_kwh: 0,
				...summary
			}
		});
	}

	it('mid-day autonomy uses hoursElapsed / demand', () => {
		simulation.batteryCapacityKWh = 10;
		// Init snapshot
		setSummaryForAutonomy('2024-11-21T00:00:00Z', {
			home_demand_kwh: 0
		});
		// At 12:00 with 5 kWh demand → autonomy = 10 * 12 / 5 = 24
		setSummaryForAutonomy('2024-11-21T12:00:00Z', {
			home_demand_kwh: 5
		});

		expect(simulation.dailyRecords[0].batteryAutonomyHours).toBeCloseTo(24, 0);
	});

	it('finalized day autonomy uses 24h / demand', () => {
		simulation.batteryCapacityKWh = 10;
		// Day 1
		setSummaryForAutonomy('2024-11-21T08:00:00Z', {
			home_demand_kwh: 0
		});
		setSummaryForAutonomy('2024-11-21T20:00:00Z', {
			home_demand_kwh: 20
		});
		// Day transition finalizes day 1 with 24h formula
		setSummaryForAutonomy('2024-11-22T02:00:00Z', {
			home_demand_kwh: 24
		});

		// Finalized day 1: demand = 24, autonomy = 10 * 24 / 24 = 10
		expect(simulation.dailyRecords[0].batteryAutonomyHours).toBeCloseTo(10, 0);
	});
});

describe('trackDailyData', () => {
	function setSummary(time: string, summary: Partial<{
		today_kwh: number;
		month_kwh: number;
		total_kwh: number;
		grid_import_kwh: number;
		grid_export_kwh: number;
		pv_production_kwh: number;
		heat_pump_kwh: number;
		heat_pump_prod_kwh: number;
		self_consumption_kwh: number;
		home_demand_kwh: number;
		battery_savings_kwh: number;
	}>) {
		handleMessage({
			type: MSG_SIM_STATE,
			payload: { time, speed: 3600, running: true }
		});
		handleMessage({
			type: MSG_SUMMARY_UPDATE,
			payload: {
				today_kwh: 0,
				month_kwh: 0,
				total_kwh: 0,
				grid_import_kwh: 0,
				grid_export_kwh: 0,
				pv_production_kwh: 0,
				heat_pump_kwh: 0,
				heat_pump_prod_kwh: 0,
				self_consumption_kwh: 0,
				home_demand_kwh: 0,
				battery_savings_kwh: 0,
				...summary
			}
		});
	}

	it('creates a daily record on first summary', () => {
		setSummary('2024-11-21T12:00:00Z', {
			grid_import_kwh: 10,
			self_consumption_kwh: 5,
			home_demand_kwh: 15,
			battery_savings_kwh: 2,
			heat_pump_kwh: 3
		});

		expect(simulation.dailyRecords).toHaveLength(1);
		expect(simulation.dailyRecords[0].date).toBe('2024-11-21');
		// First record: delta from snapshot at init. Since snapshot and cumulatives
		// are both from the same initial message, deltas should be ~0
		expect(simulation.dailyRecords[0].gridImportKWh).toBe(0);
	});

	it('accumulates within same day', () => {
		setSummary('2024-11-21T08:00:00Z', {
			grid_import_kwh: 0,
			home_demand_kwh: 0,
			self_consumption_kwh: 0,
			battery_savings_kwh: 0,
			heat_pump_kwh: 0
		});
		setSummary('2024-11-21T12:00:00Z', {
			grid_import_kwh: 5,
			home_demand_kwh: 10,
			self_consumption_kwh: 4,
			battery_savings_kwh: 1,
			heat_pump_kwh: 2
		});

		expect(simulation.dailyRecords).toHaveLength(1);
		expect(simulation.dailyRecords[0].gridImportKWh).toBe(5);
		expect(simulation.dailyRecords[0].homeDemandKWh).toBe(10);
		expect(simulation.dailyRecords[0].selfConsumptionKWh).toBe(4);
	});

	it('creates new record on day change', () => {
		setSummary('2024-11-21T23:00:00Z', {
			grid_import_kwh: 10,
			home_demand_kwh: 20,
			self_consumption_kwh: 8,
			battery_savings_kwh: 2,
			heat_pump_kwh: 5
		});
		setSummary('2024-11-22T01:00:00Z', {
			grid_import_kwh: 12,
			home_demand_kwh: 24,
			self_consumption_kwh: 10,
			battery_savings_kwh: 3,
			heat_pump_kwh: 6
		});

		expect(simulation.dailyRecords).toHaveLength(2);
		expect(simulation.dailyRecords[0].date).toBe('2024-11-21');
		expect(simulation.dailyRecords[1].date).toBe('2024-11-22');
	});

	it('computes off-grid percentage correctly', () => {
		// First message sets the snapshot
		setSummary('2024-11-21T06:00:00Z', {
			grid_import_kwh: 0,
			home_demand_kwh: 0,
			self_consumption_kwh: 0,
			battery_savings_kwh: 0
		});
		// Self-consumption 3 + battery savings 2 = 5 out of demand 10 → 50%
		setSummary('2024-11-21T18:00:00Z', {
			grid_import_kwh: 5,
			home_demand_kwh: 10,
			self_consumption_kwh: 3,
			battery_savings_kwh: 2,
			heat_pump_kwh: 0
		});

		expect(simulation.dailyRecords).toHaveLength(1);
		expect(simulation.dailyRecords[0].offGridPct).toBeCloseTo(50, 0);
	});

	it('caps off-grid at 100%', () => {
		setSummary('2024-11-21T06:00:00Z', {
			home_demand_kwh: 0,
			self_consumption_kwh: 0,
			battery_savings_kwh: 0
		});
		// Self-consumption exceeds demand
		setSummary('2024-11-21T18:00:00Z', {
			home_demand_kwh: 5,
			self_consumption_kwh: 10,
			battery_savings_kwh: 0,
			heat_pump_kwh: 0
		});

		expect(simulation.dailyRecords[0].offGridPct).toBe(100);
	});

	it('handles zero demand with zero off-grid', () => {
		setSummary('2024-11-21T12:00:00Z', {
			home_demand_kwh: 0,
			self_consumption_kwh: 0,
			battery_savings_kwh: 0
		});

		expect(simulation.dailyRecords[0].offGridPct).toBe(0);
	});

	it('autonomy with zero demand returns 0', () => {
		setSummary('2024-11-21T06:00:00Z', {
			home_demand_kwh: 0,
			self_consumption_kwh: 0,
			battery_savings_kwh: 0
		});
		setSummary('2024-11-21T12:00:00Z', {
			home_demand_kwh: 0,
			self_consumption_kwh: 0,
			battery_savings_kwh: 0
		});

		expect(simulation.dailyRecords[0].batteryAutonomyHours).toBe(0);
	});

	it('multi-day skip creates intermediate records', () => {
		// Start on Nov 21
		setSummary('2024-11-21T12:00:00Z', {
			grid_import_kwh: 0,
			home_demand_kwh: 0,
			self_consumption_kwh: 0,
			battery_savings_kwh: 0,
			heat_pump_kwh: 0
		});

		// Jump to Nov 24 — 3 days skipped
		setSummary('2024-11-24T12:00:00Z', {
			grid_import_kwh: 30,
			home_demand_kwh: 60,
			self_consumption_kwh: 15,
			battery_savings_kwh: 6,
			heat_pump_kwh: 9
		});

		// Should have 4 records: Nov 21, 22, 23, and the current Nov 24
		expect(simulation.dailyRecords.length).toBe(4);
		expect(simulation.dailyRecords[0].date).toBe('2024-11-21');
		expect(simulation.dailyRecords[1].date).toBe('2024-11-22');
		expect(simulation.dailyRecords[2].date).toBe('2024-11-23');
		expect(simulation.dailyRecords[3].date).toBe('2024-11-24');

		// Each intermediate record gets 1/3 of the total delta
		expect(simulation.dailyRecords[0].gridImportKWh).toBeCloseTo(10, 1);
		expect(simulation.dailyRecords[1].gridImportKWh).toBeCloseTo(10, 1);
		expect(simulation.dailyRecords[2].gridImportKWh).toBeCloseTo(10, 1);
	});

	it('daily records cleared on seek', () => {
		setSummary('2024-11-21T12:00:00Z', {
			grid_import_kwh: 10,
			home_demand_kwh: 20
		});

		expect(simulation.dailyRecords.length).toBe(1);

		// Seek resets
		simulation.seek('2024-11-21T00:00:00Z');
		expect(simulation.dailyRecords.length).toBe(0);
	});

	it('finalizes previous day on day transition', () => {
		// Day 1: initial snapshot
		setSummary('2024-11-21T08:00:00Z', {
			grid_import_kwh: 0,
			home_demand_kwh: 0,
			self_consumption_kwh: 0,
			battery_savings_kwh: 0,
			heat_pump_kwh: 0
		});

		// Day 1 progress
		setSummary('2024-11-21T20:00:00Z', {
			grid_import_kwh: 15,
			home_demand_kwh: 25,
			self_consumption_kwh: 8,
			battery_savings_kwh: 2,
			heat_pump_kwh: 5
		});

		// Day 2 — triggers finalization of day 1
		setSummary('2024-11-22T02:00:00Z', {
			grid_import_kwh: 18,
			home_demand_kwh: 30,
			self_consumption_kwh: 10,
			battery_savings_kwh: 3,
			heat_pump_kwh: 7
		});

		// Day 1 should be finalized with the full delta (0→18 at transition point)
		expect(simulation.dailyRecords[0].date).toBe('2024-11-21');
		expect(simulation.dailyRecords[0].gridImportKWh).toBe(18);
		expect(simulation.dailyRecords[0].homeDemandKWh).toBe(30);

		// Day 2 starts fresh from transition snapshot
		expect(simulation.dailyRecords[1].date).toBe('2024-11-22');
		expect(simulation.dailyRecords[1].gridImportKWh).toBe(0);
	});
});
