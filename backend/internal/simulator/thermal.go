package simulator

import "time"

// InsulationLevel categorizes building insulation quality.
type InsulationLevel string

const (
	InsulationVeryGood InsulationLevel = "very_good" // EP < 60 kWh/m²·year → ~100 W/°C
	InsulationGood     InsulationLevel = "good"      // EP 60-90 → ~150 W/°C
	InsulationNormal   InsulationLevel = "normal"    // EP 90-120 → ~200 W/°C
	InsulationBasic    InsulationLevel = "basic"     // EP > 120 → ~280 W/°C
)

// HeatLossForInsulation returns the heat loss coefficient (W/°C) for a ~120m² house.
func HeatLossForInsulation(level InsulationLevel) float64 {
	switch level {
	case InsulationVeryGood:
		return 100
	case InsulationGood:
		return 150
	case InsulationNormal:
		return 200
	case InsulationBasic:
		return 280
	default:
		return 150
	}
}

// ThermalModel simulates building thermal mass for pre-heating optimization.
// It runs as a shadow simulation (like arbitrage battery) tracking what heating
// cost WOULD be if the heat pump pre-heated during cheap hours.
type ThermalModel struct {
	IndoorTempC   float64         // current simulated indoor temperature
	SetpointC     float64         // target temperature (default 21°C)
	PreHeatDeltaC float64         // overheat amount during cheap hours (default 2°C)
	ThermalMassJ  float64         // building thermal capacity in joules/°C (kWh/°C * 3.6e6)
	HeatLossWC    float64         // heat loss coefficient from insulation level
	Insulation    InsulationLevel // current insulation level
	CostPLN       float64         // accumulated shadow cost
	LastTimestamp time.Time
}

// ThermalStepResult holds the output of one thermal simulation step.
type ThermalStepResult struct {
	IndoorTempC float64
	HPPowerW    float64 // electrical power consumed by HP in this step
	CostPLN     float64 // cost for this step
}

// NewThermalModel creates a thermal model with the given insulation level.
func NewThermalModel(insulation InsulationLevel) *ThermalModel {
	return &ThermalModel{
		IndoorTempC:   21.0,
		SetpointC:     21.0,
		PreHeatDeltaC: 2.0,
		ThermalMassJ:  2.0 * 3.6e6, // 2.0 kWh/°C converted to J/°C
		HeatLossWC:    HeatLossForInsulation(insulation),
		Insulation:    insulation,
	}
}

// Step advances the thermal simulation by one reading interval.
// Parameters:
//   - outdoorTempC: current outdoor temperature
//   - spotPrice: current electricity price (PLN/kWh)
//   - lowThresh, highThresh: daily P33/P67 price thresholds
//   - hpMaxPowerW: maximum heat pump electrical power
//   - cop: current coefficient of performance
//   - ts: current timestamp
func (tm *ThermalModel) Step(outdoorTempC, spotPrice, lowThresh, highThresh, hpMaxPowerW, cop float64, ts time.Time) ThermalStepResult {
	if tm.LastTimestamp.IsZero() {
		tm.LastTimestamp = ts
		return ThermalStepResult{IndoorTempC: tm.IndoorTempC}
	}

	dt := ts.Sub(tm.LastTimestamp).Seconds()
	if dt <= 0 {
		return ThermalStepResult{IndoorTempC: tm.IndoorTempC}
	}
	tm.LastTimestamp = ts

	// Heat loss from building to outside (W)
	lossW := tm.HeatLossWC * (tm.IndoorTempC - outdoorTempC)
	if lossW < 0 {
		lossW = 0 // don't model cooling when outside is warmer
	}

	// Determine HP electrical power based on pre-heating strategy
	var hpElecW float64
	hasPriceData := lowThresh != highThresh

	if hasPriceData && spotPrice <= lowThresh && tm.IndoorTempC < tm.SetpointC+tm.PreHeatDeltaC {
		// Cheap electricity: run HP at full power to pre-heat
		hpElecW = hpMaxPowerW
	} else if hasPriceData && spotPrice >= highThresh && tm.IndoorTempC > tm.SetpointC {
		// Expensive electricity: coast (HP off)
		hpElecW = 0
	} else {
		// Normal: maintain setpoint
		if tm.IndoorTempC < tm.SetpointC {
			hpElecW = hpMaxPowerW
		} else {
			hpElecW = 0
		}
	}

	// Thermal output from HP (W_thermal = W_electrical × COP)
	hpThermalW := hpElecW * cop

	// Temperature change: dT = (hpThermal - loss) × dt / thermalMass
	dT := (hpThermalW - lossW) * dt / tm.ThermalMassJ
	tm.IndoorTempC += dT

	// Clamp indoor temp to reasonable range
	if tm.IndoorTempC < outdoorTempC {
		tm.IndoorTempC = outdoorTempC
	}
	if tm.IndoorTempC > tm.SetpointC+tm.PreHeatDeltaC+2 {
		tm.IndoorTempC = tm.SetpointC + tm.PreHeatDeltaC + 2
	}

	// Track cost
	hours := dt / 3600.0
	kWh := (hpElecW * hours) / 1000.0
	cost := kWh * spotPrice
	tm.CostPLN += cost

	return ThermalStepResult{
		IndoorTempC: tm.IndoorTempC,
		HPPowerW:    hpElecW,
		CostPLN:     cost,
	}
}

// Reset resets the thermal model to initial state.
func (tm *ThermalModel) Reset() {
	tm.IndoorTempC = tm.SetpointC
	tm.CostPLN = 0
	tm.LastTimestamp = time.Time{}
}
