package ingest

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"

	"energy_simulator/internal/model"
)

// RecentParser parses Home Assistant recent measurements CSV exports.
//
// Expected format:
//
//	sensor_id,value,updated_ts
//	sensor.xxx_power,-341,1770896300.6877737
type RecentParser struct{}

func (p *RecentParser) Parse(r io.Reader) ([]model.Reading, error) {
	cr := csv.NewReader(r)

	header, err := cr.Read()
	if err != nil {
		return nil, fmt.Errorf("reading CSV header: %w", err)
	}
	if err := validateRecentHeader(header); err != nil {
		return nil, err
	}

	var readings []model.Reading
	lineNum := 1

	for {
		lineNum++
		record, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading CSV line %d: %w", lineNum, err)
		}

		reading, err := parseRecentRecord(record, lineNum)
		if err != nil {
			continue
		}

		readings = append(readings, reading)
	}

	return readings, nil
}

func validateRecentHeader(header []string) error {
	if len(header) < 3 {
		return fmt.Errorf("expected at least 3 columns, got %d", len(header))
	}

	expected := []string{"sensor_id", "value", "updated_ts"}
	for i, col := range expected {
		if strings.TrimSpace(header[i]) != col {
			return fmt.Errorf("expected column %d to be %q, got %q", i, col, header[i])
		}
	}

	return nil
}

func parseRecentRecord(record []string, lineNum int) (model.Reading, error) {
	if len(record) < 3 {
		return model.Reading{}, fmt.Errorf("line %d: expected 3 fields, got %d", lineNum, len(record))
	}

	entityID := strings.TrimSpace(record[0])
	sensorType, ok := model.HAEntityToSensorType[entityID]
	if !ok {
		return model.Reading{}, fmt.Errorf("line %d: unknown entity %q", lineNum, entityID)
	}

	value, err := strconv.ParseFloat(strings.TrimSpace(record[1]), 64)
	if err != nil {
		return model.Reading{}, fmt.Errorf("line %d: parsing value: %w", lineNum, err)
	}

	ts, err := parseUnixTimestamp(strings.TrimSpace(record[2]))
	if err != nil {
		return model.Reading{}, fmt.Errorf("line %d: parsing timestamp: %w", lineNum, err)
	}

	info := model.SensorCatalog[sensorType]

	return model.Reading{
		Timestamp: ts,
		SensorID:  entityID,
		Type:      sensorType,
		Value:     value,
		Min:       value,
		Max:       value,
		Unit:      info.Unit,
	}, nil
}
