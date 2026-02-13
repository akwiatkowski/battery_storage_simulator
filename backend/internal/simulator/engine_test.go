package simulator

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"energy_simulator/internal/model"
	"energy_simulator/internal/store"
)

type mockCallback struct {
	mu               sync.Mutex
	states           []State
	readings         []SensorReading
	summaries        []Summary
	batteryUpdates   []BatteryUpdate
	batterySummaries []BatterySummary
	arbitrageDayLogs [][]ArbitrageDayRecord
}

func (m *mockCallback) OnState(s State) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.states = append(m.states, s)
}

func (m *mockCallback) OnReading(r SensorReading) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.readings = append(m.readings, r)
}

func (m *mockCallback) OnSummary(s Summary) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.summaries = append(m.summaries, s)
}

func (m *mockCallback) OnBatteryUpdate(u BatteryUpdate) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.batteryUpdates = append(m.batteryUpdates, u)
}

func (m *mockCallback) OnBatterySummary(s BatterySummary) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.batterySummaries = append(m.batterySummaries, s)
}

func (m *mockCallback) OnArbitrageDayLog(records []ArbitrageDayRecord) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.arbitrageDayLogs = append(m.arbitrageDayLogs, records)
}

func TestSummary_OffGridCoverage(t *testing.T) {
	s := Summary{
		HomeDemandKWh:      1000,
		HeatPumpKWh:        400,
		SelfConsumptionKWh: 300,
		BatterySavingsKWh:  200,
	}

	// Full usage: non-grid=500, demand=1000 → 50%
	assert.InDelta(t, 50.0, s.OffGridCoverage(100, 100), 0.1)

	// No heat pump: demand=600 (appliances only), non-grid=500 → 83.3%
	assert.InDelta(t, 83.3, s.OffGridCoverage(0, 100), 0.1)

	// Half everything: demand=500, non-grid=500 → 100%
	assert.InDelta(t, 100.0, s.OffGridCoverage(50, 50), 0.1)

	// Zero demand → 100%
	assert.InDelta(t, 100.0, s.OffGridCoverage(0, 0), 0.1)

	// No battery savings, no self-consumption → 0%
	empty := Summary{HomeDemandKWh: 500, HeatPumpKWh: 100}
	assert.InDelta(t, 0.0, empty.OffGridCoverage(100, 100), 0.1)
}

func (m *mockCallback) readingCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.readings)
}

func (m *mockCallback) allReadings() []SensorReading {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]SensorReading, len(m.readings))
	copy(cp, m.readings)
	return cp
}

func (m *mockCallback) lastSummary() Summary {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.summaries) == 0 {
		return Summary{}
	}
	return m.summaries[len(m.summaries)-1]
}

var (
	startTime = time.Date(2024, 11, 21, 12, 0, 0, 0, time.UTC)
	hour      = time.Hour
)

func makeStore(values []float64) *store.Store {
	s := store.New()
	s.AddSensor(model.Sensor{
		ID:   "sensor.grid",
		Name: "Grid Power",
		Type: model.SensorGridPower,
		Unit: "W",
	})

	readings := make([]model.Reading, len(values))
	for i, v := range values {
		readings[i] = model.Reading{
			Timestamp: startTime.Add(time.Duration(i) * hour),
			SensorID:  "sensor.grid",
			Type:      model.SensorGridPower,
			Value:     v,
			Unit:      "W",
		}
	}
	s.AddReadings(readings)
	return s
}

func TestEngine_Init(t *testing.T) {
	s := makeStore([]float64{100, 200, 300})
	cb := &mockCallback{}
	e := New(s, cb)

	ok := e.Init()
	require.True(t, ok)

	state := e.State()
	assert.Equal(t, startTime, state.Time)
	assert.Equal(t, 3600.0, state.Speed)
	assert.False(t, state.Running)
}

func TestEngine_InitEmpty(t *testing.T) {
	s := store.New()
	cb := &mockCallback{}
	e := New(s, cb)

	ok := e.Init()
	assert.False(t, ok)
}

func TestEngine_StartPause(t *testing.T) {
	s := makeStore([]float64{100, 200, 300})
	cb := &mockCallback{}
	e := New(s, cb)
	e.Init()

	e.Start()
	assert.True(t, e.State().Running)

	time.Sleep(50 * time.Millisecond)
	e.Pause()
	assert.False(t, e.State().Running)
}

