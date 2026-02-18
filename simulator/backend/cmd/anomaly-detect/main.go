package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"energy_simulator/internal/ingest"
	"energy_simulator/internal/model"
	"energy_simulator/internal/predictor"
	"energy_simulator/internal/store"
)

type dayStats struct {
	Date         string
	ActualKWh    float64
	PredictedKWh float64
	DeviationPct float64
	ActualTemp   float64
	PredTemp     float64
	TempDevC     float64
	Category     string
	Cause        string
}

func main() {
	inputDir := flag.String("input-dir", "input", "directory containing CSV data files")
	tempModelPath := flag.String("temp-model", "model/temperature.json", "path to temperature NN model")
	powerModelPath := flag.String("power-model", "model/grid_power.json", "path to grid power NN model")
	sigma := flag.Float64("sigma", 2.0, "standard deviation threshold for flagging anomalies")
	minKWh := flag.Float64("min-kwh", 1.0, "minimum daily kWh to consider a day")
	flag.Parse()

	dataStore := loadAllData(*inputDir)

	tr, ok := dataStore.GlobalTimeRange()
	if !ok {
		log.Fatal("No data loaded")
	}

	// Load NN models
	tempModelData, err := os.ReadFile(*tempModelPath)
	if err != nil {
		log.Fatalf("Loading temperature model: %v", err)
	}
	tempPred, err := predictor.LoadTemperaturePredictor(tempModelData, 42)
	if err != nil {
		log.Fatalf("Parsing temperature model: %v", err)
	}

	powerModelData, err := os.ReadFile(*powerModelPath)
	if err != nil {
		log.Fatalf("Loading power model: %v", err)
	}
	powerPred, err := predictor.LoadPredictor(powerModelData, 42)
	if err != nil {
		log.Fatalf("Parsing power model: %v", err)
	}

	gridPowerID := findSensorID(dataStore, model.SensorGridPower)
	if gridPowerID == "" {
		log.Fatal("No grid power sensor found")
	}

	extTempID := findSensorID(dataStore, model.SensorPumpExtTemp)

	days := tr.End.Sub(tr.Start).Hours() / 24

	fmt.Println()
	fmt.Println("Consumption Anomaly Detection")
	fmt.Printf("  Data: %s to %s (%.0f days)\n", tr.Start.Format("2006-01-02"), tr.End.Format("2006-01-02"), days)
	fmt.Printf("  Sigma threshold: %.1f | Min daily kWh: %.1f\n", *sigma, *minKWh)
	fmt.Println()

	// Compute daily actual vs predicted
	allDays := computeDailyStats(dataStore, gridPowerID, extTempID, tempPred, powerPred, tr, *minKWh)

	if len(allDays) == 0 {
		fmt.Println("No days with sufficient data found.")
		return
	}

	// Compute mean and stddev of deviation percentages
	var sum, sumSq float64
	for _, d := range allDays {
		sum += d.DeviationPct
		sumSq += d.DeviationPct * d.DeviationPct
	}
	n := float64(len(allDays))
	mean := sum / n
	variance := sumSq/n - mean*mean
	if variance < 0 {
		variance = 0
	}
	stddev := math.Sqrt(variance)

	// Flag anomalies
	var flagged []dayStats
	for i := range allDays {
		d := &allDays[i]
		if math.Abs(d.DeviationPct-mean) > *sigma*stddev {
			if d.ActualKWh > d.PredictedKWh {
				d.Category = "HIGH"
			} else {
				d.Category = "LOW"
			}
			d.Cause = inferCause(d)
			flagged = append(flagged, *d)
		}
	}

	// Summary
	fmt.Printf("  Days analyzed: %d\n", len(allDays))
	fmt.Printf("  Mean deviation: %+.1f%%\n", mean)
	fmt.Printf("  Std deviation:  %.1f%%\n", stddev)
	fmt.Printf("  Anomalies found: %d (%.1f%%)\n", len(flagged), 100*float64(len(flagged))/n)
	fmt.Println()

	if len(flagged) == 0 {
		fmt.Println("  No anomalous days detected.")
		return
	}

	// Print table
	fmt.Printf("  %-12s │ %8s │ %8s │ %8s │ %6s │ %6s │ %5s │ %s\n",
		"Date", "Actual", "Predict", "Dev %", "Temp", "P.Temp", "Type", "Possible Cause")
	fmt.Printf("  ─────────────┼──────────┼──────────┼──────────┼────────┼────────┼───────┼─────────────────────\n")

	for _, d := range flagged {
		tempStr := "  n/a"
		if d.ActualTemp != 0 || d.PredTemp != 0 {
			tempStr = fmt.Sprintf("%5.1f", d.ActualTemp)
		}
		fmt.Printf("  %-12s │ %7.1f  │ %7.1f  │ %+7.1f  │ %s │ %5.1f │ %5s │ %s\n",
			d.Date, d.ActualKWh, d.PredictedKWh, d.DeviationPct,
			tempStr, d.PredTemp, d.Category, d.Cause)
	}
	fmt.Println()
}

