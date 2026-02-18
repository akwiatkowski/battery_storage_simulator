package simulator

import (
	"sort"
	"sync"
	"time"

	"energy_simulator/internal/model"
	"energy_simulator/internal/solar"
	"energy_simulator/internal/store"
)

// State represents the current simulation state.
type State struct {
	Time    time.Time `json:"time"`
	Speed   float64   `json:"speed"`
	Running bool      `json:"running"`
}

// Summary holds running energy totals.
type Summary struct {
	TodayKWh float64 `json:"today_kwh"`
	MonthKWh float64 `json:"month_kwh"`
	TotalKWh float64 `json:"total_kwh"`

	GridImportKWh      float64 `json:"grid_import_kwh"`
	GridExportKWh      float64 `json:"grid_export_kwh"`
	PVProductionKWh    float64 `json:"pv_production_kwh"`
	HeatPumpKWh        float64 `json:"heat_pump_kwh"`
	HeatPumpProdKWh    float64 `json:"heat_pump_prod_kwh"`
	HeatPumpCostPLN    float64 `json:"heat_pump_cost_pln"`
	SelfConsumptionKWh float64 `json:"self_consumption_kwh"`
	HomeDemandKWh      float64 `json:"home_demand_kwh"`
	BatterySavingsKWh  float64 `json:"battery_savings_kwh"`

	// Cost tracking (PLN)
	GridImportCostPLN    float64 `json:"grid_import_cost_pln"`
	GridExportRevenuePLN float64 `json:"grid_export_revenue_pln"`
	NetCostPLN           float64 `json:"net_cost_pln"`
	RawGridImportCostPLN    float64 `json:"raw_grid_import_cost_pln"`
	RawGridExportRevenuePLN float64 `json:"raw_grid_export_revenue_pln"`
	RawNetCostPLN           float64 `json:"raw_net_cost_pln"`
	BatterySavingsPLN       float64 `json:"battery_savings_pln"`

	// Arbitrage strategy comparison
	ArbNetCostPLN        float64 `json:"arb_net_cost_pln"`
	ArbBatterySavingsPLN float64 `json:"arb_battery_savings_pln"`

	// Cheap export tracking
	CheapExportKWh    float64 `json:"cheap_export_kwh"`
	CheapExportRevPLN float64 `json:"cheap_export_rev_pln"`
	CurrentSpotPrice  float64 `json:"current_spot_price"`

	// Net metering
	NMNetCostPLN    float64 `json:"nm_net_cost_pln"`
	NMCreditBankKWh float64 `json:"nm_credit_bank_kwh"`

	// Net billing
	NBNetCostPLN float64 `json:"nb_net_cost_pln"`
	NBDepositPLN float64 `json:"nb_deposit_pln"`

	// Pre-heating
	PreHeatCostPLN    float64 `json:"pre_heat_cost_pln"`
	PreHeatSavingsPLN float64 `json:"pre_heat_savings_pln"`

	// PV arrays
	PVArrayProduction []PVArrayProd `json:"pv_array_production,omitempty"`
}

// PVArrayProd holds per-array PV production for the summary.
type PVArrayProd struct {
	Name string  `json:"name"`
	KWh  float64 `json:"kwh"`
}

// OffGridCoverage returns the percentage of adjusted home demand that could be
// covered by non-grid sources (PV self-consumption + battery). heatPumpPct and
// appliancePct scale the respective demand components (0–100).
func (s *Summary) OffGridCoverage(heatPumpPct, appliancePct float64) float64 {
	applianceKWh := s.HomeDemandKWh - s.HeatPumpKWh
	if applianceKWh < 0 {
		applianceKWh = 0
	}
	adjustedDemand := s.HeatPumpKWh*(heatPumpPct/100) + applianceKWh*(appliancePct/100)
	if adjustedDemand <= 0 {
		return 100
	}
	nonGridKWh := s.SelfConsumptionKWh + s.BatterySavingsKWh
	coverage := nonGridKWh / adjustedDemand * 100
	if coverage > 100 {
		coverage = 100
	}
	return coverage
}

// SensorReading is a reading emitted during simulation.
type SensorReading struct {
	SensorID  string  `json:"sensor_id"`
	Value     float64 `json:"value"`
	Unit      string  `json:"unit"`
	Timestamp string  `json:"timestamp"`
}

// BatteryUpdate is emitted each time the battery processes a reading.
type BatteryUpdate struct {
	BatteryPowerW float64 `json:"battery_power_w"`
	AdjustedGridW float64 `json:"adjusted_grid_w"`
	SoCPercent    float64 `json:"soc_percent"`
	Timestamp     string  `json:"timestamp"`
}

// ArbitrageDayRecord captures one day of arbitrage battery activity.
// Charge and discharge windows are guaranteed non-overlapping: charge first, then discharge.
type ArbitrageDayRecord struct {
	Date               string  `json:"date"`
	ChargeStartTime    string  `json:"charge_start_time"`
	ChargeEndTime      string  `json:"charge_end_time"`
	ChargeKWh          float64 `json:"charge_kwh"`
	DischargeStartTime string  `json:"discharge_start_time"`
	DischargeEndTime   string  `json:"discharge_end_time"`
	DischargeKWh       float64 `json:"discharge_kwh"`
	GapMinutes         int     `json:"gap_minutes"`
	CyclesDelta        float64 `json:"cycles_delta"`
	EarningsPLN        float64 `json:"earnings_pln"`
}

// PredictionComparison holds actual vs predicted values for a single timestamp.
type PredictionComparison struct {
	ActualPowerW    float64
	PredictedPowerW float64
	ActualTempC     float64
	PredictedTempC  float64
	HasActualTemp   bool
}

// HeatingMonthStat holds per-month heating statistics.
type HeatingMonthStat struct {
	Month          string
	ConsumptionKWh float64
	ProductionKWh  float64
	COP            float64
	CostPLN        float64
	AvgTempC       float64
	TempReadings   int
}

// AnomalyDayRecord captures a day's actual vs predicted consumption deviation.
type AnomalyDayRecord struct {
	Date         string
	ActualKWh    float64
	PredictedKWh float64
	DeviationPct float64
	AvgTempC     float64
}

// HPDiagnostics holds live heat pump diagnostic values.
type HPDiagnostics struct {
	COP             float64 `json:"cop"`
	CompressorSpeed float64 `json:"compressor_speed_rpm"`
	FanSpeed        float64 `json:"fan_speed_rpm"`
	DischargeTemp   float64 `json:"discharge_temp_c"`
	HighPressure    float64 `json:"high_pressure"`
	PumpFlow        float64 `json:"pump_flow_lmin"`
	InletTemp       float64 `json:"inlet_temp_c"`
	OutletTemp      float64 `json:"outlet_temp_c"`
	ThermalPowerW   float64 `json:"thermal_power_w"`
	DHWTemp         float64 `json:"dhw_temp_c"`
	OutsidePipeTemp float64 `json:"outside_pipe_temp_c"`
	InsidePipeTemp  float64 `json:"inside_pipe_temp_c"`
	Z1TargetTemp    float64 `json:"z1_target_temp_c"`
}

// PowerQuality holds live grid power quality values.
type PowerQuality struct {
	VoltageV         float64 `json:"voltage_v"`
	PowerFactorPct   float64 `json:"power_factor_pct"`
	ReactivePowerVAR float64 `json:"reactive_power_var"`
}

// LoadShiftStats holds load shifting analysis data sent to frontend.
type LoadShiftStats struct {
	Heatmap         [7][24]HeatmapCell `json:"heatmap"`
	AvgHPPrice      float64            `json:"avg_hp_price"`
	OverallAvgPrice float64            `json:"overall_avg_price"`
	ShiftCurrentPLN float64            `json:"shift_current_pln"`
	ShiftOptimalPLN float64            `json:"shift_optimal_pln"`
	ShiftSavingsPLN float64            `json:"shift_savings_pln"`
	ShiftWindowH    int                `json:"shift_window_h"`
}

// HeatmapCell holds per-cell data for the load shift heatmap.
type HeatmapCell struct {
	KWh      float64 `json:"kwh"`
	AvgPrice float64 `json:"avg_price"`
}

// PVArrayConfig describes a single PV array's orientation.
type PVArrayConfig struct {
	Name    string  `json:"name"`
	PeakWp  float64 `json:"peak_wp"`
	Azimuth float64 `json:"azimuth"`
	Tilt    float64 `json:"tilt"`
	Enabled bool    `json:"enabled"`
}

// hourlySlot accumulates HP energy and cost per hour slot.
type hourlySlot struct {
	hpWh      float64
	hpCostPLN float64
	priceSum  float64
	priceN    int
}

