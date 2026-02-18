package ws

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewEnvelope(t *testing.T) {
	payload := SimStatePayload{
		Time:    "2024-11-21T12:00:00Z",
		Speed:   10,
		Running: true,
	}

	msg, err := NewEnvelope(TypeSimState, payload)
	require.NoError(t, err)

	var env Envelope
	err = json.Unmarshal(msg, &env)
	require.NoError(t, err)

	assert.Equal(t, TypeSimState, env.Type)

	var parsed SimStatePayload
	err = json.Unmarshal(env.Payload, &parsed)
	require.NoError(t, err)

	assert.Equal(t, "2024-11-21T12:00:00Z", parsed.Time)
	assert.Equal(t, 10.0, parsed.Speed)
	assert.True(t, parsed.Running)
}

func TestNewEnvelope_NoPayload(t *testing.T) {
	msg, err := NewEnvelope(TypeSimStart, nil)
	require.NoError(t, err)

	var env Envelope
	err = json.Unmarshal(msg, &env)
	require.NoError(t, err)

	assert.Equal(t, TypeSimStart, env.Type)
	assert.Nil(t, env.Payload)
}

func TestHub_RegisterUnregister(t *testing.T) {
	hub := NewHub()

	c := &Client{
		hub:  hub,
		send: make(chan []byte, 16),
	}

	hub.Register(c)
	assert.Equal(t, 1, hub.ClientCount())

	hub.Unregister(c)
	assert.Equal(t, 0, hub.ClientCount())
}

func TestHub_Broadcast(t *testing.T) {
	hub := NewHub()

	c1 := &Client{hub: hub, send: make(chan []byte, 16)}
	c2 := &Client{hub: hub, send: make(chan []byte, 16)}

	hub.Register(c1)
	hub.Register(c2)

	msg := []byte(`{"type":"test"}`)
	hub.Broadcast(msg)

	assert.Equal(t, msg, <-c1.send)
	assert.Equal(t, msg, <-c2.send)
}

func TestMessageTypes(t *testing.T) {
	assert.Equal(t, "sim:start", TypeSimStart)
	assert.Equal(t, "sim:pause", TypeSimPause)
	assert.Equal(t, "sim:set_speed", TypeSimSetSpeed)
	assert.Equal(t, "sim:seek", TypeSimSeek)
	assert.Equal(t, "sim:state", TypeSimState)
	assert.Equal(t, "sensor:reading", TypeSensorReading)
	assert.Equal(t, "summary:update", TypeSummaryUpdate)
	assert.Equal(t, "data:loaded", TypeDataLoaded)
}
