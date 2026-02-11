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

// Server -> Client
export const MSG_SIM_STATE = 'sim:state';
export const MSG_SENSOR_READING = 'sensor:reading';
export const MSG_SUMMARY_UPDATE = 'summary:update';
export const MSG_DATA_LOADED = 'data:loaded';

export interface SetSpeedPayload {
	speed: number;
}

export interface SeekPayload {
	timestamp: string;
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
