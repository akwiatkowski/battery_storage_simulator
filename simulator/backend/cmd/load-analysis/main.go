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
	"energy_simulator/internal/store"
)

type HourlyBucket struct {
	KWh           float64
	CostPLN       float64
	ReadingsCount int
}

type COPBucket struct {
	TempMin, TempMax float64
	ConsumptionWh    float64
	ProductionWh     float64
	Hours            float64
}

type ShiftResult struct {
	CurrentCostPLN float64
	OptimalCostPLN float64
	SavingsPLN     float64
}

func main() {
	inputDir := flag.String("input-dir", "input", "directory containing CSV data files")
	shiftWindow := flag.Int("shift-window", 4, "max hours to shift load")
	minPower := flag.Float64("min-power", 50, "min watts to count as active")
	tempBucket := flag.Float64("temp-bucket", 5, "temperature bucket width in °C")
	flag.Parse()

	dataStore := loadAllData(*inputDir)

	tr, ok := dataStore.GlobalTimeRange()
	if !ok {
		log.Fatal("No data loaded")
	}

	priceSensorID := findSensorID(dataStore, model.SensorEnergyPrice)
	if priceSensorID == "" {
		log.Fatal("No price sensor found — price data is required for load analysis")
	}

	days := tr.End.Sub(tr.Start).Hours() / 24

	fmt.Println()
	fmt.Println("Load Shifting Analysis")
	fmt.Printf("  Data: %s to %s (%.0f days)\n", tr.Start.Format("2006-01-02"), tr.End.Format("2006-01-02"), days)
	fmt.Println()

	// Compute overall average spot price
	overallAvgSpot := computeOverallAvgSpotPrice(dataStore, priceSensorID, tr)

	// Heat pump analysis (consumption + production + ext temp)
	consumptionID := findSensorID(dataStore, model.SensorPumpConsumption)
	productionID := findSensorID(dataStore, model.SensorPumpProduction)
	extTempID := findSensorID(dataStore, model.SensorPumpExtTemp)

	if consumptionID != "" {
		fmt.Println("=== Heat Pump ===")

		hourly := aggregateByHour(dataStore, consumptionID, priceSensorID, tr)
		totalKWh, totalCost := sumHourly(hourly)
		avgPrice := safeDivide(totalCost, totalKWh)

		var totalProdKWh float64
		if productionID != "" {
			prodHourly := aggregateByHour(dataStore, productionID, "", tr)
			totalProdKWh, _ = sumHourly(prodHourly)
		}

		cop := safeDivide(totalProdKWh, totalKWh)
		efficiency := safeDivide(avgPrice, overallAvgSpot)

		fmt.Printf("  Consumption: %s   Production: %s   COP: %.1f\n",
			formatKWh(totalKWh), formatKWh(totalProdKWh), cop)
		fmt.Printf("  Cost at spot: %.2f PLN   Avg price: %.2f PLN/kWh\n", totalCost, avgPrice)
		fmt.Printf("  Overall avg spot: %.2f PLN/kWh   Efficiency: %.2fx", overallAvgSpot, efficiency)
		if efficiency > 1.01 {
			fmt.Print(" (worse)")
		} else if efficiency < 0.99 {
			fmt.Print(" (better)")
		}
		fmt.Println()

		// COP by temperature
		if productionID != "" && extTempID != "" {
			copBuckets := computeCOPCurve(dataStore, consumptionID, productionID, extTempID, tr, *tempBucket, *minPower)
			if len(copBuckets) > 0 {
				fmt.Println()
				printCOPTable(copBuckets)
			}
		}

		// Hourly distribution
		fmt.Println()
		printHourlyTable(hourly, totalKWh)

		// Shift potential
		shift := computeShiftPotential(dataStore, consumptionID, priceSensorID, tr, *shiftWindow, *minPower)
		if shift.CurrentCostPLN > 0 {
			fmt.Println()
			printShiftResult(shift, *shiftWindow)
		}

		fmt.Println()
	}

	// Other appliance sensors
	applianceTypes := map[model.SensorType]string{
		model.SensorPumpHeatPower: "Heat Pump Heating",
		model.SensorPumpCWUPower:  "Heat Pump DHW",
		model.SensorElectricKettle: "Electric Kettle",
		model.SensorOven:           "Oven",
		model.SensorWashing:        "Washing Machine",
		model.SensorDrier:          "Drier",
		model.SensorTvMedia:        "TV & Media",
	}

	for sensorType, name := range applianceTypes {
		sensorID := findSensorID(dataStore, sensorType)
		if sensorID == "" {
			continue
		}
		// Skip pump sub-sensors if we already printed the main consumption
		if sensorType == model.SensorPumpHeatPower || sensorType == model.SensorPumpCWUPower {
			if consumptionID != "" {
				continue
			}
		}

		hourly := aggregateByHour(dataStore, sensorID, priceSensorID, tr)
		totalKWh, totalCost := sumHourly(hourly)
		if totalKWh < 0.1 {
			continue
		}

		avgPrice := safeDivide(totalCost, totalKWh)
		efficiency := safeDivide(avgPrice, overallAvgSpot)

		fmt.Printf("=== %s ===\n", name)
		fmt.Printf("  Consumption: %s\n", formatKWh(totalKWh))
		fmt.Printf("  Cost at spot: %.2f PLN   Avg price: %.2f PLN/kWh\n", totalCost, avgPrice)
		fmt.Printf("  Overall avg spot: %.2f PLN/kWh   Efficiency: %.2fx", overallAvgSpot, efficiency)
		if efficiency > 1.01 {
			fmt.Print(" (worse)")
		} else if efficiency < 0.99 {
			fmt.Print(" (better)")
		}
		fmt.Println()

		printHourlyTable(hourly, totalKWh)

		shift := computeShiftPotential(dataStore, sensorID, priceSensorID, tr, *shiftWindow, *minPower)
		if shift.CurrentCostPLN > 0 {
			fmt.Println()
			printShiftResult(shift, *shiftWindow)
		}
		fmt.Println()
	}
}

