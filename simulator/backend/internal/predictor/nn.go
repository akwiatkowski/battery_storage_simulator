package predictor

import (
	"encoding/json"
	"math"
	"math/rand/v2"
)

// Layer represents a fully-connected neural network layer.
type Layer struct {
	Weights [][]float64 `json:"weights"` // [out][in]
	Biases  []float64   `json:"biases"`

	// Adam optimizer state (not serialized).
	mW, vW [][]float64
	mB, vB []float64

	// Cached activations for backprop (not serialized).
	input  []float64
	output []float64
	dW     [][]float64
	dB     []float64
}

// Network is a feedforward neural network with ReLU hidden layers and linear output.
type Network struct {
	Layers []Layer `json:"layers"`
}

// TrainConfig holds hyperparameters for training.
type TrainConfig struct {
	LearningRate float64
	Beta1        float64
	Beta2        float64
	Epsilon      float64
	BatchSize    int
	Epochs       int
}

// DefaultTrainConfig returns sensible defaults for training.
func DefaultTrainConfig() TrainConfig {
	return TrainConfig{
		LearningRate: 0.001,
		Beta1:        0.9,
		Beta2:        0.999,
		Epsilon:      1e-8,
		BatchSize:    64,
		Epochs:       200,
	}
}

// NewNetwork creates a network with He initialization.
// sizes specifies the number of neurons in each layer, e.g. [5, 32, 16, 1].
func NewNetwork(sizes []int, rng *rand.Rand) *Network {
	n := &Network{
		Layers: make([]Layer, len(sizes)-1),
	}
	for i := 0; i < len(sizes)-1; i++ {
		in, out := sizes[i], sizes[i+1]
		stddev := math.Sqrt(2.0 / float64(in)) // He init
		layer := Layer{
			Weights: make([][]float64, out),
			Biases:  make([]float64, out),
		}
		for j := 0; j < out; j++ {
			layer.Weights[j] = make([]float64, in)
			for k := 0; k < in; k++ {
				layer.Weights[j][k] = rng.NormFloat64() * stddev
			}
		}
		n.Layers[i] = layer
	}
	n.initAdam()
	return n
}

func (n *Network) initAdam() {
	for i := range n.Layers {
		l := &n.Layers[i]
		out := len(l.Weights)
		in := len(l.Weights[0])
		l.mW = makeMatrix(out, in)
		l.vW = makeMatrix(out, in)
		l.mB = make([]float64, out)
		l.vB = make([]float64, out)
		l.dW = makeMatrix(out, in)
		l.dB = make([]float64, out)
	}
}

// Forward computes the network output, caching activations for backprop.
// Hidden layers use ReLU; the output layer is linear.
func (n *Network) Forward(input []float64) []float64 {
	x := input
	for i := range n.Layers {
		l := &n.Layers[i]
		l.input = make([]float64, len(x))
		copy(l.input, x)

		out := len(l.Weights)
		y := make([]float64, out)
		for j := 0; j < out; j++ {
			sum := l.Biases[j]
			for k, w := range l.Weights[j] {
				sum += w * x[k]
			}
			y[j] = sum
		}

		// ReLU for all layers except the last (linear output).
		if i < len(n.Layers)-1 {
			for j := range y {
				if y[j] < 0 {
					y[j] = 0
				}
			}
		}

		l.output = y
		x = y
	}
	return x
}

// Backward computes gradients given the derivative of loss w.r.t. the output.
// Must be called after Forward. Gradients are accumulated in layer.dW / layer.dB.
func (n *Network) Backward(dOutput []float64) {
	dx := dOutput
	for i := len(n.Layers) - 1; i >= 0; i-- {
		l := &n.Layers[i]
		out := len(l.Weights)
		in := len(l.Weights[0])

		// Apply ReLU derivative for hidden layers.
		if i < len(n.Layers)-1 {
			for j := 0; j < out; j++ {
				if l.output[j] <= 0 {
					dx[j] = 0
				}
			}
		}

		// Accumulate gradients.
		for j := 0; j < out; j++ {
			l.dB[j] += dx[j]
			for k := 0; k < in; k++ {
				l.dW[j][k] += dx[j] * l.input[k]
			}
		}

		// Propagate gradient to input.
		if i > 0 {
			dInput := make([]float64, in)
			for k := 0; k < in; k++ {
				for j := 0; j < out; j++ {
					dInput[k] += dx[j] * l.Weights[j][k]
				}
			}
			dx = dInput
		}
	}
}

// ZeroGrad resets accumulated gradients to zero.
func (n *Network) ZeroGrad() {
	for i := range n.Layers {
		l := &n.Layers[i]
		for j := range l.dW {
			for k := range l.dW[j] {
				l.dW[j][k] = 0
			}
		}
		for j := range l.dB {
			l.dB[j] = 0
		}
	}
}