// heatingMonthAcc is a private accumulator for per-month heating data.
type heatingMonthAcc struct {
	consumptionWh float64
	productionWh  float64
	costPLN       float64
	tempSum       float64
	tempCount     int
}

// Callback receives simulation events.
type Callback interface {
	OnState(state State)
	OnReading(reading SensorReading)
	OnSummary(summary Summary)
	OnBatteryUpdate(update BatteryUpdate)
	OnBatterySummary(summary BatterySummary)
	OnArbitrageDayLog(records []ArbitrageDayRecord)
	OnPredictionComparison(comp PredictionComparison)
	OnHeatingStats(stats []HeatingMonthStat)
	OnAnomalyDays(records []AnomalyDayRecord)
	OnLoadShiftStats(stats LoadShiftStats)
	OnHPDiagnostics(diag HPDiagnostics)
	OnPowerQuality(pq PowerQuality)
}

// Engine replays historical sensor data at configurable speed.
type Engine struct {
	mu       sync.Mutex
	store    *store.Store
	callback Callback

	running   bool
	speed     float64
	simTime   time.Time
	timeRange model.TimeRange

	// Battery simulation (nil when disabled)
	battery    *Battery
	altBattery *Battery // arbitrage shadow (nil when battery disabled)

	// Arbitrage cost tracking
	arbGridImportWh, arbGridExportWh             float64
	arbGridImportCostPLN, arbGridExportRevenuePLN float64

	// Price threshold cache (per day)
	arbThresholdDay  time.Time
	arbLowThreshold  float64
	arbHighThreshold float64

	// Arbitrage day log tracking
	arbitrageDayRecords                                            []ArbitrageDayRecord
	arbitrageDayLogDirty                                           bool
	arbitrageCurrentDay                                            string
	arbitrageDayChargeWh, arbitrageDayDischargeWh                  float64
	arbitrageDayChargeStart, arbitrageDayChargeEnd                 string
	arbitrageDayDischargeStart, arbitrageDayDischargeEnd           string
	arbitrageDayStartThroughputWh                                  float64
	arbitrageDayStartRawNetCost, arbitrageDayStartArbNetCost       float64

	// Prediction mode
	predictionMode bool
	prediction     *PredictionProvider
	savedTimeRange model.TimeRange

	// Temperature sensor (for prediction comparison)
	tempSensorID string

	// Tracking for energy summaries
	lastReadings map[string]model.Reading // last reading per sensor
	dayStart     time.Time
	monthStart   time.Time
	todayWh      float64
	monthWh      float64
	totalWh      float64

	// Per-source energy tracking (Wh)
	pvWh, heatPumpWh, heatPumpProdWh float64
	heatPumpCostPLN                  float64
	gridImportWh, gridExportWh       float64
	rawGridImportWh, rawGridExportWh float64 // before battery adjustment

	// Energy cost tracking (PLN)
	priceSensorID                                string
	gridImportCostPLN, gridExportRevenuePLN      float64
	rawGridImportCostPLN, rawGridExportRevenuePLN float64

	// Export coefficient (0-1, default 0.8)
	exportCoefficient float64

	// Price threshold and cheap export tracking
	priceThresholdPLN                    float64
	cheapExportWh, cheapExportRevenuePLN float64
	currentSpotPrice                     float64

	// Net metering simulation
	fixedTariffPLN    float64 // default 0.65
	distributionFeePLN float64 // default 0.20
	netMeteringRatio  float64 // default 0.8
	nmCreditBuckets   [12]float64   // rolling 12-month credit bank (kWh), indexed by month%12
	nmCreditBucketMonth [12]time.Time // month each bucket was credited
	nmImportCostPLN   float64 // total import cost under net metering
	nmCreditUsedKWh   float64 // total credits consumed
	nmCreditBankKWh   float64 // current credit balance

	// Net billing simulation
	nbDepositPLN       float64 // current PLN deposit balance
	nbImportChargedPLN float64 // total import before deposit offset
	nbDepositUsedPLN   float64 // total deposit consumed
	nbExportValuedPLN  float64 // total export valued at RCEm

	// RCEm cache (monthly average spot price)
	nbRCEmMonth time.Time
	nbRCEmValue float64

	// Per-month heating accumulators (keyed by "YYYY-MM")
	heatingMonths     map[string]*heatingMonthAcc
	heatingMonthOrder []string

	// Pre-heating thermal model (shadow, like arbitrage battery)
	thermal        *ThermalModel
	preHeatCostPLN float64
	insulationLevel InsulationLevel

	// Load shift hourly tracking
	dayOfWeekHourly [7][24]hourlySlot
	overallPriceSum float64
	overallPriceN   int
	loadShiftDirty  bool

	// Custom PV configuration
	pvCustomEnabled bool
	pvBaseProfile   *solar.PVProfile
	pvArrays        []PVArrayConfig
	pvArrayWh       []float64 // per-array production accumulators

	// HP diagnostics snapshot values
	hpDiagCOP             float64
	hpDiagCompressorSpeed float64
	hpDiagFanSpeed        float64
	hpDiagDischargeTemp   float64
	hpDiagHighPressure    float64
	hpDiagPumpFlow        float64
	hpDiagInletTemp       float64
	hpDiagOutletTemp      float64
	hpDiagDHWTemp         float64
	hpDiagOutsidePipe     float64
	hpDiagInsidePipe      float64
	hpDiagZ1Target        float64
	hpDiagDirty           bool

	// Power quality snapshot values
	pqVoltage       float64
	pqPowerFactor   float64
	pqReactivePower float64
	pqDirty         bool

	// Anomaly tracking (per-day during historical replay with prediction)
	anomalyDays           []AnomalyDayRecord
	anomalyCurrentDay     string
	anomalyActualWh       float64
	anomalyPredictedWh    float64
	anomalyTempSum        float64
	anomalyTempCount      int
	anomalyDirty          bool
	anomalyLastGridTime   time.Time
	anomalyLastActualW    float64
	anomalyLastPredictedW float64
	anomalyHasLastGrid    bool

	stopCh chan struct{}
}

func New(s *store.Store, cb Callback) *Engine {
	return &Engine{
		store:              s,
		callback:           cb,
		speed:              3600,
		exportCoefficient:  0.8,
		priceThresholdPLN:  0.1,
		fixedTariffPLN:     0.65,
		distributionFeePLN: 0.20,
		netMeteringRatio:   0.8,
		lastReadings:       make(map[string]model.Reading),
		heatingMonths:      make(map[string]*heatingMonthAcc),
	}
}

// SetExportCoefficient sets the export revenue multiplier (0-1).
func (e *Engine) SetExportCoefficient(c float64) {
	e.mu.Lock()
	e.exportCoefficient = c
	e.mu.Unlock()
}

// SetPriceThreshold sets the PLN threshold for cheap export tracking.
func (e *Engine) SetPriceThreshold(t float64) {
	e.mu.Lock()
	e.priceThresholdPLN = t
	e.mu.Unlock()
}

// SetFixedTariff sets the fixed tariff rate for net metering/billing (PLN/kWh).
func (e *Engine) SetFixedTariff(v float64) {
	e.mu.Lock()
	e.fixedTariffPLN = v
	e.mu.Unlock()
}

// SetDistributionFee sets the distribution fee for net metering (PLN/kWh).
func (e *Engine) SetDistributionFee(v float64) {
	e.mu.Lock()
	e.distributionFeePLN = v
	e.mu.Unlock()
}

// SetNetMeteringRatio sets the credit ratio for net metering (e.g. 0.8 for 1:0.8).
func (e *Engine) SetNetMeteringRatio(v float64) {
	e.mu.Lock()
	e.netMeteringRatio = v
	e.mu.Unlock()
}

// SetInsulationLevel sets the building insulation level for pre-heating simulation.
func (e *Engine) SetInsulationLevel(level InsulationLevel) {
	e.mu.Lock()
	e.insulationLevel = level
	if e.thermal != nil {
		e.thermal.HeatLossWC = HeatLossForInsulation(level)
		e.thermal.Insulation = level
	}
	e.mu.Unlock()
}

// SetPVConfig configures custom PV arrays.
func (e *Engine) SetPVConfig(enabled bool, arrays []PVArrayConfig) {
	e.mu.Lock()
	e.pvCustomEnabled = enabled
	e.pvArrays = arrays
	e.pvArrayWh = make([]float64, len(arrays))

	// Build base profile from stored PV data if needed
	if enabled && e.pvBaseProfile == nil {
		e.buildPVBaseProfile()
	}
	e.mu.Unlock()
}

