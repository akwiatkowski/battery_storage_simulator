package predictor

import (
	"encoding/json"
	"math"
	"math/rand/v2"
)

// Sample is a single training example joining power and temperature at a given time.
type Sample struct {
	Month       int
	Hour        int
	Temperature float64
	Power       float64
}

// Normalization holds z-score parameters for temperature and power.
type Normalization struct {
	TempMean  float64 `json:"temp_mean"`
	TempStd   float64 `json:"temp_std"`
	PowerMean float64 `json:"power_mean"`
	PowerStd  float64 `json:"power_std"`
}

// SavedModel is the JSON-serializable model artifact.
type SavedModel struct {
	Network        *Network       `json:"network"`
	Normalization  Normalization  `json:"normalization"`
	HourlyNoiseStd [24]float64   `json:"hourly_noise_std"`
}

// EnergyPredictor wraps a trained network for power prediction.
type EnergyPredictor struct {
	net   *Network
	norm  Normalization
	noise [24]float64
	rng   *rand.Rand
}

// EncodeFeatures converts (month, hour, normTemp) to a 5-element cyclical feature vector.
// Features: sin(month), cos(month), sin(hour), cos(hour), normTemp.
func EncodeFeatures(month, hour int, normTemp float64) []float64 {
	mAngle := 2 * math.Pi * float64(month-1) / 12.0
	hAngle := 2 * math.Pi * float64(hour) / 24.0
	return []float64{
		math.Sin(mAngle),
		math.Cos(mAngle),
		math.Sin(hAngle),
		math.Cos(hAngle),
		normTemp,
	}
}

// ComputeNormalization computes z-score parameters from training samples.
func ComputeNormalization(samples []Sample) Normalization {
	n := float64(len(samples))
	var tempSum, powerSum float64
	for _, s := range samples {
		tempSum += s.Temperature
		powerSum += s.Power
	}
	tempMean := tempSum / n
	powerMean := powerSum / n

	var tempVar, powerVar float64
	for _, s := range samples {
		dt := s.Temperature - tempMean
		dp := s.Power - powerMean
		tempVar += dt * dt
		powerVar += dp * dp
	}
	tempStd := math.Sqrt(tempVar / n)
	powerStd := math.Sqrt(powerVar / n)

	// Guard against zero std.
	if tempStd < 1e-10 {
		tempStd = 1
	}
	if powerStd < 1e-10 {
		powerStd = 1
	}

	return Normalization{
		TempMean:  tempMean,
		TempStd:   tempStd,
		PowerMean: powerMean,
		PowerStd:  powerStd,
	}
}

// TrainPredictor trains a predictor from samples and returns it along with per-epoch losses.
func TrainPredictor(samples []Sample, cfg TrainConfig, seed uint64) (*EnergyPredictor, []float64) {
	rng := rand.New(rand.NewPCG(seed, 0))
	norm := ComputeNormalization(samples)

	// Prepare feature/target matrices.
	X := make([][]float64, len(samples))
	Y := make([][]float64, len(samples))
	for i, s := range samples {
		normTemp := (s.Temperature - norm.TempMean) / norm.TempStd
		X[i] = EncodeFeatures(s.Month, s.Hour, normTemp)
		Y[i] = []float64{(s.Power - norm.PowerMean) / norm.PowerStd}
	}

	net, losses := TrainNetworkOnData(X, Y, []int{5, 32, 16, 1}, cfg, rng)

	// Compute hourly noise std from residuals on all data.
	hours := make([]int, len(samples))
	predictions := make([]float64, len(samples))
	actuals := make([]float64, len(samples))
	for i, s := range samples {
		hours[i] = s.Hour
		normTemp := (s.Temperature - norm.TempMean) / norm.TempStd
		features := EncodeFeatures(s.Month, s.Hour, normTemp)
		normPred := net.Forward(features)[0]
		predictions[i] = normPred*norm.PowerStd + norm.PowerMean
		actuals[i] = s.Power
	}
	hourlyNoise := ComputeResidualNoiseByHour(hours, predictions, actuals)

	return &EnergyPredictor{
		net:   net,
		norm:  norm,
		noise: hourlyNoise,
		rng:   rng,
	}, losses
}

