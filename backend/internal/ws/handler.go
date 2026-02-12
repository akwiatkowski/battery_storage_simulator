package ws

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"

	"energy_simulator/internal/simulator"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Handler manages WebSocket connections and routes messages to the engine.
type Handler struct {
	hub    *Hub
	engine *simulator.Engine
}

func NewHandler(hub *Hub, engine *simulator.Engine) *Handler {
	return &Handler{hub: hub, engine: engine}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	client := &Client{
		hub:  h.hub,
		conn: conn,
		send: make(chan []byte, 256),
	}

	h.hub.Register(client)
	go client.writePump()

	// Send initial data:loaded message
	h.sendDataLoaded(client)

	// Send current sim state
	h.sendSimState(client)

	// Read messages from client
	h.readPump(client)
}

func (h *Handler) readPump(c *Client) {
	defer func() {
		h.hub.Unregister(c)
		c.conn.Close()
	}()

	for {
		_, msg, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				log.Printf("WebSocket read error: %v", err)
			}
			return
		}

		h.handleMessage(msg)
	}
}

func (h *Handler) handleMessage(msg []byte) {
	var env Envelope
	if err := json.Unmarshal(msg, &env); err != nil {
		log.Printf("Invalid message: %v", err)
		return
	}

	switch env.Type {
	case TypeSimStart:
		h.engine.Start()

	case TypeSimPause:
		h.engine.Pause()

	case TypeSimSetSpeed:
		var p SetSpeedPayload
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			log.Printf("Invalid set_speed payload: %v", err)
			return
		}
		h.engine.SetSpeed(p.Speed)

	case TypeSimSeek:
		var p SeekPayload
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			log.Printf("Invalid seek payload: %v", err)
			return
		}
		t, err := time.Parse(time.RFC3339, p.Timestamp)
		if err != nil {
			log.Printf("Invalid seek timestamp: %v", err)
			return
		}
		h.engine.Seek(t)

	case TypeBatteryConfig:
		var p BatteryConfigPayload
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			log.Printf("Invalid battery config payload: %v", err)
			return
		}
		if p.Enabled {
			cfg := &simulator.BatteryConfig{
				CapacityKWh:        p.CapacityKWh,
				MaxPowerW:          p.MaxPowerW,
				DischargeToPercent: p.DischargeToPercent,
				ChargeToPercent:    p.ChargeToPercent,
			}
			h.engine.SetBattery(cfg)
		} else {
			h.engine.SetBattery(nil)
		}
		// Reset simulation to apply battery from the start
		h.engine.Seek(h.engine.TimeRange().Start)

	default:
		log.Printf("Unknown message type: %s", env.Type)
	}
}

func (h *Handler) sendDataLoaded(c *Client) {
	tr := h.engine.TimeRange()
	modelSensors := h.engine.Sensors()
	sensors := make([]SensorInfo, 0, len(modelSensors))
	for _, s := range modelSensors {
		sensors = append(sensors, SensorInfo{
			ID:   s.ID,
			Name: s.Name,
			Type: string(s.Type),
			Unit: s.Unit,
		})
	}

	payload := DataLoadedPayload{
		Sensors: sensors,
		TimeRange: TimeRangeInfo{
			Start: tr.Start.Format(time.RFC3339),
			End:   tr.End.Format(time.RFC3339),
		},
	}

	msg, err := NewEnvelope(TypeDataLoaded, payload)
	if err != nil {
		log.Printf("Error creating data:loaded message: %v", err)
		return
	}

	select {
	case c.send <- msg:
	default:
	}
}

func (h *Handler) sendSimState(c *Client) {
	state := h.engine.State()
	msg, err := NewEnvelope(TypeSimState, SimStateFromEngine(state))
	if err != nil {
		return
	}
	select {
	case c.send <- msg:
	default:
	}
}
