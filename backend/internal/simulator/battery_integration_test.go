package simulator

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"energy_simulator/internal/model"
	"energy_simulator/internal/store"
)

// makeMultiSensorStore creates a store with grid_power and a non-grid sensor,
// mimicking the real server with multiple CSV sources.
func makeMultiSensorStore(gridValues, otherValues []float64) *store.Store {
	s := store.New()

	s.AddSensor(model.Sensor{
		ID:   "sensor.grid",
		Name: "Grid Power",
		Type: model.SensorGridPower,
		Unit: "W",
	})
	s.AddSensor(model.Sensor{
		ID:   "sensor.pump_temp",
		Name: "Pump Temp",
		Type: model.SensorType("pump_ext_temp"),
		Unit: "",
	})

	gridReadings := make([]model.Reading, len(gridValues))
	for i, v := range gridValues {
		gridReadings[i] = model.Reading{
			Timestamp: startTime.Add(time.Duration(i) * hour),
			SensorID:  "sensor.grid",
			Type:      model.SensorGridPower,
			Value:     v,
			Unit:      "W",
		}
	}
	s.AddReadings(gridReadings)

	otherReadings := make([]model.Reading, len(otherValues))
	for i, v := range otherValues {
		otherReadings[i] = model.Reading{
			Timestamp: startTime.Add(time.Duration(i) * hour),
			SensorID:  "sensor.pump_temp",
			Type:      model.SensorType("pump_ext_temp"),
			Value:     v,
			Unit:      "",
		}
	}
	s.AddReadings(otherReadings)

	return s
}

func TestBatteryIntegration_MultiSensor(t *testing.T) {
	// Simulate real setup: grid_power + other sensor
	// Grid: export, export, consume, consume (4 readings, 3 intervals)
	gridValues := []float64{-1000, -1000, 2000, 2000}
	otherValues := []float64{5, 6, 7, 8}

	s := makeMultiSensorStore(gridValues, otherValues)
	cb := &mockCallback{}
	e := New(s, cb)
	ok := e.Init()
	require.True(t, ok)

	// Log all sensors
	for _, sensor := range e.Sensors() {
		t.Logf("Sensor: id=%s name=%s type=%s", sensor.ID, sensor.Name, string(sensor.Type))
	}

	e.SetBattery(&BatteryConfig{
		CapacityKWh:        10,
		MaxPowerW:          5000,
		DischargeToPercent: 10,
		ChargeToPercent:    100,
	})

	// Verify battery is set
	e.mu.Lock()
	hasBattery := e.battery != nil
	e.mu.Unlock()
	require.True(t, hasBattery, "battery should be set")

	// Step through all data
	e.Step(5 * hour)

	// Check readings
	allReadings := cb.allReadings()
	t.Logf("Total readings emitted: %d", len(allReadings))
	for _, r := range allReadings {
		t.Logf("  sensor=%s value=%.1f", r.SensorID, r.Value)
	}

	// Check battery updates
	updates := cb.allBatteryUpdates()
	t.Logf("Total battery updates: %d", len(updates))
	for i, u := range updates {
		t.Logf("  [%d] power=%.0fW adjusted=%.0fW SoC=%.1f%% ts=%s",
			i, u.BatteryPowerW, u.AdjustedGridW, u.SoCPercent, u.Timestamp)
	}

	// Should have exactly 4 battery updates (one per grid_power reading)
	require.Equal(t, 4, len(updates), "should have one battery update per grid_power reading")

	// [0]: first reading, no action (baseline)
	assert.InDelta(t, 0, updates[0].BatteryPowerW, 0.01)

	// [1]: prev was -1000W export → charges 1000Wh. SoC: 1000+1000=2000 (20%)
	assert.InDelta(t, -1000, updates[1].BatteryPowerW, 0.01, "should be charging")
	assert.InDelta(t, 20, updates[1].SoCPercent, 0.01, "SoC should increase")

	// [2]: prev was -1000W export → charges again. SoC: 2000+1000=3000 (30%)
	assert.InDelta(t, -1000, updates[2].BatteryPowerW, 0.01, "should still be charging")
	assert.InDelta(t, 30, updates[2].SoCPercent, 0.01)

	// [3]: prev was 2000W consume → discharges 2000Wh. SoC: 3000-2000=1000 (10%)
	assert.InDelta(t, 2000, updates[3].BatteryPowerW, 0.01, "should be discharging")
	assert.InDelta(t, 10, updates[3].SoCPercent, 0.01)
}
