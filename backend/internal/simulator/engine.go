package simulator

import (
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
}

// SensorReading is a reading emitted during simulation.
type SensorReading struct {
	SensorID  string  `json:"sensor_id"`
	Value     float64 `json:"value"`
	Unit      string  `json:"unit"`
	Timestamp string  `json:"timestamp"`
}

// Callback receives simulation events.
type Callback interface {
	OnState(state State)
	OnReading(reading SensorReading)
	OnSummary(summary Summary)
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

	// Tracking for energy summaries
	lastReadings map[string]model.Reading // last reading per sensor
	dayStart     time.Time
	monthStart   time.Time
	todayWh      float64
	monthWh      float64
	totalWh      float64

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

// Seek jumps to a specific time. Resets energy summaries.
func (e *Engine) Seek(t time.Time) {
	e.mu.Lock()
	if t.Before(e.timeRange.Start) {
		t = e.timeRange.Start
	}
	if t.After(e.timeRange.End) {
		t = e.timeRange.End
	}

	e.simTime = t
	e.dayStart = startOfDay(t)
	e.monthStart = startOfMonth(t)
	e.todayWh = 0
	e.monthWh = 0
	e.totalWh = 0
	e.lastReadings = make(map[string]model.Reading)
	e.mu.Unlock()

	e.broadcastState()
	e.broadcastSummary()
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
		close(e.stopCh)
		e.mu.Unlock()
		e.broadcastState()
		return true
	}

	return false
}

func (e *Engine) emitReadings(prevTime, currentTime, endTime time.Time) {
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

	if r.Type != model.SensorGridPower {
		e.lastReadings[r.SensorID] = r
		return
	}

	// Energy = average power * time interval (in hours) -> Wh
	hours := r.Timestamp.Sub(last.Timestamp).Hours()
	avgPower := (last.Value + r.Value) / 2
	wh := avgPower * hours

	// Only count positive energy (consumption). Negative = export.
	if wh > 0 {
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
	}

	e.lastReadings[r.SensorID] = r
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
	s := Summary{
		TodayKWh: e.todayWh / 1000,
		MonthKWh: e.monthWh / 1000,
		TotalKWh: e.totalWh / 1000,
	}
	e.mu.Unlock()
	e.callback.OnSummary(s)
}

func startOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func startOfMonth(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
}