// buildPVBaseProfile derives a PV generation profile from stored data.
// Must be called with mu held.
func (e *Engine) buildPVBaseProfile() {
	// Find PV sensor
	var pvSensorID string
	for _, sensor := range e.store.Sensors() {
		if sensor.Type == model.SensorPVPower {
			pvSensorID = sensor.ID
			break
		}
	}
	if pvSensorID == "" {
		return
	}

	// Get all PV readings
	tr, ok := e.store.GlobalTimeRange()
	if !ok {
		return
	}
	readings := e.store.ReadingsInRange(pvSensorID, tr.Start, tr.End)

	// Default peak based on typical east-facing installation
	peakWp := 6500.0
	profile := solar.BuildProfileFromReadings(readings, peakWp)
	e.pvBaseProfile = &profile
}

// computeCustomPV calculates total PV power from configured arrays at the given time.
// Must be called with mu held. Returns total PV watts and per-array watts.
func (e *Engine) computeCustomPV(t time.Time) (float64, []float64) {
	if e.pvBaseProfile == nil || len(e.pvArrays) == 0 {
		return 0, nil
	}

	hour := float64(t.Hour()) + float64(t.Minute())/60.0
	baseAzimuth := 90.0 // original east-facing installation

	var total float64
	perArray := make([]float64, len(e.pvArrays))
	for i, arr := range e.pvArrays {
		if !arr.Enabled || arr.PeakWp <= 0 {
			continue
		}
		oriented := solar.GenerateOrientedProfile(*e.pvBaseProfile, arr.Azimuth, arr.Tilt, baseAzimuth)
		power := oriented.PowerAt(hour, arr.PeakWp)
		perArray[i] = power
		total += power
	}
	return total, perArray
}

// SetTempOffset sets the temperature offset for NN prediction.
func (e *Engine) SetTempOffset(offset float64) {
	e.mu.Lock()
	if e.prediction != nil {
		e.prediction.SetTempOffset(offset)
	}
	e.mu.Unlock()
}

// Init sets up the engine with the store's time range.
func (e *Engine) Init() bool {
	tr, ok := e.store.GlobalTimeRange()
	if !ok {
		return false
	}
	e.mu.Lock()
	defer e.mu.Unlock()

	e.timeRange = tr
	e.simTime = tr.Start
	e.dayStart = startOfDay(tr.Start)
	e.monthStart = startOfMonth(tr.Start)

	// Lazily initialize prediction provider for historical comparison
	if e.prediction != nil {
		e.prediction.EnsureInitialized(tr.Start)
	}
	return true
}

// State returns the current simulation state.
func (e *Engine) State() State {
	e.mu.Lock()
	defer e.mu.Unlock()
	return State{
		Time:    e.simTime,
		Speed:   e.speed,
		Running: e.running,
	}
}

// Start begins the simulation loop.
func (e *Engine) Start() {
	e.mu.Lock()
	if e.running {
		e.mu.Unlock()
		return
	}
	e.running = true
	e.stopCh = make(chan struct{})
	e.mu.Unlock()

	e.broadcastState()
	go e.loop()
}

// Pause stops the simulation loop.
func (e *Engine) Pause() {
	e.mu.Lock()
	if !e.running {
		e.mu.Unlock()
		return
	}
	e.running = false
	close(e.stopCh)
	e.mu.Unlock()

	e.broadcastState()
}

// SetSpeed sets the simulation speed multiplier.
func (e *Engine) SetSpeed(speed float64) {
	if speed < 0.1 {
		speed = 0.1
	}
	if speed > 2592000 {
		speed = 2592000
	}

	e.mu.Lock()
	e.speed = speed
	e.mu.Unlock()

	e.broadcastState()
}

// SetBattery configures the battery simulation. Pass nil to disable.
func (e *Engine) SetBattery(cfg *BatteryConfig) {
	e.mu.Lock()
	if cfg == nil {
		e.battery = nil
		e.altBattery = nil
	} else {
		e.battery = NewBattery(*cfg)
		e.altBattery = NewBattery(*cfg)
	}
	e.mu.Unlock()
}

// SetPrediction stores the prediction provider (called at startup).
func (e *Engine) SetPrediction(p *PredictionProvider) {
	e.mu.Lock()
	e.prediction = p
	e.mu.Unlock()
}

// SetPredictionMode enables or disables neural network prediction mode.
func (e *Engine) SetPredictionMode(enabled bool) {
	e.mu.Lock()
	if enabled == e.predictionMode {
		e.mu.Unlock()
		return
	}

	if enabled {
		e.savedTimeRange = e.timeRange
		e.predictionMode = true
		now := time.Now().UTC()
		e.simTime = now
		e.timeRange = model.TimeRange{
			Start: now,
			End:   time.Date(2200, 1, 1, 0, 0, 0, 0, time.UTC),
		}
		e.resetAccumulators()
		if e.prediction != nil {
			e.prediction.Init(now)
		}
	} else {
		e.predictionMode = false
		e.timeRange = e.savedTimeRange
		e.simTime = e.savedTimeRange.Start
		e.resetAccumulators()
	}
	e.mu.Unlock()

	e.broadcastState()
	e.broadcastSummary()
}

// SetPriceSensor configures the sensor used for spot price lookups.
func (e *Engine) SetPriceSensor(sensorID string) {
	e.mu.Lock()
	e.priceSensorID = sensorID
	e.mu.Unlock()
}

// SetTempSensor configures the sensor used for actual temperature lookups.
func (e *Engine) SetTempSensor(sensorID string) {
	e.mu.Lock()
	e.tempSensorID = sensorID
	e.mu.Unlock()
}

// spotPrice returns the spot price at the given time. Must be called with mu held.
func (e *Engine) spotPrice(t time.Time) float64 {
	if e.priceSensorID == "" {
		return 0
	}
	r, ok := e.store.ReadingAt(e.priceSensorID, t)
	if !ok {
		return 0
	}
	return r.Value
}

// PredictionMode returns whether prediction mode is active.
func (e *Engine) PredictionMode() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.predictionMode
}

// resetAccumulators zeroes all energy counters. Must be called with mu held.
func (e *Engine) resetAccumulators() {
	e.dayStart = startOfDay(e.simTime)
	e.monthStart = startOfMonth(e.simTime)
	e.todayWh = 0
	e.monthWh = 0
	e.totalWh = 0
	e.pvWh = 0
	e.heatPumpWh = 0
	e.heatPumpProdWh = 0
	e.heatPumpCostPLN = 0
	e.gridImportWh = 0
	e.gridExportWh = 0
	e.rawGridImportWh = 0
	e.rawGridExportWh = 0
	e.gridImportCostPLN = 0
	e.gridExportRevenuePLN = 0
	e.rawGridImportCostPLN = 0
	e.rawGridExportRevenuePLN = 0
	e.arbGridImportWh = 0
	e.arbGridExportWh = 0
	e.arbGridImportCostPLN = 0
	e.arbGridExportRevenuePLN = 0
	e.cheapExportWh = 0
	e.cheapExportRevenuePLN = 0
	e.currentSpotPrice = 0
	e.arbThresholdDay = time.Time{}
	e.arbLowThreshold = 0
	e.arbHighThreshold = 0
	e.arbitrageDayRecords = nil
	e.arbitrageDayLogDirty = false
	e.arbitrageCurrentDay = ""
	e.arbitrageDayChargeWh = 0
	e.arbitrageDayDischargeWh = 0
	e.arbitrageDayChargeStart = ""
	e.arbitrageDayChargeEnd = ""
	e.arbitrageDayDischargeStart = ""
	e.arbitrageDayDischargeEnd = ""
	e.arbitrageDayStartThroughputWh = 0
	e.arbitrageDayStartRawNetCost = 0
	e.arbitrageDayStartArbNetCost = 0
	// Net metering reset
	e.nmImportCostPLN = 0
	e.nmCreditUsedKWh = 0
	e.nmCreditBankKWh = 0
	e.nmCreditBuckets = [12]float64{}
	e.nmCreditBucketMonth = [12]time.Time{}

	// Net billing reset
	e.nbDepositPLN = 0
	e.nbImportChargedPLN = 0
	e.nbDepositUsedPLN = 0
	e.nbExportValuedPLN = 0
	e.nbRCEmMonth = time.Time{}
	e.nbRCEmValue = 0

	// Heating stats reset
	e.heatingMonths = make(map[string]*heatingMonthAcc)
	e.heatingMonthOrder = nil

	// Thermal model reset
	if e.thermal != nil {
		e.thermal.Reset()
	}
	e.preHeatCostPLN = 0

	// Load shift reset
	e.dayOfWeekHourly = [7][24]hourlySlot{}
	e.overallPriceSum = 0
	e.overallPriceN = 0
	e.loadShiftDirty = false

	// PV array accumulators reset
	e.pvArrayWh = make([]float64, len(e.pvArrays))

	// HP diagnostics reset
	e.hpDiagCOP = 0
	e.hpDiagCompressorSpeed = 0
	e.hpDiagFanSpeed = 0
	e.hpDiagDischargeTemp = 0
	e.hpDiagHighPressure = 0
	e.hpDiagPumpFlow = 0
	e.hpDiagInletTemp = 0
	e.hpDiagOutletTemp = 0
	e.hpDiagDHWTemp = 0
	e.hpDiagOutsidePipe = 0
	e.hpDiagInsidePipe = 0
	e.hpDiagZ1Target = 0
	e.hpDiagDirty = false

	// Power quality reset
	e.pqVoltage = 0
	e.pqPowerFactor = 0
	e.pqReactivePower = 0
	e.pqDirty = false

	// Anomaly tracking reset
	e.anomalyDays = nil
	e.anomalyCurrentDay = ""
	e.anomalyActualWh = 0
	e.anomalyPredictedWh = 0
	e.anomalyTempSum = 0
	e.anomalyTempCount = 0
	e.anomalyDirty = false
	e.anomalyLastGridTime = time.Time{}
	e.anomalyLastActualW = 0
	e.anomalyLastPredictedW = 0
	e.anomalyHasLastGrid = false

	e.lastReadings = make(map[string]model.Reading)
	if e.battery != nil {
		e.battery.Reset()
	}
	if e.altBattery != nil {
		e.altBattery.Reset()
	}
}

