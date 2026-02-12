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
	sourceRanges := make(map[string]model.TimeRange)

	legacyRange, err := loadCSVs(*inputDir, dataStore)
	if err != nil {
		log.Fatalf("Failed to load CSV data: %v", err)
	}

	statsRange, err := loadMultiSensorCSVs(filepath.Join(*inputDir, "stats"), &ingest.StatsParser{}, dataStore)
	if err != nil {
		log.Printf("Stats data: %v", err)
	}

	recentRange, err := loadMultiSensorCSVs(filepath.Join(*inputDir, "recent"), &ingest.RecentParser{}, dataStore)
	if err != nil {
		log.Printf("Recent data: %v", err)
	}

	// Build archival range (legacy + stats)
	archivalRange := mergeTimeRanges(legacyRange, statsRange)
	if !archivalRange.Start.IsZero() {
		sourceRanges["archival"] = archivalRange
	}
	if !recentRange.Start.IsZero() {
		sourceRanges["current"] = recentRange
	}

	tr, ok := dataStore.GlobalTimeRange()
	if !ok {
		log.Fatal("No data loaded")
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
// Returns the combined time range of all loaded readings.
func loadMultiSensorCSVs(dir string, p interface{ Parse(io.Reader) ([]model.Reading, error) }, s *store.Store) (model.TimeRange, error) {
	var tr model.TimeRange
	entries, err := os.ReadDir(dir)
	if err != nil {
		return tr, fmt.Errorf("reading directory %s: %w", dir, err)
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

		readings, err := p.Parse(f)
		f.Close()
		if err != nil {
			return tr, fmt.Errorf("parsing %s: %w", path, err)
		}

		if len(readings) > 0 {
			registerSensorsFromReadings(readings, s)
			s.AddReadings(readings)
			tr = extendTimeRange(tr, readings)
			log.Printf("  Loaded %d readings from %s", len(readings), entry.Name())
		}
	}

	return tr, nil
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

func sensorTypeFromFilename(name string) (model.SensorType, string) {
	base := strings.TrimSuffix(name, ".csv")
	st := model.SensorType(base)
	if info, ok := model.SensorCatalog[st]; ok {
		return st, info.Unit
	}
	return st, ""
}