func TestEngine_SetSpeed(t *testing.T) {
	s := makeStore([]float64{100, 200, 300})
	cb := &mockCallback{}
	e := New(s, cb)
	e.Init()

	e.SetSpeed(10)
	assert.Equal(t, 10.0, e.State().Speed)

	e.SetSpeed(0.01)
	assert.Equal(t, 0.1, e.State().Speed)

	e.SetSpeed(1000000)
	assert.Equal(t, 1000000.0, e.State().Speed)

	e.SetSpeed(5000000)
	assert.Equal(t, 2592000.0, e.State().Speed)
}

func TestEngine_Seek(t *testing.T) {
	s := makeStore([]float64{100, 200, 300, 400, 500})
	cb := &mockCallback{}
	e := New(s, cb)
	e.Init()

	target := startTime.Add(2 * hour)
	e.Seek(target)
	assert.Equal(t, target, e.State().Time)

	e.Seek(startTime.Add(-10 * hour))
	assert.Equal(t, startTime, e.State().Time)

	e.Seek(startTime.Add(100 * hour))
	assert.Equal(t, startTime.Add(4*hour), e.State().Time)
}

func TestEngine_Step_EmitsReadings(t *testing.T) {
	// 5 readings, 1 hour apart
	s := makeStore([]float64{100, 200, 300, 400, 500})
	cb := &mockCallback{}
	e := New(s, cb)
	e.Init()

	// Step 90 minutes: should emit readings at 12:00 and 13:00
	e.Step(90 * time.Minute)
	assert.Equal(t, 2, cb.readingCount())

	readings := cb.allReadings()
	assert.InDelta(t, 100.0, readings[0].Value, 0.001)
	assert.InDelta(t, 200.0, readings[1].Value, 0.001)
}

func TestEngine_Step_ToEnd(t *testing.T) {
	s := makeStore([]float64{100, 200, 300})
	cb := &mockCallback{}
	e := New(s, cb)
	e.Init()

	// Step past the end
	e.Step(10 * hour)

	// All 3 readings emitted (inclusive of end boundary)
	assert.Equal(t, 3, cb.readingCount())
	// simTime should be clamped to end
	assert.Equal(t, startTime.Add(2*hour), e.State().Time)
}

func TestEngine_EnergySummary(t *testing.T) {
	// 2 readings, 1 hour apart, both 1000W -> 1000 Wh = 1 kWh
	s := makeStore([]float64{1000, 1000})
	cb := &mockCallback{}
	e := New(s, cb)
	e.Init()

	// Step past all data
	e.Step(2 * hour)

	summary := cb.lastSummary()
	assert.InDelta(t, 1.0, summary.TotalKWh, 0.01)
	assert.InDelta(t, 1.0, summary.TodayKWh, 0.01)
	assert.InDelta(t, 1.0, summary.MonthKWh, 0.01)
}

func TestEngine_EnergySummary_MultipleSteps(t *testing.T) {
	// 4 readings at 500W each, 1 hour apart -> 3 intervals * 500W * 1h = 1500 Wh = 1.5 kWh
	s := makeStore([]float64{500, 500, 500, 500})
	cb := &mockCallback{}
	e := New(s, cb)
	e.Init()

	e.Step(90 * time.Minute)  // covers 12:00, 13:00
	e.Step(90 * time.Minute)  // covers 14:00
	e.Step(90 * time.Minute)  // past end, covers 15:00 but clamped

	summary := cb.lastSummary()
	assert.InDelta(t, 1.5, summary.TotalKWh, 0.01)
}

func TestEngine_SeekResetsEnergy(t *testing.T) {
	s := makeStore([]float64{1000, 1000, 1000, 1000})
	cb := &mockCallback{}
	e := New(s, cb)
	e.Init()

	e.Step(2 * hour)
	assert.InDelta(t, 1.0, cb.lastSummary().TotalKWh, 0.01)

	// Seek resets energy
	e.Seek(startTime)
	assert.InDelta(t, 0.0, cb.lastSummary().TotalKWh, 0.01)
}

func TestEngine_TimeRange(t *testing.T) {
	s := makeStore([]float64{100, 200, 300})
	cb := &mockCallback{}
	e := New(s, cb)
	e.Init()

	tr := e.TimeRange()
	assert.Equal(t, startTime, tr.Start)
	assert.Equal(t, startTime.Add(2*hour), tr.End)
}

