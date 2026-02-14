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

	// Server -> Client
	TypeSimState              = "sim:state"
	TypeSensorReading         = "sensor:reading"
	TypeSummaryUpdate         = "summary:update"
	TypeDataLoaded            = "data:loaded"
	TypeBatteryUpdate         = "battery:update"
	TypeBatterySummary        = "battery:summary"
	TypeArbitrageDayLog       = "arbitrage:day_log"
	TypePredictionComparison  = "prediction:comparison"
)

type SetPredictionPayload struct {
	Enabled bool `json:"enabled"`
}

type ConfigUpdatePayload struct {
	ExportCoefficient float64 `json:"export_coefficient"`
	PriceThresholdPLN float64 `json:"price_threshold_pln"`
	TempOffsetC       float64 `json:"temp_offset_c"`
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
}

type BatteryUpdatePayload struct {
	BatteryPowerW float64 `json:"battery_power_w"`
	AdjustedGridW float64 `json:"adjusted_grid_w"`
	SoCPercent    float64 `json:"soc_percent"`
	Timestamp     string  `json:"timestamp"`
}

type BatterySummaryPayload struct {
	SoCPercent      float64                    `json:"soc_percent"`
	Cycles          float64                    `json:"cycles"`
	TimeAtPowerSec  map[int]float64            `json:"time_at_power_sec"`
	TimeAtSoCPctSec map[int]float64            `json:"time_at_soc_pct_sec"`
	MonthSoCSeconds map[string]map[int]float64 `json:"month_soc_seconds"`
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
	}
}
