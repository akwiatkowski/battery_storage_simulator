package simulator

import (
	"math"
	"time"
)

// BatteryConfig holds the user-configurable parameters.
type BatteryConfig struct {
	CapacityKWh        float64 `json:"capacity_kwh"`
	MaxPowerW          float64 `json:"max_power_w"`
	DischargeToPercent float64 `json:"discharge_to_percent"`
	ChargeToPercent    float64 `json:"charge_to_percent"`
}

// ProcessResult is returned by Battery.Process for each reading.
type ProcessResult struct {
	BatteryPowerW float64 // positive = discharging, negative = charging
	AdjustedGridW float64
	SoCPercent    float64
}

// BatterySummary holds stats for WS broadcast.
type BatterySummary struct {
	SoCPercent       float64                       `json:"soc_percent"`
	Cycles           float64                       `json:"cycles"`
	TimeAtPowerSec   map[int]float64               `json:"time_at_power_sec"`
	TimeAtSoCPctSec  map[int]float64               `json:"time_at_soc_pct_sec"`
	MonthSoCSeconds  map[string]map[int]float64    `json:"month_soc_seconds"`
}

// Battery simulates a home battery storage system.
type Battery struct {
	config BatteryConfig

	// State
	SoCWh      float64
	PowerW     float64
	LastTime   time.Time
	LastDemand float64 // previous reading's demand, used for backward-looking intervals

	// Stats
	TotalThroughputWh float64
	TimeAtPowerSec    map[int]float64            // 1kW buckets
	TimeAtSoCPctSec   map[int]float64            // 10% buckets
	MonthSoCSeconds   map[string]map[int]float64 // "2024-11" → {10: 3600}
}

// NewBattery creates a battery starting at the discharge floor SoC.
func NewBattery(cfg BatteryConfig) *Battery {
	capacityWh := cfg.CapacityKWh * 1000
	floorWh := capacityWh * cfg.DischargeToPercent / 100
	return &Battery{
		config:          cfg,
		SoCWh:           floorWh,
		TimeAtPowerSec:  make(map[int]float64),
		TimeAtSoCPctSec: make(map[int]float64),
		MonthSoCSeconds: make(map[string]map[int]float64),
	}
}

// Process handles one grid_power reading using self-consumption strategy.
// homeDemandW: positive = consuming from grid, negative = exporting to grid.
//
// Uses backward-looking intervals: the PREVIOUS reading's demand determines
// battery action for the interval [LastTime, timestamp]. This ensures that
// an export reading followed by a consumption reading correctly charges the
// battery during the export interval.
func (b *Battery) Process(homeDemandW float64, timestamp time.Time) ProcessResult {
	var desired float64
	if !b.LastTime.IsZero() {
		desired = b.selfConsumptionDecision(b.LastDemand)
	}
	result := b.process(desired, homeDemandW, timestamp)
	b.LastDemand = homeDemandW
	return result
}

// ProcessArbitrage handles one grid_power reading using price arbitrage strategy.
// Charges at max power when price <= lowThresh, discharges at max power when
// price >= highThresh, holds otherwise. Unlike self-consumption, this can import
// from grid to charge.
func (b *Battery) ProcessArbitrage(gridPowerW float64, timestamp time.Time, price, lowThresh, highThresh float64) ProcessResult {
	var desired float64
	if !b.LastTime.IsZero() {
		desired = b.arbitrageDecision(price, lowThresh, highThresh)
	}
	return b.process(desired, gridPowerW, timestamp)
}

// selfConsumptionDecision decides battery action based on home demand.
// Positive demand → discharge to offset import, negative → charge from excess PV.
func (b *Battery) selfConsumptionDecision(intervalDemand float64) float64 {
	capacityWh := b.config.CapacityKWh * 1000
	floorWh := capacityWh * b.config.DischargeToPercent / 100
	ceilWh := capacityWh * b.config.ChargeToPercent / 100

	if intervalDemand > 0 {
		availableWh := b.SoCWh - floorWh
		if availableWh <= 0 {
			return 0
		}
		return math.Min(intervalDemand, b.config.MaxPowerW)
	} else if intervalDemand < 0 {
		excessW := -intervalDemand
		availableWh := ceilWh - b.SoCWh
		if availableWh <= 0 {
			return 0
		}
		return -math.Min(excessW, b.config.MaxPowerW)
	}
	return 0
}

// arbitrageDecision decides battery action based on price thresholds.
// Charge at max when cheap, discharge at max when expensive, hold otherwise.
func (b *Battery) arbitrageDecision(price, lowThresh, highThresh float64) float64 {
	capacityWh := b.config.CapacityKWh * 1000
	floorWh := capacityWh * b.config.DischargeToPercent / 100
	ceilWh := capacityWh * b.config.ChargeToPercent / 100

	if price <= lowThresh {
		if ceilWh-b.SoCWh <= 0 {
			return 0
		}
		return -b.config.MaxPowerW // charge
	}
	if price >= highThresh {
		if b.SoCWh-floorWh <= 0 {
			return 0
		}
		return b.config.MaxPowerW // discharge
	}
	return 0
}