// UpdateAdam applies Adam weight updates. step is the 1-based global step count.
func (n *Network) UpdateAdam(cfg TrainConfig, step int) {
	for i := range n.Layers {
		l := &n.Layers[i]
		for j := range l.Weights {
			for k := range l.Weights[j] {
				l.mW[j][k] = cfg.Beta1*l.mW[j][k] + (1-cfg.Beta1)*l.dW[j][k]
				l.vW[j][k] = cfg.Beta2*l.vW[j][k] + (1-cfg.Beta2)*l.dW[j][k]*l.dW[j][k]
				mHat := l.mW[j][k] / (1 - math.Pow(cfg.Beta1, float64(step)))
				vHat := l.vW[j][k] / (1 - math.Pow(cfg.Beta2, float64(step)))
				l.Weights[j][k] -= cfg.LearningRate * mHat / (math.Sqrt(vHat) + cfg.Epsilon)
			}
		}
		for j := range l.Biases {
			l.mB[j] = cfg.Beta1*l.mB[j] + (1-cfg.Beta1)*l.dB[j]
			l.vB[j] = cfg.Beta2*l.vB[j] + (1-cfg.Beta2)*l.dB[j]*l.dB[j]
			mHat := l.mB[j] / (1 - math.Pow(cfg.Beta1, float64(step)))
			vHat := l.vB[j] / (1 - math.Pow(cfg.Beta2, float64(step)))
			l.Biases[j] -= cfg.LearningRate * mHat / (math.Sqrt(vHat) + cfg.Epsilon)
		}
	}
}

// Train runs mini-batch Adam training and returns per-epoch validation MSE loss.
func (n *Network) Train(trainX, trainY, valX, valY [][]float64, cfg TrainConfig, rng *rand.Rand) []float64 {
	nTrain := len(trainX)
	indices := make([]int, nTrain)
	for i := range indices {
		indices[i] = i
	}

	step := 0
	epochLosses := make([]float64, cfg.Epochs)

	for epoch := 0; epoch < cfg.Epochs; epoch++ {
		// Shuffle training data.
		rng.Shuffle(nTrain, func(i, j int) {
			indices[i], indices[j] = indices[j], indices[i]
		})

		// Mini-batch training.
		for batchStart := 0; batchStart < nTrain; batchStart += cfg.BatchSize {
			batchEnd := batchStart + cfg.BatchSize
			if batchEnd > nTrain {
				batchEnd = nTrain
			}
			batchSize := batchEnd - batchStart

			n.ZeroGrad()
			for b := batchStart; b < batchEnd; b++ {
				idx := indices[b]
				output := n.Forward(trainX[idx])
				// MSE gradient: 2*(pred - target) / batchSize
				dOutput := []float64{2 * (output[0] - trainY[idx][0]) / float64(batchSize)}
				n.Backward(dOutput)
			}

			step++
			_ = batchSize
			n.UpdateAdam(cfg, step)
		}

		// Compute validation loss.
		epochLosses[epoch] = n.MSELoss(valX, valY)
	}

	return epochLosses
}

// MSELoss computes mean squared error over a dataset.
func (n *Network) MSELoss(X, Y [][]float64) float64 {
	if len(X) == 0 {
		return 0
	}
	sum := 0.0
	for i := range X {
		output := n.Forward(X[i])
		diff := output[0] - Y[i][0]
		sum += diff * diff
	}
	return sum / float64(len(X))
}

// MarshalJSON serializes the network weights and biases.
func (n *Network) MarshalJSON() ([]byte, error) {
	type layerJSON struct {
		Weights [][]float64 `json:"weights"`
		Biases  []float64   `json:"biases"`
	}
	layers := make([]layerJSON, len(n.Layers))
	for i, l := range n.Layers {
		layers[i] = layerJSON{Weights: l.Weights, Biases: l.Biases}
	}
	return json.Marshal(struct {
		Layers []layerJSON `json:"layers"`
	}{Layers: layers})
}

// UnmarshalJSON deserializes network weights/biases and reinitializes Adam state.
func (n *Network) UnmarshalJSON(data []byte) error {
	type layerJSON struct {
		Weights [][]float64 `json:"weights"`
		Biases  []float64   `json:"biases"`
	}
	var raw struct {
		Layers []layerJSON `json:"layers"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	n.Layers = make([]Layer, len(raw.Layers))
	for i, l := range raw.Layers {
		n.Layers[i] = Layer{Weights: l.Weights, Biases: l.Biases}
	}
	n.initAdam()
	return nil
}

func makeMatrix(rows, cols int) [][]float64 {
	m := make([][]float64, rows)
	for i := range m {
		m[i] = make([]float64, cols)
	}
	return m
}