// Seek jumps to a specific time. Resets energy summaries and battery.
func (e *Engine) Seek(t time.Time) {
	e.mu.Lock()
	if t.Before(e.timeRange.Start) {
		t = e.timeRange.Start
	}
	if t.After(e.timeRange.End) {
		t = e.timeRange.End
	}

	e.simTime = t
	e.resetAccumulators()
	e.mu.Unlock()

	e.broadcastState()
	e.broadcastSummary()
}

// SetTimeRange updates the engine's time range and seeks to its start.
func (e *Engine) SetTimeRange(tr model.TimeRange) {
	e.mu.Lock()
	e.timeRange = tr
	e.mu.Unlock()
	e.Seek(tr.Start)
}

// TimeRange returns the data time range.
func (e *Engine) TimeRange() model.TimeRange {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.timeRange
}

// Sensors returns all registered sensors.
func (e *Engine) Sensors() []model.Sensor {
	return e.store.Sensors()
}

// Step advances the simulation by the given duration and emits readings.
// Useful for deterministic testing. Does not require Start().
func (e *Engine) Step(delta time.Duration) {
	e.mu.Lock()

	prevTime := e.simTime
	e.simTime = e.simTime.Add(delta)

	ended := false
	if !e.simTime.Before(e.timeRange.End) {
		e.simTime = e.timeRange.End
		ended = true
	}

	currentTime := e.simTime
	endTime := e.timeRange.End
	e.mu.Unlock()

	e.emitReadings(prevTime, currentTime, endTime)
	e.broadcastState()
	e.broadcastSummary()

	if ended {
		e.mu.Lock()
		e.running = false
		e.mu.Unlock()
		e.broadcastState()
	}
}

const tickInterval = 100 * time.Millisecond

func (e *Engine) loop() {
	ticker := time.NewTicker(tickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-e.stopCh:
			return
		case <-ticker.C:
			if e.tick() {
				return
			}
		}
	}
}

// tick advances one frame. Returns true if simulation reached the end.
func (e *Engine) tick() bool {
	e.mu.Lock()

	simDelta := time.Duration(float64(tickInterval) * e.speed)
	prevTime := e.simTime
	e.simTime = e.simTime.Add(simDelta)

	inPrediction := e.predictionMode

	ended := false
	if !inPrediction && !e.simTime.Before(e.timeRange.End) {
		e.simTime = e.timeRange.End
		ended = true
	}

	currentTime := e.simTime
	endTime := e.timeRange.End
	e.mu.Unlock()

	e.emitReadings(prevTime, currentTime, endTime)
	e.broadcastState()
	e.broadcastSummary()

	if ended {
		e.mu.Lock()
		e.running = false
		close(e.stopCh)
		e.mu.Unlock()
		e.broadcastState()
		return true
	}

	return false
}

func (e *Engine) emitReadings(prevTime, currentTime, endTime time.Time) {
	e.mu.Lock()
	inPrediction := e.predictionMode
	pred := e.prediction
	e.mu.Unlock()

	if inPrediction && pred != nil {
		e.emitPredictions(prevTime, currentTime)
		return
	}

	// ReadingsInRange is [start, end), so add a nanosecond when at end of data
	// to include the final reading.
	queryEnd := currentTime
	if !currentTime.Before(endTime) {
		queryEnd = currentTime.Add(time.Nanosecond)
	}
	for _, sensor := range e.store.Sensors() {
		readings := e.store.ReadingsInRange(sensor.ID, prevTime, queryEnd)
		for _, r := range readings {
			// Custom PV mode: replace PV readings and adjust grid readings
			e.mu.Lock()
			pvCustom := e.pvCustomEnabled
			e.mu.Unlock()

			if pvCustom && r.Type == model.SensorPVPower {
				e.mu.Lock()
				totalPV, perArray := e.computeCustomPV(r.Timestamp)
				for i, w := range perArray {
					if i < len(e.pvArrayWh) {
						// Use trapezoid: track last PV per array
						key := r.SensorID + ":pvArr"
						lastR, ok := e.lastReadings[key]
						if ok {
							hours := r.Timestamp.Sub(lastR.Timestamp).Hours()
							// We approximate: just accumulate this step
							e.pvArrayWh[i] += w * hours * 0.5 // rough: will be refined by updateEnergy
						}
					}
				}
				_ = perArray
				e.mu.Unlock()
				r.Value = totalPV
			}

			if pvCustom && r.Type == model.SensorGridPower {
				e.mu.Lock()
				// Adjust grid: remove historical PV contribution, add new PV
				// Historical PV at this time
				var historicalPV float64
				for _, s := range e.store.Sensors() {
					if s.Type == model.SensorPVPower {
						if pvR, ok := e.store.ReadingAt(s.ID, r.Timestamp); ok {
							historicalPV = pvR.Value
						}
						break
					}
				}
				newPV, _ := e.computeCustomPV(r.Timestamp)
				// grid_stored = actual_demand - historical_pv
				// we want: grid_new = actual_demand - new_pv = grid_stored + historical_pv - new_pv
				r.Value = r.Value + historicalPV - newPV
				e.mu.Unlock()
			}

			e.callback.OnReading(SensorReading{
				SensorID:  r.SensorID,
				Value:     r.Value,
				Unit:      r.Unit,
				Timestamp: r.Timestamp.Format(time.RFC3339),
			})

			// Capture HP diagnostic and power quality snapshot values
			e.captureDiagnosticSnapshot(r)

			// Accumulate temperature for heating months and anomaly tracking
			if r.Type == model.SensorPumpExtTemp {
				e.mu.Lock()
				mk := r.Timestamp.Format("2006-01")
				acc := e.getOrCreateHeatingMonth(mk)
				acc.tempSum += r.Value
				acc.tempCount++
				if e.anomalyCurrentDay != "" {
					dayKey := r.Timestamp.Format("2006-01-02")
					if dayKey == e.anomalyCurrentDay {
						e.anomalyTempSum += r.Value
						e.anomalyTempCount++
					}
				}
				e.mu.Unlock()
			}

			// When battery is active, process grid_power through battery
			e.mu.Lock()
			bat := e.battery
			altBat := e.altBattery
			priceSensor := e.priceSensorID
			localPred := e.prediction
			tempSensor := e.tempSensorID
			e.mu.Unlock()

			// Prediction comparison during historical replay
			if localPred != nil && r.Type == model.SensorGridPower {
				if predictedPower, ok := localPred.PredictedPowerAt(r.Timestamp); ok {
					comp := PredictionComparison{
						ActualPowerW:    r.Value,
						PredictedPowerW: predictedPower,
					}
					if predictedTemp, ok := localPred.PredictedTempAt(r.Timestamp); ok {
						comp.PredictedTempC = predictedTemp
						if tempSensor != "" {
							if actualTemp, ok := e.store.ReadingAt(tempSensor, r.Timestamp); ok {
								comp.ActualTempC = actualTemp.Value
								comp.HasActualTemp = true
							}
						}
					}
					e.callback.OnPredictionComparison(comp)

					// Anomaly day accumulation
					e.mu.Lock()
					dayKey := r.Timestamp.Format("2006-01-02")
					if dayKey != e.anomalyCurrentDay {
						e.finalizeAnomalyDay()
						e.anomalyCurrentDay = dayKey
						e.anomalyActualWh = 0
						e.anomalyPredictedWh = 0
						e.anomalyTempSum = 0
						e.anomalyTempCount = 0
						e.anomalyHasLastGrid = false
					}
					if e.anomalyHasLastGrid {
						hours := r.Timestamp.Sub(e.anomalyLastGridTime).Hours()
						avgActual := (e.anomalyLastActualW + r.Value) / 2
						avgPredicted := (e.anomalyLastPredictedW + predictedPower) / 2
						if avgActual > 0 {
							e.anomalyActualWh += avgActual * hours
						}
						if avgPredicted > 0 {
							e.anomalyPredictedWh += avgPredicted * hours
						}
					}
					e.anomalyLastGridTime = r.Timestamp
					e.anomalyLastActualW = r.Value
					e.anomalyLastPredictedW = predictedPower
					e.anomalyHasLastGrid = true
					e.mu.Unlock()
				}
			}

			if bat != nil && r.Type == model.SensorGridPower {
				e.updateRawGridEnergy(r)
				e.updateNetMeteringEnergy(r)
				e.updateNetBillingEnergy(r)
				result := bat.Process(r.Value, r.Timestamp)
				e.callback.OnBatteryUpdate(BatteryUpdate{
					BatteryPowerW: result.BatteryPowerW,
					AdjustedGridW: result.AdjustedGridW,
					SoCPercent:    result.SoCPercent,
					Timestamp:     r.Timestamp.Format(time.RFC3339),
				})
				// Use adjusted grid value for energy calculation
				adjusted := r
				adjusted.Value = result.AdjustedGridW
				e.updateEnergy(adjusted)

				// Shadow arbitrage battery
				if altBat != nil && priceSensor != "" {
					low, high := e.priceThresholds(r.Timestamp)
					if low != high {
						var price float64
						if pr, ok := e.store.ReadingAt(priceSensor, r.Timestamp); ok {
							price = pr.Value
						}
						arbResult := altBat.ProcessArbitrage(r.Value, r.Timestamp, price, low, high)
						arbAdjusted := r
						arbAdjusted.Value = arbResult.AdjustedGridW
						e.updateArbGridEnergy(arbAdjusted)
						e.trackArbitrageDay(arbResult.BatteryPowerW, r.Timestamp)
					}
				}
			} else {
				if r.Type == model.SensorGridPower {
					e.updateRawGridEnergy(r)
					e.updateNetMeteringEnergy(r)
					e.updateNetBillingEnergy(r)
				}
				e.updateEnergy(r)
			}
		}
	}
}