func aggregateByHour(s *store.Store, sensorID, priceSensorID string, tr model.TimeRange) [24]HourlyBucket {
	var buckets [24]HourlyBucket
	readings := s.ReadingsInRange(sensorID, tr.Start, tr.End.Add(time.Nanosecond))
	for i := 1; i < len(readings); i++ {
		prev := readings[i-1]
		cur := readings[i]
		hours := cur.Timestamp.Sub(prev.Timestamp).Hours()
		if hours <= 0 || hours > 2 {
			continue
		}
		avgPower := (prev.Value + cur.Value) / 2
		if avgPower <= 0 {
			continue
		}
		wh := avgPower * hours
		kwh := wh / 1000

		var price float64
		if priceSensorID != "" {
			if pr, ok := s.ReadingAt(priceSensorID, cur.Timestamp); ok {
				price = pr.Value
			}
		}

		h := cur.Timestamp.Hour()
		buckets[h].KWh += kwh
		buckets[h].CostPLN += kwh * price
		buckets[h].ReadingsCount++
	}
	return buckets
}

func computeCOPCurve(s *store.Store, consumptionID, productionID, extTempID string, tr model.TimeRange, bucketWidth, minPower float64) []COPBucket {
	consumptionReadings := s.ReadingsInRange(consumptionID, tr.Start, tr.End.Add(time.Nanosecond))

	// Build a map: temp bucket index → accumulator
	type accum struct {
		consumptionWh float64
		productionWh  float64
		hours         float64
	}
	bucketMap := make(map[int]*accum)

	for i := 1; i < len(consumptionReadings); i++ {
		prev := consumptionReadings[i-1]
		cur := consumptionReadings[i]
		hours := cur.Timestamp.Sub(prev.Timestamp).Hours()
		if hours <= 0 || hours > 2 {
			continue
		}
		avgConsumption := (prev.Value + cur.Value) / 2
		if avgConsumption < minPower {
			continue
		}
		consumptionWh := avgConsumption * hours

		// Look up production at same timestamp
		prodReading, ok := s.ReadingAt(productionID, cur.Timestamp)
		if !ok {
			continue
		}
		// Also need previous production for averaging
		prevProdReading, ok := s.ReadingAt(productionID, prev.Timestamp)
		if !ok {
			continue
		}
		avgProduction := (prevProdReading.Value + prodReading.Value) / 2
		if avgProduction <= 0 {
			continue
		}
		productionWh := avgProduction * hours

		// Look up ext temp
		tempReading, ok := s.ReadingAt(extTempID, cur.Timestamp)
		if !ok {
			continue
		}

		bucketIdx := int(math.Floor(tempReading.Value / bucketWidth))
		if _, exists := bucketMap[bucketIdx]; !exists {
			bucketMap[bucketIdx] = &accum{}
		}
		bucketMap[bucketIdx].consumptionWh += consumptionWh
		bucketMap[bucketIdx].productionWh += productionWh
		bucketMap[bucketIdx].hours += hours
	}

	// Convert to sorted slice
	indices := make([]int, 0, len(bucketMap))
	for idx := range bucketMap {
		indices = append(indices, idx)
	}
	sort.Ints(indices)

	result := make([]COPBucket, 0, len(indices))
	for _, idx := range indices {
		a := bucketMap[idx]
		result = append(result, COPBucket{
			TempMin:       float64(idx) * bucketWidth,
			TempMax:       float64(idx)*bucketWidth + bucketWidth,
			ConsumptionWh: a.consumptionWh,
			ProductionWh:  a.productionWh,
			Hours:         a.hours,
		})
	}
	return result
}

