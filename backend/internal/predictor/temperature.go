package predictor

import (
	"encoding/json"
	"math"
	"math/rand/v2"
)

// TempSample is a single training example for the temperature predictor.
type TempSample struct {
	DayOfYear int     // 1-366
	Hour      int     // 0-23
	Temp      float64 // °C
}

// TempNormalization holds z-score parameters for temperature output.
type TempNormalization struct {
	TempMean float64 `json:"temp_mean"`
	TempStd  float64 `json:"temp_std"`
}

// TempSavedModel is the JSON-serializable temperature model artifact.
type TempSavedModel struct {
	Network        *Network          `json:"network"`
	Normalization  TempNormalization `json:"normalization"`
	HourlyNoiseStd [24]float64      `json:"hourly_noise_std"`
}

// TemperaturePredictor wraps a trained network for temperature prediction.
type TemperaturePredictor struct {
	net   *Network
	norm  TempNormalization
	noise [24]float64
	rng   *rand.Rand
}

// EncodeTempFeatures converts (dayOfYear, hour, anomaly) to a 5-element feature vector.
// Features: sin(dayOfYear), cos(dayOfYear), sin(hour), cos(hour), anomaly.
func EncodeTempFeatures(dayOfYear, hour int, anomaly float64) []float64 {
	dAngle := 2 * math.Pi * float64(dayOfYear-1) / 365.0
	hAngle := 2 * math.Pi * float64(hour) / 24.0
	return []float64{
		math.Sin(dAngle),
		math.Cos(dAngle),
		math.Sin(hAngle),
		math.Cos(hAngle),
		anomaly,
	}
}

// ComputeTempNormalization computes z-score parameters from temperature samples.
func ComputeTempNormalization(samples []TempSample) TempNormalization {
	n := float64(len(samples))
	var tempSum float64
	for _, s := range samples {
		tempSum += s.Temp
	}
	tempMean := tempSum / n

	var tempVar float64
	for _, s := range samples {
		dt := s.Temp - tempMean
		tempVar += dt * dt
	}
	tempStd := math.Sqrt(tempVar / n)
	if tempStd < 1e-10 {
		tempStd = 1
	}

	return TempNormalization{
		TempMean: tempMean,
		TempStd:  tempStd,
	}
}

// TrainTemperaturePredictor trains a temperature predictor from samples.
// Training data is augmented with random anomaly values to teach the network
// that anomaly shifts temperature. For anomaly=+1, the expected shift is 0.1-3°C.
func TrainTemperaturePredictor(samples []TempSample, cfg TrainConfig, seed uint64) (*TemperaturePredictor, []float64) {
	rng := rand.New(rand.NewPCG(seed, 0))

	// Compute normalization from original samples (before augmentation).
	norm := ComputeTempNormalization(samples)

	// Build training data with anomaly augmentation.
	// For each real sample: 1 original (anomaly=0) + 2 augmented (random anomaly).
	augPerSample := 2
	totalSamples := len(samples) * (1 + augPerSample)
	X := make([][]float64, totalSamples)
	Y := make([][]float64, totalSamples)

	idx := 0
	for _, s := range samples {
		// Original sample: anomaly = 0.
		X[idx] = EncodeTempFeatures(s.DayOfYear, s.Hour, 0)
		Y[idx] = []float64{(s.Temp - norm.TempMean) / norm.TempStd}
		idx++

		// Augmented copies with random anomaly.
		for a := 0; a < augPerSample; a++ {
			anomaly := rng.Float64()*4 - 2          // uniform(-2, 2)
			scale := 0.1 + rng.Float64()*(3.0-0.1)  // uniform(0.1, 3.0)
			augTemp := s.Temp + anomaly*scale
			X[idx] = EncodeTempFeatures(s.DayOfYear, s.Hour, anomaly)
			Y[idx] = []float64{(augTemp - norm.TempMean) / norm.TempStd}
			idx++
		}
	}

	net, losses := TrainNetworkOnData(X, Y, []int{5, 32, 16, 1}, cfg, rng)

	// Compute hourly noise std from residuals on original (non-augmented) data only.
	hours := make([]int, len(samples))
	predictions := make([]float64, len(samples))
	actuals := make([]float64, len(samples))
	for i, s := range samples {
		hours[i] = s.Hour
		features := EncodeTempFeatures(s.DayOfYear, s.Hour, 0)
		normPred := net.Forward(features)[0]
		predictions[i] = normPred*norm.TempStd + norm.TempMean
		actuals[i] = s.Temp
	}
	hourlyNoise := ComputeResidualNoiseByHour(hours, predictions, actuals)

	return &TemperaturePredictor{
		net:   net,
		norm:  norm,
		noise: hourlyNoise,
		rng:   rng,
	}, losses
}

// Norm returns the normalization parameters.
func (p *TemperaturePredictor) Norm() TempNormalization {
	return p.norm
}

// Predict returns a temperature prediction in °C with realistic noise.
func (p *TemperaturePredictor) Predict(dayOfYear, hour int, anomaly float64) float64 {
	clean := p.PredictClean(dayOfYear, hour, anomaly)
	noise := p.rng.NormFloat64() * p.noise[hour]
	return clean + noise
}

