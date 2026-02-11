import { getClient } from '$lib/ws/client';
import {
	MSG_SIM_STATE,
	MSG_SENSOR_READING,
	MSG_SUMMARY_UPDATE,
	MSG_DATA_LOADED,
	MSG_SIM_START,
	MSG_SIM_PAUSE,
	MSG_SIM_SET_SPEED,
	MSG_SIM_SEEK,
	type SimStatePayload,
	type SensorReadingPayload,
	type SummaryPayload,
	type DataLoadedPayload,
	type SensorInfo,
	type Envelope
} from '$lib/ws/messages';

// Max data points to keep in the chart buffer
const MAX_CHART_POINTS = 500;

export interface ChartPoint {
	timestamp: Date;
	value: number;
}

class SimulationStore {
	// Connection
	connected = $state(false);

	// Simulation state
	simTime = $state('');
	speed = $state(1);
	running = $state(false);

	// Sensors
	sensors = $state<SensorInfo[]>([]);
	timeRangeStart = $state('');
	timeRangeEnd = $state('');

	// Current reading
	currentPower = $state(0);
	currentPowerTimestamp = $state('');

	// Energy summary
	todayKWh = $state(0);
	monthKWh = $state(0);
	totalKWh = $state(0);

	// Chart data
	chartData = $state<ChartPoint[]>([]);

	private client = getClient();
	private unsubscribe: (() => void) | null = null;

	init(): void {
		this.unsubscribe = this.client.onMessage((envelope: Envelope) => {
			this.handleMessage(envelope);
		});
		this.client.connect();

		// Track connection state
		$effect.root(() => {
			$effect(() => {
				this.connected = this.client.connected;
			});
		});
	}

	destroy(): void {
		this.unsubscribe?.();
		this.client.disconnect();
	}

	// Commands
	start(): void {
		this.client.send(MSG_SIM_START);
	}

	pause(): void {
		this.client.send(MSG_SIM_PAUSE);
	}

	setSpeed(speed: number): void {
		this.client.send(MSG_SIM_SET_SPEED, { speed });
	}

	seek(timestamp: string): void {
		this.client.send(MSG_SIM_SEEK, { timestamp });
		this.chartData = [];
	}

	private handleMessage(envelope: Envelope): void {
		switch (envelope.type) {
			case MSG_SIM_STATE: {
				const p = envelope.payload as SimStatePayload;
				this.simTime = p.time;
				this.speed = p.speed;
				this.running = p.running;
				break;
			}
			case MSG_SENSOR_READING: {
				const p = envelope.payload as SensorReadingPayload;
				this.currentPower = p.value;
				this.currentPowerTimestamp = p.timestamp;

				const point: ChartPoint = {
					timestamp: new Date(p.timestamp),
					value: p.value
				};

				this.chartData = [...this.chartData.slice(-MAX_CHART_POINTS + 1), point];
				break;
			}
			case MSG_SUMMARY_UPDATE: {
				const p = envelope.payload as SummaryPayload;
				this.todayKWh = p.today_kwh;
				this.monthKWh = p.month_kwh;
				this.totalKWh = p.total_kwh;
				break;
			}
			case MSG_DATA_LOADED: {
				const p = envelope.payload as DataLoadedPayload;
				this.sensors = p.sensors;
				this.timeRangeStart = p.time_range.start;
				this.timeRangeEnd = p.time_range.end;
				break;
			}
		}
	}
}

export const simulation = new SimulationStore();
