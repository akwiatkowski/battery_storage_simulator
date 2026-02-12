package ingest

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"energy_simulator/internal/model"
)

func TestRecentParser_Parse(t *testing.T) {
	input := `sensor_id,value,updated_ts
sensor.0x943469fffed2bf71_power,-341,1770896300.6877737
sensor.0x943469fffed2bf71_power,-324,1770896240.6859024`

	parser := &RecentParser{}
	readings, err := parser.Parse(strings.NewReader(input))

	require.NoError(t, err)
	require.Len(t, readings, 2)

	assert.Equal(t, model.SensorGridPower, readings[0].Type)
	assert.Equal(t, "sensor.0x943469fffed2bf71_power", readings[0].SensorID)
	assert.InDelta(t, -341.0, readings[0].Value, 0.001)
	assert.InDelta(t, -341.0, readings[0].Min, 0.001)
	assert.InDelta(t, -341.0, readings[0].Max, 0.001)
	assert.Equal(t, "W", readings[0].Unit)
	assert.Equal(t, 2026, readings[0].Timestamp.Year())

	assert.InDelta(t, -324.0, readings[1].Value, 0.001)
}

func TestRecentParser_SkipsUnknownEntities(t *testing.T) {
	input := `sensor_id,value,updated_ts
sensor.0x943469fffed2bf71_power,-341,1770896300.0
sensor.unknown_entity,999,1770896300.0
sensor.hoymiles_gateway_solarh_3054300_real_power,1200,1770896300.0`

	parser := &RecentParser{}
	readings, err := parser.Parse(strings.NewReader(input))

	require.NoError(t, err)
	require.Len(t, readings, 2)
	assert.Equal(t, model.SensorGridPower, readings[0].Type)
	assert.Equal(t, model.SensorPVPower, readings[1].Type)
}

func TestRecentParser_InvalidHeader(t *testing.T) {
	input := `wrong_col,value,updated_ts
sensor.0x943469fffed2bf71_power,-341,1770896300.0`

	parser := &RecentParser{}
	_, err := parser.Parse(strings.NewReader(input))

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "sensor_id")
}

func TestRecentParser_EmptyInput(t *testing.T) {
	parser := &RecentParser{}
	_, err := parser.Parse(strings.NewReader(""))

	assert.Error(t, err)
}

func TestRecentParser_SampleFile(t *testing.T) {
	f, err := os.Open("../../../testdata/recent_sample.csv")
	require.NoError(t, err)
	defer f.Close()

	parser := &RecentParser{}
	readings, err := parser.Parse(f)

	require.NoError(t, err)
	// 3 known sensors, 1 unknown skipped
	require.Len(t, readings, 3)

	assert.Equal(t, model.SensorGridPower, readings[0].Type)
	assert.Equal(t, model.SensorGridPower, readings[1].Type)
	assert.Equal(t, model.SensorPVPower, readings[2].Type)
}
