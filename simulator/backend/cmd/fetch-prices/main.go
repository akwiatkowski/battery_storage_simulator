// fetch-prices downloads historical DAM spot prices for Poland from the
// Energy-Charts API (https://api.energy-charts.info), converts EUR/MWh to
// PLN/kWh, and writes a CSV compatible with the RecentParser format
// (sensor_id,value,updated_ts).
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"time"
)

type apiResponse struct {
	UnixSeconds []int64   `json:"unix_seconds"`
	Price       []float64 `json:"price"`
	Unit        string    `json:"unit"`
}

func main() {
	startDate := flag.String("start", "2018-01-01", "start date (YYYY-MM-DD)")
	endDate := flag.String("end", "", "end date (YYYY-MM-DD), defaults to today")
	eurPln := flag.Float64("eur-pln", 4.3, "EUR to PLN exchange rate")
	output := flag.String("output", "input/recent/historic_spot_prices.csv", "output CSV path")
	sensorID := flag.String("sensor-id", "sensor.spotprice_now", "sensor ID in output")
	flag.Parse()

	start, err := time.Parse("2006-01-02", *startDate)
	if err != nil {
		log.Fatalf("Invalid start date: %v", err)
	}

	var end time.Time
	if *endDate == "" {
		end = time.Now().UTC().Truncate(24 * time.Hour)
	} else {
		end, err = time.Parse("2006-01-02", *endDate)
		if err != nil {
			log.Fatalf("Invalid end date: %v", err)
		}
	}

	log.Printf("Fetching PL spot prices from %s to %s (EUR/PLN=%.2f)",
		start.Format("2006-01-02"), end.Format("2006-01-02"), *eurPln)

	type record struct {
		ts    int64
		price float64
	}
	var records []record

	// Fetch in monthly chunks to stay within API limits.
	chunkStart := start
	for chunkStart.Before(end) {
		chunkEnd := chunkStart.AddDate(0, 1, 0)
		if chunkEnd.After(end) {
			chunkEnd = end
		}

		url := fmt.Sprintf(
			"https://api.energy-charts.info/price?bzn=PL&start=%s&end=%s",
			chunkStart.Format("2006-01-02T15:04Z"),
			chunkEnd.Format("2006-01-02T15:04Z"),
		)

		log.Printf("  %s → %s ...",
			chunkStart.Format("2006-01-02"), chunkEnd.Format("2006-01-02"))

		data, err := fetchWithRetry(url)
		if err != nil {
			log.Fatalf("Fetching %s → %s: %v",
				chunkStart.Format("2006-01-02"), chunkEnd.Format("2006-01-02"), err)
		}

		if len(data.UnixSeconds) != len(data.Price) {
			log.Fatalf("Mismatched array lengths: %d timestamps, %d prices",
				len(data.UnixSeconds), len(data.Price))
		}

		for i, ts := range data.UnixSeconds {
			plnKwh := data.Price[i] * *eurPln / 1000.0
			records = append(records, record{ts: ts, price: plnKwh})
		}

		chunkStart = chunkEnd
		time.Sleep(1 * time.Second)
	}

	// Sort by timestamp and deduplicate.
	sort.Slice(records, func(i, j int) bool {
		return records[i].ts < records[j].ts
	})
	deduped := records[:0]
	for i, r := range records {
		if i > 0 && r.ts == records[i-1].ts {
			continue
		}
		deduped = append(deduped, r)
	}
	records = deduped

	// Write CSV in RecentParser-compatible format: sensor_id,value,updated_ts
	f, err := os.Create(*output)
	if err != nil {
		log.Fatalf("Creating output file: %v", err)
	}
	defer f.Close()

	fmt.Fprintln(f, "sensor_id,value,updated_ts")
	for _, r := range records {
		fmt.Fprintf(f, "%s,%.4f,%d\n", *sensorID, r.price, r.ts)
	}

	log.Printf("Wrote %d records to %s", len(records), *output)
}

func fetchWithRetry(url string) (apiResponse, error) {
	const maxRetries = 5
	for attempt := range maxRetries {
		resp, err := http.Get(url)
		if err != nil {
			return apiResponse{}, fmt.Errorf("HTTP request: %w", err)
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return apiResponse{}, fmt.Errorf("reading body: %w", err)
		}

		if resp.StatusCode == 429 {
			wait := time.Duration(attempt+1) * 5 * time.Second
			log.Printf("    rate limited, waiting %s (attempt %d/%d)", wait, attempt+1, maxRetries)
			time.Sleep(wait)
			continue
		}
		if resp.StatusCode != 200 {
			return apiResponse{}, fmt.Errorf("API returned %d: %s", resp.StatusCode, body)
		}

		var data apiResponse
		if err := json.Unmarshal(body, &data); err != nil {
			return apiResponse{}, fmt.Errorf("parsing JSON: %w", err)
		}
		if len(data.UnixSeconds) != len(data.Price) {
			return apiResponse{}, fmt.Errorf("mismatched arrays: %d timestamps, %d prices",
				len(data.UnixSeconds), len(data.Price))
		}
		return data, nil
	}
	return apiResponse{}, fmt.Errorf("exhausted %d retries", maxRetries)
}
