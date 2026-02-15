package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"energy_simulator/internal/ingest"
	"energy_simulator/internal/model"
	"energy_simulator/internal/simulator"
	"energy_simulator/internal/store"
)

// collector implements simulator.Callback, keeping only the latest summaries.
type collector struct {
	summary        simulator.Summary
	batterySummary simulator.BatterySummary
}

func (c *collector) OnState(simulator.State)                        {}
func (c *collector) OnReading(simulator.SensorReading)               {}
func (c *collector) OnSummary(s simulator.Summary)                   { c.summary = s }
func (c *collector) OnBatteryUpdate(simulator.BatteryUpdate)         {}
func (c *collector) OnBatterySummary(s simulator.BatterySummary)     { c.batterySummary = s }
func (c *collector) OnArbitrageDayLog([]simulator.ArbitrageDayRecord)       {}
func (c *collector) OnPredictionComparison(simulator.PredictionComparison) {}
func (c *collector) OnHeatingStats([]simulator.HeatingMonthStat)           {}
func (c *collector) OnAnomalyDays([]simulator.AnomalyDayRecord)            {}

type result struct {
	capacity float64
	maxPower float64
	summary  simulator.Summary
	battery  simulator.BatterySummary
}

func main() {
	inputDir := flag.String("input-dir", "input", "directory containing CSV data files")
	cRate := flag.Float64("max-power-rate", 0.5, "C-rate for max charge/discharge power")
	floor := flag.Float64("discharge-floor", 10, "minimum SoC percent")
	ceiling := flag.Float64("charge-ceiling", 100, "maximum SoC percent")
	stepFlag := flag.String("step", "6h", "simulation step size (e.g. 1h, 6h, 24h)")
	capsFlag := flag.String("capacities", "5,7.5,10,12.5,15,20,25,30,40,50", "comma-separated battery capacities in kWh")
	hpPct := flag.Float64("heat-pump-pct", 100, "heat pump usage percentage for off-grid coverage (0-100)")
	appPct := flag.Float64("appliance-pct", 100, "appliance usage percentage for off-grid coverage (0-100)")
	flag.Parse()

	stepDuration, err := time.ParseDuration(*stepFlag)
	if err != nil {
		log.Fatalf("Invalid step duration %q: %v", *stepFlag, err)
	}

	capacities, err := parseCapacities(*capsFlag)
	if err != nil {
		log.Fatalf("Invalid capacities %q: %v", *capsFlag, err)
	}
	sort.Float64s(capacities)

	results := make([]result, 0, len(capacities))
	for _, cap := range capacities {
		maxPower := cap * *cRate * 1000
		dataStore := loadCSVs(*inputDir)
		cb := &collector{}
		engine := simulator.New(dataStore, cb)
		if !engine.Init() {
			log.Fatal("Failed to initialize simulation engine (no data?)")
		}
		engine.SetBattery(&simulator.BatteryConfig{
			CapacityKWh:        cap,
			MaxPowerW:          maxPower,
			DischargeToPercent: *floor,
			ChargeToPercent:    *ceiling,
		})
		tr := engine.TimeRange()
		for engine.State().Time.Before(tr.End) {
			engine.Step(stepDuration)
		}
		results = append(results, result{
			capacity: cap,
			maxPower: maxPower,
			summary:  cb.summary,
			battery:  cb.batterySummary,
		})
		fmt.Fprintf(os.Stderr, "  %.1f kWh done\n", cap)
	}

	printTable(results, *floor, *ceiling, *cRate, *hpPct, *appPct, *inputDir)
}

func printTable(results []result, floor, ceiling, cRate, hpPct, appPct float64, inputDir string) {
	if len(results) == 0 {
		return
	}

	// Header info: use time range from first result's summary context
	// We re-derive from a quick store load
	dataStore := loadCSVs(inputDir)
	tr, _ := dataStore.GlobalTimeRange()
	days := tr.End.Sub(tr.Start).Hours() / 24

	fmt.Println()
	fmt.Println("Battery Size Comparison")
	fmt.Printf("  Discharge floor: %.0f%%, Charge ceiling: %.0f%%, C-rate: %.1f\n", floor, ceiling, cRate)
	fmt.Printf("  Data: %s to %s (%.0f days)\n", tr.Start.Format("2006-01-02"), tr.End.Format("2006-01-02"), days)
	fmt.Printf("  Off-grid calc: heat pump %.0f%%, appliances %.0f%%\n", hpPct, appPct)
	fmt.Println()

	// Table header
	fmt.Printf(" %8s │ %9s │ %11s │ %9s │ %6s │ %8s │ %11s │ %8s\n",
		"Capacity", "Max Power", "Grid Import", " Savings ", "Cycles", "Marginal", "Savings/kWh", "Off-Grid")
	fmt.Printf("──────────┼───────────┼─────────────┼───────────┼────────┼──────────┼─────────────┼──────────\n")

	for i, r := range results {
		savings := r.summary.BatterySavingsKWh
		savingsPerKWh := savings / r.capacity
		offGrid := r.summary.OffGridCoverage(hpPct, appPct)

		marginal := "-"
		if i > 0 {
			prev := results[i-1]
			prevSavings := prev.summary.BatterySavingsKWh
			dCap := r.capacity - prev.capacity
			if dCap > 0 {
				m := (savings - prevSavings) / dCap
				marginal = fmt.Sprintf("%.1f", m)
			}
		}

		fmt.Printf(" %5.1f kWh │ %5.1f kW  │ %8.1f kWh │ %6.1f kWh│ %6.1f │ %8s │ %8.1f kWh │ %7.1f%%\n",
			r.capacity,
			r.maxPower/1000,
			r.summary.GridImportKWh,
			savings,
			r.battery.Cycles,
			marginal,
			savingsPerKWh,
			offGrid,
		)
	}
	fmt.Println()
}

func parseCapacities(s string) ([]float64, error) {
	parts := strings.Split(s, ",")
	caps := make([]float64, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		v, err := strconv.ParseFloat(p, 64)
		if err != nil {
			return nil, fmt.Errorf("parsing %q: %w", p, err)
		}
		if v <= 0 {
			return nil, fmt.Errorf("capacity must be positive, got %v", v)
		}
		caps = append(caps, v)
	}
	if len(caps) == 0 {
		return nil, fmt.Errorf("no capacities specified")
	}
	return caps, nil
}

func loadCSVs(dir string) *store.Store {
	dataStore := store.New()
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
			dataStore.AddSensor(model.Sensor{
				ID:   readings[0].SensorID,
				Name: name,
				Type: sensorType,
				Unit: unit,
			})
			dataStore.AddReadings(readings)
		}
	}
	return dataStore
}

func sensorTypeFromFilename(name string) (model.SensorType, string) {
	base := strings.TrimSuffix(name, ".csv")
	st := model.SensorType(base)
	if info, ok := model.SensorCatalog[st]; ok {
		return st, info.Unit
	}
	return st, ""
}

