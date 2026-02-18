package ws

import (
	"encoding/json"

	"energy_simulator/internal/simulator"
)

// Envelope wraps all WebSocket messages with a type discriminator.
type Envelope struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// Client -> Server messages

type SetSpeedPayload struct {
	Speed float64 `json:"speed"`
}

type SeekPayload struct {
	Timestamp string `json:"timestamp"`
}

type SetSourcePayload struct {
	Source string `json:"source"`
}

// Server -> Client messages

type SimStatePayload struct {
	Time    string  `json:"time"`
	Speed   float64 `json:"speed"`
	Running bool    `json:"running"`
}

type SensorReadingPayload struct {
	SensorID  string  `json:"sensor_id"`
	Value     float64 `json:"value"`
	Unit      string  `json:"unit"`
	Timestamp string  `json:"timestamp"`
}

type SummaryPayload struct {
	TodayKWh           float64 `json:"today_kwh"`
	MonthKWh           float64 `json:"month_kwh"`
	TotalKWh           float64 `json:"total_kwh"`
	GridImportKWh      float64 `json:"grid_import_kwh"`
	GridExportKWh      float64 `json:"grid_export_kwh"`
	PVProductionKWh    float64 `json:"pv_production_kwh"`
	HeatPumpKWh        float64 `json:"heat_pump_kwh"`
	HeatPumpProdKWh    float64 `json:"heat_pump_prod_kwh"`
	HeatPumpCostPLN    float64 `json:"heat_pump_cost_pln"`
	SelfConsumptionKWh float64 `json:"self_consumption_kwh"`
	HomeDemandKWh      float64 `json:"home_demand_kwh"`
	BatterySavingsKWh  float64 `json:"battery_savings_kwh"`

	GridImportCostPLN       float64 `json:"grid_import_cost_pln"`
	GridExportRevenuePLN    float64 `json:"grid_export_revenue_pln"`
	NetCostPLN              float64 `json:"net_cost_pln"`
	RawGridImportCostPLN    float64 `json:"raw_grid_import_cost_pln"`
	RawGridExportRevenuePLN float64 `json:"raw_grid_export_revenue_pln"`
	RawNetCostPLN           float64 `json:"raw_net_cost_pln"`
	BatterySavingsPLN       float64 `json:"battery_savings_pln"`

	ArbNetCostPLN        float64 `json:"arb_net_cost_pln"`
	ArbBatterySavingsPLN float64 `json:"arb_battery_savings_pln"`

	CheapExportKWh    float64 `json:"cheap_export_kwh"`
	CheapExportRevPLN float64 `json:"cheap_export_rev_pln"`
	CurrentSpotPrice  float64 `json:"current_spot_price"`

	NMNetCostPLN    float64 `json:"nm_net_cost_pln"`
	NMCreditBankKWh float64 `json:"nm_credit_bank_kwh"`
	NBNetCostPLN    float64 `json:"nb_net_cost_pln"`
	NBDepositPLN    float64 `json:"nb_deposit_pln"`

	PreHeatCostPLN    float64             `json:"pre_heat_cost_pln"`
	PreHeatSavingsPLN float64             `json:"pre_heat_savings_pln"`
	PVArrayProduction []PVArrayProdPayload `json:"pv_array_production,omitempty"`
}

type PVArrayProdPayload struct {
	Name string  `json:"name"`
	KWh  float64 `json:"kwh"`
}

type SensorInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
	Unit string `json:"unit"`
}

type TimeRangeInfo struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

type DataLoadedPayload struct {
	Sensors   []SensorInfo  `json:"sensors"`
	TimeRange TimeRangeInfo `json:"time_range"`
}

// Message type constants
const (
	// Client -> Server
	TypeSimStart         = "sim:start"
	TypeSimPause         = "sim:pause"
	TypeSimSetSpeed      = "sim:set_speed"
	TypeSimSeek          = "sim:seek"
	TypeSimSetSource     = "sim:set_source"
	TypeBatteryConfig    = "battery:config"
	TypeSimSetPrediction = "sim:set_prediction"
	TypeConfigUpdate     = "config:update"
	TypePVConfig         = "pv:config"

	// Server -> Client
	TypeSimState              = "sim:state"
	TypeSensorReading         = "sensor:reading"
	TypeSummaryUpdate         = "summary:update"
	TypeDataLoaded            = "data:loaded"
	TypeBatteryUpdate         = "battery:update"
	TypeBatterySummary        = "battery:summary"
	TypeArbitrageDayLog       = "arbitrage:day_log"
	TypePredictionComparison  = "prediction:comparison"
	TypeHeatingStats          = "heating:stats"
	TypeAnomalyDays           = "anomaly:days"
	TypeLoadShiftStats        = "load_shift:stats"
	TypeHPDiagnostics         = "hp:diagnostics"
	TypePowerQuality          = "power:quality"
)

