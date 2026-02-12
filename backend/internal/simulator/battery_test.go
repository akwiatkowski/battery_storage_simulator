package simulator

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var defaultBatteryConfig = BatteryConfig{
	CapacityKWh:        10,
	MaxPowerW:          5000,
	DischargeToPercent: 10,
	ChargeToPercent:    100,
}

var t0 = time.Date(2024, 11, 21, 12, 0, 0, 0, time.UTC)

func TestBattery_NewStartsAtFloor(t *testing.T) {
	b := NewBattery(defaultBatteryConfig)
	// 10 kWh * 10% = 1 kWh = 1000 Wh
	assert.InDelta(t, 1000, b.SoCWh, 0.01)
}

func TestBattery_ChargeOnExport(t *testing.T) {
	b := NewBattery(defaultBatteryConfig)

	// First call sets LastTime and LastDemand, no energy change
	r := b.Process(-2000, t0)
	assert.InDelta(t, 0, r.BatteryPowerW, 0.01)
	assert.InDelta(t, -2000, r.AdjustedGridW, 0.01)

	// Second call: uses PREVIOUS demand (-2000W) for the interval
	// Battery charges at 2000W for 1h = 2000Wh
	r = b.Process(-2000, t0.Add(time.Hour))
	assert.InDelta(t, -2000, r.BatteryPowerW, 0.01) // charging
	assert.InDelta(t, 0, r.AdjustedGridW, 0.01)     // export absorbed
	// SoC: 1000 + 2000 = 3000 Wh = 30%
	assert.InDelta(t, 30, r.SoCPercent, 0.01)
}

func TestBattery_DischargeOnConsumption(t *testing.T) {
	cfg := defaultBatteryConfig
	b := NewBattery(cfg)
	// Pre-charge: set SoC to 5000 Wh (50%)
	b.SoCWh = 5000

	r := b.Process(3000, t0)
	// First reading, no time delta — sets LastTime and LastDemand
	assert.InDelta(t, 0, r.BatteryPowerW, 0.01)

	// Second call: uses PREVIOUS demand (3000W) for the interval
	r = b.Process(3000, t0.Add(time.Hour))
	assert.InDelta(t, 3000, r.BatteryPowerW, 0.01) // discharging
	assert.InDelta(t, 0, r.AdjustedGridW, 0.01)    // grid fully offset
	// SoC: 5000 - 3000 = 2000 Wh = 20%
	assert.InDelta(t, 20, r.SoCPercent, 0.01)
}

func TestBattery_SoCFloorLimit(t *testing.T) {
	b := NewBattery(defaultBatteryConfig)
	// SoC at floor (1000 Wh = 10%)
	// Try to discharge — should not go below floor

	b.Process(5000, t0)
	r := b.Process(5000, t0.Add(time.Hour))
	// Should not discharge (already at floor)
	assert.InDelta(t, 0, r.BatteryPowerW, 0.01)
	assert.InDelta(t, 5000, r.AdjustedGridW, 0.01)
	assert.InDelta(t, 10, r.SoCPercent, 0.01)
}

func TestBattery_SoCCeilingLimit(t *testing.T) {
	cfg := defaultBatteryConfig
	cfg.ChargeToPercent = 90
	b := NewBattery(cfg)
	// Set SoC to 8500 Wh — ceiling is 9000 Wh (90%)
	b.SoCWh = 8500

	b.Process(-5000, t0)
	r := b.Process(-5000, t0.Add(time.Hour))
	// Can only charge 500 Wh in 1h = 500W effective
	assert.InDelta(t, -500, r.BatteryPowerW, 0.01)
	assert.InDelta(t, -4500, r.AdjustedGridW, 0.01) // rest still exported
	assert.InDelta(t, 90, r.SoCPercent, 0.01)
}