func (e *Engine) emitPredictions(prevTime, currentTime time.Time) {
	e.mu.Lock()
	pred := e.prediction
	bat := e.battery
	e.mu.Unlock()

	readings := pred.ReadingsForRange(prevTime, currentTime)
	for _, sr := range readings {
		ts, _ := time.Parse(time.RFC3339, sr.Timestamp)

		// Adjust predicted grid power for custom PV
		e.mu.Lock()
		pvCustom := e.pvCustomEnabled
		e.mu.Unlock()
		if pvCustom {
			e.mu.Lock()
			// NN grid power includes implicit reference PV
			// Compute reference PV from base profile, then compute new PV
			var refPV float64
			if e.pvBaseProfile != nil {
				hour := float64(ts.Hour()) + float64(ts.Minute())/60.0
				refPV = e.pvBaseProfile.PowerAt(hour, e.pvBaseProfile.PeakWp)
			}
			newPV, _ := e.computeCustomPV(ts)
			sr.Value = sr.Value + refPV - newPV
			e.mu.Unlock()
		}

		e.callback.OnReading(sr)

		r := model.Reading{
			Timestamp: ts,
			SensorID:  sr.SensorID,
			Type:      model.SensorGridPower,
			Value:     sr.Value,
			Unit:      sr.Unit,
		}

		if bat != nil {
			e.updateRawGridEnergy(r)
			e.updateNetMeteringEnergy(r)
			e.updateNetBillingEnergy(r)
			result := bat.Process(r.Value, r.Timestamp)
			e.callback.OnBatteryUpdate(BatteryUpdate{
				BatteryPowerW: result.BatteryPowerW,
				AdjustedGridW: result.AdjustedGridW,
				SoCPercent:    result.SoCPercent,
				Timestamp:     sr.Timestamp,
			})
			adjusted := r
			adjusted.Value = result.AdjustedGridW
			e.updateEnergy(adjusted)
		} else {
			e.updateRawGridEnergy(r)
			e.updateNetMeteringEnergy(r)
			e.updateNetBillingEnergy(r)
			e.updateEnergy(r)
		}
	}
}

func (e *Engine) updateEnergy(r model.Reading) {
	e.mu.Lock()
	defer e.mu.Unlock()

	last, exists := e.lastReadings[r.SensorID]
	if !exists {
		e.lastReadings[r.SensorID] = r
		return
	}

	hours := r.Timestamp.Sub(last.Timestamp).Hours()
	avgPower := (last.Value + r.Value) / 2
	wh := avgPower * hours

	switch r.Type {
	case model.SensorGridPower:
		// Split into import (positive) and export (negative)
		price := e.spotPrice(r.Timestamp)
		e.currentSpotPrice = price
		if wh > 0 {
			e.gridImportWh += wh
			e.gridImportCostPLN += (wh / 1000) * price

			newDay := startOfDay(r.Timestamp)
			if newDay.After(e.dayStart) {
				e.dayStart = newDay
				e.todayWh = 0
			}
			newMonth := startOfMonth(r.Timestamp)
			if newMonth.After(e.monthStart) {
				e.monthStart = newMonth
				e.monthWh = 0
			}
			e.todayWh += wh
			e.monthWh += wh
			e.totalWh += wh
		} else if wh < 0 {
			exportWh := -wh
			e.gridExportWh += exportWh
			e.gridExportRevenuePLN += (exportWh / 1000) * price * e.exportCoefficient
			// Track cheap export
			if price < e.priceThresholdPLN {
				e.cheapExportWh += exportWh
				e.cheapExportRevenuePLN += (exportWh / 1000) * price * e.exportCoefficient
			}
		}
	case model.SensorPVPower:
		if wh > 0 {
			e.pvWh += wh
		}
	case model.SensorPumpConsumption:
		if wh > 0 {
			e.heatPumpWh += wh
			price := e.spotPrice(r.Timestamp)
			cost := (wh / 1000) * price
			e.heatPumpCostPLN += cost
			mk := r.Timestamp.Format("2006-01")
			acc := e.getOrCreateHeatingMonth(mk)
			acc.consumptionWh += wh
			acc.costPLN += cost

			// Hourly load shift tracking
			dow := int(r.Timestamp.Weekday())
			hour := r.Timestamp.Hour()
			e.dayOfWeekHourly[dow][hour].hpWh += wh
			e.dayOfWeekHourly[dow][hour].hpCostPLN += cost
			if price > 0 {
				e.dayOfWeekHourly[dow][hour].priceSum += price
				e.dayOfWeekHourly[dow][hour].priceN++
			}
			if price > 0 {
				e.overallPriceSum += price
				e.overallPriceN++
			}
			e.loadShiftDirty = true

			// Pre-heating thermal shadow
			if e.thermal == nil {
				e.thermal = NewThermalModel(e.insulationLevel)
			}
			if e.priceSensorID != "" {
				low, high := e.arbLowThreshold, e.arbHighThreshold
				// Get outdoor temp from heating month data
				outdoorTemp := 10.0 // default fallback
				if acc.tempCount > 0 {
					outdoorTemp = acc.tempSum / float64(acc.tempCount)
				}
				cop := 1.0
				if acc.consumptionWh > 0 && acc.productionWh > 0 {
					cop = acc.productionWh / acc.consumptionWh
				}
				if cop < 1 {
					cop = 1
				}
				e.thermal.Step(outdoorTemp, price, low, high, r.Value, cop, r.Timestamp)
				e.preHeatCostPLN = e.thermal.CostPLN
			}
		}
	case model.SensorPumpProduction:
		if wh > 0 {
			e.heatPumpProdWh += wh
			mk := r.Timestamp.Format("2006-01")
			acc := e.getOrCreateHeatingMonth(mk)
			acc.productionWh += wh
		}
	}

	e.lastReadings[r.SensorID] = r
}