type SetPredictionPayload struct {
	Enabled bool `json:"enabled"`
}

type ConfigUpdatePayload struct {
	ExportCoefficient  float64 `json:"export_coefficient"`
	PriceThresholdPLN  float64 `json:"price_threshold_pln"`
	TempOffsetC        float64 `json:"temp_offset_c"`
	FixedTariffPLN     float64 `json:"fixed_tariff_pln"`
	DistributionFeePLN float64 `json:"distribution_fee_pln"`
	NetMeteringRatio   float64 `json:"net_metering_ratio"`
	InsulationLevel    string  `json:"insulation_level,omitempty"`
}

// PV config payloads

type PVConfigPayload struct {
	Enabled bool                   `json:"enabled"`
	Arrays  []PVArrayConfigPayload `json:"arrays"`
}

type PVArrayConfigPayload struct {
	Name    string  `json:"name"`
	PeakWp  float64 `json:"peak_wp"`
	Azimuth float64 `json:"azimuth"`
	Tilt    float64 `json:"tilt"`
	Enabled bool    `json:"enabled"`
}

type PredictionComparisonPayload struct {
	ActualPowerW    float64 `json:"actual_power_w"`
	PredictedPowerW float64 `json:"predicted_power_w"`
	ActualTempC     float64 `json:"actual_temp_c"`
	PredictedTempC  float64 `json:"predicted_temp_c"`
	HasActualTemp   bool    `json:"has_actual_temp"`
}

// Battery payloads

type BatteryConfigPayload struct {
	Enabled            bool    `json:"enabled"`
	CapacityKWh        float64 `json:"capacity_kwh"`
	MaxPowerW          float64 `json:"max_power_w"`
	DischargeToPercent float64 `json:"discharge_to_percent"`
	ChargeToPercent    float64 `json:"charge_to_percent"`
	DegradationCycles  float64 `json:"degradation_cycles"`
}

type BatteryUpdatePayload struct {
	BatteryPowerW float64 `json:"battery_power_w"`
	AdjustedGridW float64 `json:"adjusted_grid_w"`
	SoCPercent    float64 `json:"soc_percent"`
	Timestamp     string  `json:"timestamp"`
}

type BatterySummaryPayload struct {
	SoCPercent           float64                    `json:"soc_percent"`
	Cycles               float64                    `json:"cycles"`
	EffectiveCapacityKWh float64                    `json:"effective_capacity_kwh"`
	DegradationPct       float64                    `json:"degradation_pct"`
	TimeAtPowerSec       map[int]float64            `json:"time_at_power_sec"`
	TimeAtSoCPctSec      map[int]float64            `json:"time_at_soc_pct_sec"`
	MonthSoCSeconds      map[string]map[int]float64 `json:"month_soc_seconds"`
}

func NewEnvelope(msgType string, payload any) ([]byte, error) {
	var raw json.RawMessage
	if payload != nil {
		var err error
		raw, err = json.Marshal(payload)
		if err != nil {
			return nil, err
		}
	}
	return json.Marshal(Envelope{Type: msgType, Payload: raw})
}

func SimStateFromEngine(s simulator.State) SimStatePayload {
	return SimStatePayload{
		Time:    s.Time.Format("2006-01-02T15:04:05Z"),
		Speed:   s.Speed,
		Running: s.Running,
	}
}

type ArbitrageDayRecordPayload struct {
	Date               string  `json:"date"`
	ChargeStartTime    string  `json:"charge_start_time"`
	ChargeEndTime      string  `json:"charge_end_time"`
	ChargeKWh          float64 `json:"charge_kwh"`
	DischargeStartTime string  `json:"discharge_start_time"`
	DischargeEndTime   string  `json:"discharge_end_time"`
	DischargeKWh       float64 `json:"discharge_kwh"`
	GapMinutes         int     `json:"gap_minutes"`
	CyclesDelta        float64 `json:"cycles_delta"`
	EarningsPLN        float64 `json:"earnings_pln"`
}

type ArbitrageDayLogPayload struct {
	Records []ArbitrageDayRecordPayload `json:"records"`
}

