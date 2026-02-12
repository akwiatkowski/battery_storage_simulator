package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"

	"energy_simulator/internal/ingest"
	"energy_simulator/internal/model"
	"energy_simulator/internal/predictor"
)

func main() {
	statsPath := flag.String("stats", "input/stats/export.csv", "path to stats CSV")
	tempModelPath := flag.String("temp-output", "model/temperature.json", "path to write temperature model JSON")
	powerModelPath := flag.String("power-output", "model/grid_power.json", "path to write power model JSON")
	epochs := flag.Int("epochs", 300, "training epochs")
	lr := flag.Float64("lr", 0.005, "learning rate")
	batchSize := flag.Int("batch-size", 64, "mini-batch size")
	seed := flag.Uint64("seed", 42, "random seed")
	flag.Parse()

	// Parse stats CSV.
	f, err := os.Open(*statsPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening stats CSV: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	parser := &ingest.StatsParser{}
	readings, err := parser.Parse(f)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing stats CSV: %v\n", err)
		os.Exit(1)
	}

	// Separate grid power and external temperature readings.
	gridEntityID := model.SensorHomeAssistantID[model.SensorGridPower]
	tempEntityID := model.SensorHomeAssistantID[model.SensorPumpExtTemp]

	// Index by Unix timestamp for joining.
	type indexed struct {
		value float64
		month int
		hour  int
	}
	powerByTS := make(map[int64]indexed)

	type tempIndexed struct {
		value     float64
		dayOfYear int
		hour      int
	}
	tempByTS := make(map[int64]tempIndexed)

	var nPower, nTemp int
	for _, r := range readings {
		ts := r.Timestamp.Unix()
		switch r.SensorID {
		case gridEntityID:
			powerByTS[ts] = indexed{
				value: r.Value,
				month: int(r.Timestamp.Month()),
				hour:  r.Timestamp.Hour(),
			}
			nPower++
		case tempEntityID:
			tempByTS[ts] = tempIndexed{
				value:     r.Value,
				dayOfYear: r.Timestamp.YearDay(),
				hour:      r.Timestamp.Hour(),
			}
			nTemp++
		}
	}

	fmt.Printf("Parsed readings: %d grid power, %d ext temperature\n", nPower, nTemp)

	cfg := predictor.TrainConfig{
		LearningRate: *lr,
		Beta1:        0.9,
		Beta2:        0.999,
		Epsilon:      1e-8,
		BatchSize:    *batchSize,
		Epochs:       *epochs,
	}

	// --- Train temperature model ---
	fmt.Println("\n=== Temperature Model ===")

	var tempSamples []predictor.TempSample
	for _, ti := range tempByTS {
		tempSamples = append(tempSamples, predictor.TempSample{
			DayOfYear: ti.dayOfYear,
			Hour:      ti.hour,
			Temp:      ti.value,
		})
	}

	fmt.Printf("Temperature training samples: %d\n", len(tempSamples))
	if len(tempSamples) == 0 {
		fmt.Fprintln(os.Stderr, "No temperature samples found.")
		os.Exit(1)
	}

	fmt.Printf("Training: epochs=%d lr=%.4f batch_size=%d seed=%d\n", cfg.Epochs, cfg.LearningRate, cfg.BatchSize, *seed)

	tempPred, tempLosses := predictor.TrainTemperaturePredictor(tempSamples, cfg, *seed)

	fmt.Printf("Initial val loss: %.6f\n", tempLosses[0])
	fmt.Printf("Final val loss:   %.6f\n", tempLosses[len(tempLosses)-1])
	fmt.Printf("Loss reduction:   %.1fx\n", tempLosses[0]/tempLosses[len(tempLosses)-1])

	// Sample temperature predictions.
	fmt.Println("\nSample predictions (clean, anomaly=0):")
	for _, day := range []int{1, 91, 182, 274} {
		labels := map[int]string{1: "Jan 1", 91: "Apr 1", 182: "Jul 1", 274: "Oct 1"}
		p := tempPred.PredictClean(day, 12, 0)
		fmt.Printf("  %s 12:00 → %.1f°C\n", labels[day], p)
	}

	// Anomaly effect.
	fmt.Println("\nAnomaly effect (day=182 12:00):")
	base := tempPred.PredictClean(182, 12, 0)
	warm := tempPred.PredictClean(182, 12, 1)
	fmt.Printf("  anomaly=0 → %.1f°C\n", base)
	fmt.Printf("  anomaly=1 → %.1f°C (diff: %+.1f°C)\n", warm, warm-base)

	tempData, err := tempPred.Save()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error serializing temperature model: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(*tempModelPath, tempData, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing temperature model to %s: %v\n", *tempModelPath, err)
		os.Exit(1)
	}
	fmt.Printf("\nTemperature model saved to %s (%d bytes)\n", *tempModelPath, len(tempData))

	// --- Train power model ---
	fmt.Println("\n=== Power Model ===")

	// Join power + temperature by timestamp.
	var powerSamples []predictor.Sample
	for ts, p := range powerByTS {
		ti, ok := tempByTS[ts]
		if !ok {
			continue
		}
		powerSamples = append(powerSamples, predictor.Sample{
			Month:       p.month,
			Hour:        p.hour,
			Temperature: ti.value,
			Power:       p.value,
		})
	}

	fmt.Printf("Joined training samples: %d\n", len(powerSamples))
	if len(powerSamples) == 0 {
		fmt.Fprintln(os.Stderr, "No matching samples found. Check that stats CSV contains both grid power and ext temperature data.")
		os.Exit(1)
	}

	fmt.Printf("Training: epochs=%d lr=%.4f batch_size=%d seed=%d\n", cfg.Epochs, cfg.LearningRate, cfg.BatchSize, *seed)

	powerPred, powerLosses := predictor.TrainPredictor(powerSamples, cfg, *seed)

	fmt.Printf("Initial val loss: %.6f\n", powerLosses[0])
	fmt.Printf("Final val loss:   %.6f\n", powerLosses[len(powerLosses)-1])
	fmt.Printf("Loss reduction:   %.1fx\n", powerLosses[0]/powerLosses[len(powerLosses)-1])

	// Print some sample predictions.
	fmt.Println("\nSample predictions (clean, no noise):")
	for _, hour := range []int{0, 6, 12, 18} {
		p := powerPred.PredictClean(6, hour, 20.0) // June, 20°C
		fmt.Printf("  June %02d:00 @ 20°C → %.0fW\n", hour, p)
	}

	// Print hourly noise std.
	fmt.Println("\nHourly noise std (W):")
	powerData, err := powerPred.Save()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error serializing power model: %v\n", err)
		os.Exit(1)
	}

	var savedModel predictor.SavedModel
	if err := json.Unmarshal(powerData, &savedModel); err == nil {
		minNoise, maxNoise := math.Inf(1), math.Inf(-1)
		for h := 0; h < 24; h++ {
			n := savedModel.HourlyNoiseStd[h]
			if n < minNoise {
				minNoise = n
			}
			if n > maxNoise {
				maxNoise = n
			}
			if h%6 == 0 {
				fmt.Printf("  %02d:00 → %.0fW\n", h, n)
			}
		}
		fmt.Printf("  Range: %.0f - %.0fW\n", minNoise, maxNoise)
	}

	// Write power model.
	if err := os.WriteFile(*powerModelPath, powerData, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing power model to %s: %v\n", *powerModelPath, err)
		os.Exit(1)
	}
	fmt.Printf("\nPower model saved to %s (%d bytes)\n", *powerModelPath, len(powerData))
}