// Norm returns the normalization parameters used during training.
func (p *EnergyPredictor) Norm() Normalization {
	return p.norm
}

// Predict returns a power prediction in watts with realistic noise.
func (p *EnergyPredictor) Predict(month, hour int, tempC float64) float64 {
	clean := p.PredictClean(month, hour, tempC)
	noise := p.rng.NormFloat64() * p.noise[hour]
	return clean + noise
}

// PredictClean returns a power prediction without noise.
func (p *EnergyPredictor) PredictClean(month, hour int, tempC float64) float64 {
	normTemp := (tempC - p.norm.TempMean) / p.norm.TempStd
	features := EncodeFeatures(month, hour, normTemp)
	normPred := p.net.Forward(features)[0]
	return normPred*p.norm.PowerStd + p.norm.PowerMean
}

// Save serializes the model to JSON.
func (p *EnergyPredictor) Save() ([]byte, error) {
	m := SavedModel{
		Network:        p.net,
		Normalization:  p.norm,
		HourlyNoiseStd: p.noise,
	}
	return json.MarshalIndent(m, "", "  ")
}

// LoadPredictor deserializes a model from JSON.
func LoadPredictor(data []byte, seed uint64) (*EnergyPredictor, error) {
	var m SavedModel
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &EnergyPredictor{
		net:   m.Network,
		norm:  m.Normalization,
		noise: m.HourlyNoiseStd,
		rng:   rand.New(rand.NewPCG(seed, 0)),
	}, nil
}

// ShuffleAndSplit shuffles data and returns a 90/10 train/val split.
func ShuffleAndSplit(X, Y [][]float64, rng *rand.Rand) (trainX, trainY, valX, valY [][]float64) {
	n := len(X)
	nVal := n / 10
	if nVal < 1 {
		nVal = 1
	}
	nTrain := n - nVal

	indices := make([]int, n)
	for i := range indices {
		indices[i] = i
	}
	rng.Shuffle(len(indices), func(i, j int) {
		indices[i], indices[j] = indices[j], indices[i]
	})

	trainX = make([][]float64, nTrain)
	trainY = make([][]float64, nTrain)
	valX = make([][]float64, nVal)
	valY = make([][]float64, nVal)
	for i := 0; i < nTrain; i++ {
		trainX[i] = X[indices[i]]
		trainY[i] = Y[indices[i]]
	}
	for i := 0; i < nVal; i++ {
		valX[i] = X[indices[nTrain+i]]
		valY[i] = Y[indices[nTrain+i]]
	}
	return
}

// TrainNetworkOnData creates a network, shuffles/splits data, trains, and returns the network + per-epoch losses.
func TrainNetworkOnData(X, Y [][]float64, sizes []int, cfg TrainConfig, rng *rand.Rand) (*Network, []float64) {
	trainX, trainY, valX, valY := ShuffleAndSplit(X, Y, rng)
	net := NewNetwork(sizes, rng)
	losses := net.Train(trainX, trainY, valX, valY, cfg, rng)
	return net, losses
}

// ComputeResidualNoiseByHour computes the standard deviation of residuals per hour-of-day (0-23).
func ComputeResidualNoiseByHour(hours []int, predictions, actuals []float64) [24]float64 {
	var sums [24]float64
	var sumsSq [24]float64
	var counts [24]int

	for i := range hours {
		residual := actuals[i] - predictions[i]
		h := hours[i]
		sums[h] += residual
		sumsSq[h] += residual * residual
		counts[h]++
	}

	var result [24]float64
	for h := 0; h < 24; h++ {
		if counts[h] > 1 {
			mean := sums[h] / float64(counts[h])
			variance := sumsSq[h]/float64(counts[h]) - mean*mean
			if variance > 0 {
				result[h] = math.Sqrt(variance)
			}
		}
	}
	return result
}