func computeShiftPotential(s *store.Store, sensorID, priceSensorID string, tr model.TimeRange, shiftWindow int, minPower float64) ShiftResult {
	readings := s.ReadingsInRange(sensorID, tr.Start, tr.End.Add(time.Nanosecond))
	if len(readings) < 2 {
		return ShiftResult{}
	}

	// Group energy+price into calendar-day hourly slots
	type hourSlot struct {
		kwh   float64
		price float64
	}
	type dayData struct {
		slots [24]hourSlot
	}
	days := make(map[string]*dayData)

	for i := 1; i < len(readings); i++ {
		prev := readings[i-1]
		cur := readings[i]
		hours := cur.Timestamp.Sub(prev.Timestamp).Hours()
		if hours <= 0 || hours > 2 {
			continue
		}
		avgPower := (prev.Value + cur.Value) / 2
		if avgPower < minPower {
			continue
		}
		wh := avgPower * hours
		kwh := wh / 1000

		var price float64
		if pr, ok := s.ReadingAt(priceSensorID, cur.Timestamp); ok {
			price = pr.Value
		}

		dayKey := cur.Timestamp.Format("2006-01-02")
		h := cur.Timestamp.Hour()

		d, exists := days[dayKey]
		if !exists {
			d = &dayData{}
			days[dayKey] = d
		}
		d.slots[h].kwh += kwh
		if d.slots[h].price == 0 {
			d.slots[h].price = price
		} else {
			d.slots[h].price = (d.slots[h].price + price) / 2
		}
	}

	// Now compute day prices for shifting
	// For each day, get the price at each hour
	dayPrices := make(map[string][24]float64)
	priceReadings := s.ReadingsInRange(priceSensorID, tr.Start, tr.End.Add(time.Nanosecond))
	for _, r := range priceReadings {
		dayKey := r.Timestamp.Format("2006-01-02")
		h := r.Timestamp.Hour()
		dayPrices[dayKey] = func() [24]float64 {
			p := dayPrices[dayKey]
			p[h] = r.Value
			return p
		}()
	}

	var currentCost, optimalCost float64
	for dayKey, d := range days {
		prices, hasPrices := dayPrices[dayKey]
		if !hasPrices {
			continue
		}
		for h := 0; h < 24; h++ {
			kwh := d.slots[h].kwh
			if kwh <= 0 {
				continue
			}
			currentCost += kwh * prices[h]

			// Find cheapest hour within window
			bestPrice := prices[h]
			for offset := -shiftWindow; offset <= shiftWindow; offset++ {
				candidate := h + offset
				if candidate < 0 || candidate >= 24 {
					continue
				}
				if prices[candidate] < bestPrice {
					bestPrice = prices[candidate]
				}
			}
			optimalCost += kwh * bestPrice
		}
	}

	return ShiftResult{
		CurrentCostPLN: currentCost,
		OptimalCostPLN: optimalCost,
		SavingsPLN:     currentCost - optimalCost,
	}
}

