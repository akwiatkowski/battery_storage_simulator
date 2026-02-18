package predictor

import (
	"encoding/json"
	"math"
	"math/rand/v2"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNetwork_ForwardDimensions(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 0))
	net := NewNetwork([]int{5, 32, 16, 1}, rng)

	input := []float64{0.1, 0.2, 0.3, 0.4, 0.5}
	output := net.Forward(input)

	assert.Len(t, output, 1, "output should have 1 element")
	assert.False(t, math.IsNaN(output[0]), "output should not be NaN")
}

func TestNetwork_XOR(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 0))
	net := NewNetwork([]int{2, 8, 1}, rng)

	trainX := [][]float64{{0, 0}, {0, 1}, {1, 0}, {1, 1}}
	trainY := [][]float64{{0}, {1}, {1}, {0}}

	cfg := TrainConfig{
		LearningRate: 0.05,
		Beta1:        0.9,
		Beta2:        0.999,
		Epsilon:      1e-8,
		BatchSize:    4,
		Epochs:       3000,
	}

	losses := net.Train(trainX, trainY, trainX, trainY, cfg, rng)

	// Final loss should be very low.
	finalLoss := losses[len(losses)-1]
	assert.Less(t, finalLoss, 0.01, "XOR should converge, final MSE: %f", finalLoss)

	// Verify predictions.
	for i, x := range trainX {
		pred := net.Forward(x)[0]
		expected := trainY[i][0]
		assert.InDelta(t, expected, pred, 0.15, "XOR input %v: expected %.1f, got %.3f", x, expected, pred)
	}
}

func TestNetwork_SaveLoadRoundtrip(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 0))
	net := NewNetwork([]int{5, 32, 16, 1}, rng)

	input := []float64{0.1, 0.2, 0.3, 0.4, 0.5}
	outputBefore := net.Forward(input)[0]

	data, err := json.Marshal(net)
	require.NoError(t, err)

	var loaded Network
	err = json.Unmarshal(data, &loaded)
	require.NoError(t, err)

	outputAfter := loaded.Forward(input)[0]
	assert.Equal(t, outputBefore, outputAfter, "output should be identical after roundtrip")
}

func TestNetwork_GradientCheck(t *testing.T) {
	rng := rand.New(rand.NewPCG(123, 0))
	net := NewNetwork([]int{3, 4, 1}, rng)

	input := []float64{0.5, -0.3, 0.8}
	target := 1.0
	eps := 1e-5

	// Compute analytical gradient.
	net.ZeroGrad()
	output := net.Forward(input)
	dOutput := []float64{2 * (output[0] - target)}
	net.Backward(dOutput)

	// Numerical gradient check on each weight.
	for i := range net.Layers {
		for j := range net.Layers[i].Weights {
			for k := range net.Layers[i].Weights[j] {
				orig := net.Layers[i].Weights[j][k]

				// f(w + eps)
				net.Layers[i].Weights[j][k] = orig + eps
				outPlus := net.Forward(input)[0]
				lossPlus := (outPlus - target) * (outPlus - target)

				// f(w - eps)
				net.Layers[i].Weights[j][k] = orig - eps
				outMinus := net.Forward(input)[0]
				lossMinus := (outMinus - target) * (outMinus - target)

				net.Layers[i].Weights[j][k] = orig

				numerical := (lossPlus - lossMinus) / (2 * eps)
				analytical := net.Layers[i].dW[j][k]

				// Relative error.
				denom := math.Max(math.Abs(numerical)+math.Abs(analytical), 1e-8)
				relErr := math.Abs(numerical-analytical) / denom

				assert.Less(t, relErr, 1e-4,
					"gradient check failed at layer %d weight [%d][%d]: numerical=%.8f analytical=%.8f relErr=%.8f",
					i, j, k, numerical, analytical, relErr)
			}
		}
	}
}
