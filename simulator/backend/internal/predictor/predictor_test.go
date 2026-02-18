package predictor

import (
	"math"
	"math/rand/v2"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncodeFeatures(t *testing.T) {
	// January (month=1), midnight (hour=0), normTemp=0.
	f := EncodeFeatures(1, 0, 0.0)
	require.Len(t, f, 5)

	// sin(0) = 0, cos(0) = 1 for month=1 → angle=0
	assert.InDelta(t, 0.0, f[0], 1e-10, "sin(month=1)")
	assert.InDelta(t, 1.0, f[1], 1e-10, "cos(month=1)")

	// sin(0) = 0, cos(0) = 1 for hour=0 → angle=0
	assert.InDelta(t, 0.0, f[2], 1e-10, "sin(hour=0)")
	assert.InDelta(t, 1.0, f[3], 1e-10, "cos(hour=0)")

	assert.Equal(t, 0.0, f[4], "normTemp")

	// July (month=7), noon (hour=12).
	f2 := EncodeFeatures(7, 12, 1.5)
	// month=7 → angle = 2π*6/12 = π → sin(π)≈0, cos(π)≈-1
	assert.InDelta(t, 0.0, f2[0], 1e-10, "sin(month=7)")
	assert.InDelta(t, -1.0, f2[1], 1e-10, "cos(month=7)")
	// hour=12 → angle = 2π*12/24 = π → sin(π)≈0, cos(π)≈-1
	assert.InDelta(t, 0.0, f2[2], 1e-10, "sin(hour=12)")
	assert.InDelta(t, -1.0, f2[3], 1e-10, "cos(hour=12)")
	assert.Equal(t, 1.5, f2[4])
}

func TestComputeNormalization(t *testing.T) {
	samples := []Sample{
		{Temperature: 10, Power: 100},
		{Temperature: 20, Power: 200},
		{Temperature: 30, Power: 300},
	}
	norm := ComputeNormalization(samples)

	assert.InDelta(t, 20.0, norm.TempMean, 1e-10)
	assert.InDelta(t, 200.0, norm.PowerMean, 1e-10)

	// std of [10,20,30] = sqrt(((−10)²+0²+10²)/3) = sqrt(200/3) ≈ 8.165
	expectedStd := math.Sqrt(200.0 / 3.0)
	assert.InDelta(t, expectedStd, norm.TempStd, 1e-10)

	// std of [100,200,300] = sqrt(((−100)²+0²+100²)/3) = sqrt(20000/3) ≈ 81.65
	expectedPowerStd := math.Sqrt(20000.0 / 3.0)
	assert.InDelta(t, expectedPowerStd, norm.PowerStd, 1e-10)
}

func TestPredictor_SaveLoadRoundtrip(t *testing.T) {
	// Create a small trained predictor.
	samples := generateSyntheticSamples(500, 42)
	cfg := DefaultTrainConfig()
	cfg.Epochs = 10

	pred, _ := TrainPredictor(samples, cfg, 42)

	data, err := pred.Save()
	require.NoError(t, err)

	loaded, err := LoadPredictor(data, 99)
	require.NoError(t, err)

	// PredictClean should be identical (no noise dependency).
	for _, s := range samples[:10] {
		expected := pred.PredictClean(s.Month, s.Hour, s.Temperature)
		actual := loaded.PredictClean(s.Month, s.Hour, s.Temperature)
		assert.Equal(t, expected, actual, "PredictClean should match after roundtrip")
	}
}

func TestTrainPredictor_ConvergesOnSyntheticData(t *testing.T) {
	// Synthetic: power = 500 + 200·sin(2π·hour/24) + 10·temp
	samples := generateSyntheticSamples(2000, 42)

	cfg := DefaultTrainConfig()
	cfg.Epochs = 300
	cfg.LearningRate = 0.005
	cfg.BatchSize = 64

	pred, losses := TrainPredictor(samples, cfg, 42)

	// Loss should decrease.
	initialLoss := losses[0]
	finalLoss := losses[len(losses)-1]
	assert.Less(t, finalLoss, initialLoss, "final loss should be less than initial")

	// RMSE on clean predictions should be < 50W.
	var sumSqErr float64
	for _, s := range samples {
		predicted := pred.PredictClean(s.Month, s.Hour, s.Temperature)
		diff := predicted - s.Power
		sumSqErr += diff * diff
	}
	rmse := math.Sqrt(sumSqErr / float64(len(samples)))
	assert.Less(t, rmse, 50.0, "RMSE should be < 50W, got %.1fW", rmse)
}

func generateSyntheticSamples(n int, seed uint64) []Sample {
	rng := rand.New(rand.NewPCG(seed, 0))
	samples := make([]Sample, n)
	for i := range samples {
		month := rng.IntN(12) + 1
		hour := rng.IntN(24)
		temp := -5.0 + 30.0*rng.Float64()
		hAngle := 2 * math.Pi * float64(hour) / 24.0
		power := 500 + 200*math.Sin(hAngle) + 10*temp + rng.NormFloat64()*20
		samples[i] = Sample{
			Month:       month,
			Hour:        hour,
			Temperature: temp,
			Power:       power,
		}
	}
	return samples
}
