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
	mu        sync.Mutex
	states    []State
	readings  []SensorReading
	summaries []Summary
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
	assert.Equal(t, 604800.0, e.State().Speed)
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

func TestStartOfDay(t *testing.T) {
	ts := time.Date(2024, 11, 21, 15, 30, 45, 0, time.UTC)
	assert.Equal(t, time.Date(2024, 11, 21, 0, 0, 0, 0, time.UTC), startOfDay(ts))
}

func TestStartOfMonth(t *testing.T) {
	ts := time.Date(2024, 11, 21, 15, 30, 45, 0, time.UTC)
	assert.Equal(t, time.Date(2024, 11, 1, 0, 0, 0, 0, time.UTC), startOfMonth(ts))
}
