package ws

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"energy_simulator/internal/simulator"
)

var startTime = time.Date(2024, 11, 21, 12, 0, 0, 0, time.UTC)

func TestSimStateFromEngine(t *testing.T) {
	state := simulator.State{
		Time:    startTime,
		Speed:   7200,
		Running: true,
	}

	p := SimStateFromEngine(state)

	assert.Equal(t, "2024-11-21T12:00:00Z", p.Time)
	assert.Equal(t, 7200.0, p.Speed)
	assert.True(t, p.Running)
}

func TestSimStateFromEngine_Stopped(t *testing.T) {
	state := simulator.State{
		Time:    time.Date(2025, 1, 15, 8, 30, 0, 0, time.UTC),
		Speed:   3600,
		Running: false,
	}

	p := SimStateFromEngine(state)

	assert.Equal(t, "2025-01-15T08:30:00Z", p.Time)
	assert.Equal(t, 3600.0, p.Speed)
	assert.False(t, p.Running)
}

func TestSummaryFromEngine(t *testing.T) {
	summary := simulator.Summary{
		TodayKWh:           1.5,
		MonthKWh:           45.2,
		TotalKWh:           500.0,
		GridImportKWh:      400.0,
		GridExportKWh:      100.0,
		PVProductionKWh:    300.0,
		HeatPumpKWh:        50.0,
		HeatPumpProdKWh:    120.0,
		SelfConsumptionKWh: 200.0,
		HomeDemandKWh:      600.0,
		BatterySavingsKWh:  25.0,
	}

	p := SummaryFromEngine(summary)

	assert.InDelta(t, 1.5, p.TodayKWh, 0.001)
	assert.InDelta(t, 45.2, p.MonthKWh, 0.001)
	assert.InDelta(t, 500.0, p.TotalKWh, 0.001)
	assert.InDelta(t, 400.0, p.GridImportKWh, 0.001)
	assert.InDelta(t, 100.0, p.GridExportKWh, 0.001)
	assert.InDelta(t, 300.0, p.PVProductionKWh, 0.001)
	assert.InDelta(t, 50.0, p.HeatPumpKWh, 0.001)
	assert.InDelta(t, 120.0, p.HeatPumpProdKWh, 0.001)
	assert.InDelta(t, 200.0, p.SelfConsumptionKWh, 0.001)
	assert.InDelta(t, 600.0, p.HomeDemandKWh, 0.001)
	assert.InDelta(t, 25.0, p.BatterySavingsKWh, 0.001)
}

func TestSummaryFromEngine_Zeros(t *testing.T) {
	p := SummaryFromEngine(simulator.Summary{})

	assert.InDelta(t, 0.0, p.TodayKWh, 0.001)
	assert.InDelta(t, 0.0, p.TotalKWh, 0.001)
	assert.InDelta(t, 0.0, p.GridImportKWh, 0.001)
	assert.InDelta(t, 0.0, p.PVProductionKWh, 0.001)
	assert.InDelta(t, 0.0, p.BatterySavingsKWh, 0.001)
}
