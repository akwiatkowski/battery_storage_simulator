package ingest

import (
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"time"

	"energy_simulator/internal/model"
)

// StatsParser parses Home Assistant long-term statistics CSV exports.
//
// Expected format:
//
//	sensor_id,start_time,avg,min_val,max_val
//	sensor.xxx_power,1732186800.0,-368.85,-810.0,-162.0
type StatsParser struct{}

func (p *StatsParser) Parse(r io.Reader) ([]model.Reading, error) {
	cr := csv.NewReader(r)

	header, err := cr.Read()
	if err != nil {
		return nil, fmt.Errorf("reading CSV header: %w", err)
	}
	if err := validateStatsHeader(header); err != nil {
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

		reading, err := parseStatsRecord(record, lineNum)
		if err != nil {
			continue
		}

		readings = append(readings, reading)
	}

	return readings, nil
}

func validateStatsHeader(header []string) error {
	if len(header) < 5 {
		return fmt.Errorf("expected at least 5 columns, got %d", len(header))
	}

	expected := []string{"sensor_id", "start_time", "avg", "min_val", "max_val"}
	for i, col := range expected {
		if strings.TrimSpace(header[i]) != col {
			return fmt.Errorf("expected column %d to be %q, got %q", i, col, header[i])
		}
	}

	return nil
}

func parseStatsRecord(record []string, lineNum int) (model.Reading, error) {
	if len(record) < 5 {
		return model.Reading{}, fmt.Errorf("line %d: expected 5 fields, got %d", lineNum, len(record))
	}

	entityID := strings.TrimSpace(record[0])
	sensorType, ok := model.HAEntityToSensorType[entityID]
	if !ok {
		return model.Reading{}, fmt.Errorf("line %d: unknown entity %q", lineNum, entityID)
	}

	ts, err := parseUnixTimestamp(strings.TrimSpace(record[1]))
	if err != nil {
		return model.Reading{}, fmt.Errorf("line %d: parsing timestamp: %w", lineNum, err)
	}

	avg, err := strconv.ParseFloat(strings.TrimSpace(record[2]), 64)
	if err != nil {
		return model.Reading{}, fmt.Errorf("line %d: parsing avg: %w", lineNum, err)
	}

	minVal, err := strconv.ParseFloat(strings.TrimSpace(record[3]), 64)
	if err != nil {
		return model.Reading{}, fmt.Errorf("line %d: parsing min_val: %w", lineNum, err)
	}

	maxVal, err := strconv.ParseFloat(strings.TrimSpace(record[4]), 64)
	if err != nil {
		return model.Reading{}, fmt.Errorf("line %d: parsing max_val: %w", lineNum, err)
	}

	info := model.SensorCatalog[sensorType]

	return model.Reading{
		Timestamp: ts,
		SensorID:  entityID,
		Type:      sensorType,
		Value:     avg,
		Min:       minVal,
		Max:       maxVal,
		Unit:      info.Unit,
	}, nil
}

// parseUnixTimestamp parses a Unix epoch float (seconds) into a time.Time.
func parseUnixTimestamp(s string) (time.Time, error) {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("parsing %q as unix timestamp: %w", s, err)
	}
	sec, frac := math.Modf(f)
	return time.Unix(int64(sec), int64(frac*1e9)).UTC(), nil
}
