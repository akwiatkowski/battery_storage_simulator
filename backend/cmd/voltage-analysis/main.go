package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"energy_simulator/internal/ingest"
	"energy_simulator/internal/model"
	"energy_simulator/internal/store"
)

type curtailmentEvent struct {
	Start      time.Time
	End        time.Time
	PeakPV     float64
	MinPV      float64
	MaxVoltage float64
	LostWh     float64
	LostPLN    float64
}

func main() {
	inputDir := flag.String("input-dir", "input", "directory containing CSV data files")
	voltageThreshold := flag.Float64("voltage-threshold", 253, "voltage threshold for curtailment detection (V)")
	minPV := flag.Float64("min-pv", 500, "minimum PV power to consider (W)")
	pvDropPct := flag.Float64("pv-drop-pct", 20, "PV drop percentage to flag curtailment")
	peakWindow := flag.Int("peak-window", 30, "rolling peak window (number of readings)")
	csvOut := flag.String("csv-out", "", "optional CSV output for scatter data")
	daylightStart := flag.Int("daylight-start", 9, "daylight start hour for curtailment detection")
	daylightEnd := flag.Int("daylight-end", 16, "daylight end hour for curtailment detection")
	flag.Parse()

	dataStore := loadAllData(*inputDir)

	tr, ok := dataStore.GlobalTimeRange()
	if !ok {
		log.Fatal("No data loaded")
	}

	days := tr.End.Sub(tr.Start).Hours() / 24

	fmt.Println()
	fmt.Println("Voltage & PV Curtailment Analysis")
	fmt.Printf("  Data: %s to %s (%.0f days)\n", tr.Start.Format("2006-01-02"), tr.End.Format("2006-01-02"), days)
	fmt.Println()

	voltageID := findSensorID(dataStore, model.SensorGridVoltage)
	pvID := findSensorID(dataStore, model.SensorPVPower)
	gridID := findSensorID(dataStore, model.SensorGridPower)
	priceID := findSensorID(dataStore, model.SensorEnergyPrice)

	if pvID == "" {
		log.Fatal("No PV power sensor found — PV data is required")
	}

	// Export summary (always available if we have PV + grid)
	if gridID != "" {
		printExportSummary(dataStore, gridID, pvID, priceID, tr)
	}

	if voltageID == "" {
		fmt.Println("  ⚠  No voltage sensor found — voltage analysis unavailable.")
		fmt.Println("     Run 'make ha-fetch-history' to fetch voltage data from Home Assistant.")
		fmt.Println()
		return
	}

	// Voltage summary
	printVoltageSummary(dataStore, voltageID, gridID, tr)

	// Curtailment detection
	events := detectCurtailment(
		dataStore, voltageID, pvID, priceID, tr,
		*voltageThreshold, *minPV, *pvDropPct, *peakWindow,
		*daylightStart, *daylightEnd,
	)

	if len(events) > 0 {
		printCurtailmentEvents(events)
	} else {
		fmt.Println("  No curtailment events detected.")
		fmt.Println()
	}

	// Scatter CSV export
	if *csvOut != "" && gridID != "" {
		writeScatterCSV(dataStore, voltageID, gridID, pvID, tr, *csvOut)
	}
}

func printExportSummary(s *store.Store, gridID, pvID, priceID string, tr model.TimeRange) {
	gridReadings := s.ReadingsInRange(gridID, tr.Start, tr.End.Add(time.Nanosecond))

	var exportWh, maxExportW float64
	var exportRevPLN float64

	for i := 1; i < len(gridReadings); i++ {
		prev := gridReadings[i-1]
		cur := gridReadings[i]
		hours := cur.Timestamp.Sub(prev.Timestamp).Hours()
		if hours <= 0 || hours > 2 {
			continue
		}
		avgPower := (prev.Value + cur.Value) / 2
		if avgPower < 0 {
			exportW := -avgPower
			exportWh += exportW * hours
			if exportW > maxExportW {
				maxExportW = exportW
			}
			if priceID != "" {
				if pr, ok := s.ReadingAt(priceID, cur.Timestamp); ok {
					exportRevPLN += (exportW * hours / 1000) * pr.Value
				}
			}
		}
	}

	fmt.Println("=== Export Summary ===")
	fmt.Printf("  Total export: %.1f kWh\n", exportWh/1000)
	fmt.Printf("  Max export power: %.0f W\n", maxExportW)
	if priceID != "" {
		fmt.Printf("  Export revenue: %.2f PLN\n", exportRevPLN)
	}
	fmt.Println()
}

