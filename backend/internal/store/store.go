package store

import (
	"sort"
	"sync"
	"time"

	"energy_simulator/internal/model"
)

// Store holds sensor readings in memory, indexed by sensor ID.
type Store struct {
	mu       sync.RWMutex
	sensors  map[string]model.Sensor
	readings map[string][]model.Reading // keyed by sensor ID, sorted by timestamp
}

func New() *Store {
	return &Store{
		sensors:  make(map[string]model.Sensor),
		readings: make(map[string][]model.Reading),
	}
}

// AddSensor registers a sensor.
func (s *Store) AddSensor(sensor model.Sensor) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sensors[sensor.ID] = sensor
}

// AddReadings adds readings for a sensor, then sorts by timestamp.
func (s *Store) AddReadings(readings []model.Reading) {
	if len(readings) == 0 {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, r := range readings {
		s.readings[r.SensorID] = append(s.readings[r.SensorID], r)
	}

	// Sort each affected sensor's readings
	seen := make(map[string]bool)
	for _, r := range readings {
		if !seen[r.SensorID] {
			seen[r.SensorID] = true
			sort.Slice(s.readings[r.SensorID], func(i, j int) bool {
				return s.readings[r.SensorID][i].Timestamp.Before(s.readings[r.SensorID][j].Timestamp)
			})
		}
	}
}

// Sensors returns all registered sensors.
func (s *Store) Sensors() []model.Sensor {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sensors := make([]model.Sensor, 0, len(s.sensors))
	for _, sensor := range s.sensors {
		sensors = append(sensors, sensor)
	}
	return sensors
}

// ReadingCount returns the total number of readings for a sensor.
func (s *Store) ReadingCount(sensorID string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.readings[sensorID])
}

// TimeRange returns the time range covered by a sensor's readings.
func (s *Store) TimeRange(sensorID string) (model.TimeRange, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	readings := s.readings[sensorID]
	if len(readings) == 0 {
		return model.TimeRange{}, false
	}

	return model.TimeRange{
		Start: readings[0].Timestamp,
		End:   readings[len(readings)-1].Timestamp,
	}, true
}

// GlobalTimeRange returns the union of all sensors' time ranges.
func (s *Store) GlobalTimeRange() (model.TimeRange, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var start, end time.Time
	first := true

	for _, readings := range s.readings {
		if len(readings) == 0 {
			continue
		}
		rStart := readings[0].Timestamp
		rEnd := readings[len(readings)-1].Timestamp

		if first || rStart.Before(start) {
			start = rStart
		}
		if first || rEnd.After(end) {
			end = rEnd
		}
		first = false
	}

	if first {
		return model.TimeRange{}, false
	}
	return model.TimeRange{Start: start, End: end}, true
}

// ReadingsInRange returns readings for a sensor between start (inclusive) and end (exclusive).
func (s *Store) ReadingsInRange(sensorID string, start, end time.Time) []model.Reading {
	s.mu.RLock()
	defer s.mu.RUnlock()

	all := s.readings[sensorID]
	if len(all) == 0 {
		return nil
	}

	// Binary search for start index
	startIdx := sort.Search(len(all), func(i int) bool {
		return !all[i].Timestamp.Before(start)
	})

	// Binary search for end index
	endIdx := sort.Search(len(all), func(i int) bool {
		return !all[i].Timestamp.Before(end)
	})

	if startIdx >= endIdx {
		return nil
	}

	result := make([]model.Reading, endIdx-startIdx)
	copy(result, all[startIdx:endIdx])
	return result
}

// ReadingAt returns the most recent reading at or before the given timestamp.
func (s *Store) ReadingAt(sensorID string, t time.Time) (model.Reading, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	all := s.readings[sensorID]
	if len(all) == 0 {
		return model.Reading{}, false
	}

	// Find first reading after t
	idx := sort.Search(len(all), func(i int) bool {
		return all[i].Timestamp.After(t)
	})

	if idx == 0 {
		return model.Reading{}, false
	}

	return all[idx-1], true
}
