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
	TodayKWh float64 `json:"today_kwh"`
	MonthKWh float64 `json:"month_kwh"`
	TotalKWh float64 `json:"total_kwh"`
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
	TypeSimStart    = "sim:start"
	TypeSimPause    = "sim:pause"
	TypeSimSetSpeed = "sim:set_speed"
	TypeSimSeek     = "sim:seek"

	// Server -> Client
	TypeSimState      = "sim:state"
	TypeSensorReading = "sensor:reading"
	TypeSummaryUpdate = "summary:update"
	TypeDataLoaded    = "data:loaded"
)

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

func SummaryFromEngine(s simulator.Summary) SummaryPayload {
	return SummaryPayload{
		TodayKWh: s.TodayKWh,
		MonthKWh: s.MonthKWh,
		TotalKWh: s.TotalKWh,
	}
}
