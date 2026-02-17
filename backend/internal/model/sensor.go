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
	SensorGridPowerFactor    SensorType = "grid_power_factor"
	SensorGridPowerReactive  SensorType = "grid_power_reactive"
	SensorGridEnergyReactive SensorType = "grid_energy_reactive"
	SensorPumpHeaterRoom     SensorType = "pump_heater_room_hours"
	SensorPumpHeaterDHW      SensorType = "pump_heater_dhw_hours"
	SensorPumpFlow           SensorType = "pump_flow"
	SensorPumpDHWTemp        SensorType = "pump_dhw_temp"
	SensorPumpFanSpeed       SensorType = "pump_fan_speed"
	SensorPumpHighPressure   SensorType = "pump_high_pressure"
	SensorPumpCompressorSpeed SensorType = "pump_compressor_speed"
	SensorPumpDischargeTemp  SensorType = "pump_discharge_temp"
	SensorPumpOutsidePipe    SensorType = "pump_outside_pipe_temp"
	SensorPumpZ1TargetTemp   SensorType = "pump_z1_target_temp"
	SensorPumpCOP            SensorType = "pump_cop"
	SensorPumpInsidePipeTemp SensorType = "pump_inside_pipe_temp"
	// Indoor climate sensors
	SensorTempBedroom1       SensorType = "temp_bedroom1"
	SensorHumBedroom1        SensorType = "hum_bedroom1"
	SensorTempBedroom2       SensorType = "temp_bedroom2"
	SensorHumBedroom2        SensorType = "hum_bedroom2"
	SensorTempKitchen        SensorType = "temp_kitchen"
	SensorHumKitchen         SensorType = "hum_kitchen"
	SensorTempOffice1        SensorType = "temp_office1"
	SensorHumOffice1         SensorType = "hum_office1"
	SensorTempOffice2        SensorType = "temp_office2"
	SensorHumOffice2         SensorType = "hum_office2"
	SensorTempBathroom       SensorType = "temp_bathroom"
	SensorHumBathroom        SensorType = "hum_bathroom"
	SensorTempWorkshop       SensorType = "temp_workshop"
	SensorHumWorkshop        SensorType = "hum_workshop"
	SensorTempWorkshopExt    SensorType = "temp_workshop_ext"
	// Netatmo bedroom (remote module)
	SensorNetatmoTemp        SensorType = "netatmo_temp"
	SensorNetatmoHum         SensorType = "netatmo_hum"
	SensorNetatmoCO2         SensorType = "netatmo_co2"
	// Netatmo living room (base station)
	SensorNetatmoLivingTemp     SensorType = "netatmo_living_temp"
	SensorNetatmoLivingHum      SensorType = "netatmo_living_hum"
	SensorNetatmoLivingCO2      SensorType = "netatmo_living_co2"
	SensorNetatmoLivingPressure SensorType = "netatmo_living_pressure"
	SensorNetatmoLivingNoise    SensorType = "netatmo_living_noise"
	// Netatmo outdoor module
	SensorNetatmoOutdoorTemp    SensorType = "netatmo_outdoor_temp"
	SensorNetatmoOutdoorHum     SensorType = "netatmo_outdoor_hum"
	// Netatmo wind module
	SensorNetatmoWindSpeed      SensorType = "netatmo_wind_speed"
	SensorNetatmoWindAngle      SensorType = "netatmo_wind_angle"
	SensorNetatmoGustSpeed      SensorType = "netatmo_gust_speed"
	SensorNetatmoGustAngle      SensorType = "netatmo_gust_angle"
	// Netatmo rain gauge
	SensorNetatmoRain           SensorType = "netatmo_rain"
	// Per-circuit voltage sensors
	SensorVoltageOffice2     SensorType = "voltage_office2"
	SensorVoltageExternal    SensorType = "voltage_external"
	SensorVoltageOffice1     SensorType = "voltage_office1"
	SensorVoltageLivingLamp  SensorType = "voltage_living_lamp"
	SensorVoltageLivingMedia SensorType = "voltage_living_media"
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
	SensorEnergyPrice:      "sensor.spotprice_now",
	SensorGridVoltage:      "sensor.0x943469fffed2bf71_voltage",
	SensorGridPowerFactor:  "sensor.0x943469fffed2bf71_power_factor",
	SensorGridPowerReactive: "sensor.0x943469fffed2bf71_power_reactive",
	SensorGridEnergyReactive: "sensor.0x943469fffed2bf71_energy_reactive",
	SensorPumpHeaterRoom:   "sensor.panasonic_heat_pump_main_room_heater_operations_hours",
	SensorPumpHeaterDHW:    "sensor.panasonic_heat_pump_main_dhw_heater_operations_hours",
	SensorPumpFlow:         "sensor.panasonic_heat_pump_main_pump_flow",
	SensorPumpDHWTemp:      "sensor.panasonic_heat_pump_main_dhw_temp",
	SensorPumpFanSpeed:     "sensor.panasonic_heat_pump_main_fan1_motor_speed",
	SensorPumpHighPressure: "sensor.panasonic_heat_pump_main_high_pressure",
	SensorPumpCompressorSpeed: "sensor.panasonic_heat_pump_main_pump_speed",
	SensorPumpDischargeTemp: "sensor.panasonic_heat_pump_main_discharge_temp",
	SensorPumpOutsidePipe:  "sensor.panasonic_heat_pump_main_outside_pipe_temp",
	SensorPumpZ1TargetTemp: "sensor.panasonic_heat_pump_main_z1_water_target_temp",
	SensorPumpCOP:          "sensor.panasonic_heat_pump_cop",
	SensorPumpInsidePipeTemp: "sensor.panasonic_heat_pump_main_inside_pipe_temp",
	// Indoor climate
	SensorTempBedroom1:    "sensor.lozeczko_zosii_z2m_temperature",
	SensorHumBedroom1:     "sensor.lozeczko_zosii_z2m_humidity",
	SensorTempBedroom2:    "sensor.temperatura_pokoj_zosi_temperature",
	SensorHumBedroom2:     "sensor.temperatura_pokoj_zosi_humidity",
	SensorTempKitchen:     "sensor.temperatura_w_kuchni_temperature",
	SensorHumKitchen:      "sensor.temperatura_w_kuchni_humidity",
	SensorTempOffice1:     "sensor.termometr_olek_z2m_temperature",
	SensorHumOffice1:      "sensor.termometr_olek_z2m_humidity",
	SensorTempOffice2:     "sensor.termometr_beata_z2m_temperature",
	SensorHumOffice2:      "sensor.termometr_beata_z2m_humidity",
	SensorTempBathroom:    "sensor.termometr_lazienka_gorna_z2m_temperature",
	SensorHumBathroom:     "sensor.termometr_lazienka_gorna_z2m_humidity",
	SensorTempWorkshop:    "sensor.warsztat_termometr_temperature",
	SensorHumWorkshop:     "sensor.warsztat_termometr_humidity",
	SensorTempWorkshopExt: "sensor.warsztat_zewnatrz_termometr_temperature",
	// Netatmo bedroom
	SensorNetatmoTemp:     "sensor.unknown_70_ee_50_a9_6a_b8_sypialnia_temperature",
	SensorNetatmoHum:      "sensor.unknown_70_ee_50_a9_6a_b8_sypialnia_humidity",
	SensorNetatmoCO2:      "sensor.unknown_70_ee_50_a9_6a_b8_sypialnia_carbon_dioxide",
	// Netatmo living room
	SensorNetatmoLivingTemp:     "sensor.unknown_70_ee_50_a9_6a_b8_temperature",
	SensorNetatmoLivingHum:      "sensor.unknown_70_ee_50_a9_6a_b8_humidity",
	SensorNetatmoLivingCO2:      "sensor.unknown_70_ee_50_a9_6a_b8_carbon_dioxide",
	SensorNetatmoLivingPressure: "sensor.unknown_70_ee_50_a9_6a_b8_atmospheric_pressure",
	SensorNetatmoLivingNoise:    "sensor.unknown_70_ee_50_a9_6a_b8_noise",
	// Netatmo outdoor
	SensorNetatmoOutdoorTemp:    "sensor.unknown_70_ee_50_a9_6a_b8_na_zewnatrz_temperature",
	SensorNetatmoOutdoorHum:     "sensor.unknown_70_ee_50_a9_6a_b8_na_zewnatrz_humidity",
	// Netatmo wind
	SensorNetatmoWindSpeed:      "sensor.unknown_70_ee_50_a9_6a_b8_wiatr_zachod_wind_speed",
	SensorNetatmoWindAngle:      "sensor.unknown_70_ee_50_a9_6a_b8_wiatr_zachod_wind_angle",
	SensorNetatmoGustSpeed:      "sensor.unknown_70_ee_50_a9_6a_b8_wiatr_zachod_gust_strength",
	SensorNetatmoGustAngle:      "sensor.unknown_70_ee_50_a9_6a_b8_wiatr_zachod_gust_angle",
	// Netatmo rain
	SensorNetatmoRain:           "sensor.unknown_70_ee_50_a9_6a_b8_deszcz_precipitation",
	// Per-circuit voltage
	SensorVoltageOffice2:     "sensor.beata_biurko_voltage",
	SensorVoltageExternal:    "sensor.obciazenie_zewnetrzne_1_voltage",
	SensorVoltageOffice1:     "sensor.olek_tylne_cieple_oswietlenie_voltage",
	SensorVoltageLivingLamp:  "sensor.salon_oswietlenie_w_szafie_podschodowe_voltage",
	SensorVoltageLivingMedia: "sensor.salon_tv_i_media_voltage",
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
	SensorEnergyPrice:       {Name: "Energy Price", Unit: "PLN/kWh"},
	SensorGridVoltage:       {Name: "Grid Voltage", Unit: "V"},
	SensorGridPowerFactor:   {Name: "Power Factor", Unit: "%"},
	SensorGridPowerReactive: {Name: "Reactive Power", Unit: "VAR"},
	SensorGridEnergyReactive: {Name: "Reactive Energy", Unit: "kvarh"},
	SensorPumpHeaterRoom:    {Name: "Backup Heater Room Hours", Unit: "h"},
	SensorPumpHeaterDHW:     {Name: "Backup Heater DHW Hours", Unit: "h"},
	SensorPumpFlow:          {Name: "Pump Flow", Unit: "L/min"},
	SensorPumpDHWTemp:       {Name: "DHW Tank Temperature", Unit: "°C"},
	SensorPumpFanSpeed:      {Name: "Fan Speed", Unit: "R/min"},
	SensorPumpHighPressure:  {Name: "High Pressure", Unit: "Kgf/cm2"},
	SensorPumpCompressorSpeed: {Name: "Compressor Speed", Unit: "R/min"},
	SensorPumpDischargeTemp: {Name: "Discharge Temperature", Unit: "°C"},
	SensorPumpOutsidePipe:   {Name: "Outside Pipe Temperature", Unit: "°C"},
	SensorPumpZ1TargetTemp:  {Name: "Zone 1 Target Temperature", Unit: "°C"},
	SensorPumpCOP:           {Name: "Heat Pump COP", Unit: ""},
	SensorPumpInsidePipeTemp: {Name: "Inside Pipe Temperature", Unit: "°C"},
	// Indoor climate
	SensorTempBedroom1:    {Name: "Bedroom 1 Temperature", Unit: "°C"},
	SensorHumBedroom1:     {Name: "Bedroom 1 Humidity", Unit: "%"},
	SensorTempBedroom2:    {Name: "Bedroom 2 Temperature", Unit: "°C"},
	SensorHumBedroom2:     {Name: "Bedroom 2 Humidity", Unit: "%"},
	SensorTempKitchen:     {Name: "Kitchen Temperature", Unit: "°C"},
	SensorHumKitchen:      {Name: "Kitchen Humidity", Unit: "%"},
	SensorTempOffice1:     {Name: "Office 1 Temperature", Unit: "°C"},
	SensorHumOffice1:      {Name: "Office 1 Humidity", Unit: "%"},
	SensorTempOffice2:     {Name: "Office 2 Temperature", Unit: "°C"},
	SensorHumOffice2:      {Name: "Office 2 Humidity", Unit: "%"},
	SensorTempBathroom:    {Name: "Bathroom Temperature", Unit: "°C"},
	SensorHumBathroom:     {Name: "Bathroom Humidity", Unit: "%"},
	SensorTempWorkshop:    {Name: "Workshop Temperature", Unit: "°C"},
	SensorHumWorkshop:     {Name: "Workshop Humidity", Unit: "%"},
	SensorTempWorkshopExt: {Name: "Workshop Exterior Temperature", Unit: "°C"},
	// Netatmo bedroom
	SensorNetatmoTemp:     {Name: "Netatmo Bedroom Temperature", Unit: "°C"},
	SensorNetatmoHum:      {Name: "Netatmo Bedroom Humidity", Unit: "%"},
	SensorNetatmoCO2:      {Name: "Netatmo Bedroom CO2", Unit: "ppm"},
	// Netatmo living room
	SensorNetatmoLivingTemp:     {Name: "Living Room Temperature", Unit: "°C"},
	SensorNetatmoLivingHum:      {Name: "Living Room Humidity", Unit: "%"},
	SensorNetatmoLivingCO2:      {Name: "Living Room CO2", Unit: "ppm"},
	SensorNetatmoLivingPressure: {Name: "Atmospheric Pressure", Unit: "mbar"},
	SensorNetatmoLivingNoise:    {Name: "Living Room Noise", Unit: "dB"},
	// Netatmo outdoor
	SensorNetatmoOutdoorTemp:    {Name: "Netatmo Outdoor Temperature", Unit: "°C"},
	SensorNetatmoOutdoorHum:     {Name: "Netatmo Outdoor Humidity", Unit: "%"},
	// Netatmo wind
	SensorNetatmoWindSpeed:      {Name: "Wind Speed", Unit: "km/h"},
	SensorNetatmoWindAngle:      {Name: "Wind Angle", Unit: "°"},
	SensorNetatmoGustSpeed:      {Name: "Wind Gust Speed", Unit: "km/h"},
	SensorNetatmoGustAngle:      {Name: "Wind Gust Angle", Unit: "°"},
	// Netatmo rain
	SensorNetatmoRain:           {Name: "Precipitation", Unit: "mm"},
	// Per-circuit voltage
	SensorVoltageOffice2:     {Name: "Office 2 Voltage", Unit: "V"},
	SensorVoltageExternal:    {Name: "External Circuit Voltage", Unit: "V"},
	SensorVoltageOffice1:     {Name: "Office 1 Voltage", Unit: "V"},
	SensorVoltageLivingLamp:  {Name: "Living Room Lamp Voltage", Unit: "V"},
	SensorVoltageLivingMedia: {Name: "Living Room Media Voltage", Unit: "V"},
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
