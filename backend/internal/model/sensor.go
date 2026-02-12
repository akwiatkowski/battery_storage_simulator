package model

import "time"

type SensorType string

const (
	SensorGridPower       SensorType = "grid_power"
	SensorPVPower         SensorType = "pv_power"
	SensorPumpConsumption SensorType = "pump_total_consumption"
	SensorPumpProduction  SensorType = "pump_total_production"
)

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
