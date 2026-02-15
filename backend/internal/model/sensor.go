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
	SensorEnergyPrice        SensorType = "energy_price"
	SensorGridVoltage        SensorType = "grid_voltage"
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
	SensorEnergyPrice:     "sensor.spotprice_now",
	SensorGridVoltage:     "sensor.0x943469fffed2bf71_voltage",
}

// HAEntityToSensorType is the reverse of SensorHomeAssistantID.
var HAEntityToSensorType map[string]SensorType

func init() {
	HAEntityToSensorType = make(map[string]SensorType, len(SensorHomeAssistantID))
	for st, entity := range SensorHomeAssistantID {
		HAEntityToSensorType[entity] = st
	}
}

// SensorInfo holds display name and unit for a sensor type.
type SensorInfo struct {
	Name string
	Unit string
}

// SensorCatalog maps every known SensorType to its display name and unit.
var SensorCatalog = map[SensorType]SensorInfo{
	SensorGridPower:       {Name: "Grid Power", Unit: "W"},
	SensorPVPower:         {Name: "PV Power", Unit: "W"},
	SensorPumpHeatPower:   {Name: "Heat Pump Heating Power", Unit: "W"},
	SensorPumpCWUPower:    {Name: "Heat Pump DHW Power", Unit: "W"},
	SensorPumpConsumption: {Name: "Heat Pump Consumption", Unit: "W"},
	SensorPumpProduction:  {Name: "Heat Pump Production", Unit: "W"},
	SensorPumpExtTemp:     {Name: "Outside Temperature", Unit: "°C"},
	SensorPumpInletTemp:   {Name: "Inlet Temperature", Unit: "°C"},
	SensorPumpOutletTemp:  {Name: "Outlet Temperature", Unit: "°C"},
	SensorPumpZone1Temp:   {Name: "Zone 1 Temperature", Unit: "°C"},
	SensorElectricKettle:  {Name: "Electric Kettle", Unit: "W"},
	SensorOven:            {Name: "Oven", Unit: "W"},
	SensorWashing:         {Name: "Washing Machine", Unit: "W"},
	SensorDrier:           {Name: "Drier", Unit: "W"},
	SensorTvMedia:         {Name: "TV & Media", Unit: "W"},
	SensorOlek1:           {Name: "Olek Desk", Unit: "W"},
	SensorOlek2:           {Name: "Olek Closet", Unit: "W"},
	SensorBeata:           {Name: "Beata Desk", Unit: "W"},
	SensorNetwork:         {Name: "Network", Unit: "W"},
	SensorExternal:        {Name: "External Power", Unit: "W"},
	SensorEnergyPrice:     {Name: "Energy Price", Unit: "PLN/kWh"},
	SensorGridVoltage:     {Name: "Grid Voltage", Unit: "V"},
}

type Reading struct {
	Timestamp time.Time
	SensorID  string
	Type      SensorType
	Value     float64
	Min       float64
	Max       float64
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