func computeOverallAvgSpotPrice(s *store.Store, priceSensorID string, tr model.TimeRange) float64 {
	readings := s.ReadingsInRange(priceSensorID, tr.Start, tr.End.Add(time.Nanosecond))
	if len(readings) == 0 {
		return 0
	}
	var sum float64
	for _, r := range readings {
		sum += r.Value
	}
	return sum / float64(len(readings))
}

// --- Output formatting ---

func printCOPTable(buckets []COPBucket) {
	fmt.Println("  COP by Temperature:")
	fmt.Printf("   %16s │ %6s │ %7s │ %9s\n", "Temp Range", "COP", "Hours", "kWh")
	fmt.Printf("  ─────────────────┼────────┼─────────┼──────────\n")
	for _, b := range buckets {
		cop := safeDivide(b.ProductionWh, b.ConsumptionWh)
		kwh := b.ConsumptionWh / 1000
		fmt.Printf("   %4.0f to %3.0f °C  │ %6.1f │ %7.1f │ %8.1f\n",
			b.TempMin, b.TempMax, cop, b.Hours, kwh)
	}
}

func printHourlyTable(hourly [24]HourlyBucket, totalKWh float64) {
	fmt.Println("  Hourly Distribution:")
	fmt.Printf("   %4s │ %8s │ %10s │ %9s │ %5s\n", "Hour", "kWh", "Avg Price", "Cost", "Share")
	fmt.Printf("  ──────┼──────────┼────────────┼───────────┼──────\n")

	// Find the most expensive hour
	var maxCostHour int
	var maxCost float64
	for h := 0; h < 24; h++ {
		if hourly[h].CostPLN > maxCost {
			maxCost = hourly[h].CostPLN
			maxCostHour = h
		}
	}

	for h := 0; h < 24; h++ {
		b := hourly[h]
		if b.KWh < 0.01 {
			continue
		}
		avgPrice := safeDivide(b.CostPLN, b.KWh)
		share := safeDivide(b.KWh, totalKWh) * 100
		marker := ""
		if h == maxCostHour && maxCost > 0 {
			marker = " ← expensive"
		}
		fmt.Printf("     %02d │ %8.1f │ %10.2f │ %9.2f │ %4.1f%%%s\n",
			h, b.KWh, avgPrice, b.CostPLN, share, marker)
	}
}

func printShiftResult(r ShiftResult, window int) {
	savingsPct := safeDivide(r.SavingsPLN, r.CurrentCostPLN) * 100
	fmt.Printf("  Shift Potential (±%dh window):\n", window)
	fmt.Printf("    Current cost:  %.2f PLN\n", r.CurrentCostPLN)
	fmt.Printf("    Optimal cost:  %.2f PLN\n", r.OptimalCostPLN)
	fmt.Printf("    Savings:       %.2f PLN (%.1f%%)\n", r.SavingsPLN, savingsPct)
}

// --- Data loading ---

func loadAllData(inputDir string) *store.Store {
	dataStore := store.New()

	// Load legacy per-sensor CSVs from root
	loadLegacyCSVs(inputDir, dataStore)

	// Load multi-sensor recent CSVs (contains spot prices + more sensors)
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

	// Load stats CSVs
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

// --- Helpers ---

func sumHourly(buckets [24]HourlyBucket) (totalKWh, totalCost float64) {
	for _, b := range buckets {
		totalKWh += b.KWh
		totalCost += b.CostPLN
	}
	return
}

func safeDivide(a, b float64) float64 {
	if b == 0 {
		return 0
	}
	return a / b
}

func formatKWh(v float64) string {
	if v >= 1000 {
		return fmt.Sprintf("%.1f MWh", v/1000)
	}
	return fmt.Sprintf("%.1f kWh", v)
}
