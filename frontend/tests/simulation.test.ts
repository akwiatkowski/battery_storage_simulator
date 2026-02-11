import { describe, it, expect } from 'vitest';
import {
	MSG_SIM_START,
	MSG_SIM_PAUSE,
	MSG_SIM_STATE,
	MSG_SENSOR_READING,
	MSG_SUMMARY_UPDATE,
	MSG_DATA_LOADED,
	type SimStatePayload,
	type SensorReadingPayload,
	type SummaryPayload,
	type DataLoadedPayload
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
			total_kwh: 1234.5
		};
		expect(payload.today_kwh).toBe(12.3);
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