func ArbitrageDayLogFromEngine(records []simulator.ArbitrageDayRecord) ArbitrageDayLogPayload {
	out := make([]ArbitrageDayRecordPayload, len(records))
	for i, r := range records {
		out[i] = ArbitrageDayRecordPayload{
			Date:               r.Date,
			ChargeStartTime:    r.ChargeStartTime,
			ChargeEndTime:      r.ChargeEndTime,
			ChargeKWh:          r.ChargeKWh,
			DischargeStartTime: r.DischargeStartTime,
			DischargeEndTime:   r.DischargeEndTime,
			DischargeKWh:       r.DischargeKWh,
			GapMinutes:         r.GapMinutes,
			CyclesDelta:        r.CyclesDelta,
			EarningsPLN:        r.EarningsPLN,
		}
	}
	return ArbitrageDayLogPayload{Records: out}
}

// Heating stats payloads

type HeatingMonthStatPayload struct {
	Month          string  `json:"month"`
	ConsumptionKWh float64 `json:"consumption_kwh"`
	ProductionKWh  float64 `json:"production_kwh"`
	COP            float64 `json:"cop"`
	CostPLN        float64 `json:"cost_pln"`
	AvgTempC       float64 `json:"avg_temp_c"`
}

func HeatingStatsFromEngine(stats []simulator.HeatingMonthStat) []HeatingMonthStatPayload {
	out := make([]HeatingMonthStatPayload, len(stats))
	for i, s := range stats {
		out[i] = HeatingMonthStatPayload{
			Month:          s.Month,
			ConsumptionKWh: s.ConsumptionKWh,
			ProductionKWh:  s.ProductionKWh,
			COP:            s.COP,
			CostPLN:        s.CostPLN,
			AvgTempC:       s.AvgTempC,
		}
	}
	return out
}

// Anomaly day payloads

type AnomalyDayPayload struct {
	Date         string  `json:"date"`
	ActualKWh    float64 `json:"actual_kwh"`
	PredictedKWh float64 `json:"predicted_kwh"`
	DeviationPct float64 `json:"deviation_pct"`
	AvgTempC     float64 `json:"avg_temp_c"`
}

func AnomalyDaysFromEngine(records []simulator.AnomalyDayRecord) []AnomalyDayPayload {
	out := make([]AnomalyDayPayload, len(records))
	for i, r := range records {
		out[i] = AnomalyDayPayload{
			Date:         r.Date,
			ActualKWh:    r.ActualKWh,
			PredictedKWh: r.PredictedKWh,
			DeviationPct: r.DeviationPct,
			AvgTempC:     r.AvgTempC,
		}
	}
	return out
}

func SummaryFromEngine(s simulator.Summary) SummaryPayload {
	return SummaryPayload{
		TodayKWh:           s.TodayKWh,
		MonthKWh:           s.MonthKWh,
		TotalKWh:           s.TotalKWh,
		GridImportKWh:      s.GridImportKWh,
		GridExportKWh:      s.GridExportKWh,
		PVProductionKWh:    s.PVProductionKWh,
		HeatPumpKWh:        s.HeatPumpKWh,
		HeatPumpProdKWh:    s.HeatPumpProdKWh,
		HeatPumpCostPLN:    s.HeatPumpCostPLN,
		SelfConsumptionKWh: s.SelfConsumptionKWh,
		HomeDemandKWh:      s.HomeDemandKWh,
		BatterySavingsKWh:  s.BatterySavingsKWh,

		GridImportCostPLN:       s.GridImportCostPLN,
		GridExportRevenuePLN:    s.GridExportRevenuePLN,
		NetCostPLN:              s.NetCostPLN,
		RawGridImportCostPLN:    s.RawGridImportCostPLN,
		RawGridExportRevenuePLN: s.RawGridExportRevenuePLN,
		RawNetCostPLN:           s.RawNetCostPLN,
		BatterySavingsPLN:       s.BatterySavingsPLN,

		ArbNetCostPLN:        s.ArbNetCostPLN,
		ArbBatterySavingsPLN: s.ArbBatterySavingsPLN,

		CheapExportKWh:    s.CheapExportKWh,
		CheapExportRevPLN: s.CheapExportRevPLN,
		CurrentSpotPrice:  s.CurrentSpotPrice,

		NMNetCostPLN:    s.NMNetCostPLN,
		NMCreditBankKWh: s.NMCreditBankKWh,
		NBNetCostPLN:    s.NBNetCostPLN,
		NBDepositPLN:    s.NBDepositPLN,

		PreHeatCostPLN:    s.PreHeatCostPLN,
		PreHeatSavingsPLN: s.PreHeatSavingsPLN,
		PVArrayProduction: pvArrayProdFromEngine(s.PVArrayProduction),
	}
}