func (e *Engine) updateRawGridEnergy(r model.Reading) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Use a separate key to avoid colliding with the adjusted grid tracking
	key := r.SensorID + ":raw"
	last, exists := e.lastReadings[key]
	if !exists {
		e.lastReadings[key] = r
		return
	}

	hours := r.Timestamp.Sub(last.Timestamp).Hours()
	avgPower := (last.Value + r.Value) / 2
	wh := avgPower * hours

	price := e.spotPrice(r.Timestamp)
	if wh > 0 {
		e.rawGridImportWh += wh
		e.rawGridImportCostPLN += (wh / 1000) * price
	} else if wh < 0 {
		e.rawGridExportWh += -wh
		e.rawGridExportRevenuePLN += (-wh / 1000) * price * e.exportCoefficient
	}

	e.lastReadings[key] = r
}

// priceThresholds returns daily P33/P67 price thresholds for arbitrage.
// Returns (0, 0) if no price data available, which makes low == high and skips arb.
func (e *Engine) priceThresholds(t time.Time) (low, high float64) {
	day := startOfDay(t)

	e.mu.Lock()
	if day.Equal(e.arbThresholdDay) {
		low, high = e.arbLowThreshold, e.arbHighThreshold
		e.mu.Unlock()
		return
	}
	priceSensor := e.priceSensorID
	e.mu.Unlock()

	if priceSensor == "" {
		return 0, 0
	}

	dayEnd := day.Add(24 * time.Hour)
	readings := e.store.ReadingsInRange(priceSensor, day, dayEnd)
	if len(readings) == 0 {
		return 0, 0
	}

	prices := make([]float64, len(readings))
	for i, r := range readings {
		prices[i] = r.Value
	}
	sort.Float64s(prices)

	n := len(prices)
	p33 := prices[(n-1)*33/100]
	p67 := prices[(n-1)*67/100]

	e.mu.Lock()
	e.arbThresholdDay = day
	e.arbLowThreshold = p33
	e.arbHighThreshold = p67
	e.mu.Unlock()

	return p33, p67
}

func (e *Engine) updateArbGridEnergy(r model.Reading) {
	e.mu.Lock()
	defer e.mu.Unlock()

	key := r.SensorID + ":arb"
	last, exists := e.lastReadings[key]
	if !exists {
		e.lastReadings[key] = r
		return
	}

	hours := r.Timestamp.Sub(last.Timestamp).Hours()
	avgPower := (last.Value + r.Value) / 2
	wh := avgPower * hours

	price := e.spotPrice(r.Timestamp)
	if wh > 0 {
		e.arbGridImportWh += wh
		e.arbGridImportCostPLN += (wh / 1000) * price
	} else if wh < 0 {
		e.arbGridExportWh += -wh
		e.arbGridExportRevenuePLN += (-wh / 1000) * price * e.exportCoefficient
	}

	e.lastReadings[key] = r
}

func (e *Engine) updateNetMeteringEnergy(r model.Reading) {
	e.mu.Lock()
	defer e.mu.Unlock()

	key := r.SensorID + ":nm"
	last, exists := e.lastReadings[key]
	if !exists {
		e.lastReadings[key] = r
		return
	}

	hours := r.Timestamp.Sub(last.Timestamp).Hours()
	avgPower := (last.Value + r.Value) / 2
	wh := avgPower * hours
	kwh := wh / 1000

	curMonth := startOfMonth(r.Timestamp)

	if kwh < 0 {
		// Export: store credits at ratio
		exportKWh := -kwh
		creditKWh := exportKWh * e.netMeteringRatio
		idx := int(r.Timestamp.Month()-1) % 12
		e.nmCreditBuckets[idx] += creditKWh
		e.nmCreditBucketMonth[idx] = curMonth
	} else if kwh > 0 {
		// Import: consume oldest non-expired credits first (FIFO)
		remaining := kwh

		// Expire old buckets and consume in order
		for i := 0; i < 12 && remaining > 0; i++ {
			// Start from oldest month relative to current
			idx := (int(r.Timestamp.Month()) + i) % 12
			if e.nmCreditBuckets[idx] <= 0 {
				continue
			}
			// Check expiry: credit must be within 12 months
			if !e.nmCreditBucketMonth[idx].IsZero() {
				age := curMonth.Sub(e.nmCreditBucketMonth[idx])
				if age > 365*24*time.Hour {
					e.nmCreditBuckets[idx] = 0
					continue
				}
			}
			used := e.nmCreditBuckets[idx]
			if used > remaining {
				used = remaining
			}
			e.nmCreditBuckets[idx] -= used
			remaining -= used
			e.nmCreditUsedKWh += used
			// Credited energy still pays distribution fee
			e.nmImportCostPLN += used * e.distributionFeePLN
		}

		// Uncredited remainder pays full fixed tariff
		if remaining > 0 {
			e.nmImportCostPLN += remaining * e.fixedTariffPLN
		}
	}

	// Update credit bank total
	var total float64
	for _, v := range e.nmCreditBuckets {
		total += v
	}
	e.nmCreditBankKWh = total

	e.lastReadings[key] = r
}

func (e *Engine) updateNetBillingEnergy(r model.Reading) {
	e.mu.Lock()
	defer e.mu.Unlock()

	key := r.SensorID + ":nb"
	last, exists := e.lastReadings[key]
	if !exists {
		e.lastReadings[key] = r
		return
	}

	hours := r.Timestamp.Sub(last.Timestamp).Hours()
	avgPower := (last.Value + r.Value) / 2
	wh := avgPower * hours
	kwh := wh / 1000

	if kwh < 0 {
		// Export: value at RCEm (monthly average spot price) → add to deposit
		exportKWh := -kwh
		rcem := e.monthlyAvgSpotPriceLocked(r.Timestamp)
		value := exportKWh * rcem
		e.nbDepositPLN += value
		e.nbExportValuedPLN += value
	} else if kwh > 0 {
		// Import: charge at fixed tariff, deduct from deposit
		importCost := kwh * e.fixedTariffPLN
		e.nbImportChargedPLN += importCost

		if e.nbDepositPLN > 0 {
			deduct := importCost
			if deduct > e.nbDepositPLN {
				deduct = e.nbDepositPLN
			}
			e.nbDepositPLN -= deduct
			e.nbDepositUsedPLN += deduct
		}
	}

	e.lastReadings[key] = r
}

// monthlyAvgSpotPriceLocked returns the monthly average spot price. Must be called with mu held.
func (e *Engine) monthlyAvgSpotPriceLocked(t time.Time) float64 {
	month := startOfMonth(t)
	if month.Equal(e.nbRCEmMonth) {
		return e.nbRCEmValue
	}

	if e.priceSensorID == "" {
		return 0
	}

	monthEnd := month.AddDate(0, 1, 0)
	readings := e.store.ReadingsInRange(e.priceSensorID, month, monthEnd)
	if len(readings) == 0 {
		return 0
	}

	var sum float64
	for _, r := range readings {
		sum += r.Value
	}
	avg := sum / float64(len(readings))

	e.nbRCEmMonth = month
	e.nbRCEmValue = avg
	return avg
}

func (e *Engine) broadcastState() {
	e.mu.Lock()
	s := State{
		Time:    e.simTime,
		Speed:   e.speed,
		Running: e.running,
	}
	e.mu.Unlock()
	e.callback.OnState(s)
}

// trackArbitrageDay accumulates per-day arbitrage stats.
// Called after altBat.ProcessArbitrage() in emitReadings.
func (e *Engine) trackArbitrageDay(batteryPowerW float64, ts time.Time) {
	e.mu.Lock()
	defer e.mu.Unlock()

	day := ts.Format("2006-01-02")
	hhmm := ts.Format("15:04")

	// Day boundary crossed — finalize previous day
	if e.arbitrageCurrentDay != "" && day != e.arbitrageCurrentDay {
		e.finalizeArbitrageDay()
	}

	// New day — reset accumulators and snapshot cumulative values
	if e.arbitrageCurrentDay != day {
		e.arbitrageCurrentDay = day
		e.arbitrageDayChargeWh = 0
		e.arbitrageDayDischargeWh = 0
		e.arbitrageDayChargeStart = ""
		e.arbitrageDayChargeEnd = ""
		e.arbitrageDayDischargeStart = ""
		e.arbitrageDayDischargeEnd = ""
		if e.altBattery != nil {
			e.arbitrageDayStartThroughputWh = e.altBattery.TotalThroughputWh
		}
		e.arbitrageDayStartRawNetCost = e.rawGridImportCostPLN - e.rawGridExportRevenuePLN
		e.arbitrageDayStartArbNetCost = e.arbGridImportCostPLN - e.arbGridExportRevenuePLN
	}

	// Track charge/discharge windows as non-overlapping phases:
	// charge phase first, then discharge phase (no interleaving).
	if batteryPowerW < 0 {
		// Charging — only extend charge window if discharge hasn't started
		if e.arbitrageDayDischargeStart == "" {
			if e.arbitrageDayChargeStart == "" {
				e.arbitrageDayChargeStart = hhmm
			}
			e.arbitrageDayChargeEnd = hhmm
		}
	} else if batteryPowerW > 0 {
		// Discharging
		if e.arbitrageDayDischargeStart == "" {
			e.arbitrageDayDischargeStart = hhmm
		}
		e.arbitrageDayDischargeEnd = hhmm
	}
}

