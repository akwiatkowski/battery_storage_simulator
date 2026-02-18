// sample-predict demonstrates using the trained neural networks to generate
// sample grid power readings. First the temperature NN predicts external
// temperature from day-of-year, hour, and anomaly, then the power NN
// uses that temperature to predict grid power.
//
// Usage:
//
//	sample-predict
//	sample-predict -anomaly 1.0
//	sample-predict -hours 72 -clean
//	sample-predict -temp-model model/temperature.json -power-model model/grid_power.json
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"energy_simulator/internal/predictor"
)

func main() {
	tempModelPath := flag.String("temp-model", "model/temperature.json", "path to temperature model JSON")
	powerModelPath := flag.String("power-model", "model/grid_power.json", "path to power model JSON")
	hours := flag.Int("hours", 48, "number of hours to predict")
	anomaly := flag.Float64("anomaly", 0, "temperature anomaly (0=normal, +1=warmer by 0.1-3°C)")
	clean := flag.Bool("clean", false, "omit noise (deterministic output)")
	seed := flag.Uint64("seed", 0, "random seed for noise (0 = use current time)")
	csvOut := flag.Bool("csv", false, "output as CSV")
	flag.Parse()

	// Load temperature model.
	tempData, err := os.ReadFile(*tempModelPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading temperature model: %v\n", err)
		os.Exit(1)
	}

	if *seed == 0 {
		*seed = uint64(time.Now().UnixNano())
	}

	tempPred, err := predictor.LoadTemperaturePredictor(tempData, *seed)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading temperature model: %v\n", err)
		os.Exit(1)
	}

	// Load power model.
	powerData, err := os.ReadFile(*powerModelPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading power model: %v\n", err)
		os.Exit(1)
	}

	powerPred, err := predictor.LoadPredictor(powerData, *seed+1)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading power model: %v\n", err)
		os.Exit(1)
	}

	now := time.Now()

	if !*csvOut {
		fmt.Printf("Generating %d hours of predictions starting from %s\n", *hours, now.Format("2006-01-02 15:04"))
		fmt.Printf("Anomaly: %.1f\n", *anomaly)
		if *clean {
			fmt.Println("Mode: clean (no noise)")
		} else {
			fmt.Println("Mode: with noise")
		}
		fmt.Println()
		fmt.Printf("%-20s  %9s  %9s\n", "Time", "Temp (°C)", "Power (W)")
		fmt.Printf("%-20s  %9s  %9s\n", "--------------------", "---------", "---------")
	} else {
		fmt.Println("timestamp,temp_c,power_w")
	}

	// Generate temperature sequence (correlated noise + rate constraints).
	startDay := now.YearDay()
	startHour := now.Hour()
	var temps []float64
	if *clean {
		temps = tempPred.PredictCleanSequence(startDay, startHour, *hours, *anomaly)
	} else {
		temps = tempPred.PredictSequence(startDay, startHour, *hours, *anomaly)
	}

	// Feed each temperature into the power model.
	for i := 0; i < *hours; i++ {
		t := now.Add(time.Duration(i) * time.Hour)
		month := int(t.Month())
		hour := t.Hour()
		temp := temps[i]

		var power float64
		if *clean {
			power = powerPred.PredictClean(month, hour, temp)
		} else {
			power = powerPred.Predict(month, hour, temp)
		}

		if *csvOut {
			fmt.Printf("%s,%.1f,%.1f\n", t.Format(time.RFC3339), temp, power)
		} else {
			fmt.Printf("%-20s  %9.1f  %9.0f\n", t.Format("2006-01-02 15:04"), temp, power)
		}
	}
}
