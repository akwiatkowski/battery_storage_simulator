import { getClient, type WSClient } from '$lib/ws/client.svelte';
import {
	MSG_SIM_STATE,
	MSG_SENSOR_READING,
	MSG_SUMMARY_UPDATE,
	MSG_DATA_LOADED,
	MSG_BATTERY_UPDATE,
	MSG_BATTERY_SUMMARY,
	MSG_SIM_START,
	MSG_SIM_PAUSE,
	MSG_SIM_SET_SPEED,
	MSG_SIM_SEEK,
	MSG_SIM_SET_SOURCE,
	MSG_BATTERY_CONFIG,
	type SimStatePayload,
	type SensorReadingPayload,
	type SummaryPayload,
	type DataLoadedPayload,
	type BatteryUpdatePayload,
	type BatterySummaryPayload,
	type SensorInfo,
	type Envelope
} from '$lib/ws/messages';

// Max data points to keep in the chart buffer
const MAX_CHART_POINTS = 500;

export interface ChartPoint {
	timestamp: Date;
	value: number;
}

export interface DailyRecord {
	date: string; // "YYYY-MM-DD"
	dayOfWeek: number; // 0=Sun … 6=Sat
	gridImportKWh: number;
	selfConsumptionKWh: number;
	batterySavingsKWh: number;
	homeDemandKWh: number;
	heatPumpKWh: number;
	offGridPct: number;
	batteryAutonomyHours: number; // hours a full battery could power the house
}

class SimulationStore {
	// Connection
	connected = $state(false);

	// Simulation state
	simTime = $state('');
	speed = $state(3600);
	running = $state(false);
	dataSource = $state('all');

	// Sensors
	sensors = $state<SensorInfo[]>([]);
	timeRangeStart = $state('');
	timeRangeEnd = $state('');

	// Sensor IDs (resolved from data:loaded)
	private gridPowerSensorId = '';
	private pvSensorId = '';
	private heatPumpSensorId = '';
	private heatPumpProdSensorId = '';

	// Current readings
	currentPower = $state(0);
	currentPowerTimestamp = $state('');
	currentPVPower = $state(0);
	currentHeatPumpPower = $state(0);
	currentHeatPumpProdPower = $state(0);

	// Energy summary
	todayKWh = $state(0);
	monthKWh = $state(0);
	totalKWh = $state(0);
	gridImportKWh = $state(0);
	gridExportKWh = $state(0);
	pvProductionKWh = $state(0);
	heatPumpKWh = $state(0);
	heatPumpProdKWh = $state(0);
	selfConsumptionKWh = $state(0);
	homeDemandKWh = $state(0);
	batterySavingsKWh = $state(0);

	// Chart data
	chartData = $state<ChartPoint[]>([]);

	// Battery state
	batteryEnabled = $state(false);
	batteryCapacityKWh = $state(10);
	batteryMaxPowerKW = $state(5);
	batteryDischargeToPercent = $state(10);
	batteryChargeToPercent = $state(100);
	batterySoCPercent = $state(0);
	batteryPowerW = $state(0);
	adjustedGridW = $state(0);
	batteryCycles = $state(0);
	batteryTimeAtPowerSec = $state<Record<string, number>>({});
	batteryTimeAtSoCPctSec = $state<Record<string, number>>({});
	batteryMonthSoCSeconds = $state<Record<string, Record<string, number>>>({});

	// Daily off-grid tracking
	dailyRecords = $state<DailyRecord[]>([]);
	private currentDayKey = '';
	private dayStartSnapshot = {
		gridImportKWh: 0,
		selfConsumptionKWh: 0,
		batterySavingsKWh: 0,
		homeDemandKWh: 0,
		heatPumpKWh: 0
	};

	private client: WSClient | null = null;
	private unsubscribe: (() => void) | null = null;

	private getOrCreateClient(): WSClient {
		if (!this.client) {
			this.client = getClient();
		}
		return this.client;
	}

	init(): void {
		const client = this.getOrCreateClient();
		this.unsubscribe = client.onMessage((envelope: Envelope) => {
			this.handleMessage(envelope);
		});
		client.connect();

		// Track connection state
		$effect.root(() => {
			$effect(() => {
				this.connected = client.connected;
			});
		});
	}

