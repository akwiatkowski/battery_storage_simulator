package store

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"energy_simulator/internal/model"
)

func makeReadings(sensorID string, values []float64, startTime time.Time, interval time.Duration) []model.Reading {
	readings := make([]model.Reading, len(values))
	for i, v := range values {
		readings[i] = model.Reading{
			Timestamp: startTime.Add(time.Duration(i) * interval),
			SensorID:  sensorID,
			Type:      model.SensorGridPower,
			Value:     v,
			Unit:      "W",
		}
	}
	return readings
}

var (
	sensorID  = "sensor.grid"
	startTime = time.Date(2024, 11, 21, 12, 0, 0, 0, time.UTC)
	hour      = time.Hour
)

func TestStore_AddAndQuery(t *testing.T) {
	s := New()
	readings := makeReadings(sensorID, []float64{100, 200, 300, 400, 500}, startTime, hour)
	s.AddReadings(readings)

	assert.Equal(t, 5, s.ReadingCount(sensorID))
	assert.Equal(t, 0, s.ReadingCount("nonexistent"))
}

func TestStore_TimeRange(t *testing.T) {
	s := New()
	readings := makeReadings(sensorID, []float64{100, 200, 300}, startTime, hour)
	s.AddReadings(readings)

	tr, ok := s.TimeRange(sensorID)
	require.True(t, ok)
	assert.Equal(t, startTime, tr.Start)
	assert.Equal(t, startTime.Add(2*hour), tr.End)

	_, ok = s.TimeRange("nonexistent")
	assert.False(t, ok)
}

func TestStore_ReadingsInRange(t *testing.T) {
	s := New()
	readings := makeReadings(sensorID, []float64{100, 200, 300, 400, 500}, startTime, hour)
	s.AddReadings(readings)

	// Get readings from hour 1 to hour 3 (exclusive)
	result := s.ReadingsInRange(sensorID, startTime.Add(hour), startTime.Add(3*hour))
	require.Len(t, result, 2)
	assert.InDelta(t, 200.0, result[0].Value, 0.001)
	assert.InDelta(t, 300.0, result[1].Value, 0.001)

	// Empty range
	result = s.ReadingsInRange(sensorID, startTime.Add(10*hour), startTime.Add(11*hour))
	assert.Empty(t, result)

	// Nonexistent sensor
	result = s.ReadingsInRange("nonexistent", startTime, startTime.Add(hour))
	assert.Empty(t, result)
}

func TestStore_ReadingAt(t *testing.T) {
	s := New()
	readings := makeReadings(sensorID, []float64{100, 200, 300}, startTime, hour)
	s.AddReadings(readings)

	// Exact timestamp
	r, ok := s.ReadingAt(sensorID, startTime.Add(hour))
	require.True(t, ok)
	assert.InDelta(t, 200.0, r.Value, 0.001)

	// Between readings — returns most recent before
	r, ok = s.ReadingAt(sensorID, startTime.Add(90*time.Minute))
	require.True(t, ok)
	assert.InDelta(t, 200.0, r.Value, 0.001)

	// Before first reading
	_, ok = s.ReadingAt(sensorID, startTime.Add(-time.Hour))
	assert.False(t, ok)
}

func TestStore_Sensors(t *testing.T) {
	s := New()
	s.AddSensor(model.Sensor{ID: "sensor.grid", Name: "Grid Power", Type: model.SensorGridPower, Unit: "W"})

	sensors := s.Sensors()
	require.Len(t, sensors, 1)
	assert.Equal(t, "sensor.grid", sensors[0].ID)
}

func TestStore_GlobalTimeRange(t *testing.T) {
	s := New()

	_, ok := s.GlobalTimeRange()
	assert.False(t, ok)

	// sensor.a: 12:00 – 13:00
	// sensor.b: 11:00 – 14:00
	// union: 11:00 – 14:00
	r1 := makeReadings("sensor.a", []float64{100, 200}, startTime, hour)
	r2 := makeReadings("sensor.b", []float64{300, 400}, startTime.Add(-hour), 3*hour)
	s.AddReadings(r1)
	s.AddReadings(r2)

	tr, ok := s.GlobalTimeRange()
	require.True(t, ok)
	assert.Equal(t, startTime.Add(-hour), tr.Start)
	assert.Equal(t, startTime.Add(2*hour), tr.End)
}

func TestStore_GlobalTimeRange_NonOverlapping(t *testing.T) {
	s := New()

	// Non-overlapping sensors — union spans full range
	r1 := makeReadings("sensor.a", []float64{100, 200}, startTime, hour)
	r2 := makeReadings("sensor.b", []float64{300, 400}, startTime.Add(10*hour), hour)
	s.AddReadings(r1)
	s.AddReadings(r2)

	tr, ok := s.GlobalTimeRange()
	require.True(t, ok)
	assert.Equal(t, startTime, tr.Start)
	assert.Equal(t, startTime.Add(11*hour), tr.End)
}

func TestStore_AddReadingsUnsorted(t *testing.T) {
	s := New()

	// Add in reverse order
	readings := []model.Reading{
		{Timestamp: startTime.Add(2 * hour), SensorID: sensorID, Value: 300},
		{Timestamp: startTime, SensorID: sensorID, Value: 100},
		{Timestamp: startTime.Add(hour), SensorID: sensorID, Value: 200},
	}
	s.AddReadings(readings)

	result := s.ReadingsInRange(sensorID, startTime, startTime.Add(3*hour))
	require.Len(t, result, 3)
	assert.InDelta(t, 100.0, result[0].Value, 0.001)
	assert.InDelta(t, 200.0, result[1].Value, 0.001)
	assert.InDelta(t, 300.0, result[2].Value, 0.001)
}