// PredictClean returns a temperature prediction without noise.
func (p *TemperaturePredictor) PredictClean(dayOfYear, hour int, anomaly float64) float64 {
	features := EncodeTempFeatures(dayOfYear, hour, anomaly)
	normPred := p.net.Forward(features)[0]
	return normPred*p.norm.TempStd + p.norm.TempMean
}

// Save serializes the temperature model to JSON.
func (p *TemperaturePredictor) Save() ([]byte, error) {
	m := TempSavedModel{
		Network:        p.net,
		Normalization:  p.norm,
		HourlyNoiseStd: p.noise,
	}
	return json.MarshalIndent(m, "", "  ")
}

// LoadTemperaturePredictor deserializes a temperature model from JSON.
func LoadTemperaturePredictor(data []byte, seed uint64) (*TemperaturePredictor, error) {
	var m TempSavedModel
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &TemperaturePredictor{
		net:   m.Network,
		norm:  m.Normalization,
		noise: m.HourlyNoiseStd,
		rng:   rand.New(rand.NewPCG(seed, 0)),
	}, nil
}

// TempRateConstraint defines a maximum allowed temperature change over a time window.
type TempRateConstraint struct {
	WindowHours int
	MaxDeltaC   float64
}

// DefaultTempRateConstraints are physical limits on how fast outdoor temperature changes.
var DefaultTempRateConstraints = []TempRateConstraint{
	{1, 5.0},
	{4, 10.0},
	{10, 15.0},
	{14, 20.0},
}

// PredictSequence generates a sequence of hourly temperature predictions with
// temporally correlated noise (AR(1) process) and rate-of-change constraints.
// startDay is 1-366, startHour is 0-23.
func (p *TemperaturePredictor) PredictSequence(startDay, startHour, hours int, anomaly float64) []float64 {
	temps := make([]float64, hours)

	// Step 1: Clean predictions.
	for i := range temps {
		day, hour := AdvanceDayHour(startDay, startHour, i)
		temps[i] = p.PredictClean(day, hour, anomaly)
	}

	// Step 2: Add temporally correlated noise (AR(1) process).
	// alpha=0.9 gives lag-1 correlation of 0.9, making consecutive noise values
	// similar. The sqrt(1-alpha^2) factor preserves marginal variance.
	const alpha = 0.9
	scale := math.Sqrt(1 - alpha*alpha)
	var prev float64
	for i := range temps {
		_, hour := AdvanceDayHour(startDay, startHour, i)
		innovation := p.rng.NormFloat64() * p.noise[hour]
		prev = alpha*prev + scale*innovation
		temps[i] += prev
	}

	// Step 3: Enforce rate-of-change constraints.
	EnforceTempRateConstraints(temps, DefaultTempRateConstraints)

	return temps
}

// PredictCleanSequence generates a sequence of clean (no noise) predictions.
func (p *TemperaturePredictor) PredictCleanSequence(startDay, startHour, hours int, anomaly float64) []float64 {
	temps := make([]float64, hours)
	for i := range temps {
		day, hour := AdvanceDayHour(startDay, startHour, i)
		temps[i] = p.PredictClean(day, hour, anomaly)
	}
	return temps
}

// AdvanceDayHour computes (dayOfYear, hour) after advancing offsetHours from a start point.
func AdvanceDayHour(startDay, startHour, offsetHours int) (day, hour int) {
	total := startHour + offsetHours
	hour = total % 24
	day = startDay + total/24
	day = ((day - 1) % 365) + 1
	return
}

// EnforceTempRateConstraints clamps a temperature sequence so that all
// rate-of-change constraints are satisfied simultaneously. Uses iterative
// bidirectional passes: forward (lookback) and backward (lookahead) to
// propagate constraints in both directions.
func EnforceTempRateConstraints(temps []float64, constraints []TempRateConstraint) {
	for pass := 0; pass < 50; pass++ {
		changed := false

		// Forward: constrain each point based on lookback windows.
		for i := 1; i < len(temps); i++ {
			if clampToConstraints(temps, i, constraints, true) {
				changed = true
			}
		}

		// Backward: constrain each point based on lookahead windows.
		for i := len(temps) - 2; i >= 0; i-- {
			if clampToConstraints(temps, i, constraints, false) {
				changed = true
			}
		}

		if !changed {
			break
		}
	}
}

// clampToConstraints computes the allowed range for temps[i] from all constraint
// windows and clamps the value. When forward=true, uses lookback (past);
// when false, uses lookahead (future).
func clampToConstraints(temps []float64, i int, constraints []TempRateConstraint, forward bool) bool {
	lo := math.Inf(-1)
	hi := math.Inf(1)

	for _, c := range constraints {
		var j int
		if forward {
			j = i - c.WindowHours
		} else {
			j = i + c.WindowHours
		}
		if j < 0 || j >= len(temps) {
			continue
		}
		ref := temps[j]
		lo = max(lo, ref-c.MaxDeltaC)
		hi = min(hi, ref+c.MaxDeltaC)
	}

	if lo > hi {
		mid := (lo + hi) / 2
		lo, hi = mid, mid
	}

	if temps[i] < lo {
		temps[i] = lo
		return true
	}
	if temps[i] > hi {
		temps[i] = hi
		return true
	}
	return false
}
