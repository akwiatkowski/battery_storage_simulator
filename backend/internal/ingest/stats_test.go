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

func TestStatsParser_Parse(t *testing.T) {
	input := `sensor_id,start_time,avg,min_val,max_val
sensor.0x943469fffed2bf71_power,1732186800.0,-368.85,-810.0,-162.0
sensor.0x943469fffed2bf71_power,1732190400.0,759.59,-286.0,2214.0`

	parser := &StatsParser{}
	readings, err := parser.Parse(strings.NewReader(input))

	require.NoError(t, err)
	require.Len(t, readings, 2)

	assert.Equal(t, model.SensorGridPower, readings[0].Type)
	assert.Equal(t, "sensor.0x943469fffed2bf71_power", readings[0].SensorID)
	assert.InDelta(t, -368.85, readings[0].Value, 0.001)
	assert.InDelta(t, -810.0, readings[0].Min, 0.001)
	assert.InDelta(t, -162.0, readings[0].Max, 0.001)
	assert.Equal(t, "W", readings[0].Unit)
	assert.Equal(t, time.Date(2024, 11, 21, 11, 0, 0, 0, time.UTC), readings[0].Timestamp)

	assert.InDelta(t, 759.59, readings[1].Value, 0.001)
	assert.InDelta(t, -286.0, readings[1].Min, 0.001)
	assert.InDelta(t, 2214.0, readings[1].Max, 0.001)
}

func TestStatsParser_SkipsUnknownEntities(t *testing.T) {
	input := `sensor_id,start_time,avg,min_val,max_val
sensor.0x943469fffed2bf71_power,1732186800.0,-368.85,-810.0,-162.0
sensor.unknown_entity,1732186800.0,100.0,50.0,150.0
sensor.hoymiles_gateway_solarh_3054300_real_power,1732186800.0,1500.0,200.0,3000.0`

	parser := &StatsParser{}
	readings, err := parser.Parse(strings.NewReader(input))

	require.NoError(t, err)
	require.Len(t, readings, 2)
	assert.Equal(t, model.SensorGridPower, readings[0].Type)
	assert.Equal(t, model.SensorPVPower, readings[1].Type)
}

func TestStatsParser_InvalidHeader(t *testing.T) {
	input := `wrong_col,start_time,avg,min_val,max_val
sensor.0x943469fffed2bf71_power,1732186800.0,-368.85,-810.0,-162.0`

	parser := &StatsParser{}
	_, err := parser.Parse(strings.NewReader(input))

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "sensor_id")
}

func TestStatsParser_EmptyInput(t *testing.T) {
	parser := &StatsParser{}
	_, err := parser.Parse(strings.NewReader(""))

	assert.Error(t, err)
}

func TestStatsParser_SampleFile(t *testing.T) {
	f, err := os.Open("../../../testdata/stats_sample.csv")
	require.NoError(t, err)
	defer f.Close()

	parser := &StatsParser{}
	readings, err := parser.Parse(f)

	require.NoError(t, err)
	// 3 known sensors, 1 unknown skipped
	require.Len(t, readings, 3)

	assert.Equal(t, model.SensorGridPower, readings[0].Type)
	assert.Equal(t, model.SensorGridPower, readings[1].Type)
	assert.Equal(t, model.SensorPVPower, readings[2].Type)
}
