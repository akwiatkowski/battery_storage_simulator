package ws

import (
	"log"

	"energy_simulator/internal/simulator"
)

// Bridge implements simulator.Callback and broadcasts events to the WebSocket hub.
type Bridge struct {
	hub *Hub
}

func NewBridge(hub *Hub) *Bridge {
	return &Bridge{hub: hub}
}

func (b *Bridge) OnState(s simulator.State) {
	msg, err := NewEnvelope(TypeSimState, SimStateFromEngine(s))
	if err != nil {
		log.Printf("Error marshaling sim state: %v", err)
		return
	}
	b.hub.Broadcast(msg)
}

func (b *Bridge) OnReading(r simulator.SensorReading) {
	msg, err := NewEnvelope(TypeSensorReading, SensorReadingPayload{
		SensorID:  r.SensorID,
		Value:     r.Value,
		Unit:      r.Unit,
		Timestamp: r.Timestamp,
	})
	if err != nil {
		log.Printf("Error marshaling sensor reading: %v", err)
		return
	}
	b.hub.Broadcast(msg)
}

func (b *Bridge) OnSummary(s simulator.Summary) {
	msg, err := NewEnvelope(TypeSummaryUpdate, SummaryFromEngine(s))
	if err != nil {
		log.Printf("Error marshaling summary: %v", err)
		return
	}
	b.hub.Broadcast(msg)
}