func computeDailyStats(
	s *store.Store,
	gridPowerID, extTempID string,
	tempPred *predictor.TemperaturePredictor,
	powerPred *predictor.EnergyPredictor,
	tr model.TimeRange,
	minKWh float64,
) []dayStats {
	readings := s.ReadingsInRange(gridPowerID, tr.Start, tr.End.Add(time.Nanosecond))
	if len(readings) < 2 {
		return nil
	}

	// Group actual import by calendar day (trapezoidal integration of positive power)
	type dayAccum struct {
		importWh     float64
		tempSum      float64
		tempCount    int
		readingCount int
	}
	dayMap := make(map[string]*dayAccum)

	for i := 1; i < len(readings); i++ {
		prev := readings[i-1]
		cur := readings[i]
		hours := cur.Timestamp.Sub(prev.Timestamp).Hours()
		if hours <= 0 || hours > 2 {
			continue
		}
		avgPower := (prev.Value + cur.Value) / 2
		if avgPower > 0 {
			dayKey := cur.Timestamp.Format("2006-01-02")
			acc, exists := dayMap[dayKey]
			if !exists {
				acc = &dayAccum{}
				dayMap[dayKey] = acc
			}
			acc.importWh += avgPower * hours
			acc.readingCount++
		}
	}

	// Gather actual temperatures by day
	if extTempID != "" {
		tempReadings := s.ReadingsInRange(extTempID, tr.Start, tr.End.Add(time.Nanosecond))
		for _, r := range tempReadings {
			dayKey := r.Timestamp.Format("2006-01-02")
			acc, exists := dayMap[dayKey]
			if !exists {
				acc = &dayAccum{}
				dayMap[dayKey] = acc
			}
			acc.tempSum += r.Value
			acc.tempCount++
		}
	}

	// Sort days
	dayKeys := make([]string, 0, len(dayMap))
	for k := range dayMap {
		dayKeys = append(dayKeys, k)
	}
	sort.Strings(dayKeys)

	var result []dayStats
	for _, dayKey := range dayKeys {
		acc := dayMap[dayKey]
		actualKWh := acc.importWh / 1000
		if actualKWh < minKWh {
			continue
		}

		t, _ := time.Parse("2006-01-02", dayKey)
		dayOfYear := t.YearDay()
		month := int(t.Month())

		// Predict: for each hour, get temp prediction then power prediction
		var predictedWh float64
		var predTempSum float64
		for hour := 0; hour < 24; hour++ {
			temp := tempPred.PredictClean(dayOfYear, hour, 0)
			predTempSum += temp
			power := powerPred.PredictClean(month, hour, temp)
			if power > 0 {
				predictedWh += power // 1 hour × watts = Wh
			}
		}
		predictedKWh := predictedWh / 1000
		predAvgTemp := predTempSum / 24

		var actualAvgTemp float64
		if acc.tempCount > 0 {
			actualAvgTemp = acc.tempSum / float64(acc.tempCount)
		}

		var deviationPct float64
		if predictedKWh > 0 {
			deviationPct = (actualKWh - predictedKWh) / predictedKWh * 100
		}

		result = append(result, dayStats{
			Date:         dayKey,
			ActualKWh:    actualKWh,
			PredictedKWh: predictedKWh,
			DeviationPct: deviationPct,
			ActualTemp:   actualAvgTemp,
			PredTemp:     predAvgTemp,
			TempDevC:     actualAvgTemp - predAvgTemp,
		})
	}

	return result
}