// finalizeArbitrageDay builds an ArbitrageDayRecord for the completed day.
// Must be called with mu held.
func (e *Engine) finalizeArbitrageDay() {
	if e.arbitrageCurrentDay == "" || e.altBattery == nil {
		return
	}

	capacityWh := e.altBattery.config.CapacityKWh * 1000
	throughputDelta := e.altBattery.TotalThroughputWh - e.arbitrageDayStartThroughputWh
	var cyclesDelta float64
	if capacityWh > 0 {
		cyclesDelta = throughputDelta / 2 / capacityWh
	}

	rawNetCostNow := e.rawGridImportCostPLN - e.rawGridExportRevenuePLN
	arbNetCostNow := e.arbGridImportCostPLN - e.arbGridExportRevenuePLN
	rawDelta := rawNetCostNow - e.arbitrageDayStartRawNetCost
	arbDelta := arbNetCostNow - e.arbitrageDayStartArbNetCost
	earnings := rawDelta - arbDelta

	// Compute charge/discharge kWh from throughput: split by direction
	// throughputDelta is total abs energy. We can approximate using the
	// battery's charge/discharge split, but simpler: use half each if we
	// don't track separately. Actually let's compute from the time ranges.
	// Since we don't track energy per direction in the day tracker,
	// we'll use throughput/2 for each (one full cycle = charge + discharge of same energy).
	chargeKWh := throughputDelta / 2 / 1000
	dischargeKWh := throughputDelta / 2 / 1000

	// Compute gap between charge end and discharge start
	var gapMinutes int
	if e.arbitrageDayChargeEnd != "" && e.arbitrageDayDischargeStart != "" {
		chEnd, err1 := time.Parse("15:04", e.arbitrageDayChargeEnd)
		dsStart, err2 := time.Parse("15:04", e.arbitrageDayDischargeStart)
		if err1 == nil && err2 == nil {
			gapMinutes = int(dsStart.Sub(chEnd).Minutes())
		}
	}

	rec := ArbitrageDayRecord{
		Date:               e.arbitrageCurrentDay,
		ChargeStartTime:    e.arbitrageDayChargeStart,
		ChargeEndTime:      e.arbitrageDayChargeEnd,
		ChargeKWh:          chargeKWh,
		DischargeStartTime: e.arbitrageDayDischargeStart,
		DischargeEndTime:   e.arbitrageDayDischargeEnd,
		DischargeKWh:       dischargeKWh,
		GapMinutes:         gapMinutes,
		CyclesDelta:        cyclesDelta,
		EarningsPLN:        earnings,
	}

	e.arbitrageDayRecords = append(e.arbitrageDayRecords, rec)
	e.arbitrageDayLogDirty = true
}

func (e *Engine) broadcastSummary() {
	e.mu.Lock()
	pvKWh := e.pvWh / 1000
	gridExportKWh := e.gridExportWh / 1000
	gridImportKWh := e.gridImportWh / 1000

	selfConsumption := pvKWh - gridExportKWh
	if selfConsumption < 0 {
		selfConsumption = 0
	}
	homeDemand := gridImportKWh + pvKWh - gridExportKWh
	if homeDemand < 0 {
		homeDemand = 0
	}

	var batterySavings float64
	if e.battery != nil {
		batterySavings = (e.rawGridImportWh - e.gridImportWh) / 1000
		if batterySavings < 0 {
			batterySavings = 0
		}
	}

	netCost := e.gridImportCostPLN - e.gridExportRevenuePLN
	rawNetCost := e.rawGridImportCostPLN - e.rawGridExportRevenuePLN
	var batterySavingsPLN float64
	if e.battery != nil {
		batterySavingsPLN = rawNetCost - netCost
		if batterySavingsPLN < 0 {
			batterySavingsPLN = 0
		}
	}

	var arbNetCost, arbSavingsPLN float64
	if e.altBattery != nil {
		arbNetCost = e.arbGridImportCostPLN - e.arbGridExportRevenuePLN
		arbSavingsPLN = rawNetCost - arbNetCost
		if arbSavingsPLN < 0 {
			arbSavingsPLN = 0
		}
	}

	s := Summary{
		TodayKWh:           e.todayWh / 1000,
		MonthKWh:           e.monthWh / 1000,
		TotalKWh:           e.totalWh / 1000,
		GridImportKWh:      gridImportKWh,
		GridExportKWh:      gridExportKWh,
		PVProductionKWh:    pvKWh,
		HeatPumpKWh:        e.heatPumpWh / 1000,
		HeatPumpProdKWh:    e.heatPumpProdWh / 1000,
		HeatPumpCostPLN:    e.heatPumpCostPLN,
		SelfConsumptionKWh: selfConsumption,
		HomeDemandKWh:      homeDemand,
		BatterySavingsKWh:  batterySavings,

		GridImportCostPLN:       e.gridImportCostPLN,
		GridExportRevenuePLN:    e.gridExportRevenuePLN,
		NetCostPLN:              netCost,
		RawGridImportCostPLN:    e.rawGridImportCostPLN,
		RawGridExportRevenuePLN: e.rawGridExportRevenuePLN,
		RawNetCostPLN:           rawNetCost,
		BatterySavingsPLN:       batterySavingsPLN,

		ArbNetCostPLN:        arbNetCost,
		ArbBatterySavingsPLN: arbSavingsPLN,

		CheapExportKWh:    e.cheapExportWh / 1000,
		CheapExportRevPLN: e.cheapExportRevenuePLN,
		CurrentSpotPrice:  e.currentSpotPrice,

		NMNetCostPLN:    e.nmImportCostPLN,
		NMCreditBankKWh: e.nmCreditBankKWh,
		NBNetCostPLN:    e.nbImportChargedPLN - e.nbDepositUsedPLN,
		NBDepositPLN:    e.nbDepositPLN,

		PreHeatCostPLN:    e.preHeatCostPLN,
		PreHeatSavingsPLN: e.heatPumpCostPLN - e.preHeatCostPLN,
	}
	// PV array production breakdown
	if e.pvCustomEnabled && len(e.pvArrayWh) > 0 {
		for i, arr := range e.pvArrays {
			if i < len(e.pvArrayWh) {
				s.PVArrayProduction = append(s.PVArrayProduction, PVArrayProd{
					Name: arr.Name,
					KWh:  e.pvArrayWh[i] / 1000,
				})
			}
		}
	}
	bat := e.battery
	e.mu.Unlock()

	e.callback.OnSummary(s)
	if bat != nil {
		e.callback.OnBatterySummary(bat.Summary())
	}

	// Broadcast arb day log if dirty
	e.mu.Lock()
	dirty := e.arbitrageDayLogDirty
	var arbRecords []ArbitrageDayRecord
	if dirty {
		arbRecords = make([]ArbitrageDayRecord, len(e.arbitrageDayRecords))
		copy(arbRecords, e.arbitrageDayRecords)
		e.arbitrageDayLogDirty = false
	}
	e.mu.Unlock()
	if dirty {
		e.callback.OnArbitrageDayLog(arbRecords)
	}

	// Broadcast heating stats
	e.mu.Lock()
	var heatingStats []HeatingMonthStat
	if len(e.heatingMonths) > 0 {
		for _, mk := range e.heatingMonthOrder {
			acc := e.heatingMonths[mk]
			cop := 0.0
			if acc.consumptionWh > 0 {
				cop = acc.productionWh / acc.consumptionWh
			}
			avgTemp := 0.0
			if acc.tempCount > 0 {
				avgTemp = acc.tempSum / float64(acc.tempCount)
			}
			heatingStats = append(heatingStats, HeatingMonthStat{
				Month:          mk,
				ConsumptionKWh: acc.consumptionWh / 1000,
				ProductionKWh:  acc.productionWh / 1000,
				COP:            cop,
				CostPLN:        acc.costPLN,
				AvgTempC:       avgTemp,
				TempReadings:   acc.tempCount,
			})
		}
	}
	e.mu.Unlock()
	e.callback.OnHeatingStats(heatingStats)

	// Broadcast anomaly days if dirty
	e.mu.Lock()
	anomalyDirty := e.anomalyDirty
	var anomalyRecords []AnomalyDayRecord
	if anomalyDirty {
		anomalyRecords = make([]AnomalyDayRecord, len(e.anomalyDays))
		copy(anomalyRecords, e.anomalyDays)
		e.anomalyDirty = false
	}
	e.mu.Unlock()
	if anomalyDirty {
		e.callback.OnAnomalyDays(anomalyRecords)
	}

	// Broadcast load shift stats if dirty
	e.mu.Lock()
	lsDirty := e.loadShiftDirty
	var loadShiftStats LoadShiftStats
	if lsDirty {
		loadShiftStats = e.buildLoadShiftStats()
		e.loadShiftDirty = false
	}
	e.mu.Unlock()
	if lsDirty {
		e.callback.OnLoadShiftStats(loadShiftStats)
	}

	// Broadcast HP diagnostics if dirty
	e.mu.Lock()
	hpDirty := e.hpDiagDirty
	var hpDiag HPDiagnostics
	if hpDirty {
		hpDiag = HPDiagnostics{
			COP:             e.hpDiagCOP,
			CompressorSpeed: e.hpDiagCompressorSpeed,
			FanSpeed:        e.hpDiagFanSpeed,
			DischargeTemp:   e.hpDiagDischargeTemp,
			HighPressure:    e.hpDiagHighPressure,
			PumpFlow:        e.hpDiagPumpFlow,
			InletTemp:       e.hpDiagInletTemp,
			OutletTemp:      e.hpDiagOutletTemp,
			DHWTemp:         e.hpDiagDHWTemp,
			OutsidePipeTemp: e.hpDiagOutsidePipe,
			InsidePipeTemp:  e.hpDiagInsidePipe,
			Z1TargetTemp:    e.hpDiagZ1Target,
		}
		// Compute true thermal power: flow (L/min) × ΔT (°C) × 69.77 W/(L/min·°C)
		deltaT := e.hpDiagOutletTemp - e.hpDiagInletTemp
		if deltaT > 0 && e.hpDiagPumpFlow > 0 {
			hpDiag.ThermalPowerW = e.hpDiagPumpFlow * deltaT * 69.77
		}
		e.hpDiagDirty = false
	}
	e.mu.Unlock()
	if hpDirty {
		e.callback.OnHPDiagnostics(hpDiag)
	}

	// Broadcast power quality if dirty
	e.mu.Lock()
	pqDirtyFlag := e.pqDirty
	var pq PowerQuality
	if pqDirtyFlag {
		pq = PowerQuality{
			VoltageV:         e.pqVoltage,
			PowerFactorPct:   e.pqPowerFactor,
			ReactivePowerVAR: e.pqReactivePower,
		}
		e.pqDirty = false
	}
	e.mu.Unlock()
	if pqDirtyFlag {
		e.callback.OnPowerQuality(pq)
	}
}

