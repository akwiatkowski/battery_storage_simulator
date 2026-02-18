package ws

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"energy_simulator/internal/model"
	"energy_simulator/internal/simulator"
	"energy_simulator/internal/store"
)

// testEngine creates a store, mock callback, and initialized engine for handler tests.
func testEngine() (*simulator.Engine, *store.Store) {
	s := store.New()
	s.AddSensor(model.Sensor{
		ID:   "sensor.grid",
		Name: "Grid Power",
		Type: model.SensorGridPower,
		Unit: "W",
	})

	base := time.Date(2024, 11, 21, 12, 0, 0, 0, time.UTC)
	readings := make([]model.Reading, 5)
	for i := range readings {
		readings[i] = model.Reading{
			Timestamp: base.Add(time.Duration(i) * time.Hour),
			SensorID:  "sensor.grid",
			Type:      model.SensorGridPower,
			Value:     float64(100 * (i + 1)),
			Unit:      "W",
		}
	}
	s.AddReadings(readings)

	bridge := NewBridge(NewHub()) // separate hub, not used for client reads
	engine := simulator.New(s, bridge)
	engine.Init()
	return engine, s
}

// dialHandler sets up a test server with the handler and returns a WS connection.
func dialHandler(t *testing.T, handler *Handler) (*websocket.Conn, func()) {
	t.Helper()
	server := httptest.NewServer(handler)
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	return conn, func() {
		conn.Close()
		server.Close()
	}
}

// readJSON reads the next JSON message from the connection.
func readJSON(t *testing.T, conn *websocket.Conn) Envelope {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err := conn.ReadMessage()
	require.NoError(t, err)
	var env Envelope
	require.NoError(t, json.Unmarshal(msg, &env))
	return env
}

// sendJSON sends a JSON message on the connection.
func sendJSON(t *testing.T, conn *websocket.Conn, msgType string, payload any) {
	t.Helper()
	data, err := NewEnvelope(msgType, payload)
	require.NoError(t, err)
	require.NoError(t, conn.WriteMessage(websocket.TextMessage, data))
}

func TestHandler_InitialMessages(t *testing.T) {
	engine, _ := testEngine()
	hub := NewHub()
	sourceRanges := map[string]model.TimeRange{
		"all": engine.TimeRange(),
	}
	handler := NewHandler(hub, engine, sourceRanges)

	conn, cleanup := dialHandler(t, handler)
	defer cleanup()

	// First message should be data:loaded
	env1 := readJSON(t, conn)
	assert.Equal(t, TypeDataLoaded, env1.Type)

	var dl DataLoadedPayload
	require.NoError(t, json.Unmarshal(env1.Payload, &dl))
	assert.NotEmpty(t, dl.Sensors)
	assert.NotEmpty(t, dl.TimeRange.Start)
	assert.NotEmpty(t, dl.TimeRange.End)

	// Second message should be sim:state
	env2 := readJSON(t, conn)
	assert.Equal(t, TypeSimState, env2.Type)

	var ss SimStatePayload
	require.NoError(t, json.Unmarshal(env2.Payload, &ss))
	assert.False(t, ss.Running)
	assert.Equal(t, 3600.0, ss.Speed)
}

func TestHandler_StartPause(t *testing.T) {
	engine, _ := testEngine()
	hub := NewHub()
	handler := NewHandler(hub, engine, map[string]model.TimeRange{"all": engine.TimeRange()})

	conn, cleanup := dialHandler(t, handler)
	defer cleanup()

	// Drain initial messages
	readJSON(t, conn) // data:loaded
	readJSON(t, conn) // sim:state

	// Send start
	sendJSON(t, conn, TypeSimStart, nil)

	// Should get a sim:state with running=true (broadcast)
	// Wait briefly for the engine to process
	time.Sleep(50 * time.Millisecond)

	// Send pause
	sendJSON(t, conn, TypeSimPause, nil)
	time.Sleep(50 * time.Millisecond)

	state := engine.State()
	assert.False(t, state.Running)
}

func TestHandler_SetSpeed(t *testing.T) {
	engine, _ := testEngine()
	hub := NewHub()
	handler := NewHandler(hub, engine, map[string]model.TimeRange{"all": engine.TimeRange()})

	conn, cleanup := dialHandler(t, handler)
	defer cleanup()

	// Drain initial messages
	readJSON(t, conn)
	readJSON(t, conn)

	sendJSON(t, conn, TypeSimSetSpeed, SetSpeedPayload{Speed: 7200})
	time.Sleep(50 * time.Millisecond)

	assert.Equal(t, 7200.0, engine.State().Speed)
}

func TestHandler_Seek(t *testing.T) {
	engine, _ := testEngine()
	hub := NewHub()
	handler := NewHandler(hub, engine, map[string]model.TimeRange{"all": engine.TimeRange()})

	conn, cleanup := dialHandler(t, handler)
	defer cleanup()

	readJSON(t, conn)
	readJSON(t, conn)

	target := time.Date(2024, 11, 21, 14, 0, 0, 0, time.UTC)
	sendJSON(t, conn, TypeSimSeek, SeekPayload{Timestamp: target.Format(time.RFC3339)})
	time.Sleep(50 * time.Millisecond)

	assert.Equal(t, target, engine.State().Time)
}