func TestEngine_SetTimeRange(t *testing.T) {
	s := makeStore([]float64{100, 200, 300, 400, 500})
	cb := &mockCallback{}
	e := New(s, cb)
	e.Init()

	// Step to accumulate some energy
	e.Step(2 * hour)
	assert.Greater(t, cb.lastSummary().TotalKWh, 0.0)

	// Set a new time range — should reset energy and seek to new start
	newStart := startTime.Add(2 * hour)
	newEnd := startTime.Add(4 * hour)
	newRange := model.TimeRange{Start: newStart, End: newEnd}
	e.SetTimeRange(newRange)

	tr := e.TimeRange()
	assert.Equal(t, newStart, tr.Start)
	assert.Equal(t, newEnd, tr.End)
	assert.Equal(t, newStart, e.State().Time)
	// Energy should be reset
	assert.InDelta(t, 0.0, cb.lastSummary().TotalKWh, 0.001)
}

func TestEngine_SetTimeRange_ResetsAndReplays(t *testing.T) {
	s := makeStore([]float64{100, 200, 300, 400, 500})
	cb := &mockCallback{}
	e := New(s, cb)
	e.Init()

	// Narrow the time range
	newRange := model.TimeRange{
		Start: startTime.Add(1 * hour),
		End:   startTime.Add(3 * hour),
	}
	e.SetTimeRange(newRange)

	// Step through new range
	e.Step(3 * hour)

	// Should have readings from the narrowed range (200, 300, 400)
	readings := cb.allReadings()
	found := false
	for _, r := range readings {
		if r.Value == 300.0 {
			found = true
		}
	}
	assert.True(t, found, "should find readings within new time range")
	assert.Equal(t, newRange.End, e.State().Time)
}

func TestStartOfDay(t *testing.T) {
	ts := time.Date(2024, 11, 21, 15, 30, 45, 0, time.UTC)
	assert.Equal(t, time.Date(2024, 11, 21, 0, 0, 0, 0, time.UTC), startOfDay(ts))
}

func TestStartOfMonth(t *testing.T) {
	ts := time.Date(2024, 11, 21, 15, 30, 45, 0, time.UTC)
	assert.Equal(t, time.Date(2024, 11, 1, 0, 0, 0, 0, time.UTC), startOfMonth(ts))
}

func (m *mockCallback) lastBatteryUpdate() BatteryUpdate {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.batteryUpdates) == 0 {
		return BatteryUpdate{}
	}
	return m.batteryUpdates[len(m.batteryUpdates)-1]
}

func (m *mockCallback) lastBatterySummary() BatterySummary {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.batterySummaries) == 0 {
		return BatterySummary{}
	}
	return m.batterySummaries[len(m.batterySummaries)-1]
}

func TestEngine_BatteryReducesGrid(t *testing.T) {
	// 3 readings at 2000W consumption, 1 hour apart
	s := makeStore([]float64{2000, 2000, 2000})
	cb := &mockCallback{}
	e := New(s, cb)
	e.Init()

	e.SetBattery(&BatteryConfig{
		CapacityKWh:        10,
		MaxPowerW:          5000,
		DischargeToPercent: 0,
		ChargeToPercent:    100,
	})
	// Pre-charge: set battery SoC high so it can discharge
	e.mu.Lock()
	e.battery.SoCWh = 5000
	e.mu.Unlock()

	e.Step(3 * hour)

	// Battery should have produced updates
	bu := cb.lastBatteryUpdate()
	assert.InDelta(t, 2000, bu.BatteryPowerW, 0.01) // discharging at 2000W
	assert.InDelta(t, 0, bu.AdjustedGridW, 0.01)    // grid fully offset

	// With backward-looking: first reading no action, readings 2+3 fully offset.
	// Adjusted values: [2000, 0, 0]. Intervals: avg(2000,0)*1h=1000Wh, avg(0,0)*1h=0.
	summary := cb.lastSummary()
	assert.InDelta(t, 1.0, summary.TotalKWh, 0.01)
}