	destroy(): void {
		this.unsubscribe?.();
		this.client?.disconnect();
	}

	// Commands
	start(): void {
		this.client?.send(MSG_SIM_START);
	}

	pause(): void {
		this.client?.send(MSG_SIM_PAUSE);
	}

	setSpeed(speed: number): void {
		this.client?.send(MSG_SIM_SET_SPEED, { speed });
	}

	seek(timestamp: string): void {
		this.client?.send(MSG_SIM_SEEK, { timestamp });
		this.chartData = [];
		this.dailyRecords = [];
		this.currentDayKey = '';
	}

	setDataSource(source: string): void {
		this.dataSource = source;
		this.client?.send(MSG_SIM_SET_SOURCE, { source });
		this.chartData = [];
		this.dailyRecords = [];
		this.currentDayKey = '';
	}

	reset(): void {
		if (this.timeRangeStart) {
			this.seek(this.timeRangeStart);
		}
	}

	setBatteryConfig(): void {
		this.client?.send(MSG_BATTERY_CONFIG, {
			enabled: this.batteryEnabled,
			capacity_kwh: this.batteryCapacityKWh,
			max_power_w: this.batteryMaxPowerKW * 1000,
			discharge_to_percent: this.batteryDischargeToPercent,
			charge_to_percent: this.batteryChargeToPercent
		});
		this.chartData = [];
		this.dailyRecords = [];
		this.currentDayKey = '';
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
				if (p.sensor_id === this.pvSensorId) {
					this.currentPVPower = p.value;
					break;
				}
				if (p.sensor_id === this.heatPumpSensorId) {
					this.currentHeatPumpPower = p.value;
					break;
				}
				if (p.sensor_id === this.heatPumpProdSensorId) {
					this.currentHeatPumpProdPower = p.value;
					break;
				}
				if (this.gridPowerSensorId && p.sensor_id !== this.gridPowerSensorId) {
					break;
				}
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
				this.gridImportKWh = p.grid_import_kwh;
				this.gridExportKWh = p.grid_export_kwh;
				this.pvProductionKWh = p.pv_production_kwh;
				this.heatPumpKWh = p.heat_pump_kwh;
				this.heatPumpProdKWh = p.heat_pump_prod_kwh;
				this.selfConsumptionKWh = p.self_consumption_kwh;
				this.homeDemandKWh = p.home_demand_kwh;
				this.batterySavingsKWh = p.battery_savings_kwh;
				this.trackDailyData(p);
				break;
			}
			case MSG_BATTERY_UPDATE: {
				const p = envelope.payload as BatteryUpdatePayload;
				this.batteryPowerW = p.battery_power_w;
				this.adjustedGridW = p.adjusted_grid_w;
				this.batterySoCPercent = p.soc_percent;
				break;
			}
			case MSG_BATTERY_SUMMARY: {
				const p = envelope.payload as BatterySummaryPayload;
				this.batterySoCPercent = p.soc_percent;
				this.batteryCycles = p.cycles;
				this.batteryTimeAtPowerSec = p.time_at_power_sec;
				this.batteryTimeAtSoCPctSec = p.time_at_soc_pct_sec;
				this.batteryMonthSoCSeconds = p.month_soc_seconds ?? {};
				break;
			}
			case MSG_DATA_LOADED: {
				const p = envelope.payload as DataLoadedPayload;
				this.sensors = p.sensors;
				this.timeRangeStart = p.time_range.start;
				this.timeRangeEnd = p.time_range.end;

				// Resolve sensor IDs by type
				for (const s of p.sensors) {
					switch (s.type) {
						case 'grid_power':
							this.gridPowerSensorId = s.id;
							break;
						case 'pv_power':
							this.pvSensorId = s.id;
							break;
						case 'pump_total_consumption':
							this.heatPumpSensorId = s.id;
							break;
						case 'pump_total_production':
							this.heatPumpProdSensorId = s.id;
							break;
					}
				}
				break;
			}
		}
	}
	private getHoursFromMidnight(): number {
		if (!this.simTime || this.simTime.length < 19) return 0;
		const h = parseInt(this.simTime.slice(11, 13));
		const m = parseInt(this.simTime.slice(14, 16));
		const s = parseInt(this.simTime.slice(17, 19));
		return h + m / 60 + s / 3600;
	}

	private finalizeDayRecord(p: SummaryPayload): void {
		// Recompute ALL deltas using current cumulatives before resetting snapshot.
		// At high sim speeds, ticks can span entire days, so the previous in-progress
		// record may have stale/zero deltas — this recalculates from snapshot → now.
		const gridImport = p.grid_import_kwh - this.dayStartSnapshot.gridImportKWh;
		const selfCons = p.self_consumption_kwh - this.dayStartSnapshot.selfConsumptionKWh;
		const batSavings = p.battery_savings_kwh - this.dayStartSnapshot.batterySavingsKWh;
		const demand = p.home_demand_kwh - this.dayStartSnapshot.homeDemandKWh;
		const heatPump = p.heat_pump_kwh - this.dayStartSnapshot.heatPumpKWh;
		const offGrid = demand > 0 ? Math.min(100, ((selfCons + batSavings) / demand) * 100) : 0;
		const autonomy = demand > 0 ? (this.batteryCapacityKWh * 24) / demand : 0;

		if (this.dailyRecords.length > 0) {
			const records = [...this.dailyRecords];
			records[records.length - 1] = {
				...records[records.length - 1],
				gridImportKWh: gridImport,
				selfConsumptionKWh: selfCons,
				batterySavingsKWh: batSavings,
				homeDemandKWh: demand,
				heatPumpKWh: heatPump,
				offGridPct: offGrid,
				batteryAutonomyHours: autonomy
			};
			this.dailyRecords = records;
		}
	}

	private trackDailyData(p: SummaryPayload): void {
		if (!this.simTime) return;
		const dayKey = this.simTime.slice(0, 10);

		if (this.currentDayKey && dayKey !== this.currentDayKey) {
			// Day changed — recompute previous day's values from full delta, then reset
			this.finalizeDayRecord(p);
			this.dayStartSnapshot = {
				gridImportKWh: p.grid_import_kwh,
				selfConsumptionKWh: p.self_consumption_kwh,
				batterySavingsKWh: p.battery_savings_kwh,
				homeDemandKWh: p.home_demand_kwh,
				heatPumpKWh: p.heat_pump_kwh
			};
		}

		if (!this.currentDayKey) {
			this.dayStartSnapshot = {
				gridImportKWh: p.grid_import_kwh,
				selfConsumptionKWh: p.self_consumption_kwh,
				batterySavingsKWh: p.battery_savings_kwh,
				homeDemandKWh: p.home_demand_kwh,
				heatPumpKWh: p.heat_pump_kwh
			};
		}

		this.currentDayKey = dayKey;

		const gridImport = p.grid_import_kwh - this.dayStartSnapshot.gridImportKWh;
		const selfCons = p.self_consumption_kwh - this.dayStartSnapshot.selfConsumptionKWh;
		const batSavings = p.battery_savings_kwh - this.dayStartSnapshot.batterySavingsKWh;
		const demand = p.home_demand_kwh - this.dayStartSnapshot.homeDemandKWh;
		const heatPump = p.heat_pump_kwh - this.dayStartSnapshot.heatPumpKWh;
		const offGrid = demand > 0 ? Math.min(100, ((selfCons + batSavings) / demand) * 100) : 0;

		const hoursElapsed = Math.max(0.01, this.getHoursFromMidnight());
		const autonomy = demand > 0
			? (this.batteryCapacityKWh * hoursElapsed) / demand
			: 0;

		const d = new Date(dayKey + 'T00:00:00');
		const record: DailyRecord = {
			date: dayKey,
			dayOfWeek: d.getDay(),
			gridImportKWh: gridImport,
			selfConsumptionKWh: selfCons,
			batterySavingsKWh: batSavings,
			homeDemandKWh: demand,
			heatPumpKWh: heatPump,
			offGridPct: offGrid,
			batteryAutonomyHours: autonomy
		};

		const records = [...this.dailyRecords];
		if (records.length > 0 && records[records.length - 1].date === dayKey) {
			records[records.length - 1] = record;
		} else {
			records.push(record);
		}
		this.dailyRecords = records;
	}
}

export const simulation = new SimulationStore();
