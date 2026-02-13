package simulator

import (
	"sort"
	"sync"
	"time"

	"energy_simulator/internal/model"
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

// Callback receives simulation events.
type Callback interface {
	OnState(state State)
	OnReading(reading SensorReading)
	OnSummary(summary Summary)
	OnBatteryUpdate(update BatteryUpdate)
	OnBatterySummary(summary BatterySummary)
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

	// Prediction mode
	predictionMode bool
	prediction     *PredictionProvider
	savedTimeRange model.TimeRange

	// Tracking for energy summaries
	lastReadings map[string]model.Reading // last reading per sensor
	dayStart     time.Time
	monthStart   time.Time
	todayWh      float64
	monthWh      float64
	totalWh      float64

	// Per-source energy tracking (Wh)
	pvWh, heatPumpWh, heatPumpProdWh float64
	gridImportWh, gridExportWh       float64
	rawGridImportWh, rawGridExportWh float64 // before battery adjustment

	// Energy cost tracking (PLN)
	priceSensorID                                string
	gridImportCostPLN, gridExportRevenuePLN      float64
	rawGridImportCostPLN, rawGridExportRevenuePLN float64

	stopCh chan struct{}
}

func New(s *store.Store, cb Callback) *Engine {
	return &Engine{
		store:        s,
		callback:     cb,
		speed:        3600,
		lastReadings: make(map[string]model.Reading),
	}
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
	if speed > 604800 {
		speed = 604800
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
	e.arbThresholdDay = time.Time{}
	e.arbLowThreshold = 0
	e.arbHighThreshold = 0
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
			e.callback.OnReading(SensorReading{
				SensorID:  r.SensorID,
				Value:     r.Value,
				Unit:      r.Unit,
				Timestamp: r.Timestamp.Format(time.RFC3339),
			})

			// When battery is active, process grid_power through battery
			e.mu.Lock()
			bat := e.battery
			altBat := e.altBattery
			priceSensor := e.priceSensorID
			e.mu.Unlock()

			if bat != nil && r.Type == model.SensorGridPower {
				e.updateRawGridEnergy(r)
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
					}
				}
			} else {
				if r.Type == model.SensorGridPower {
					e.updateRawGridEnergy(r)
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
		e.callback.OnReading(sr)

		ts, _ := time.Parse(time.RFC3339, sr.Timestamp)
		r := model.Reading{
			Timestamp: ts,
			SensorID:  sr.SensorID,
			Type:      model.SensorGridPower,
			Value:     sr.Value,
			Unit:      sr.Unit,
		}

		if bat != nil {
			e.updateRawGridEnergy(r)
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
			e.gridExportWh += -wh
			e.gridExportRevenuePLN += (-wh / 1000) * price
		}
	case model.SensorPVPower:
		if wh > 0 {
			e.pvWh += wh
		}
	case model.SensorPumpConsumption:
		if wh > 0 {
			e.heatPumpWh += wh
		}
	case model.SensorPumpProduction:
		if wh > 0 {
			e.heatPumpProdWh += wh
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
		e.rawGridExportRevenuePLN += (-wh / 1000) * price
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
		e.arbGridExportRevenuePLN += (-wh / 1000) * price
	}

	e.lastReadings[key] = r
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
	}
	bat := e.battery
	e.mu.Unlock()

	e.callback.OnSummary(s)
	if bat != nil {
		e.callback.OnBatterySummary(bat.Summary())
	}
}

func startOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func startOfMonth(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
}