func TestEngine_BatteryAbsorbsExport(t *testing.T) {
	// 3 readings at -1500W (export), 1 hour apart
	s := makeStore([]float64{-1500, -1500, -1500})
	cb := &mockCallback{}
	e := New(s, cb)
	e.Init()

	e.SetBattery(&BatteryConfig{
		CapacityKWh:        10,
		MaxPowerW:          5000,
		DischargeToPercent: 0,
		ChargeToPercent:    100,
	})

	e.Step(3 * hour)

	// Battery should be charging
	bu := cb.lastBatteryUpdate()
	assert.Less(t, bu.BatteryPowerW, 0.0)     // negative = charging
	assert.Greater(t, bu.AdjustedGridW, -1500.0) // less export to grid
}

func TestEngine_SeekResetsBattery(t *testing.T) {
	s := makeStore([]float64{1000, 1000, 1000, 1000})
	cb := &mockCallback{}
	e := New(s, cb)
	e.Init()

	cfg := &BatteryConfig{
		CapacityKWh:        10,
		MaxPowerW:          5000,
		DischargeToPercent: 10,
		ChargeToPercent:    100,
	}
	e.SetBattery(cfg)
	e.mu.Lock()
	e.battery.SoCWh = 5000
	e.mu.Unlock()

	e.Step(2 * hour)

	// Seek should reset battery
	e.Seek(startTime)

	e.mu.Lock()
	soc := e.battery.SoCWh
	e.mu.Unlock()

	// Should be back to discharge floor: 10% of 10kWh = 1000 Wh
	assert.InDelta(t, 1000, soc, 0.01)
}

func TestEngine_BatteryChargesAcrossSteps(t *testing.T) {
	// Simulate incremental steps like the real tick loop.
	// 5 readings: export, export, export, consume, consume
	// Backward-looking: interval action uses previous reading's demand.
	//   [0] -1000W → baseline
	//   [1] -1000W → prev=-1000 → charge 1000Wh. SoC: 1000+1000=2000 (20%)
	//   [2] -1000W → prev=-1000 → charge 1000Wh. SoC: 2000+1000=3000 (30%)
	//   [3]  2000W → prev=-1000 → charge 1000Wh. SoC: 3000+1000=4000 (40%)
	//   [4]  2000W → prev=2000  → discharge 2000Wh. SoC: 4000-2000=2000 (20%)
	s := makeStore([]float64{-1000, -1000, -1000, 2000, 2000})
	cb := &mockCallback{}
	e := New(s, cb)
	e.Init()

	e.SetBattery(&BatteryConfig{
		CapacityKWh:        10,
		MaxPowerW:          5000,
		DischargeToPercent: 10,
		ChargeToPercent:    100,
	})

	for i := 0; i < 8; i++ {
		e.Step(30 * time.Minute)
	}

	updates := cb.allBatteryUpdates()
	t.Logf("Total battery updates: %d", len(updates))
	for i, u := range updates {
		t.Logf("  [%d] power=%.0fW adjusted=%.0fW SoC=%.1f%% ts=%s",
			i, u.BatteryPowerW, u.AdjustedGridW, u.SoCPercent, u.Timestamp)
	}

	require.Equal(t, 5, len(updates))

	// [2]: after 3 charges of 1000Wh. SoC: 1000+1000+1000=3000 (30%)
	assert.InDelta(t, 30, updates[2].SoCPercent, 0.01)

	// [3]: prev was still export → charges. SoC: 4000 (40%)
	assert.InDelta(t, -1000, updates[3].BatteryPowerW, 0.01)
	assert.InDelta(t, 40, updates[3].SoCPercent, 0.01)

	// [4]: prev was consume 2000W → discharges. SoC: 4000-2000=2000 (20%)
	assert.InDelta(t, 2000, updates[4].BatteryPowerW, 0.01)
	assert.InDelta(t, 20, updates[4].SoCPercent, 0.01)

	bs := cb.lastBatterySummary()
	assert.Greater(t, bs.Cycles, 0.0)
	assert.NotEmpty(t, bs.TimeAtPowerSec)
}

func TestEngine_NoBatteryNoUpdates(t *testing.T) {
	s := makeStore([]float64{1000, 1000})
	cb := &mockCallback{}
	e := New(s, cb)
	e.Init()

	e.Step(2 * hour)

	cb.mu.Lock()
	defer cb.mu.Unlock()
	assert.Empty(t, cb.batteryUpdates)
	assert.Empty(t, cb.batterySummaries)
}

