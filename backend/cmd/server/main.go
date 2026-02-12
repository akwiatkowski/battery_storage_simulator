package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"energy_simulator/internal/ingest"
	"energy_simulator/internal/model"
	"energy_simulator/internal/simulator"
	"energy_simulator/internal/store"
	"energy_simulator/internal/ws"
)

func main() {
	inputDir := flag.String("input-dir", "input", "directory containing CSV data files")
	frontendDir := flag.String("frontend-dir", "frontend/build", "directory containing frontend build")
	addr := flag.String("addr", ":8080", "listen address")
	flag.Parse()

	// Load CSV data
	dataStore := store.New()
	if err := loadCSVs(*inputDir, dataStore); err != nil {
		log.Fatalf("Failed to load CSV data: %v", err)
	}

	tr, ok := dataStore.GlobalTimeRange()
	if !ok {
		log.Fatal("No data loaded")
	}
	log.Printf("Data loaded: %s to %s", tr.Start.Format("2006-01-02"), tr.End.Format("2006-01-02"))

	// Set up WebSocket hub and simulator
	hub := ws.NewHub()
	bridge := ws.NewBridge(hub)
	engine := simulator.New(dataStore, bridge)
	if !engine.Init() {
		log.Fatal("Failed to initialize simulation engine")
	}

	handler := ws.NewHandler(hub, engine)

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

func loadCSVs(dir string, s *store.Store) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("reading input directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".csv") {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		log.Printf("Loading %s...", path)

		f, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("opening %s: %w", path, err)
		}

		// Determine sensor type from filename
		sensorType, unit := sensorTypeFromFilename(entry.Name())

		parser := ingest.NewHomeAssistantParser(sensorType, unit)
		readings, err := parser.Parse(f)
		f.Close()
		if err != nil {
			return fmt.Errorf("parsing %s: %w", path, err)
		}

		if len(readings) > 0 {
			s.AddSensor(model.Sensor{
				ID:   readings[0].SensorID,
				Name: sensorNameFromType(sensorType),
				Type: sensorType,
				Unit: unit,
			})
			s.AddReadings(readings)
			log.Printf("  Loaded %d readings from %s", len(readings), entry.Name())
		}
	}

	return nil
}

func sensorTypeFromFilename(name string) (model.SensorType, string) {
	base := strings.TrimSuffix(name, ".csv")
	switch base {
	case "grid_power":
		return model.SensorGridPower, "W"
	case "pv_power":
		return model.SensorPVPower, "W"
	case "pump_total_consumption":
		return model.SensorPumpConsumption, "W"
	case "pump_total_production":
		return model.SensorPumpProduction, "W"
	default:
		return model.SensorType(base), ""
	}
}

func sensorNameFromType(t model.SensorType) string {
	switch t {
	case model.SensorGridPower:
		return "Grid Power"
	case model.SensorPVPower:
		return "PV Power"
	case model.SensorPumpConsumption:
		return "Heat Pump Consumption"
	case model.SensorPumpProduction:
		return "Heat Pump Production"
	default:
		return string(t)
	}
}