func inferCause(d *dayStats) string {
	if d.Category == "HIGH" {
		if d.TempDevC < -3 {
			return "Unexpected cold → extra heating"
		}
		if d.DeviationPct > 100 {
			return "Very high usage — guests or appliance fault?"
		}
		return "Above-normal consumption"
	}
	// LOW
	if d.TempDevC > 3 {
		return "Warmer than expected → less heating"
	}
	if d.DeviationPct < -50 {
		return "Very low usage — away from home?"
	}
	return "Below-normal consumption"
}

// --- Data loading (shared with load-analysis) ---

func loadAllData(inputDir string) *store.Store {
	dataStore := store.New()

	loadLegacyCSVs(inputDir, dataStore)

	recentDir := filepath.Join(inputDir, "recent")
	if entries, err := os.ReadDir(recentDir); err == nil {
		parser := &ingest.RecentParser{}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".csv") {
				continue
			}
			path := filepath.Join(recentDir, entry.Name())
			f, err := os.Open(path)
			if err != nil {
				log.Printf("Warning: opening %s: %v", path, err)
				continue
			}
			readings, err := parser.Parse(f)
			f.Close()
			if err != nil {
				log.Printf("Warning: parsing %s: %v", path, err)
				continue
			}
			if len(readings) > 0 {
				registerSensors(readings, dataStore)
				dataStore.AddReadings(readings)
			}
		}
	}

	statsDir := filepath.Join(inputDir, "stats")
	if entries, err := os.ReadDir(statsDir); err == nil {
		parser := &ingest.StatsParser{}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".csv") {
				continue
			}
			path := filepath.Join(statsDir, entry.Name())
			f, err := os.Open(path)
			if err != nil {
				log.Printf("Warning: opening %s: %v", path, err)
				continue
			}
			readings, err := parser.Parse(f)
			f.Close()
			if err != nil {
				log.Printf("Warning: parsing %s: %v", path, err)
				continue
			}
			if len(readings) > 0 {
				registerSensors(readings, dataStore)
				dataStore.AddReadings(readings)
			}
		}
	}

	return dataStore
}

func loadLegacyCSVs(dir string, s *store.Store) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		log.Fatalf("Reading input directory %s: %v", dir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".csv") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		f, err := os.Open(path)
		if err != nil {
			log.Fatalf("Opening %s: %v", path, err)
		}

		sensorType, unit := sensorTypeFromFilename(entry.Name())
		parser := ingest.NewHomeAssistantParser(sensorType, unit)
		readings, err := parser.Parse(f)
		f.Close()
		if err != nil {
			log.Fatalf("Parsing %s: %v", path, err)
		}

		if len(readings) > 0 {
			name := string(sensorType)
			if info, ok := model.SensorCatalog[sensorType]; ok {
				name = info.Name
			}
			s.AddSensor(model.Sensor{
				ID:   readings[0].SensorID,
				Name: name,
				Type: sensorType,
				Unit: unit,
			})
			s.AddReadings(readings)
		}
	}
}

func registerSensors(readings []model.Reading, s *store.Store) {
	seen := make(map[model.SensorType]bool)
	for _, r := range readings {
		if seen[r.Type] {
			continue
		}
		seen[r.Type] = true
		name := string(r.Type)
		unit := r.Unit
		if info, ok := model.SensorCatalog[r.Type]; ok {
			name = info.Name
			unit = info.Unit
		}
		s.AddSensor(model.Sensor{
			ID:   r.SensorID,
			Name: name,
			Type: r.Type,
			Unit: unit,
		})
	}
}

func findSensorID(s *store.Store, st model.SensorType) string {
	for _, sensor := range s.Sensors() {
		if sensor.Type == st {
			return sensor.ID
		}
	}
	return ""
}

func sensorTypeFromFilename(name string) (model.SensorType, string) {
	base := strings.TrimSuffix(name, ".csv")
	st := model.SensorType(base)
	if info, ok := model.SensorCatalog[st]; ok {
		return st, info.Unit
	}
	return st, ""
}