func printVoltageSummary(s *store.Store, voltageID, gridID string, tr model.TimeRange) {
	readings := s.ReadingsInRange(voltageID, tr.Start, tr.End.Add(time.Nanosecond))
	if len(readings) == 0 {
		fmt.Println("  No voltage readings found.")
		return
	}

	var sum, maxV float64
	var count int
	var exportSum float64
	var exportCount int

	for _, r := range readings {
		sum += r.Value
		count++
		if r.Value > maxV {
			maxV = r.Value
		}
	}

	// During export moments
	if gridID != "" {
		for _, r := range readings {
			if gr, ok := s.ReadingAt(gridID, r.Timestamp); ok && gr.Value < 0 {
				exportSum += r.Value
				exportCount++
			}
		}
	}

	fmt.Println("=== Voltage Summary ===")
	fmt.Printf("  Readings: %d\n", count)
	fmt.Printf("  Avg voltage: %.1f V\n", sum/float64(count))
	fmt.Printf("  Max voltage: %.1f V\n", maxV)
	if exportCount > 0 {
		fmt.Printf("  Avg voltage during export: %.1f V (%d readings)\n", exportSum/float64(exportCount), exportCount)
	}
	fmt.Println()
}

func detectCurtailment(
	s *store.Store,
	voltageID, pvID, priceID string,
	tr model.TimeRange,
	voltageThresh, minPV, pvDropPct float64,
	peakWindow, daylightStart, daylightEnd int,
) []curtailmentEvent {
	pvReadings := s.ReadingsInRange(pvID, tr.Start, tr.End.Add(time.Nanosecond))
	if len(pvReadings) < 2 {
		return nil
	}

	dropFraction := 1 - pvDropPct/100

	// Track rolling PV peak
	var recentPV []float64
	var events []curtailmentEvent
	var current *curtailmentEvent

	for i := 1; i < len(pvReadings); i++ {
		cur := pvReadings[i]
		prev := pvReadings[i-1]
		hours := cur.Timestamp.Sub(prev.Timestamp).Hours()
		if hours <= 0 || hours > 2 {
			recentPV = nil
			if current != nil {
				events = append(events, *current)
				current = nil
			}
			continue
		}

		hour := cur.Timestamp.Hour()
		if hour < daylightStart || hour >= daylightEnd {
			if current != nil {
				events = append(events, *current)
				current = nil
			}
			continue
		}

		pvW := math.Abs(cur.Value) // PV may be reported as negative
		recentPV = append(recentPV, pvW)
		if len(recentPV) > peakWindow {
			recentPV = recentPV[1:]
		}

		if pvW < minPV {
			if current != nil {
				events = append(events, *current)
				current = nil
			}
			continue
		}

		// Rolling peak
		var peak float64
		for _, v := range recentPV {
			if v > peak {
				peak = v
			}
		}

		// Check voltage
		vr, hasVoltage := s.ReadingAt(voltageID, cur.Timestamp)
		if !hasVoltage {
			continue
		}

		isCurtailed := vr.Value > voltageThresh && pvW < peak*dropFraction && peak > minPV

		if isCurtailed {
			lostW := peak - pvW
			lostWh := lostW * hours

			var lostPLN float64
			if priceID != "" {
				if pr, ok := s.ReadingAt(priceID, cur.Timestamp); ok {
					lostPLN = (lostWh / 1000) * pr.Value
				}
			}

			if current != nil {
				// Extend current event
				current.End = cur.Timestamp
				if pvW < current.MinPV {
					current.MinPV = pvW
				}
				if peak > current.PeakPV {
					current.PeakPV = peak
				}
				if vr.Value > current.MaxVoltage {
					current.MaxVoltage = vr.Value
				}
				current.LostWh += lostWh
				current.LostPLN += lostPLN
			} else {
				current = &curtailmentEvent{
					Start:      cur.Timestamp,
					End:        cur.Timestamp,
					PeakPV:     peak,
					MinPV:      pvW,
					MaxVoltage: vr.Value,
					LostWh:     lostWh,
					LostPLN:    lostPLN,
				}
			}
		} else {
			if current != nil {
				events = append(events, *current)
				current = nil
			}
		}
	}
	if current != nil {
		events = append(events, *current)
	}

	return events
}