// buildLoadShiftStats computes load shift analysis from hourly accumulators.
// Must be called with mu held.
func (e *Engine) buildLoadShiftStats() LoadShiftStats {
	const shiftWindow = 4

	var stats LoadShiftStats
	stats.ShiftWindowH = shiftWindow

	// Build heatmap
	var totalHPWh, totalHPCost float64
	for dow := 0; dow < 7; dow++ {
		for h := 0; h < 24; h++ {
			slot := e.dayOfWeekHourly[dow][h]
			kwh := slot.hpWh / 1000
			avgPrice := 0.0
			if slot.priceN > 0 {
				avgPrice = slot.priceSum / float64(slot.priceN)
			}
			stats.Heatmap[dow][h] = HeatmapCell{
				KWh:      kwh,
				AvgPrice: avgPrice,
			}
			totalHPWh += slot.hpWh
			totalHPCost += slot.hpCostPLN
		}
	}

	// Average HP price
	if totalHPWh > 0 {
		stats.AvgHPPrice = totalHPCost / (totalHPWh / 1000)
	}

	// Overall average price
	if e.overallPriceN > 0 {
		stats.OverallAvgPrice = e.overallPriceSum / float64(e.overallPriceN)
	}

	// Shift potential: for each dow+hour HP consumption,
	// find the cheapest price within ±shiftWindow hours
	stats.ShiftCurrentPLN = totalHPCost
	var optimalCost float64
	for dow := 0; dow < 7; dow++ {
		for h := 0; h < 24; h++ {
			slot := e.dayOfWeekHourly[dow][h]
			if slot.hpWh <= 0 {
				continue
			}
			kwh := slot.hpWh / 1000
			// Find cheapest price in window
			bestPrice := slot.priceSum / max(1, float64(slot.priceN))
			for dh := -shiftWindow; dh <= shiftWindow; dh++ {
				nh := (h + dh + 24) % 24
				neighborSlot := e.dayOfWeekHourly[dow][nh]
				if neighborSlot.priceN > 0 {
					neighborPrice := neighborSlot.priceSum / float64(neighborSlot.priceN)
					if neighborPrice < bestPrice {
						bestPrice = neighborPrice
					}
				}
			}
			optimalCost += kwh * bestPrice
		}
	}
	stats.ShiftOptimalPLN = optimalCost
	stats.ShiftSavingsPLN = totalHPCost - optimalCost

	return stats
}

// getOrCreateHeatingMonth returns the accumulator for the given month key.
// Must be called with mu held.
func (e *Engine) getOrCreateHeatingMonth(mk string) *heatingMonthAcc {
	acc, ok := e.heatingMonths[mk]
	if !ok {
		acc = &heatingMonthAcc{}
		e.heatingMonths[mk] = acc
		e.heatingMonthOrder = append(e.heatingMonthOrder, mk)
	}
	return acc
}

// finalizeAnomalyDay builds an AnomalyDayRecord for the completed day.
// Must be called with mu held.
func (e *Engine) finalizeAnomalyDay() {
	if e.anomalyCurrentDay == "" || e.anomalyPredictedWh == 0 {
		return
	}
	deviationPct := (e.anomalyActualWh - e.anomalyPredictedWh) / e.anomalyPredictedWh * 100
	avgTemp := 0.0
	if e.anomalyTempCount > 0 {
		avgTemp = e.anomalyTempSum / float64(e.anomalyTempCount)
	}
	e.anomalyDays = append(e.anomalyDays, AnomalyDayRecord{
		Date:         e.anomalyCurrentDay,
		ActualKWh:    e.anomalyActualWh / 1000,
		PredictedKWh: e.anomalyPredictedWh / 1000,
		DeviationPct: deviationPct,
		AvgTempC:     avgTemp,
	})
	e.anomalyDirty = true
}

// captureDiagnosticSnapshot stores HP diagnostic and power quality values.
func (e *Engine) captureDiagnosticSnapshot(r model.Reading) {
	e.mu.Lock()
	defer e.mu.Unlock()
	switch r.Type {
	case model.SensorPumpCOP:
		e.hpDiagCOP = r.Value
		e.hpDiagDirty = true
	case model.SensorPumpCompressorSpeed:
		e.hpDiagCompressorSpeed = r.Value
		e.hpDiagDirty = true
	case model.SensorPumpFanSpeed:
		e.hpDiagFanSpeed = r.Value
		e.hpDiagDirty = true
	case model.SensorPumpDischargeTemp:
		e.hpDiagDischargeTemp = r.Value
		e.hpDiagDirty = true
	case model.SensorPumpHighPressure:
		e.hpDiagHighPressure = r.Value
		e.hpDiagDirty = true
	case model.SensorPumpFlow:
		e.hpDiagPumpFlow = r.Value
		e.hpDiagDirty = true
	case model.SensorPumpInletTemp:
		e.hpDiagInletTemp = r.Value
		e.hpDiagDirty = true
	case model.SensorPumpOutletTemp:
		e.hpDiagOutletTemp = r.Value
		e.hpDiagDirty = true
	case model.SensorPumpDHWTemp:
		e.hpDiagDHWTemp = r.Value
		e.hpDiagDirty = true
	case model.SensorPumpOutsidePipe:
		e.hpDiagOutsidePipe = r.Value
		e.hpDiagDirty = true
	case model.SensorPumpInsidePipeTemp:
		e.hpDiagInsidePipe = r.Value
		e.hpDiagDirty = true
	case model.SensorPumpZ1TargetTemp:
		e.hpDiagZ1Target = r.Value
		e.hpDiagDirty = true
	case model.SensorGridVoltage:
		e.pqVoltage = r.Value
		e.pqDirty = true
	case model.SensorGridPowerFactor:
		e.pqPowerFactor = r.Value
		e.pqDirty = true
	case model.SensorGridPowerReactive:
		e.pqReactivePower = r.Value
		e.pqDirty = true
	}
}

func startOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func startOfMonth(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
}
