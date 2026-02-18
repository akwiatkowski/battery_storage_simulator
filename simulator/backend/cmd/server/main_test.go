package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"energy_simulator/internal/model"
	"energy_simulator/internal/store"
)

func TestSensorTypeFromFilename(t *testing.T) {
	tests := []struct {
		name         string
		filename     string
		expectedType model.SensorType
		expectedUnit string
	}{
		{"grid power", "grid_power.csv", model.SensorGridPower, "W"},
		{"pv power", "pv_power.csv", model.SensorPVPower, "W"},
		{"pump consumption", "pump_total_consumption.csv", model.SensorPumpConsumption, "W"},
		{"pump production", "pump_total_production.csv", model.SensorPumpProduction, "W"},
		{"pump ext temp", "pump_ext_temp.csv", model.SensorPumpExtTemp, "°C"},
		{"unknown sensor", "unknown_sensor.csv", model.SensorType("unknown_sensor"), ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st, unit := sensorTypeFromFilename(tt.filename)
			assert.Equal(t, tt.expectedType, st)
			assert.Equal(t, tt.expectedUnit, unit)
		})
	}
}

func TestExtendTimeRange(t *testing.T) {
	t1 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
	t3 := time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC)

	t.Run("from zero", func(t *testing.T) {
		readings := []model.Reading{
			{Timestamp: t2},
			{Timestamp: t1},
			{Timestamp: t3},
		}
		tr := extendTimeRange(model.TimeRange{}, readings)
		assert.Equal(t, t1, tr.Start)
		assert.Equal(t, t3, tr.End)
	})

	t.Run("extends existing range", func(t *testing.T) {
		tr := model.TimeRange{Start: t2, End: t2}
		readings := []model.Reading{
			{Timestamp: t1},
			{Timestamp: t3},
		}
		tr = extendTimeRange(tr, readings)
		assert.Equal(t, t1, tr.Start)
		assert.Equal(t, t3, tr.End)
	})

	t.Run("no change when within range", func(t *testing.T) {
		tr := model.TimeRange{Start: t1, End: t3}
		readings := []model.Reading{
			{Timestamp: t2},
		}
		tr = extendTimeRange(tr, readings)
		assert.Equal(t, t1, tr.Start)
		assert.Equal(t, t3, tr.End)
	})

	t.Run("empty readings", func(t *testing.T) {
		tr := model.TimeRange{Start: t1, End: t2}
		tr = extendTimeRange(tr, nil)
		assert.Equal(t, t1, tr.Start)
		assert.Equal(t, t2, tr.End)
	})
}

func TestMergeTimeRanges(t *testing.T) {
	t1 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	t3 := time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)
	t4 := time.Date(2024, 12, 1, 0, 0, 0, 0, time.UTC)

	t.Run("both valid", func(t *testing.T) {
		a := model.TimeRange{Start: t1, End: t2}
		b := model.TimeRange{Start: t3, End: t4}
		result := mergeTimeRanges(a, b)
		assert.Equal(t, t1, result.Start) // earlier
		assert.Equal(t, t4, result.End)   // later
	})

	t.Run("a is zero", func(t *testing.T) {
		b := model.TimeRange{Start: t3, End: t4}
		result := mergeTimeRanges(model.TimeRange{}, b)
		assert.Equal(t, t3, result.Start)
		assert.Equal(t, t4, result.End)
	})

	t.Run("b is zero", func(t *testing.T) {
		a := model.TimeRange{Start: t1, End: t2}
		result := mergeTimeRanges(a, model.TimeRange{})
		assert.Equal(t, t1, result.Start)
		assert.Equal(t, t2, result.End)
	})

	t.Run("both zero", func(t *testing.T) {
		result := mergeTimeRanges(model.TimeRange{}, model.TimeRange{})
		assert.True(t, result.Start.IsZero())
		assert.True(t, result.End.IsZero())
	})

	t.Run("b starts earlier", func(t *testing.T) {
		a := model.TimeRange{Start: t3, End: t4}
		b := model.TimeRange{Start: t1, End: t2}
		result := mergeTimeRanges(a, b)
		assert.Equal(t, t1, result.Start)
		assert.Equal(t, t4, result.End)
	})
}

func TestFindSensorID(t *testing.T) {
	s := store.New()
	s.AddSensor(model.Sensor{ID: "sensor.grid", Name: "Grid", Type: model.SensorGridPower, Unit: "W"})
	s.AddSensor(model.Sensor{ID: "sensor.pv", Name: "PV", Type: model.SensorPVPower, Unit: "W"})

	t.Run("found", func(t *testing.T) {
		id := findSensorID(s, model.SensorGridPower)
		assert.Equal(t, "sensor.grid", id)
	})

	t.Run("found pv", func(t *testing.T) {
		id := findSensorID(s, model.SensorPVPower)
		assert.Equal(t, "sensor.pv", id)
	})

	t.Run("not found", func(t *testing.T) {
		id := findSensorID(s, model.SensorPumpConsumption)
		assert.Empty(t, id)
	})
}
