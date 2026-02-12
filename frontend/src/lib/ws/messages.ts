// Message types — mirrors backend ws/messages.go

export interface Envelope {
	type: string;
	payload?: unknown;
}

// Client -> Server
export const MSG_SIM_START = 'sim:start';
export const MSG_SIM_PAUSE = 'sim:pause';
export const MSG_SIM_SET_SPEED = 'sim:set_speed';
export const MSG_SIM_SEEK = 'sim:seek';
export const MSG_SIM_SET_SOURCE = 'sim:set_source';
export const MSG_BATTERY_CONFIG = 'battery:config';
export const MSG_SIM_SET_PREDICTION = 'sim:set_prediction';

// Server -> Client
export const MSG_SIM_STATE = 'sim:state';
export const MSG_SENSOR_READING = 'sensor:reading';
export const MSG_SUMMARY_UPDATE = 'summary:update';
export const MSG_DATA_LOADED = 'data:loaded';
export const MSG_BATTERY_UPDATE = 'battery:update';
export const MSG_BATTERY_SUMMARY = 'battery:summary';

export interface SetSpeedPayload {
	speed: number;
}

export interface SeekPayload {
	timestamp: string;
}

export interface SetSourcePayload {
	source: string;
}

export interface SimStatePayload {
	time: string;
	speed: number;
	running: boolean;
}

export interface SensorReadingPayload {
	sensor_id: string;
	value: number;
	unit: string;
	timestamp: string;
}

export interface SummaryPayload {
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
}

export interface SensorInfo {
	id: string;
	name: string;
	type: string;
	unit: string;
}

export interface TimeRangeInfo {
	start: string;
	end: string;
}

export interface DataLoadedPayload {
	sensors: SensorInfo[];
	time_range: TimeRangeInfo;
}

// Battery

export interface BatteryConfigPayload {
	enabled: boolean;
	capacity_kwh: number;
	max_power_w: number;
	discharge_to_percent: number;
	charge_to_percent: number;
}

export interface BatteryUpdatePayload {
	battery_power_w: number;
	adjusted_grid_w: number;
	soc_percent: number;
	timestamp: string;
}

export interface BatterySummaryPayload {
	soc_percent: number;
	cycles: number;
	time_at_power_sec: Record<string, number>;
	time_at_soc_pct_sec: Record<string, number>;
	month_soc_seconds: Record<string, Record<string, number>>;
}
