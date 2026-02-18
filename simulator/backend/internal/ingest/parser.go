package ingest

import (
	"io"

	"energy_simulator/internal/model"
)

// Parser reads sensor data from a source and returns readings.
type Parser interface {
	Parse(r io.Reader) ([]model.Reading, error)
}