func TestHandler_SetSource(t *testing.T) {
	engine, _ := testEngine()
	hub := NewHub()

	tr := engine.TimeRange()
	// Create a smaller "current" range
	currentRange := model.TimeRange{
		Start: tr.Start.Add(time.Hour),
		End:   tr.End,
	}
	sourceRanges := map[string]model.TimeRange{
		"all":     tr,
		"current": currentRange,
	}
	handler := NewHandler(hub, engine, sourceRanges)

	conn, cleanup := dialHandler(t, handler)
	defer cleanup()

	readJSON(t, conn)
	readJSON(t, conn)

	sendJSON(t, conn, TypeSimSetSource, SetSourcePayload{Source: "current"})
	time.Sleep(50 * time.Millisecond)

	assert.Equal(t, currentRange.Start, engine.TimeRange().Start)
	assert.Equal(t, currentRange.End, engine.TimeRange().End)
}

func TestHandler_BatteryConfig(t *testing.T) {
	engine, _ := testEngine()
	hub := NewHub()
	handler := NewHandler(hub, engine, map[string]model.TimeRange{"all": engine.TimeRange()})

	conn, cleanup := dialHandler(t, handler)
	defer cleanup()

	readJSON(t, conn)
	readJSON(t, conn)

	sendJSON(t, conn, TypeBatteryConfig, BatteryConfigPayload{
		Enabled:            true,
		CapacityKWh:        10,
		MaxPowerW:          5000,
		DischargeToPercent: 10,
		ChargeToPercent:    90,
	})
	time.Sleep(50 * time.Millisecond)

	// Engine should be seeked to start (battery config resets sim)
	assert.Equal(t, engine.TimeRange().Start, engine.State().Time)
}

func TestHandler_BatteryDisable(t *testing.T) {
	engine, _ := testEngine()
	hub := NewHub()
	handler := NewHandler(hub, engine, map[string]model.TimeRange{"all": engine.TimeRange()})

	conn, cleanup := dialHandler(t, handler)
	defer cleanup()

	readJSON(t, conn)
	readJSON(t, conn)

	// Enable first
	sendJSON(t, conn, TypeBatteryConfig, BatteryConfigPayload{
		Enabled:     true,
		CapacityKWh: 10,
		MaxPowerW:   5000,
	})
	time.Sleep(50 * time.Millisecond)

	// Disable
	sendJSON(t, conn, TypeBatteryConfig, BatteryConfigPayload{
		Enabled: false,
	})
	time.Sleep(50 * time.Millisecond)

	assert.Equal(t, engine.TimeRange().Start, engine.State().Time)
}

func TestHandler_InvalidMessage(t *testing.T) {
	engine, _ := testEngine()
	hub := NewHub()
	handler := NewHandler(hub, engine, map[string]model.TimeRange{"all": engine.TimeRange()})

	conn, cleanup := dialHandler(t, handler)
	defer cleanup()

	readJSON(t, conn)
	readJSON(t, conn)

	// Send invalid JSON — should not crash
	require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte("not json")))
	time.Sleep(50 * time.Millisecond)

	// Connection should still be alive; engine state unchanged
	assert.False(t, engine.State().Running)
}

func TestHandler_UnknownSource(t *testing.T) {
	engine, _ := testEngine()
	hub := NewHub()
	handler := NewHandler(hub, engine, map[string]model.TimeRange{"all": engine.TimeRange()})

	conn, cleanup := dialHandler(t, handler)
	defer cleanup()

	readJSON(t, conn)
	readJSON(t, conn)

	origRange := engine.TimeRange()

	// Send unknown source — should be ignored
	sendJSON(t, conn, TypeSimSetSource, SetSourcePayload{Source: "nonexistent"})
	time.Sleep(50 * time.Millisecond)

	// Time range should not have changed
	assert.Equal(t, origRange, engine.TimeRange())
}

func TestHandler_InvalidSeekTimestamp(t *testing.T) {
	engine, _ := testEngine()
	hub := NewHub()
	handler := NewHandler(hub, engine, map[string]model.TimeRange{"all": engine.TimeRange()})

	conn, cleanup := dialHandler(t, handler)
	defer cleanup()

	readJSON(t, conn)
	readJSON(t, conn)

	origTime := engine.State().Time

	sendJSON(t, conn, TypeSimSeek, SeekPayload{Timestamp: "not-a-timestamp"})
	time.Sleep(50 * time.Millisecond)

	assert.Equal(t, origTime, engine.State().Time)
}

func TestHandler_DataLoadedPayloadContent(t *testing.T) {
	engine, _ := testEngine()
	hub := NewHub()
	handler := NewHandler(hub, engine, map[string]model.TimeRange{"all": engine.TimeRange()})

	// Use ServeHTTP directly for more control
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler.ServeHTTP(w, r)
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer conn.Close()

	// Read data:loaded
	env := readJSON(t, conn)
	require.Equal(t, TypeDataLoaded, env.Type)

	var dl DataLoadedPayload
	require.NoError(t, json.Unmarshal(env.Payload, &dl))

	// Should have our grid sensor
	found := false
	for _, s := range dl.Sensors {
		if s.ID == "sensor.grid" {
			assert.Equal(t, "Grid Power", s.Name)
			assert.Equal(t, "grid_power", s.Type)
			assert.Equal(t, "W", s.Unit)
			found = true
		}
	}
	assert.True(t, found, "grid sensor should be in data:loaded payload")
}
