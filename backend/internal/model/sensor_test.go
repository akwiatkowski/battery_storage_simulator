package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSensorType(t *testing.T) {
	assert.Equal(t, SensorType("grid_power"), SensorGridPower)
}

func TestReading(t *testing.T) {
	ts := time.Date(2024, 11, 21, 12, 0, 0, 0, time.UTC)
	r := Reading{
		Timestamp: ts,
		SensorID:  "sensor.zigbee_power",
		Type:      SensorGridPower,
		Value:     759.59,
		Unit:      "W",
	}

	assert.Equal(t, ts, r.Timestamp)
	assert.Equal(t, "sensor.zigbee_power", r.SensorID)
	assert.Equal(t, SensorGridPower, r.Type)
	assert.InDelta(t, 759.59, r.Value, 0.001)
	assert.Equal(t, "W", r.Unit)
}

func TestSensor(t *testing.T) {
	s := Sensor{
		ID:   "sensor.zigbee_power",
		Name: "Grid Power",
		Type: SensorGridPower,
		Unit: "W",
	}

	assert.Equal(t, "sensor.zigbee_power", s.ID)
	assert.Equal(t, SensorGridPower, s.Type)
}
