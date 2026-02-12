package model

import "time"

type SensorType string

const (
	SensorGridPower          SensorType = "grid_power"
	SensorPVPower            SensorType = "pv_power"
	SensorPumpHeatPower      SensorType = "pump_heat_power_consumed"
	SensorPumpCWUPower       SensorType = "pump_cwu_power_consumed"
	SensorPumpConsumption    SensorType = "pump_total_consumption"
	SensorPumpProduction     SensorType = "pump_total_production"
	SensorPumpExtTemp        SensorType = "pump_ext_temp"
	SensorPumpInletTemp      SensorType = "pump_inlet_temp"
	SensorPumpOutletTemp     SensorType = "pump_outlet_temp"
	SensorPumpZone1Temp      SensorType = "pump_zone1_temp"
	SensorElectricKettle     SensorType = "electric_kettle"
	SensorOven               SensorType = "oven"
	SensorWashing            SensorType = "washing"
	SensorDrier              SensorType = "drier"
	SensorTvMedia            SensorType = "tv_media"
	SensorOlek1              SensorType = "olek1"
	SensorOlek2              SensorType = "olek2"
	SensorBeata              SensorType = "beata"
	SensorNetwork            SensorType = "network"
	SensorExternal           SensorType = "external"
)

// SensorHomeAssistantID maps our sensor slugs to Home Assistant entity IDs.
var SensorHomeAssistantID = map[SensorType]string{
	SensorGridPower:       "sensor.0x943469fffed2bf71_power",
	SensorPVPower:         "sensor.hoymiles_gateway_solarh_3054300_real_power",
	SensorPumpHeatPower:   "sensor.panasonic_heat_pump_main_heat_power_consumption",
	SensorPumpCWUPower:    "sensor.panasonic_heat_pump_main_dhw_power_consumption",
	SensorPumpConsumption: "sensor.panasonic_heat_pump_consumption",
	SensorPumpProduction:  "sensor.panasonic_heat_pump_production",
	SensorPumpExtTemp:     "sensor.panasonic_heat_pump_main_outside_temp",
	SensorPumpInletTemp:   "sensor.panasonic_heat_pump_main_main_inlet_temp",
	SensorPumpOutletTemp:  "sensor.panasonic_heat_pump_main_main_outlet_temp",
	SensorPumpZone1Temp:   "sensor.panasonic_heat_pump_main_z1_temp",
	SensorElectricKettle:  "sensor.59_power",
	SensorOven:            "sensor.piekarnik_z2m_power",
	SensorWashing:         "sensor.pralka_z2m_power",
	SensorDrier:           "sensor.70_power",
	SensorTvMedia:         "sensor.salon_tv_i_media_power",
	SensorOlek1:           "sensor.olek_zasilanie_biurka_68_power",
	SensorOlek2:           "sensor.zasilanie_szafy_olek_z2m_power",
	SensorBeata:           "sensor.beata_biurko_power",
	SensorNetwork:         "sensor.siec_z2m_power",
	SensorExternal:        "sensor.moc_do_arka",
}

type Reading struct {
	Timestamp time.Time
	SensorID  string
	Type      SensorType
	Value     float64
	Unit      string
}

type Sensor struct {
	ID   string
	Name string
	Type SensorType
	Unit string
}

type TimeRange struct {
	Start time.Time
	End   time.Time
}