func printCurtailmentEvents(events []curtailmentEvent) {
	var totalLostKWh, totalLostPLN float64
	var totalDuration time.Duration
	for _, e := range events {
		totalLostKWh += e.LostWh / 1000
		totalLostPLN += e.LostPLN
		totalDuration += e.End.Sub(e.Start)
	}

	fmt.Println("=== PV Curtailment Detection ===")
	fmt.Printf("  Events: %d\n", len(events))
	fmt.Printf("  Total duration: %s\n", formatDuration(totalDuration))
	fmt.Printf("  Estimated lost energy: %.2f kWh\n", totalLostKWh)
	if totalLostPLN > 0 {
		fmt.Printf("  Estimated lost revenue: %.2f PLN\n", totalLostPLN)
	}
	fmt.Println()

	// Show up to 20 events
	limit := len(events)
	if limit > 20 {
		limit = 20
	}

	fmt.Printf("  %-19s │ %8s │ %7s │ %7s │ %6s │ %8s │ %8s\n",
		"Start", "Duration", "Peak PV", "Min PV", "Max V", "Lost kWh", "Lost PLN")
	fmt.Printf("  ────────────────────┼──────────┼─────────┼─────────┼────────┼──────────┼─────────\n")

	for i := 0; i < limit; i++ {
		e := events[i]
		dur := e.End.Sub(e.Start)
		fmt.Printf("  %-19s │ %8s │ %6.0f W │ %6.0f W │ %5.1f │ %8.3f │ %8.3f\n",
			e.Start.Format("2006-01-02 15:04"),
			formatDuration(dur),
			e.PeakPV, e.MinPV, e.MaxVoltage,
			e.LostWh/1000, e.LostPLN)
	}
	if len(events) > limit {
		fmt.Printf("  ... and %d more events\n", len(events)-limit)
	}
	fmt.Println()
}

func writeScatterCSV(s *store.Store, voltageID, gridID, pvID string, tr model.TimeRange, path string) {
	gridReadings := s.ReadingsInRange(gridID, tr.Start, tr.End.Add(time.Nanosecond))

	type scatterPoint struct {
		timestamp time.Time
		voltage   float64
		exportW   float64
		pvW       float64
	}

	var points []scatterPoint
	for _, r := range gridReadings {
		if r.Value >= 0 {
			continue // only export moments
		}
		vr, ok := s.ReadingAt(voltageID, r.Timestamp)
		if !ok {
			continue
		}
		var pvW float64
		if pr, ok := s.ReadingAt(pvID, r.Timestamp); ok {
			pvW = math.Abs(pr.Value)
		}
		points = append(points, scatterPoint{
			timestamp: r.Timestamp,
			voltage:   vr.Value,
			exportW:   -r.Value,
			pvW:       pvW,
		})
	}

	f, err := os.Create(path)
	if err != nil {
		log.Fatalf("Creating CSV file: %v", err)
	}
	defer f.Close()

	w := csv.NewWriter(f)
	w.Write([]string{"timestamp", "voltage_v", "export_w", "pv_w"})
	for _, p := range points {
		w.Write([]string{
			p.timestamp.Format(time.RFC3339),
			fmt.Sprintf("%.1f", p.voltage),
			fmt.Sprintf("%.0f", p.exportW),
			fmt.Sprintf("%.0f", p.pvW),
		})
	}
	w.Flush()
	if err := w.Error(); err != nil {
		log.Fatalf("Writing CSV: %v", err)
	}

	fmt.Printf("  Scatter data written to %s (%d points)\n\n", path, len(points))
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	return fmt.Sprintf("%dh%02dm", h, m)
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