func (m *mockCallback) allBatteryUpdates() []BatteryUpdate {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]BatteryUpdate, len(m.batteryUpdates))
	copy(cp, m.batteryUpdates)
	return cp
}

func makeEnergyStore() *store.Store {
	s := store.New()
	s.AddSensor(model.Sensor{ID: "sensor.grid", Name: "Grid Power", Type: model.SensorGridPower, Unit: "W"})
	s.AddSensor(model.Sensor{ID: "sensor.pv", Name: "PV Power", Type: model.SensorPVPower, Unit: "W"})
	s.AddSensor(model.Sensor{ID: "sensor.pump", Name: "Heat Pump", Type: model.SensorPumpConsumption, Unit: "W"})

	// Grid: 2 readings at 500W, 1h apart
	s.AddReadings([]model.Reading{
		{Timestamp: startTime, SensorID: "sensor.grid", Type: model.SensorGridPower, Value: 500, Unit: "W"},
		{Timestamp: startTime.Add(hour), SensorID: "sensor.grid", Type: model.SensorGridPower, Value: 500, Unit: "W"},
	})
	// PV: 2 readings at 1000W
	s.AddReadings([]model.Reading{
		{Timestamp: startTime, SensorID: "sensor.pv", Type: model.SensorPVPower, Value: 1000, Unit: "W"},
		{Timestamp: startTime.Add(hour), SensorID: "sensor.pv", Type: model.SensorPVPower, Value: 1000, Unit: "W"},
	})
	// Heat pump: 2 readings at 300W
	s.AddReadings([]model.Reading{
		{Timestamp: startTime, SensorID: "sensor.pump", Type: model.SensorPumpConsumption, Value: 300, Unit: "W"},
		{Timestamp: startTime.Add(hour), SensorID: "sensor.pump", Type: model.SensorPumpConsumption, Value: 300, Unit: "W"},
	})
	return s
}

func TestEngine_PVAccumulation(t *testing.T) {
	s := makeEnergyStore()
	cb := &mockCallback{}
	e := New(s, cb)
	e.Init()

	e.Step(2 * hour)

	summary := cb.lastSummary()
	// PV: 1000W * 1h = 1000 Wh = 1.0 kWh
	assert.InDelta(t, 1.0, summary.PVProductionKWh, 0.01)
}

func TestEngine_HeatPumpAccumulation(t *testing.T) {
	s := makeEnergyStore()
	cb := &mockCallback{}
	e := New(s, cb)
	e.Init()

	e.Step(2 * hour)

	summary := cb.lastSummary()
	// Heat pump: 300W * 1h = 300 Wh = 0.3 kWh
	assert.InDelta(t, 0.3, summary.HeatPumpKWh, 0.01)
}

func TestEngine_GridImportExportSplit(t *testing.T) {
	// 3 readings: +1000, -500, -500
	s := store.New()
	s.AddSensor(model.Sensor{ID: "sensor.grid", Name: "Grid Power", Type: model.SensorGridPower, Unit: "W"})
	s.AddReadings([]model.Reading{
		{Timestamp: startTime, SensorID: "sensor.grid", Type: model.SensorGridPower, Value: 1000, Unit: "W"},
		{Timestamp: startTime.Add(hour), SensorID: "sensor.grid", Type: model.SensorGridPower, Value: -500, Unit: "W"},
		{Timestamp: startTime.Add(2 * hour), SensorID: "sensor.grid", Type: model.SensorGridPower, Value: -500, Unit: "W"},
	})
	cb := &mockCallback{}
	e := New(s, cb)
	e.Init()

	e.Step(3 * hour)

	summary := cb.lastSummary()
	// Interval 1: avg(1000,-500)*1h = 250Wh (positive → import)
	// Interval 2: avg(-500,-500)*1h = -500Wh (negative → export 500Wh)
	assert.InDelta(t, 0.25, summary.GridImportKWh, 0.01)
	assert.InDelta(t, 0.5, summary.GridExportKWh, 0.01)
}

