package predictor

import (
	"math"
	"math/rand/v2"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncodeTempFeatures(t *testing.T) {
	// Jan 1 (dayOfYear=1), midnight (hour=0), anomaly=0.
	f := EncodeTempFeatures(1, 0, 0.0)
	require.Len(t, f, 5)

	// sin(0) = 0, cos(0) = 1 for dayOfYear=1 → angle=0
	assert.InDelta(t, 0.0, f[0], 1e-10, "sin(day=1)")
	assert.InDelta(t, 1.0, f[1], 1e-10, "cos(day=1)")

	// sin(0) = 0, cos(0) = 1 for hour=0 → angle=0
	assert.InDelta(t, 0.0, f[2], 1e-10, "sin(hour=0)")
	assert.InDelta(t, 1.0, f[3], 1e-10, "cos(hour=0)")

	assert.Equal(t, 0.0, f[4], "anomaly")

	// Mid-year (dayOfYear=183), noon (hour=12), anomaly=1.5.
	f2 := EncodeTempFeatures(183, 12, 1.5)
	// dayOfYear=183 → angle ≈ π → sin(π)≈0, cos(π)≈-1
	assert.InDelta(t, 0.0, f2[0], 0.02, "sin(day=183)")
	assert.InDelta(t, -1.0, f2[1], 0.02, "cos(day=183)")
	// hour=12 → angle = π → sin(π)≈0, cos(π)≈-1
	assert.InDelta(t, 0.0, f2[2], 1e-10, "sin(hour=12)")
	assert.InDelta(t, -1.0, f2[3], 1e-10, "cos(hour=12)")
	assert.Equal(t, 1.5, f2[4])
}

func TestComputeTempNormalization(t *testing.T) {
	samples := []TempSample{
		{Temp: 0},
		{Temp: 10},
		{Temp: 20},
	}
	norm := ComputeTempNormalization(samples)

	assert.InDelta(t, 10.0, norm.TempMean, 1e-10)
	// std of [0,10,20] = sqrt(((−10)²+0²+10²)/3) = sqrt(200/3) ≈ 8.165
	expectedStd := math.Sqrt(200.0 / 3.0)
	assert.InDelta(t, expectedStd, norm.TempStd, 1e-10)
}

func TestTemperaturePredictor_SaveLoadRoundtrip(t *testing.T) {
	samples := generateSyntheticTempSamples(500, 42)
	cfg := DefaultTrainConfig()
	cfg.Epochs = 10

	pred, _ := TrainTemperaturePredictor(samples, cfg, 42)

	data, err := pred.Save()
	require.NoError(t, err)

	loaded, err := LoadTemperaturePredictor(data, 99)
	require.NoError(t, err)

	// PredictClean should be identical (no noise dependency).
	for _, s := range samples[:10] {
		expected := pred.PredictClean(s.DayOfYear, s.Hour, 0)
		actual := loaded.PredictClean(s.DayOfYear, s.Hour, 0)
		assert.Equal(t, expected, actual, "PredictClean should match after roundtrip")
	}
}

func TestTrainTemperaturePredictor_ConvergesOnSyntheticData(t *testing.T) {
	// Synthetic: temp = 10 + 15·sin(2π·(day-1)/365) - 5·cos(2π·hour/24)
	samples := generateSyntheticTempSamples(2000, 42)

	cfg := DefaultTrainConfig()
	cfg.Epochs = 300
	cfg.LearningRate = 0.005
	cfg.BatchSize = 64

	pred, losses := TrainTemperaturePredictor(samples, cfg, 42)

	// Loss should decrease.
	initialLoss := losses[0]
	finalLoss := losses[len(losses)-1]
	assert.Less(t, finalLoss, initialLoss, "final loss should be less than initial")

	// RMSE on clean predictions (anomaly=0) should be < 3°C.
	var sumSqErr float64
	for _, s := range samples {
		predicted := pred.PredictClean(s.DayOfYear, s.Hour, 0)
		diff := predicted - s.Temp
		sumSqErr += diff * diff
	}
	rmse := math.Sqrt(sumSqErr / float64(len(samples)))
	assert.Less(t, rmse, 3.0, "RMSE should be < 3°C, got %.2f°C", rmse)
}

func TestTemperaturePredictor_AnomalyIncreasesTemp(t *testing.T) {
	samples := generateSyntheticTempSamples(2000, 42)

	cfg := DefaultTrainConfig()
	cfg.Epochs = 300
	cfg.LearningRate = 0.005

	pred, _ := TrainTemperaturePredictor(samples, cfg, 42)

	// With anomaly=+1, temperature should be higher than anomaly=0.
	// Test across several conditions.
	var totalDiff float64
	nChecks := 0
	for _, day := range []int{1, 91, 182, 274} {
		for _, hour := range []int{0, 6, 12, 18} {
			base := pred.PredictClean(day, hour, 0)
			warm := pred.PredictClean(day, hour, 1)
			diff := warm - base
			totalDiff += diff
			nChecks++
		}
	}
	avgDiff := totalDiff / float64(nChecks)
	assert.Greater(t, avgDiff, 0.1, "anomaly=+1 should increase temperature by avg >= 0.1°C")
	assert.Less(t, avgDiff, 3.0, "anomaly=+1 should increase temperature by avg <= 3.0°C")
}

func TestAdvanceDayHour(t *testing.T) {
	// Simple advance within same day.
	day, hour := AdvanceDayHour(1, 0, 5)
	assert.Equal(t, 1, day)
	assert.Equal(t, 5, hour)

	// Crosses midnight.
	day, hour = AdvanceDayHour(1, 22, 5)
	assert.Equal(t, 2, day)
	assert.Equal(t, 3, hour)

	// Wraps year boundary (day 365 + 1 day = day 1).
	day, hour = AdvanceDayHour(365, 12, 24)
	assert.Equal(t, 1, day)
	assert.Equal(t, 12, hour)
}

func TestEnforceTempRateConstraints(t *testing.T) {
	// Construct a sequence with a 10°C jump in 1 hour.
	temps := []float64{0, 10, 10, 10, 10}
	EnforceTempRateConstraints(temps, DefaultTempRateConstraints)

	// 1h constraint: max 5°C change.
	for i := 1; i < len(temps); i++ {
		delta := math.Abs(temps[i] - temps[i-1])
		assert.LessOrEqual(t, delta, 5.0+1e-9,
			"1h constraint violated at index %d: delta=%.2f", i, delta)
	}
}

func TestEnforceTempRateConstraints_MultiHour(t *testing.T) {
	// Steady climb of 4°C/hour for 20 hours → 80°C total.
	temps := make([]float64, 20)
	for i := range temps {
		temps[i] = float64(i) * 4
	}

	EnforceTempRateConstraints(temps, DefaultTempRateConstraints)

	// Check all window constraints.
	for _, c := range DefaultTempRateConstraints {
		for i := c.WindowHours; i < len(temps); i++ {
			delta := math.Abs(temps[i] - temps[i-c.WindowHours])
			assert.LessOrEqual(t, delta, c.MaxDeltaC+1e-9,
				"%dh constraint violated at index %d: delta=%.2f", c.WindowHours, i, delta)
		}
	}
}

func TestPredictSequence_RespectsRateConstraints(t *testing.T) {
	samples := generateSyntheticTempSamples(2000, 42)
	cfg := DefaultTrainConfig()
	cfg.Epochs = 100
	cfg.LearningRate = 0.005

	pred, _ := TrainTemperaturePredictor(samples, cfg, 42)

	// Generate several sequences and verify constraints.
	for _, seed := range []uint64{1, 2, 3, 4, 5} {
		loaded, err := reloadWithSeed(pred, seed)
		require.NoError(t, err)

		temps := loaded.PredictSequence(1, 0, 72, 0)

		for _, c := range DefaultTempRateConstraints {
			for i := c.WindowHours; i < len(temps); i++ {
				delta := math.Abs(temps[i] - temps[i-c.WindowHours])
				assert.LessOrEqual(t, delta, c.MaxDeltaC+1e-9,
					"seed=%d %dh constraint at i=%d: delta=%.2f", seed, c.WindowHours, i, delta)
			}
		}
	}
}

func TestPredictSequence_LengthMatches(t *testing.T) {
	samples := generateSyntheticTempSamples(500, 42)
	cfg := DefaultTrainConfig()
	cfg.Epochs = 10
	pred, _ := TrainTemperaturePredictor(samples, cfg, 42)

	for _, n := range []int{1, 24, 48, 168} {
		temps := pred.PredictSequence(100, 12, n, 0)
		assert.Len(t, temps, n)
	}
}

// reloadWithSeed saves and reloads a predictor with a new RNG seed.
func reloadWithSeed(p *TemperaturePredictor, seed uint64) (*TemperaturePredictor, error) {
	data, err := p.Save()
	if err != nil {
		return nil, err
	}
	return LoadTemperaturePredictor(data, seed)
}

func generateSyntheticTempSamples(n int, seed uint64) []TempSample {
	rng := rand.New(rand.NewPCG(seed, 0))
	samples := make([]TempSample, n)
	for i := range samples {
		day := rng.IntN(365) + 1
		hour := rng.IntN(24)
		dAngle := 2 * math.Pi * float64(day-1) / 365.0
		hAngle := 2 * math.Pi * float64(hour) / 24.0
		// Seasonal pattern + daily variation + noise.
		temp := 10 + 15*math.Sin(dAngle) - 5*math.Cos(hAngle) + rng.NormFloat64()*2
		samples[i] = TempSample{
			DayOfYear: day,
			Hour:      hour,
			Temp:      temp,
		}
	}
	return samples
}
