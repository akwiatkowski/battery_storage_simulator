package ingest

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"energy_simulator/internal/model"
)

func TestHomeAssistantParser_Parse(t *testing.T) {
	input := `entity_id,state,last_changed
sensor.zigbee_power,-368.85,2024-11-21T12:00:00.000Z
sensor.zigbee_power,759.59,2024-11-21T13:00:00.000Z
sensor.zigbee_power,562.78,2024-11-21T14:00:00.000Z`

	parser := NewHomeAssistantParser(model.SensorGridPower, "W")
	readings, err := parser.Parse(strings.NewReader(input))

	require.NoError(t, err)
	require.Len(t, readings, 3)

	assert.Equal(t, "sensor.zigbee_power", readings[0].SensorID)
	assert.Equal(t, model.SensorGridPower, readings[0].Type)
	assert.InDelta(t, -368.85, readings[0].Value, 0.001)
	assert.Equal(t, "W", readings[0].Unit)
	assert.Equal(t, time.Date(2024, 11, 21, 12, 0, 0, 0, time.UTC), readings[0].Timestamp)

	assert.InDelta(t, 759.59, readings[1].Value, 0.001)
	assert.Equal(t, time.Date(2024, 11, 21, 13, 0, 0, 0, time.UTC), readings[1].Timestamp)
}

func TestHomeAssistantParser_SkipsUnavailable(t *testing.T) {
	input := `entity_id,state,last_changed
sensor.zigbee_power,759.59,2024-11-21T13:00:00.000Z
sensor.zigbee_power,unavailable,2024-11-21T14:00:00.000Z
sensor.zigbee_power,562.78,2024-11-21T15:00:00.000Z`

	parser := NewHomeAssistantParser(model.SensorGridPower, "W")
	readings, err := parser.Parse(strings.NewReader(input))

	require.NoError(t, err)
	require.Len(t, readings, 2)
	assert.InDelta(t, 759.59, readings[0].Value, 0.001)
	assert.InDelta(t, 562.78, readings[1].Value, 0.001)
}

func TestHomeAssistantParser_InvalidHeader(t *testing.T) {
	input := `wrong_col,state,last_changed
sensor.zigbee_power,759.59,2024-11-21T13:00:00.000Z`

	parser := NewHomeAssistantParser(model.SensorGridPower, "W")
	_, err := parser.Parse(strings.NewReader(input))

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "entity_id")
}

func TestHomeAssistantParser_EmptyInput(t *testing.T) {
	parser := NewHomeAssistantParser(model.SensorGridPower, "W")
	_, err := parser.Parse(strings.NewReader(""))

	assert.Error(t, err)
}

func TestHomeAssistantParser_SampleFile(t *testing.T) {
	f, err := os.Open("../../../testdata/grid_power_sample.csv")
	require.NoError(t, err)
	defer f.Close()

	parser := NewHomeAssistantParser(model.SensorGridPower, "W")
	readings, err := parser.Parse(f)

	require.NoError(t, err)
	require.Len(t, readings, 20)

	// First reading is negative (export)
	assert.InDelta(t, -368.85, readings[0].Value, 0.001)
	// Last reading
	assert.InDelta(t, 1263.14, readings[len(readings)-1].Value, 0.001)

	// All readings have the same sensor ID
	for _, r := range readings {
		assert.Equal(t, "sensor.0x943469fffed2bf71_power", r.SensorID)
		assert.Equal(t, model.SensorGridPower, r.Type)
		assert.Equal(t, "W", r.Unit)
	}
}

func TestHomeAssistantParser_RFC3339Nano(t *testing.T) {
	input := `entity_id,state,last_changed
sensor.zigbee_power,321,2026-02-11T18:49:18.424Z`

	parser := NewHomeAssistantParser(model.SensorGridPower, "W")
	readings, err := parser.Parse(strings.NewReader(input))

	require.NoError(t, err)
	require.Len(t, readings, 1)
	assert.InDelta(t, 321.0, readings[0].Value, 0.001)
	assert.Equal(t, 2026, readings[0].Timestamp.Year())
}