func TestEngine_ArbitrageSavings(t *testing.T) {
	// Create 48 hours of grid power (constant 1000W import) + 48 hourly prices
	// Prices: hours 0-7 cheap (0.20), hours 8-23 expensive (0.80)
	s := store.New()
	s.AddSensor(model.Sensor{ID: "sensor.grid", Name: "Grid Power", Type: model.SensorGridPower, Unit: "W"})
	s.AddSensor(model.Sensor{ID: "sensor.price", Name: "Price", Type: model.SensorEnergyPrice, Unit: "PLN/kWh"})

	base := time.Date(2024, 11, 21, 0, 0, 0, 0, time.UTC)
	var gridReadings, priceReadings []model.Reading
	for h := 0; h < 48; h++ {
		ts := base.Add(time.Duration(h) * hour)
		gridReadings = append(gridReadings, model.Reading{
			Timestamp: ts, SensorID: "sensor.grid", Type: model.SensorGridPower, Value: 1000, Unit: "W",
		})
		price := 0.80
		if h%24 < 8 {
			price = 0.20
		}
		priceReadings = append(priceReadings, model.Reading{
			Timestamp: ts, SensorID: "sensor.price", Type: model.SensorEnergyPrice, Value: price, Unit: "PLN/kWh",
		})
	}
	s.AddReadings(gridReadings)
	s.AddReadings(priceReadings)

	cb := &mockCallback{}
	e := New(s, cb)
	e.Init()
	e.SetPriceSensor("sensor.price")
	e.SetBattery(&BatteryConfig{
		CapacityKWh:        10,
		MaxPowerW:          5000,
		DischargeToPercent: 10,
		ChargeToPercent:    100,
	})

	e.Step(48 * hour)

	summary := cb.lastSummary()
	assert.Greater(t, summary.ArbNetCostPLN, 0.0, "arb net cost should be tracked")
	assert.Greater(t, summary.ArbBatterySavingsPLN, 0.0, "arb should produce savings")
	assert.Less(t, summary.ArbNetCostPLN, summary.RawNetCostPLN, "arb should cost less than raw")
}

func TestEngine_BatterySavings(t *testing.T) {
	// 3 readings at 2000W consumption, battery fully offsets
	s := makeStore([]float64{2000, 2000, 2000})
	cb := &mockCallback{}
	e := New(s, cb)
	e.Init()

	e.SetBattery(&BatteryConfig{
		CapacityKWh:        10,
		MaxPowerW:          5000,
		DischargeToPercent: 0,
		ChargeToPercent:    100,
	})
	e.mu.Lock()
	e.battery.SoCWh = 5000
	e.mu.Unlock()

	e.Step(3 * hour)

	summary := cb.lastSummary()
	// Raw grid import: 2000W * 2h = 4000 Wh = 4.0 kWh
	// With battery, adjusted grid = 0 for intervals 2+3 (backward-looking)
	// Battery savings = rawImport - adjustedImport > 0
	assert.Greater(t, summary.BatterySavingsKWh, 0.0)
}

func (m *mockCallback) lastArbitrageDayLog() []ArbitrageDayRecord {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.arbitrageDayLogs) == 0 {
		return nil
	}
	return m.arbitrageDayLogs[len(m.arbitrageDayLogs)-1]
}

