import { getClient, type WSClient } from '$lib/ws/client.svelte';
import {
	MSG_SIM_STATE,
	MSG_SENSOR_READING,
	MSG_SUMMARY_UPDATE,
	MSG_DATA_LOADED,
	MSG_BATTERY_UPDATE,
	MSG_BATTERY_SUMMARY,
	MSG_ARBITRAGE_DAY_LOG,
	MSG_PREDICTION_COMPARISON,
	MSG_SIM_START,
	MSG_SIM_PAUSE,
	MSG_SIM_SET_SPEED,
	MSG_SIM_SEEK,
	MSG_SIM_SET_SOURCE,
	MSG_BATTERY_CONFIG,
	MSG_SIM_SET_PREDICTION,
	MSG_CONFIG_UPDATE,
	type SimStatePayload,
	type SensorReadingPayload,
	type SummaryPayload,
	type DataLoadedPayload,
	type BatteryUpdatePayload,
	type BatterySummaryPayload,
	type ArbitrageDayLogPayload,
	type ArbitrageDayRecord,
	type PredictionComparisonPayload,
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
	predictionEnabled = $state(false);

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

	// Cost tracking (PLN)
	gridImportCostPLN = $state(0);
	gridExportRevenuePLN = $state(0);
	netCostPLN = $state(0);
	rawGridImportCostPLN = $state(0);
	rawGridExportRevenuePLN = $state(0);
	rawNetCostPLN = $state(0);
	batterySavingsPLN = $state(0);
	arbNetCostPLN = $state(0);
	arbBatterySavingsPLN = $state(0);

	// Cheap export tracking
	cheapExportKWh = $state(0);
	cheapExportRevPLN = $state(0);
	currentSpotPrice = $state(0);

	// Config
	exportCoefficient = $state(0.8);
	priceThresholdPLN = $state(0.1);
	tempOffsetC = $state(0);
	fixedTariffPLN = $state(0.65);
	distributionFeePLN = $state(0.20);
	netMeteringRatio = $state(0.8);
	batteryCostPerKWh = $state(1000);

	// Net metering/billing
	nmNetCostPLN = $state(0);
	nmCreditBankKWh = $state(0);
	nbNetCostPLN = $state(0);
	nbDepositPLN = $state(0);

	// Prediction comparison
	predActualPowerW = $state(0);
	predPredictedPowerW = $state(0);
	predActualTempC = $state(0);
	predPredictedTempC = $state(0);
	predHasActualTemp = $state(false);
	predHasData = $state(false);
	predPowerErrors = $state<number[]>([]);
	predTempErrors = $state<number[]>([]);

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
	batteryDegradationCycles = $state(4000);
	batteryEffectiveCapacityKWh = $state(0);
	batteryDegradationPct = $state(0);
	batteryTimeAtPowerSec = $state<Record<string, number>>({});
	batteryTimeAtSoCPctSec = $state<Record<string, number>>({});
	batteryMonthSoCSeconds = $state<Record<string, Record<string, number>>>({});

	// Arbitrage day log
	arbitrageDayRecords = $state<ArbitrageDayRecord[]>([]);

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
		this.arbitrageDayRecords = [];
		this.currentDayKey = '';
	}

	setDataSource(source: string): void {
		this.dataSource = source;
		this.client?.send(MSG_SIM_SET_SOURCE, { source });
		this.chartData = [];
		this.dailyRecords = [];
		this.arbitrageDayRecords = [];
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
			charge_to_percent: this.batteryChargeToPercent,
			degradation_cycles: this.batteryDegradationCycles
		});
		this.chartData = [];
		this.dailyRecords = [];
		this.arbitrageDayRecords = [];
		this.currentDayKey = '';
	}

	setPredictionMode(): void {
		this.client?.send(MSG_SIM_SET_PREDICTION, { enabled: this.predictionEnabled });
		this.chartData = [];
		this.dailyRecords = [];
		this.arbitrageDayRecords = [];
		this.currentDayKey = '';
		this.predHasData = false;
		this.predPowerErrors = [];
		this.predTempErrors = [];
	}

	sendConfig(): void {
		this.client?.send(MSG_CONFIG_UPDATE, {
			export_coefficient: this.exportCoefficient,
			price_threshold_pln: this.priceThresholdPLN,
			temp_offset_c: this.tempOffsetC,
			fixed_tariff_pln: this.fixedTariffPLN,
			distribution_fee_pln: this.distributionFeePLN,
			net_metering_ratio: this.netMeteringRatio
		});
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
				this.gridImportCostPLN = p.grid_import_cost_pln;
				this.gridExportRevenuePLN = p.grid_export_revenue_pln;
				this.netCostPLN = p.net_cost_pln;
				this.rawGridImportCostPLN = p.raw_grid_import_cost_pln;
				this.rawGridExportRevenuePLN = p.raw_grid_export_revenue_pln;
				this.rawNetCostPLN = p.raw_net_cost_pln;
				this.batterySavingsPLN = p.battery_savings_pln;
				this.arbNetCostPLN = p.arb_net_cost_pln;
				this.arbBatterySavingsPLN = p.arb_battery_savings_pln;
				this.cheapExportKWh = p.cheap_export_kwh;
				this.cheapExportRevPLN = p.cheap_export_rev_pln;
				this.currentSpotPrice = p.current_spot_price;
				this.nmNetCostPLN = p.nm_net_cost_pln;
				this.nmCreditBankKWh = p.nm_credit_bank_kwh;
				this.nbNetCostPLN = p.nb_net_cost_pln;
				this.nbDepositPLN = p.nb_deposit_pln;
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
				this.batteryEffectiveCapacityKWh = p.effective_capacity_kwh;
				this.batteryDegradationPct = p.degradation_pct;
				this.batteryTimeAtPowerSec = p.time_at_power_sec;
				this.batteryTimeAtSoCPctSec = p.time_at_soc_pct_sec;
				this.batteryMonthSoCSeconds = p.month_soc_seconds ?? {};
				break;
			}
			case MSG_ARBITRAGE_DAY_LOG: {
				const p = envelope.payload as ArbitrageDayLogPayload;
				this.arbitrageDayRecords = p.records;
				break;
			}
			case MSG_PREDICTION_COMPARISON: {
				const p = envelope.payload as PredictionComparisonPayload;
				this.predActualPowerW = p.actual_power_w;
				this.predPredictedPowerW = p.predicted_power_w;
				this.predActualTempC = p.actual_temp_c;
				this.predPredictedTempC = p.predicted_temp_c;
				this.predHasActualTemp = p.has_actual_temp;
				this.predHasData = true;
				// Track rolling errors (keep last 500)
				this.predPowerErrors = [...this.predPowerErrors.slice(-499), Math.abs(p.actual_power_w - p.predicted_power_w)];
				if (p.has_actual_temp) {
					this.predTempErrors = [...this.predTempErrors.slice(-499), Math.abs(p.actual_temp_c - p.predicted_temp_c)];
				}
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
			// Calculate how many days were skipped (use UTC to avoid timezone issues)
			const prevMs = Date.UTC(
				+this.currentDayKey.slice(0, 4),
				+this.currentDayKey.slice(5, 7) - 1,
				+this.currentDayKey.slice(8, 10)
			);
			const newMs = Date.UTC(
				+dayKey.slice(0, 4),
				+dayKey.slice(5, 7) - 1,
				+dayKey.slice(8, 10)
			);
			const dayGap = Math.round((newMs - prevMs) / 86400000);

			// Total delta across the entire gap
			const totalGridImport = p.grid_import_kwh - this.dayStartSnapshot.gridImportKWh;
			const totalSelfCons = p.self_consumption_kwh - this.dayStartSnapshot.selfConsumptionKWh;
			const totalBatSavings = p.battery_savings_kwh - this.dayStartSnapshot.batterySavingsKWh;
			const totalDemand = p.home_demand_kwh - this.dayStartSnapshot.homeDemandKWh;
			const totalHeatPump = p.heat_pump_kwh - this.dayStartSnapshot.heatPumpKWh;

			if (dayGap > 1) {
				// Multi-day skip: distribute energy equally across all days in the gap
				const records = [...this.dailyRecords];
				for (let i = 0; i < dayGap; i++) {
					const d = new Date(prevMs + i * 86400000);
					const dk = d.toISOString().slice(0, 10);
					const fraction = 1 / dayGap;
					const gridImport = totalGridImport * fraction;
					const selfCons = totalSelfCons * fraction;
					const batSavings = totalBatSavings * fraction;
					const demand = totalDemand * fraction;
					const heatPump = totalHeatPump * fraction;
					const offGrid = demand > 0 ? Math.min(100, ((selfCons + batSavings) / demand) * 100) : 0;
					const autonomy = demand > 0 ? (this.batteryCapacityKWh * 24) / (demand * dayGap) : 0;

					const rec: DailyRecord = {
						date: dk,
						dayOfWeek: d.getDay(),
						gridImportKWh: gridImport,
						selfConsumptionKWh: selfCons,
						batterySavingsKWh: batSavings,
						homeDemandKWh: demand,
						heatPumpKWh: heatPump,
						offGridPct: offGrid,
						batteryAutonomyHours: autonomy
					};

					// Replace existing in-progress record or append
					if (records.length > 0 && records[records.length - 1].date === dk) {
						records[records.length - 1] = rec;
					} else {
						records.push(rec);
					}
				}
				this.dailyRecords = records;
			} else {
				// Single day transition — finalize previous day
				this.finalizeDayRecord(p);
			}

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
