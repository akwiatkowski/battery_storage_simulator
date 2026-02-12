package simulator

import (
	"sync"
	"time"

	"energy_simulator/internal/predictor"
)

// PredictionProvider generates synthetic sensor readings from neural networks.
type PredictionProvider struct {
	tempPred  *predictor.TemperaturePredictor
	powerPred *predictor.EnergyPredictor
	gridSensorID string

	mu           sync.Mutex
	tempSequence []float64
	seqStartTime time.Time // truncated to hour
}

// NewPredictionProvider creates a provider wrapping both NN models.
func NewPredictionProvider(tempPred *predictor.TemperaturePredictor, powerPred *predictor.EnergyPredictor, gridSensorID string) *PredictionProvider {
	return &PredictionProvider{
		tempPred:     tempPred,
		powerPred:    powerPred,
		gridSensorID: gridSensorID,
	}
}

// Init pre-generates a year of temperature predictions starting from startTime.
func (p *PredictionProvider) Init(startTime time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.seqStartTime = startTime.Truncate(time.Hour)
	startDay := p.seqStartTime.YearDay()
	startHour := p.seqStartTime.Hour()
	p.tempSequence = p.tempPred.PredictSequence(startDay, startHour, 8760, 0)
}

// ReadingsForRange returns grid power readings for each hour in [from, to).
func (p *PredictionProvider) ReadingsForRange(from, to time.Time) []SensorReading {
	if !to.After(from) {
		return nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// Compute hour indices relative to sequence start
	fromHour := int(from.Truncate(time.Hour).Sub(p.seqStartTime).Hours())
	toHour := int(to.Add(time.Hour - time.Nanosecond).Truncate(time.Hour).Sub(p.seqStartTime).Hours())

	if fromHour < 0 {
		fromHour = 0
	}

	// Extend temperature buffer if needed
	for toHour >= len(p.tempSequence) {
		extStart := len(p.tempSequence)
		extTime := p.seqStartTime.Add(time.Duration(extStart) * time.Hour)
		extDay := extTime.YearDay()
		extHour := extTime.Hour()
		extra := p.tempPred.PredictSequence(extDay, extHour, 8760, 0)
		p.tempSequence = append(p.tempSequence, extra...)
	}

	var readings []SensorReading
	for i := fromHour; i <= toHour; i++ {
		t := p.seqStartTime.Add(time.Duration(i) * time.Hour)
		if t.Before(from) || !t.Before(to) {
			continue
		}

		temp := p.tempSequence[i]
		power := p.powerPred.Predict(int(t.Month()), t.Hour(), temp)

		readings = append(readings, SensorReading{
			SensorID:  p.gridSensorID,
			Value:     power,
			Unit:      "W",
			Timestamp: t.Format(time.RFC3339),
		})
	}

	return readings
}