func TestEngine_ArbitrageDayLog(t *testing.T) {
	// 48 hours across 2 days with price data.
	// Prices: hours 0-7 cheap (0.20), hours 8-23 expensive (0.80)
	// Battery should charge during cheap hours and discharge during expensive hours.
	s := store.New()
	s.AddSensor(model.Sensor{ID: "sensor.grid", Name: "Grid Power", Type: model.SensorGridPower, Unit: "W"})
	s.AddSensor(model.Sensor{ID: "sensor.price", Name: "Price", Type: model.SensorEnergyPrice, Unit: "PLN/kWh"})

	base := time.Date(2024, 11, 21, 0, 0, 0, 0, time.UTC)
	var gridReadings, priceReadings []model.Reading
	for h := 0; h < 49; h++ { // 49 readings = 48 intervals
		ts := base.Add(time.Duration(h) * hour)
		gridReadings = append(gridReadings, model.Reading{
			Timestamp: ts, SensorID: "sensor.grid", Type: model.SensorGridPower, Value: 1000, Unit: "W",
		})
		price := 0.80
		if h%24 < 8 {
			price = 0.20
		}
		priceReadings = append(priceReadings, model.Reading{
			Timestamp: ts, SensorID: "sensor.price", Type: model.SensorEnergyPrice, Value: price, Unit: "PLN/kWh",
		})
	}
	s.AddReadings(gridReadings)
	s.AddReadings(priceReadings)

	cb := &mockCallback{}
	e := New(s, cb)
	e.Init()
	e.SetPriceSensor("sensor.price")
	e.SetBattery(&BatteryConfig{
		CapacityKWh:        10,
		MaxPowerW:          5000,
		DischargeToPercent: 10,
		ChargeToPercent:    100,
	})

	// Step through all 48 hours
	e.Step(49 * hour)

	records := cb.lastArbitrageDayLog()
	require.NotNil(t, records, "should have arbitrage day log records")
	// Day boundary at hour 24 finalizes day 1; day 2 may not be finalized yet
	// since finalizeArbitrageDay is called on day boundary crossing.
	require.GreaterOrEqual(t, len(records), 1, "should have at least 1 completed day")

	rec := records[0]
	assert.Equal(t, "2024-11-21", rec.Date)
	assert.NotEmpty(t, rec.ChargeStartTime, "should have charge start time")
	assert.NotEmpty(t, rec.DischargeStartTime, "should have discharge start time")
	assert.Greater(t, rec.CyclesDelta, 0.0, "should have cycles")
	assert.Greater(t, rec.EarningsPLN, 0.0, "should have positive earnings from arbitrage")

	t.Logf("Day record: date=%s charge=%s-%s discharge=%s-%s cycles=%.2f earnings=%.2f PLN",
		rec.Date, rec.ChargeStartTime, rec.ChargeEndTime,
		rec.DischargeStartTime, rec.DischargeEndTime,
		rec.CyclesDelta, rec.EarningsPLN)
}

func TestEngine_ArbitrageDayLog_NonOverlappingWindowsAndGap(t *testing.T) {
	// 48 hours across 2 days with price data that could cause interleaving:
	// Hours 0-5: cheap (0.10) → charge
	// Hours 6-7: mid (0.40) → hold
	// Hours 8-15: expensive (0.90) → discharge
	// Hours 16-19: mid (0.40) → hold
	// Hours 20-23: cheap again (0.10) → would charge, but must not extend charge window
	//
	// The charge window should be [00:00, 05:00] and discharge [08:00, 15:00].
	// Late-night cheap hours (20-23) must NOT create a second charge window or
	// extend the existing one, because discharge has already started.
	// Gap = 08:00 - 05:00 = 180 minutes (3h).
	s := store.New()
	s.AddSensor(model.Sensor{ID: "sensor.grid", Name: "Grid Power", Type: model.SensorGridPower, Unit: "W"})
	s.AddSensor(model.Sensor{ID: "sensor.price", Name: "Price", Type: model.SensorEnergyPrice, Unit: "PLN/kWh"})

	base := time.Date(2024, 11, 21, 0, 0, 0, 0, time.UTC)
	var gridReadings, priceReadings []model.Reading
	for h := 0; h < 49; h++ {
		ts := base.Add(time.Duration(h) * hour)
		gridReadings = append(gridReadings, model.Reading{
			Timestamp: ts, SensorID: "sensor.grid", Type: model.SensorGridPower, Value: 1000, Unit: "W",
		})
		hod := h % 24
		var price float64
		switch {
		case hod < 6:
			price = 0.10 // cheap → charge
		case hod < 8:
			price = 0.40 // mid → hold
		case hod < 16:
			price = 0.90 // expensive → discharge
		case hod < 20:
			price = 0.40 // mid → hold
		default:
			price = 0.10 // cheap again → would charge if allowed
		}
		priceReadings = append(priceReadings, model.Reading{
			Timestamp: ts, SensorID: "sensor.price", Type: model.SensorEnergyPrice, Value: price, Unit: "PLN/kWh",
		})
	}
	s.AddReadings(gridReadings)
	s.AddReadings(priceReadings)

	cb := &mockCallback{}
	e := New(s, cb)
	e.Init()
	e.SetPriceSensor("sensor.price")
	e.SetBattery(&BatteryConfig{
		CapacityKWh:        10,
		MaxPowerW:          5000,
		DischargeToPercent: 10,
		ChargeToPercent:    100,
	})

	e.Step(49 * hour)

	records := cb.lastArbitrageDayLog()
	require.NotNil(t, records, "should have arbitrage day log records")
	require.GreaterOrEqual(t, len(records), 1)

	rec := records[0]
	assert.Equal(t, "2024-11-21", rec.Date)

	// Charge window must end before discharge window starts (non-overlapping)
	assert.NotEmpty(t, rec.ChargeStartTime)
	assert.NotEmpty(t, rec.ChargeEndTime)
	assert.NotEmpty(t, rec.DischargeStartTime)
	assert.NotEmpty(t, rec.DischargeEndTime)
	assert.Less(t, rec.ChargeEndTime, rec.DischargeStartTime,
		"charge window must end before discharge window starts")

	// Gap should be > 0 when both windows exist
	assert.Greater(t, rec.GapMinutes, 0, "gap between charge end and discharge start should be positive")

	t.Logf("Day record: date=%s charge=%s-%s discharge=%s-%s gap=%dm cycles=%.2f earnings=%.2f PLN",
		rec.Date, rec.ChargeStartTime, rec.ChargeEndTime,
		rec.DischargeStartTime, rec.DischargeEndTime,
		rec.GapMinutes, rec.CyclesDelta, rec.EarningsPLN)
}

