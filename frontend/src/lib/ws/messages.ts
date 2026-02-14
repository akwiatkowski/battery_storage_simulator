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
export const MSG_CONFIG_UPDATE = 'config:update';

// Server -> Client
export const MSG_SIM_STATE = 'sim:state';
export const MSG_SENSOR_READING = 'sensor:reading';
export const MSG_SUMMARY_UPDATE = 'summary:update';
export const MSG_DATA_LOADED = 'data:loaded';
export const MSG_BATTERY_UPDATE = 'battery:update';
export const MSG_BATTERY_SUMMARY = 'battery:summary';
export const MSG_ARBITRAGE_DAY_LOG = 'arbitrage:day_log';
export const MSG_PREDICTION_COMPARISON = 'prediction:comparison';

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

	grid_import_cost_pln: number;
	grid_export_revenue_pln: number;
	net_cost_pln: number;
	raw_grid_import_cost_pln: number;
	raw_grid_export_revenue_pln: number;
	raw_net_cost_pln: number;
	battery_savings_pln: number;

	arb_net_cost_pln: number;
	arb_battery_savings_pln: number;

	cheap_export_kwh: number;
	cheap_export_rev_pln: number;
	current_spot_price: number;
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

export interface ArbitrageDayRecord {
	date: string;
	charge_start_time: string;
	charge_end_time: string;
	charge_kwh: number;
	discharge_start_time: string;
	discharge_end_time: string;
	discharge_kwh: number;
	gap_minutes: number;
	cycles_delta: number;
	earnings_pln: number;
}

export interface ArbitrageDayLogPayload {
	records: ArbitrageDayRecord[];
}

export interface ConfigUpdatePayload {
	export_coefficient: number;
	price_threshold_pln: number;
	temp_offset_c: number;
}

export interface PredictionComparisonPayload {
	actual_power_w: number;
	predicted_power_w: number;
	actual_temp_c: number;
	predicted_temp_c: number;
	has_actual_temp: boolean;
}
