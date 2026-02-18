package ws

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"energy_simulator/internal/simulator"
)

func newTestBridge() (*Bridge, *Client) {
	hub := NewHub()
	client := &Client{hub: hub, send: make(chan []byte, 256)}
	hub.Register(client)
	bridge := NewBridge(hub)
	return bridge, client
}

func receiveEnvelope(t *testing.T, c *Client) Envelope {
	t.Helper()
	msg := <-c.send
	var env Envelope
	require.NoError(t, json.Unmarshal(msg, &env))
	return env
}

func TestBridge_OnState(t *testing.T) {
	bridge, client := newTestBridge()

	bridge.OnState(simulator.State{
		Time:    startTime,
		Speed:   1800,
		Running: true,
	})

	env := receiveEnvelope(t, client)
	assert.Equal(t, TypeSimState, env.Type)

	var p SimStatePayload
	require.NoError(t, json.Unmarshal(env.Payload, &p))
	assert.Equal(t, "2024-11-21T12:00:00Z", p.Time)
	assert.Equal(t, 1800.0, p.Speed)
	assert.True(t, p.Running)
}

func TestBridge_OnReading(t *testing.T) {
	bridge, client := newTestBridge()

	bridge.OnReading(simulator.SensorReading{
		SensorID:  "sensor.grid",
		Value:     1500.5,
		Unit:      "W",
		Timestamp: "2024-11-21T13:00:00Z",
	})

	env := receiveEnvelope(t, client)
	assert.Equal(t, TypeSensorReading, env.Type)

	var p SensorReadingPayload
	require.NoError(t, json.Unmarshal(env.Payload, &p))
	assert.Equal(t, "sensor.grid", p.SensorID)
	assert.InDelta(t, 1500.5, p.Value, 0.001)
	assert.Equal(t, "W", p.Unit)
	assert.Equal(t, "2024-11-21T13:00:00Z", p.Timestamp)
}

func TestBridge_OnSummary(t *testing.T) {
	bridge, client := newTestBridge()

	bridge.OnSummary(simulator.Summary{
		TodayKWh:           1.5,
		MonthKWh:           30.0,
		TotalKWh:           100.0,
		GridImportKWh:      80.0,
		GridExportKWh:      20.0,
		PVProductionKWh:    50.0,
		HeatPumpKWh:        10.0,
		HeatPumpProdKWh:    25.0,
		SelfConsumptionKWh: 30.0,
		HomeDemandKWh:      110.0,
		BatterySavingsKWh:  5.0,
	})

	env := receiveEnvelope(t, client)
	assert.Equal(t, TypeSummaryUpdate, env.Type)

	var p SummaryPayload
	require.NoError(t, json.Unmarshal(env.Payload, &p))
	assert.InDelta(t, 1.5, p.TodayKWh, 0.001)
	assert.InDelta(t, 30.0, p.MonthKWh, 0.001)
	assert.InDelta(t, 100.0, p.TotalKWh, 0.001)
	assert.InDelta(t, 80.0, p.GridImportKWh, 0.001)
	assert.InDelta(t, 20.0, p.GridExportKWh, 0.001)
	assert.InDelta(t, 50.0, p.PVProductionKWh, 0.001)
	assert.InDelta(t, 10.0, p.HeatPumpKWh, 0.001)
	assert.InDelta(t, 25.0, p.HeatPumpProdKWh, 0.001)
	assert.InDelta(t, 30.0, p.SelfConsumptionKWh, 0.001)
	assert.InDelta(t, 110.0, p.HomeDemandKWh, 0.001)
	assert.InDelta(t, 5.0, p.BatterySavingsKWh, 0.001)
}

func TestBridge_OnBatteryUpdate(t *testing.T) {
	bridge, client := newTestBridge()

	bridge.OnBatteryUpdate(simulator.BatteryUpdate{
		BatteryPowerW: 2000,
		AdjustedGridW: -500,
		SoCPercent:    75.5,
		Timestamp:     "2024-11-21T14:00:00Z",
	})

	env := receiveEnvelope(t, client)
	assert.Equal(t, TypeBatteryUpdate, env.Type)

	var p BatteryUpdatePayload
	require.NoError(t, json.Unmarshal(env.Payload, &p))
	assert.InDelta(t, 2000.0, p.BatteryPowerW, 0.001)
	assert.InDelta(t, -500.0, p.AdjustedGridW, 0.001)
	assert.InDelta(t, 75.5, p.SoCPercent, 0.001)
	assert.Equal(t, "2024-11-21T14:00:00Z", p.Timestamp)
}

func TestBridge_OnBatterySummary(t *testing.T) {
	bridge, client := newTestBridge()

	bridge.OnBatterySummary(simulator.BatterySummary{
		SoCPercent:      60.0,
		Cycles:          12.5,
		TimeAtPowerSec:  map[int]float64{0: 3600, 1: 1800, -1: 900},
		TimeAtSoCPctSec: map[int]float64{50: 7200, 60: 3600},
		MonthSoCSeconds: map[string]map[int]float64{
			"2024-11": {50: 3600, 60: 1800},
		},
	})

	env := receiveEnvelope(t, client)
	assert.Equal(t, TypeBatterySummary, env.Type)

	var p BatterySummaryPayload
	require.NoError(t, json.Unmarshal(env.Payload, &p))
	assert.InDelta(t, 60.0, p.SoCPercent, 0.001)
	assert.InDelta(t, 12.5, p.Cycles, 0.001)
	assert.InDelta(t, 3600.0, p.TimeAtPowerSec[0], 0.001)
	assert.InDelta(t, 1800.0, p.TimeAtPowerSec[1], 0.001)
	assert.InDelta(t, 900.0, p.TimeAtPowerSec[-1], 0.001)
	assert.InDelta(t, 7200.0, p.TimeAtSoCPctSec[50], 0.001)
	assert.InDelta(t, 3600.0, p.MonthSoCSeconds["2024-11"][50], 0.001)
}
