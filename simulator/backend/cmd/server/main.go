package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"energy_simulator/internal/ingest"
	"energy_simulator/internal/model"
	"energy_simulator/internal/predictor"
	"energy_simulator/internal/simulator"
	"energy_simulator/internal/store"
	"energy_simulator/internal/ws"
)

func main() {
	inputDir := flag.String("input-dir", "input", "directory containing CSV data files")
	frontendDir := flag.String("frontend-dir", "simulator/frontend/build", "directory containing frontend build")
	addr := flag.String("addr", ":8080", "listen address")
	flag.Parse()

	// Load CSV data
	dataStore := store.New()
	sourceRanges := make(map[string]model.TimeRange)

	legacyRange, err := loadCSVs(*inputDir, dataStore)
	if err != nil {
		log.Fatalf("Failed to load CSV data: %v", err)
	}

	statsRange, _, err := loadMultiSensorCSVs(filepath.Join(*inputDir, "stats"), &ingest.StatsParser{}, dataStore)
	if err != nil {
		log.Printf("Stats data: %v", err)
	}

	recentRange, recentGPRange, err := loadMultiSensorCSVs(filepath.Join(*inputDir, "recent"), &ingest.RecentParser{}, dataStore)
	if err != nil {
		log.Printf("Recent data: %v", err)
	}

	// Build archival range (legacy + stats)
	archivalRange := mergeTimeRanges(legacyRange, statsRange)
	if !archivalRange.Start.IsZero() {
		sourceRanges["archival"] = archivalRange
	}
	// Use grid power range for "current" source — other sensors like spot
	// prices span years of history and would drag the start time back.
	if !recentGPRange.Start.IsZero() {
		sourceRanges["current"] = recentGPRange
	} else if !recentRange.Start.IsZero() {
		sourceRanges["current"] = recentRange
	}

	tr, ok := dataStore.GlobalTimeRange()
	if !ok {
		log.Fatal("No data loaded")
	}

	// Constrain start to first grid power reading — other sensors may have
	// earlier data but the simulation is grid-power-centric.
	if gpID := findSensorID(dataStore, model.SensorGridPower); gpID != "" {
		if gpRange, ok := dataStore.TimeRange(gpID); ok {
			tr.Start = gpRange.Start
		}
	}

	sourceRanges["all"] = tr
	log.Printf("Data loaded: %s to %s", tr.Start.Format("2006-01-02"), tr.End.Format("2006-01-02"))

	// Set up WebSocket hub and simulator
	hub := ws.NewHub()
	bridge := ws.NewBridge(hub)
	engine := simulator.New(dataStore, bridge)
	if !engine.Init() {
		log.Fatal("Failed to initialize simulation engine")
	}
	engine.SetTimeRange(tr)

	// Configure price sensor for cost tracking
	if priceID := findSensorID(dataStore, model.SensorEnergyPrice); priceID != "" {
		engine.SetPriceSensor(priceID)
		log.Printf("Price sensor configured: %s", priceID)
	}

	// Configure temperature sensor for prediction comparison
	if tempID := findSensorID(dataStore, model.SensorPumpExtTemp); tempID != "" {
		engine.SetTempSensor(tempID)
		log.Printf("Temperature sensor configured: %s", tempID)
	}

	// Attempt to load NN models for prediction mode
	loadPredictionModels(engine, dataStore)

	handler := ws.NewHandler(hub, engine, sourceRanges)

	// Routes
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "ok")
	})
	mux.Handle("/ws", handler)

	// Serve frontend static files
	if _, err := os.Stat(*frontendDir); err == nil {
		log.Printf("Serving frontend from %s", *frontendDir)
		mux.Handle("/", http.FileServer(http.Dir(*frontendDir)))
	}

	log.Printf("Starting server on %s", *addr)
	if err := http.ListenAndServe(*addr, mux); err != nil {
		log.Fatal(err)
	}
}

