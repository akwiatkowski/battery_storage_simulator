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
	tempOffsetC  float64

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

// SetTempOffset sets the temperature offset applied during power prediction.
func (p *PredictionProvider) SetTempOffset(offset float64) {
	p.mu.Lock()
	p.tempOffsetC = offset
	p.mu.Unlock()
}

// EnsureInitialized lazily generates a temperature sequence for the given start
// time if one doesn't already exist. Safe to call multiple times.
func (p *PredictionProvider) EnsureInitialized(startTime time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.tempSequence) > 0 {
		return
	}
	p.seqStartTime = startTime.Truncate(time.Hour)
	startDay := p.seqStartTime.YearDay()
	startHour := p.seqStartTime.Hour()
	p.tempSequence = p.tempPred.PredictSequence(startDay, startHour, 8760, 0)
}

// PredictedTempAt returns the predicted temperature at the given time.
func (p *PredictionProvider) PredictedTempAt(t time.Time) (float64, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.tempSequence) == 0 {
		return 0, false
	}
	idx := int(t.Truncate(time.Hour).Sub(p.seqStartTime).Hours())
	if idx < 0 || idx >= len(p.tempSequence) {
		return 0, false
	}
	return p.tempSequence[idx] + p.tempOffsetC, true
}

// PredictedPowerAt returns the predicted grid power at the given time.
func (p *PredictionProvider) PredictedPowerAt(t time.Time) (float64, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.tempSequence) == 0 {
		return 0, false
	}
	idx := int(t.Truncate(time.Hour).Sub(p.seqStartTime).Hours())
	if idx < 0 || idx >= len(p.tempSequence) {
		return 0, false
	}
	temp := p.tempSequence[idx] + p.tempOffsetC
	power := p.powerPred.Predict(int(t.Month()), t.Hour(), temp)
	return power, true
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

		temp := p.tempSequence[i] + p.tempOffsetC
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