func TestEngine_BatteryFullSimulation(t *testing.T) {
	// Simulate realistic pattern: export (charges battery), then consumption (discharges).
	// Backward-looking: battery action for interval [i-1, i] uses reading[i-1]'s demand.
	//
	// 7 readings, 1 hour apart:
	//   [0] -2000W  → first reading, baseline only
	//   [1] -2000W  → interval uses prev=-2000W → charges 2000Wh
	//   [2] -2000W  → interval uses prev=-2000W → charges 2000Wh
	//   [3]  1000W  → interval uses prev=-2000W → charges 2000Wh (prev was still export!)
	//   [4]  1000W  → interval uses prev=1000W  → discharges 1000Wh
	//   [5]  1000W  → interval uses prev=1000W  → discharges 1000Wh
	//   [6]  1000W  → interval uses prev=1000W  → discharges 1000Wh
	s := makeStore([]float64{-2000, -2000, -2000, 1000, 1000, 1000, 1000})
	cb := &mockCallback{}
	e := New(s, cb)
	e.Init()

	e.SetBattery(&BatteryConfig{
		CapacityKWh:        10,
		MaxPowerW:          5000,
		DischargeToPercent: 0,
		ChargeToPercent:    100,
	})

	e.Step(7 * hour)

	updates := cb.allBatteryUpdates()
	require.Equal(t, 7, len(updates), "should have one battery update per reading")

	for i, u := range updates {
		t.Logf("  Update[%d]: power=%.0fW adjusted=%.0fW SoC=%.1f%%", i, u.BatteryPowerW, u.AdjustedGridW, u.SoCPercent)
	}

	// [0]: first reading, no dt
	assert.InDelta(t, 0, updates[0].BatteryPowerW, 0.01)
	assert.InDelta(t, 0, updates[0].SoCPercent, 0.01)

	// [1]: charges using prev=-2000W. SoC: 0+2000=2000
	assert.InDelta(t, -2000, updates[1].BatteryPowerW, 0.01)
	assert.InDelta(t, 20, updates[1].SoCPercent, 0.01)

	// [2]: charges. SoC: 2000+2000=4000
	assert.InDelta(t, -2000, updates[2].BatteryPowerW, 0.01)
	assert.InDelta(t, 40, updates[2].SoCPercent, 0.01)

	// [3]: prev was still -2000W (export), so charges. SoC: 4000+2000=6000
	assert.InDelta(t, -2000, updates[3].BatteryPowerW, 0.01)
	assert.InDelta(t, 60, updates[3].SoCPercent, 0.01)

	// [4]: prev=1000W (consume), discharges. SoC: 6000-1000=5000
	assert.InDelta(t, 1000, updates[4].BatteryPowerW, 0.01)
	assert.InDelta(t, 50, updates[4].SoCPercent, 0.01)

	// [5]: discharge. SoC: 5000-1000=4000
	assert.InDelta(t, 40, updates[5].SoCPercent, 0.01)

	// [6]: discharge. SoC: 4000-1000=3000
	assert.InDelta(t, 30, updates[6].SoCPercent, 0.01)

	bs := cb.lastBatterySummary()
	assert.Greater(t, bs.Cycles, 0.0)

	summary := cb.lastSummary()
	t.Logf("Energy: total=%.3f kWh, Battery: SoC=%.1f%% Cycles=%.3f", summary.TotalKWh, bs.SoCPercent, bs.Cycles)
}
