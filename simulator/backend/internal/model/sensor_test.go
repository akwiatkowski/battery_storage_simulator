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
		Min:       100.0,
		Max:       1200.0,
		Unit:      "W",
	}

	assert.Equal(t, ts, r.Timestamp)
	assert.Equal(t, "sensor.zigbee_power", r.SensorID)
	assert.Equal(t, SensorGridPower, r.Type)
	assert.InDelta(t, 759.59, r.Value, 0.001)
	assert.InDelta(t, 100.0, r.Min, 0.001)
	assert.InDelta(t, 1200.0, r.Max, 0.001)
	assert.Equal(t, "W", r.Unit)
}

func TestHAEntityToSensorType(t *testing.T) {
	// Reverse map should contain all entries from SensorHomeAssistantID
	assert.Equal(t, len(SensorHomeAssistantID), len(HAEntityToSensorType))

	st, ok := HAEntityToSensorType["sensor.0x943469fffed2bf71_power"]
	assert.True(t, ok)
	assert.Equal(t, SensorGridPower, st)

	st, ok = HAEntityToSensorType["sensor.hoymiles_gateway_solarh_3054300_real_power"]
	assert.True(t, ok)
	assert.Equal(t, SensorPVPower, st)

	_, ok = HAEntityToSensorType["nonexistent"]
	assert.False(t, ok)
}

func TestSensorCatalog(t *testing.T) {
	info, ok := SensorCatalog[SensorGridPower]
	assert.True(t, ok)
	assert.Equal(t, "Grid Power", info.Name)
	assert.Equal(t, "W", info.Unit)

	info, ok = SensorCatalog[SensorPumpExtTemp]
	assert.True(t, ok)
	assert.Equal(t, "Outside Temperature", info.Name)
	assert.Equal(t, "°C", info.Unit)

	// Every sensor type in SensorHomeAssistantID should be in catalog
	for st := range SensorHomeAssistantID {
		_, ok := SensorCatalog[st]
		assert.True(t, ok, "SensorCatalog missing entry for %s", st)
	}
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