// loadCSVs loads legacy per-sensor CSV files from the root input directory.
// Returns the combined time range of all loaded readings.
func loadCSVs(dir string, s *store.Store) (model.TimeRange, error) {
	var tr model.TimeRange
	entries, err := os.ReadDir(dir)
	if err != nil {
		return tr, fmt.Errorf("reading input directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".csv") {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		log.Printf("Loading %s...", path)

		f, err := os.Open(path)
		if err != nil {
			return tr, fmt.Errorf("opening %s: %w", path, err)
		}

		sensorType, unit := sensorTypeFromFilename(entry.Name())

		parser := ingest.NewHomeAssistantParser(sensorType, unit)
		readings, err := parser.Parse(f)
		f.Close()
		if err != nil {
			return tr, fmt.Errorf("parsing %s: %w", path, err)
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
			tr = extendTimeRange(tr, readings)
			log.Printf("  Loaded %d readings from %s", len(readings), entry.Name())
		}
	}

	return tr, nil
}

// loadMultiSensorCSVs loads CSV files from a subdirectory using a multi-sensor
// parser (StatsParser or RecentParser). It registers any new sensors discovered.
// Returns the combined time range and the grid-power-only time range.
func loadMultiSensorCSVs(dir string, p interface{ Parse(io.Reader) ([]model.Reading, error) }, s *store.Store) (all, gridPower model.TimeRange, err error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return all, gridPower, fmt.Errorf("reading directory %s: %w", dir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".csv") {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		log.Printf("Loading %s...", path)

		f, err := os.Open(path)
		if err != nil {
			return all, gridPower, fmt.Errorf("opening %s: %w", path, err)
		}

		readings, err := p.Parse(f)
		f.Close()
		if err != nil {
			return all, gridPower, fmt.Errorf("parsing %s: %w", path, err)
		}

		if len(readings) > 0 {
			registerSensorsFromReadings(readings, s)
			s.AddReadings(readings)
			all = extendTimeRange(all, readings)
			for _, r := range readings {
				if r.Type == model.SensorGridPower {
					if gridPower.Start.IsZero() || r.Timestamp.Before(gridPower.Start) {
						gridPower.Start = r.Timestamp
					}
					if r.Timestamp.After(gridPower.End) {
						gridPower.End = r.Timestamp
					}
				}
			}
			log.Printf("  Loaded %d readings from %s", len(readings), entry.Name())
		}
	}

	return all, gridPower, nil
}

// registerSensorsFromReadings registers sensors discovered in multi-sensor files.
func registerSensorsFromReadings(readings []model.Reading, s *store.Store) {
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

// findSensorID returns the ID of the first sensor matching the given type, or "".
func findSensorID(s *store.Store, st model.SensorType) string {
	for _, sensor := range s.Sensors() {
		if sensor.Type == st {
			return sensor.ID
		}
	}
	return ""
}

// extendTimeRange extends tr to include the min/max timestamps from readings.
func extendTimeRange(tr model.TimeRange, readings []model.Reading) model.TimeRange {
	for _, r := range readings {
		if tr.Start.IsZero() || r.Timestamp.Before(tr.Start) {
			tr.Start = r.Timestamp
		}
		if r.Timestamp.After(tr.End) {
			tr.End = r.Timestamp
		}
	}
	return tr
}

// mergeTimeRanges returns the union of two time ranges. Zero-value ranges are ignored.
func mergeTimeRanges(a, b model.TimeRange) model.TimeRange {
	if a.Start.IsZero() {
		return b
	}
	if b.Start.IsZero() {
		return a
	}
	result := a
	if b.Start.Before(result.Start) {
		result.Start = b.Start
	}
	if b.End.After(result.End) {
		result.End = b.End
	}
	return result
}

func loadPredictionModels(engine *simulator.Engine, s *store.Store) {
	tempData, err := os.ReadFile("simulator/backend/model/temperature.json")
	if err != nil {
		log.Printf("Temperature model not found: %v (prediction mode unavailable)", err)
		return
	}
	powerData, err := os.ReadFile("simulator/backend/model/grid_power.json")
	if err != nil {
		log.Printf("Grid power model not found: %v (prediction mode unavailable)", err)
		return
	}

	tempPred, err := predictor.LoadTemperaturePredictor(tempData, 42)
	if err != nil {
		log.Printf("Failed to load temperature model: %v", err)
		return
	}
	powerPred, err := predictor.LoadPredictor(powerData, 42)
	if err != nil {
		log.Printf("Failed to load grid power model: %v", err)
		return
	}

	gridID := findSensorID(s, model.SensorGridPower)
	if gridID == "" {
		log.Printf("No grid power sensor found, prediction mode unavailable")
		return
	}

	provider := simulator.NewPredictionProvider(tempPred, powerPred, gridID)
	engine.SetPrediction(provider)
	log.Printf("NN prediction models loaded successfully")
}

func sensorTypeFromFilename(name string) (model.SensorType, string) {
	base := strings.TrimSuffix(name, ".csv")
	st := model.SensorType(base)
	if info, ok := model.SensorCatalog[st]; ok {
		return st, info.Unit
	}
	return st, ""
}