// process applies a decided battery action for the interval ending at timestamp.
// desiredPowerW: positive=discharge, negative=charge.
// gridPowerW: raw grid power (for AdjustedGridW calculation).
func (b *Battery) process(desiredPowerW, gridPowerW float64, timestamp time.Time) ProcessResult {
	capacityWh := b.config.CapacityKWh * 1000
	floorWh := capacityWh * b.config.DischargeToPercent / 100
	ceilWh := capacityWh * b.config.ChargeToPercent / 100

	// Record stats for time spent at previous power/SoC
	if !b.LastTime.IsZero() {
		dt := timestamp.Sub(b.LastTime).Seconds()
		if dt > 0 {
			b.recordStats(dt)
		}
	}

	// First reading: store time baseline, no energy change yet.
	if b.LastTime.IsZero() {
		b.PowerW = 0
		b.LastTime = timestamp

		socPct := 0.0
		if capacityWh > 0 {
			socPct = b.SoCWh / capacityWh * 100
		}
		return ProcessResult{
			BatteryPowerW: 0,
			AdjustedGridW: gridPowerW,
			SoCPercent:    socPct,
		}
	}

	dt := timestamp.Sub(b.LastTime).Seconds()
	hours := dt / 3600

	batteryPowerW := desiredPowerW

	// Apply energy constraints based on time delta
	if dt > 0 {
		energyWh := batteryPowerW * hours

		if batteryPowerW > 0 {
			// Discharging: don't go below floor
			maxDrainWh := b.SoCWh - floorWh
			if energyWh > maxDrainWh {
				energyWh = maxDrainWh
				if hours > 0 {
					batteryPowerW = energyWh / hours
				}
			}
		} else if batteryPowerW < 0 {
			// Charging: don't go above ceiling
			maxFillWh := ceilWh - b.SoCWh
			if -energyWh > maxFillWh {
				energyWh = -maxFillWh
				if hours > 0 {
					batteryPowerW = energyWh / hours
				}
			}
		}

		b.SoCWh -= energyWh
		b.TotalThroughputWh += math.Abs(energyWh)
	}

	b.PowerW = batteryPowerW
	b.LastTime = timestamp

	adjustedGridW := gridPowerW - batteryPowerW

	socPct := 0.0
	if capacityWh > 0 {
		socPct = b.SoCWh / capacityWh * 100
	}

	return ProcessResult{
		BatteryPowerW: batteryPowerW,
		AdjustedGridW: adjustedGridW,
		SoCPercent:    socPct,
	}
}

// recordStats accumulates time-at-power and time-at-SoC histograms.
func (b *Battery) recordStats(dtSec float64) {
	// Power bucket: round to nearest 1kW
	powerKW := int(math.Round(b.PowerW / 1000))
	b.TimeAtPowerSec[powerKW] += dtSec

	// SoC bucket: round down to nearest 10%
	capacityWh := b.config.CapacityKWh * 1000
	socPct := 0.0
	if capacityWh > 0 {
		socPct = b.SoCWh / capacityWh * 100
	}
	socBucket := int(math.Floor(socPct/10)) * 10
	if socBucket < 0 {
		socBucket = 0
	}
	if socBucket > 100 {
		socBucket = 100
	}
	b.TimeAtSoCPctSec[socBucket] += dtSec

	// Monthly SoC tracking
	month := b.LastTime.Format("2006-01")
	if b.MonthSoCSeconds[month] == nil {
		b.MonthSoCSeconds[month] = make(map[int]float64)
	}
	b.MonthSoCSeconds[month][socBucket] += dtSec
}

// Cycles returns the equivalent full cycle count.
func (b *Battery) Cycles() float64 {
	capacityWh := b.config.CapacityKWh * 1000
	if capacityWh <= 0 {
		return 0
	}
	return b.TotalThroughputWh / 2 / capacityWh
}

// Summary returns the current battery summary for broadcasting.
func (b *Battery) Summary() BatterySummary {
	capacityWh := b.config.CapacityKWh * 1000
	socPct := 0.0
	if capacityWh > 0 {
		socPct = b.SoCWh / capacityWh * 100
	}
	return BatterySummary{
		SoCPercent:      socPct,
		Cycles:          b.Cycles(),
		TimeAtPowerSec:  b.TimeAtPowerSec,
		TimeAtSoCPctSec: b.TimeAtSoCPctSec,
		MonthSoCSeconds: b.MonthSoCSeconds,
	}
}

// Reset clears state and stats, setting SoC to discharge floor.
func (b *Battery) Reset() {
	capacityWh := b.config.CapacityKWh * 1000
	b.SoCWh = capacityWh * b.config.DischargeToPercent / 100
	b.PowerW = 0
	b.LastTime = time.Time{}
	b.LastDemand = 0
	b.TotalThroughputWh = 0
	b.TimeAtPowerSec = make(map[int]float64)
	b.TimeAtSoCPctSec = make(map[int]float64)
	b.MonthSoCSeconds = make(map[string]map[int]float64)
}