func TestBattery_MaxPowerLimit(t *testing.T) {
	cfg := BatteryConfig{
		CapacityKWh:        10,
		MaxPowerW:          2000, // low max power
		DischargeToPercent: 0,
		ChargeToPercent:    100,
	}
	b := NewBattery(cfg)
	b.SoCWh = 5000

	// Try to discharge 8000W — capped at 2000W
	b.Process(8000, t0)
	r := b.Process(8000, t0.Add(time.Hour))
	assert.InDelta(t, 2000, r.BatteryPowerW, 0.01)
	assert.InDelta(t, 6000, r.AdjustedGridW, 0.01)
}

func TestBattery_CycleCounting(t *testing.T) {
	b := NewBattery(defaultBatteryConfig)
	// 10 kWh capacity. One full cycle = charge 10kWh + discharge 10kWh = 20kWh throughput.
	b.TotalThroughputWh = 20000
	assert.InDelta(t, 1.0, b.Cycles(), 0.01)

	b.TotalThroughputWh = 10000
	assert.InDelta(t, 0.5, b.Cycles(), 0.01)
}

func TestBattery_HistogramAccumulation(t *testing.T) {
	b := NewBattery(defaultBatteryConfig)

	// Process a few readings to build histograms
	b.Process(0, t0)
	b.Process(0, t0.Add(30*time.Minute))  // 30 min at 0W, SoC 10%
	b.Process(0, t0.Add(60*time.Minute))  // 30 min more at 0W

	// Should have time at 0kW power bucket
	assert.Greater(t, b.TimeAtPowerSec[0], 0.0)
	// Should have time at 10% SoC bucket
	assert.Greater(t, b.TimeAtSoCPctSec[10], 0.0)
}

func TestBattery_MonthSoCSeconds(t *testing.T) {
	b := NewBattery(defaultBatteryConfig)

	// Process readings across a month boundary
	nov := time.Date(2024, 11, 30, 23, 0, 0, 0, time.UTC)
	dec := time.Date(2024, 12, 1, 0, 0, 0, 0, time.UTC)

	b.Process(0, nov)                        // baseline
	b.Process(0, nov.Add(30*time.Minute))    // 30min in Nov at SoC 10%
	b.Process(0, dec)                        // 30min in Nov at SoC 10%
	b.Process(0, dec.Add(time.Hour))         // 1h in Dec at SoC 10%

	assert.Contains(t, b.MonthSoCSeconds, "2024-11")
	assert.Contains(t, b.MonthSoCSeconds, "2024-12")
	// Nov: 2 intervals of 30min = 3600s at SoC bucket 10
	assert.InDelta(t, 3600, b.MonthSoCSeconds["2024-11"][10], 0.01)
	// Dec: 1 interval of 1h = 3600s
	assert.InDelta(t, 3600, b.MonthSoCSeconds["2024-12"][10], 0.01)
}

func TestBattery_Reset(t *testing.T) {
	b := NewBattery(defaultBatteryConfig)
	b.SoCWh = 5000
	b.TotalThroughputWh = 10000
	b.PowerW = 3000
	b.TimeAtPowerSec[3] = 100
	b.TimeAtSoCPctSec[50] = 100

	b.Reset()

	assert.InDelta(t, 1000, b.SoCWh, 0.01)  // back to floor
	assert.InDelta(t, 0, b.PowerW, 0.01)
	assert.InDelta(t, 0, b.TotalThroughputWh, 0.01)
	assert.Empty(t, b.TimeAtPowerSec)
	assert.Empty(t, b.TimeAtSoCPctSec)
	assert.Empty(t, b.MonthSoCSeconds)
	assert.True(t, b.LastTime.IsZero())
}

func TestBattery_Summary(t *testing.T) {
	b := NewBattery(defaultBatteryConfig)
	b.SoCWh = 5000
	b.TotalThroughputWh = 20000

	s := b.Summary()
	assert.InDelta(t, 50, s.SoCPercent, 0.01)
	assert.InDelta(t, 1.0, s.Cycles, 0.01)
}
