package ingest

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"energy_simulator/internal/model"
)

// HomeAssistantParser parses Home Assistant CSV exports.
//
// Expected format:
//
//	entity_id,state,last_changed
//	sensor.xxx_power,759.59,2024-11-21T13:00:00.000Z
type HomeAssistantParser struct {
	// SensorType to assign to parsed readings.
	SensorType model.SensorType
	// Unit for the sensor values (e.g. "W" for power).
	Unit string
}

func NewHomeAssistantParser(sensorType model.SensorType, unit string) *HomeAssistantParser {
	return &HomeAssistantParser{
		SensorType: sensorType,
		Unit:       unit,
	}
}

func (p *HomeAssistantParser) Parse(r io.Reader) ([]model.Reading, error) {
	cr := csv.NewReader(r)

	// Read header
	header, err := cr.Read()
	if err != nil {
		return nil, fmt.Errorf("reading CSV header: %w", err)
	}
	if err := validateHeader(header); err != nil {
		return nil, err
	}

	var readings []model.Reading
	lineNum := 1 // header was line 1

	for {
		lineNum++
		record, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading CSV line %d: %w", lineNum, err)
		}

		reading, err := p.parseRecord(record, lineNum)
		if err != nil {
			// Skip unparseable rows (e.g. "unavailable" state)
			continue
		}

		readings = append(readings, reading)
	}

	return readings, nil
}

func validateHeader(header []string) error {
	if len(header) < 3 {
		return fmt.Errorf("expected at least 3 columns, got %d", len(header))
	}

	expected := []string{"entity_id", "state", "last_changed"}
	for i, col := range expected {
		if strings.TrimSpace(header[i]) != col {
			return fmt.Errorf("expected column %d to be %q, got %q", i, col, header[i])
		}
	}

	return nil
}

func (p *HomeAssistantParser) parseRecord(record []string, lineNum int) (model.Reading, error) {
	if len(record) < 3 {
		return model.Reading{}, fmt.Errorf("line %d: expected 3 fields, got %d", lineNum, len(record))
	}

	entityID := strings.TrimSpace(record[0])

	value, err := strconv.ParseFloat(strings.TrimSpace(record[1]), 64)
	if err != nil {
		return model.Reading{}, fmt.Errorf("line %d: parsing value %q: %w", lineNum, record[1], err)
	}

	ts, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(record[2]))
	if err != nil {
		// Try alternate formats
		ts, err = time.Parse("2006-01-02T15:04:05.000Z", strings.TrimSpace(record[2]))
		if err != nil {
			return model.Reading{}, fmt.Errorf("line %d: parsing timestamp %q: %w", lineNum, record[2], err)
		}
	}

	return model.Reading{
		Timestamp: ts,
		SensorID:  entityID,
		Type:      p.SensorType,
		Value:     value,
		Min:       value,
		Max:       value,
		Unit:      p.Unit,
	}, nil
}
