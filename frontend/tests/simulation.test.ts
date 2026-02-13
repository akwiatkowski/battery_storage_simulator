import { describe, it, expect } from 'vitest';
import {
	MSG_SIM_START,
	MSG_SIM_PAUSE,
	MSG_SIM_STATE,
	MSG_SENSOR_READING,
	MSG_SUMMARY_UPDATE,
	MSG_DATA_LOADED,
	MSG_ARBITRAGE_DAY_LOG,
	type SimStatePayload,
	type SensorReadingPayload,
	type SummaryPayload,
	type DataLoadedPayload,
	type ArbitrageDayLogPayload
} from '$lib/ws/messages';

describe('Message types', () => {
	it('has correct client->server message types', () => {
		expect(MSG_SIM_START).toBe('sim:start');
		expect(MSG_SIM_PAUSE).toBe('sim:pause');
	});

	it('has correct server->client message types', () => {
		expect(MSG_SIM_STATE).toBe('sim:state');
		expect(MSG_SENSOR_READING).toBe('sensor:reading');
		expect(MSG_SUMMARY_UPDATE).toBe('summary:update');
		expect(MSG_DATA_LOADED).toBe('data:loaded');
	});
});

describe('Message payload types', () => {
	it('SimStatePayload has correct shape', () => {
		const payload: SimStatePayload = {
			time: '2024-11-21T12:00:00Z',
			speed: 10,
			running: true
		};
		expect(payload.time).toBe('2024-11-21T12:00:00Z');
		expect(payload.speed).toBe(10);
		expect(payload.running).toBe(true);
	});

	it('SensorReadingPayload has correct shape', () => {
		const payload: SensorReadingPayload = {
			sensor_id: 'sensor.grid',
			value: 759.59,
			unit: 'W',
			timestamp: '2024-11-21T13:00:00Z'
		};
		expect(payload.sensor_id).toBe('sensor.grid');
		expect(payload.value).toBe(759.59);
	});

	it('SummaryPayload has correct shape', () => {
		const payload: SummaryPayload = {
			today_kwh: 12.3,
			month_kwh: 345.6,
			total_kwh: 1234.5,
			grid_import_kwh: 1234.5,
			grid_export_kwh: 100.2,
			pv_production_kwh: 500.0,
			heat_pump_kwh: 200.0,
			heat_pump_prod_kwh: 600.0,
			self_consumption_kwh: 400.0,
			home_demand_kwh: 1634.3,
			battery_savings_kwh: 50.0,
			grid_import_cost_pln: 100.0,
			grid_export_revenue_pln: 20.0,
			net_cost_pln: 80.0,
			raw_grid_import_cost_pln: 120.0,
			raw_grid_export_revenue_pln: 20.0,
			raw_net_cost_pln: 100.0,
			battery_savings_pln: 20.0,
			arb_net_cost_pln: 65.0,
			arb_battery_savings_pln: 35.0
		};
		expect(payload.today_kwh).toBe(12.3);
	});

	it('ArbitrageDayLogPayload has correct shape', () => {
		const payload: ArbitrageDayLogPayload = {
			records: [
				{
					date: '2024-11-21',
					charge_start_time: '01:00',
					charge_end_time: '06:00',
					charge_kwh: 8.5,
					discharge_start_time: '14:00',
					discharge_end_time: '19:00',
					discharge_kwh: 7.2,
					gap_minutes: 480,
					cycles_delta: 0.85,
					earnings_pln: 2.3
				}
			]
		};
		expect(payload.records).toHaveLength(1);
		expect(payload.records[0].date).toBe('2024-11-21');
		expect(payload.records[0].earnings_pln).toBe(2.3);
	});

	it('has correct arbitrage day log message type', () => {
		expect(MSG_ARBITRAGE_DAY_LOG).toBe('arbitrage:day_log');
	});

	it('DataLoadedPayload has correct shape', () => {
		const payload: DataLoadedPayload = {
			sensors: [{ id: 'grid', name: 'Grid Power', type: 'grid_power', unit: 'W' }],
			time_range: {
				start: '2024-11-21T12:00:00Z',
				end: '2026-02-11T18:57:18Z'
			}
		};
		expect(payload.sensors).toHaveLength(1);
		expect(payload.time_range.start).toBe('2024-11-21T12:00:00Z');
	});
});