func pvArrayProdFromEngine(prods []simulator.PVArrayProd) []PVArrayProdPayload {
	if len(prods) == 0 {
		return nil
	}
	out := make([]PVArrayProdPayload, len(prods))
	for i, p := range prods {
		out[i] = PVArrayProdPayload{Name: p.Name, KWh: p.KWh}
	}
	return out
}

// Load shift stats payloads

type LoadShiftHeatmapCell struct {
	KWh      float64 `json:"kwh"`
	AvgPrice float64 `json:"avg_price"`
}

type LoadShiftStatsPayload struct {
	Heatmap         [7][24]LoadShiftHeatmapCell `json:"heatmap"`
	AvgHPPrice      float64                     `json:"avg_hp_price"`
	OverallAvgPrice float64                     `json:"overall_avg_price"`
	ShiftCurrentPLN float64                     `json:"shift_current_pln"`
	ShiftOptimalPLN float64                     `json:"shift_optimal_pln"`
	ShiftSavingsPLN float64                     `json:"shift_savings_pln"`
	ShiftWindowH    int                         `json:"shift_window_h"`
}

// HP diagnostics payload

type HPDiagnosticsPayload struct {
	COP             float64 `json:"cop"`
	CompressorSpeed float64 `json:"compressor_speed_rpm"`
	FanSpeed        float64 `json:"fan_speed_rpm"`
	DischargeTemp   float64 `json:"discharge_temp_c"`
	HighPressure    float64 `json:"high_pressure"`
	PumpFlow        float64 `json:"pump_flow_lmin"`
	InletTemp       float64 `json:"inlet_temp_c"`
	OutletTemp      float64 `json:"outlet_temp_c"`
	ThermalPowerW   float64 `json:"thermal_power_w"`
	DHWTemp         float64 `json:"dhw_temp_c"`
	OutsidePipeTemp float64 `json:"outside_pipe_temp_c"`
	InsidePipeTemp  float64 `json:"inside_pipe_temp_c"`
	Z1TargetTemp    float64 `json:"z1_target_temp_c"`
}

func HPDiagnosticsFromEngine(d simulator.HPDiagnostics) HPDiagnosticsPayload {
	return HPDiagnosticsPayload{
		COP:             d.COP,
		CompressorSpeed: d.CompressorSpeed,
		FanSpeed:        d.FanSpeed,
		DischargeTemp:   d.DischargeTemp,
		HighPressure:    d.HighPressure,
		PumpFlow:        d.PumpFlow,
		InletTemp:       d.InletTemp,
		OutletTemp:      d.OutletTemp,
		ThermalPowerW:   d.ThermalPowerW,
		DHWTemp:         d.DHWTemp,
		OutsidePipeTemp: d.OutsidePipeTemp,
		InsidePipeTemp:  d.InsidePipeTemp,
		Z1TargetTemp:    d.Z1TargetTemp,
	}
}

// Power quality payload

type PowerQualityPayload struct {
	VoltageV         float64 `json:"voltage_v"`
	PowerFactorPct   float64 `json:"power_factor_pct"`
	ReactivePowerVAR float64 `json:"reactive_power_var"`
}

func PowerQualityFromEngine(pq simulator.PowerQuality) PowerQualityPayload {
	return PowerQualityPayload{
		VoltageV:         pq.VoltageV,
		PowerFactorPct:   pq.PowerFactorPct,
		ReactivePowerVAR: pq.ReactivePowerVAR,
	}
}

func LoadShiftStatsFromEngine(stats simulator.LoadShiftStats) LoadShiftStatsPayload {
	var heatmap [7][24]LoadShiftHeatmapCell
	for dow := 0; dow < 7; dow++ {
		for h := 0; h < 24; h++ {
			heatmap[dow][h] = LoadShiftHeatmapCell{
				KWh:      stats.Heatmap[dow][h].KWh,
				AvgPrice: stats.Heatmap[dow][h].AvgPrice,
			}
		}
	}
	return LoadShiftStatsPayload{
		Heatmap:         heatmap,
		AvgHPPrice:      stats.AvgHPPrice,
		OverallAvgPrice: stats.OverallAvgPrice,
		ShiftCurrentPLN: stats.ShiftCurrentPLN,
		ShiftOptimalPLN: stats.ShiftOptimalPLN,
		ShiftSavingsPLN: stats.ShiftSavingsPLN,
		ShiftWindowH:    stats.ShiftWindowH,
	}
}
