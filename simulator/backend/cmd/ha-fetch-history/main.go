package main

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"energy_simulator/internal/model"
)

type record struct {
	sensorID string
	value    float64
	ts       float64 // unix epoch seconds
}

// weekKey returns the ISO week string for a unix timestamp, e.g. "2026-W07".
func weekKey(ts float64) string {
	t := time.Unix(int64(ts), int64((ts-float64(int64(ts)))*1e9))
	year, week := t.ISOWeek()
	return fmt.Sprintf("%04d-W%02d", year, week)
}

func main() {
	urlFlag := flag.String("url", "", "Home Assistant base URL (overrides HA_URL)")
	tokenFlag := flag.String("token", "", "Long-lived access token (overrides HA_TOKEN)")
	outputDir := flag.String("output", "input/recent", "Output directory for weekly CSV files")
	sinceFlag := flag.String("since", "", "Force fetch from this date (YYYY-MM-DD), ignoring existing timestamps")
	flag.Parse()

	loadDotEnv(".env")

	haURL := resolveFlag(*urlFlag, "HA_URL")
	haToken := resolveFlag(*tokenFlag, "HA_TOKEN")
	if haURL == "" {
		log.Fatal("HA_URL not set — use -url flag or set HA_URL in .env")
	}
	if haToken == "" {
		log.Fatal("HA_TOKEN not set — use -token flag or set HA_TOKEN in .env")
	}
	haURL = strings.TrimRight(haURL, "/")

	entityIDs := collectEntityIDs()
	if len(entityIDs) == 0 {
		log.Fatal("no entity IDs found in model.SensorHomeAssistantID")
	}

	existing, earliestTS, latestTS := loadExistingDir(*outputDir)

	endTime := time.Now()
	client := &http.Client{Timeout: 30 * time.Second}
	entityIDStr := strings.Join(entityIDs, ",")

	var newRecords []record

	// If -since is set, skip normal backfill/forward logic and fetch everything from that date
	if *sinceFlag != "" {
		sinceTime, err := time.ParseInLocation("2006-01-02", *sinceFlag, time.Now().Location())
		if err != nil {
			log.Fatalf("invalid -since date %q: %v", *sinceFlag, err)
		}
		log.Printf("forced re-fetch from %s to %s", sinceTime.Format("2006-01-02"), endTime.Format("2006-01-02"))
		fetched, err := fetchRange(client, haURL, haToken, sinceTime, endTime, entityIDStr)
		if err != nil {
			log.Fatalf("fetch: %v", err)
		}
		newRecords = fetched
	} else {
		// Backfill: fetch data before the earliest existing record
		backfillStart := endTime.AddDate(-2, 0, 0) // 2 years back — HA returns empty for missing periods
		if earliestTS > 0 {
			backfillEnd := time.Unix(int64(earliestTS), 0).Add(1 * time.Minute)
			if backfillStart.Before(backfillEnd) {
				log.Printf("backfilling from %s to %s", backfillStart.Format("2006-01-02"), backfillEnd.Format("2006-01-02"))
				backfill, err := fetchRange(client, haURL, haToken, backfillStart, backfillEnd, entityIDStr)
				if err != nil {
					log.Fatalf("backfill: %v", err)
				}
				newRecords = append(newRecords, backfill...)
			}
		}

		// Forward: fetch new data from latest timestamp onward (or from 2 years ago on first run)
		var startTime time.Time
		if latestTS > 0 {
			startTime = time.Unix(int64(latestTS), 0).Add(-1 * time.Minute)
			log.Printf("fetching new data from %s", startTime.Format(time.RFC3339))
		} else {
			startTime = backfillStart
			log.Printf("first run — fetching all available data from %s", startTime.Format("2006-01-02"))
		}

		forward, err := fetchRange(client, haURL, haToken, startTime, endTime, entityIDStr)
		if err != nil {
			log.Fatalf("fetch: %v", err)
		}
		newRecords = append(newRecords, forward...)
	}

	if len(newRecords) == 0 {
		log.Printf("no new records fetched")
		return
	}

	// Group new records by week
	newByWeek := groupByWeek(newRecords)

	// Merge with existing and write only affected week files
	if err := os.MkdirAll(*outputDir, 0o755); err != nil {
		log.Fatalf("creating output directory: %v", err)
	}

	totalExisting := len(existing)
	totalWritten := 0
	filesWritten := 0
	for week, newWeekRecords := range newByWeek {
		path := filepath.Join(*outputDir, week+".csv")
		existingWeek := loadCSVFile(path)
		merged := mergeRecords(existingWeek, newWeekRecords)
		if err := writeCSV(path, merged); err != nil {
			log.Fatalf("writing %s: %v", path, err)
		}
		totalWritten += len(merged)
		filesWritten++
	}

	log.Printf("wrote %d records across %d weekly files (had %d existing, fetched %d new)",
		totalWritten, filesWritten, totalExisting, len(newRecords))

	// Per-sensor summary
	sensorCounts := make(map[string]int)
	for _, r := range newRecords {
		sensorCounts[r.sensorID]++
	}
	var sensorIDs []string
	for sid := range sensorCounts {
		sensorIDs = append(sensorIDs, sid)
	}
	sort.Strings(sensorIDs)
	log.Printf("per-sensor breakdown (%d sensors with new data):", len(sensorIDs))
	for _, sid := range sensorIDs {
		name := sid
		if st, ok := model.HAEntityToSensorType[sid]; ok {
			if info, ok := model.SensorCatalog[st]; ok {
				name = info.Name
			}
		}
		log.Printf("  %-35s %5d records", name, sensorCounts[sid])
	}
}

// loadDotEnv reads a .env file and sets variables not already in the environment.
func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return // silently skip if .env doesn't exist
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		if _, exists := os.LookupEnv(key); !exists {
			os.Setenv(key, val)
		}
	}
}

func resolveFlag(flagVal, envKey string) string {
	if flagVal != "" {
		return flagVal
	}
	return os.Getenv(envKey)
}

func collectEntityIDs() []string {
	ids := make([]string, 0, len(model.SensorHomeAssistantID))
	for _, entityID := range model.SensorHomeAssistantID {
		ids = append(ids, entityID)
	}
	sort.Strings(ids)
	return ids
}

// loadExistingDir scans all CSV files in the output directory to find earliest/latest timestamps.
func loadExistingDir(dir string) (allRecords []record, earliestTS, latestTS float64) {
	matches, err := filepath.Glob(filepath.Join(dir, "*.csv"))
	if err != nil || len(matches) == 0 {
		return nil, 0, 0
	}

	for _, path := range matches {
		records := loadCSVFile(path)
		for _, r := range records {
			if r.ts > latestTS {
				latestTS = r.ts
			}
			if earliestTS == 0 || r.ts < earliestTS {
				earliestTS = r.ts
			}
		}
		allRecords = append(allRecords, records...)
	}

	return allRecords, earliestTS, latestTS
}

func loadCSVFile(path string) []record {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	cr := csv.NewReader(f)
	// skip header
	if _, err := cr.Read(); err != nil {
		return nil
	}

	var records []record
	for {
		row, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil || len(row) < 3 {
			continue
		}

		value, err := strconv.ParseFloat(row[1], 64)
		if err != nil {
			continue
		}
		ts, err := strconv.ParseFloat(row[2], 64)
		if err != nil {
			continue
		}

		records = append(records, record{
			sensorID: row[0],
			value:    value,
			ts:       ts,
		})
	}

	return records
}

func groupByWeek(records []record) map[string][]record {
	byWeek := make(map[string][]record)
	for _, r := range records {
		wk := weekKey(r.ts)
		byWeek[wk] = append(byWeek[wk], r)
	}
	return byWeek
}

func fetchRange(client *http.Client, baseURL, token string, start, end time.Time, entityIDs string) ([]record, error) {
	var allRecords []record
	for day := start; day.Before(end); day = day.Add(24 * time.Hour) {
		dayEnd := day.Add(24 * time.Hour)
		if dayEnd.After(end) {
			dayEnd = end
		}

		dayRecords, err := fetchDay(client, baseURL, token, day, dayEnd, entityIDs)
		if err != nil {
			return nil, fmt.Errorf("fetching %s: %w", day.Format("2006-01-02"), err)
		}
		allRecords = append(allRecords, dayRecords...)

		if len(dayRecords) > 0 {
			log.Printf("  %s: %d records", day.Format("2006-01-02"), len(dayRecords))
		}

		// Only sleep between requests that returned data to speed up backfill over empty periods
		if len(dayRecords) > 0 && dayEnd.Before(end) {
			time.Sleep(500 * time.Millisecond)
		}
	}
	return allRecords, nil
}

func fetchDay(client *http.Client, baseURL, token string, start, end time.Time, entityIDs string) ([]record, error) {
	url := fmt.Sprintf("%s/api/history/period/%s?end_time=%s&filter_entity_id=%s&minimal_response&no_attributes",
		baseURL,
		start.UTC().Format(time.RFC3339),
		end.UTC().Format(time.RFC3339),
		entityIDs,
	)

	var body []byte
	var err error
	for attempt := range 5 {
		body, err = doRequest(client, url, token)
		if err == nil {
			break
		}
		if isRetryable(err) {
			wait := time.Duration(math.Pow(2, float64(attempt))) * time.Second
			log.Printf("  retrying in %s: %v", wait, err)
			time.Sleep(wait)
			continue
		}
		return nil, err
	}
	if err != nil {
		return nil, fmt.Errorf("after 5 attempts: %w", err)
	}

	return parseHistoryResponse(body)
}

type apiError struct {
	statusCode int
	message    string
}

func (e *apiError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.statusCode, e.message)
}

func isRetryable(err error) bool {
	ae, ok := err.(*apiError)
	if !ok {
		return true // network errors are retryable
	}
	return ae.statusCode == 429 || ae.statusCode >= 500
}

func doRequest(client *http.Client, url, token string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode == 401 {
		return nil, &apiError{statusCode: 401, message: "authentication failed — check your HA_TOKEN"}
	}
	if resp.StatusCode != 200 {
		return nil, &apiError{statusCode: resp.StatusCode, message: string(body)}
	}
	return body, nil
}

// parseHistoryResponse parses the HA history API response.
// Format: array of arrays. Each inner array is one entity's history.
// With minimal_response, only the first entry has entity_id.
func parseHistoryResponse(data []byte) ([]record, error) {
	var outer [][]json.RawMessage
	if err := json.Unmarshal(data, &outer); err != nil {
		return nil, fmt.Errorf("parsing JSON: %w", err)
	}

	var records []record
	for _, entityHistory := range outer {
		var currentEntityID string
		for i, raw := range entityHistory {
			var entry struct {
				EntityID    string `json:"entity_id"`
				State       string `json:"state"`
				LastChanged string `json:"last_changed"`
			}
			if err := json.Unmarshal(raw, &entry); err != nil {
				continue
			}

			if i == 0 {
				currentEntityID = entry.EntityID
			}
			if entry.EntityID != "" {
				currentEntityID = entry.EntityID
			}

			// Skip non-numeric states
			if entry.State == "unavailable" || entry.State == "unknown" || entry.State == "" {
				continue
			}

			// Must be a known entity
			if _, ok := model.HAEntityToSensorType[currentEntityID]; !ok {
				continue
			}

			value, err := strconv.ParseFloat(entry.State, 64)
			if err != nil {
				continue
			}

			ts, err := time.Parse(time.RFC3339Nano, entry.LastChanged)
			if err != nil {
				// Try alternate format without nanoseconds
				ts, err = time.Parse("2006-01-02T15:04:05+00:00", entry.LastChanged)
				if err != nil {
					continue
				}
			}

			records = append(records, record{
				sensorID: currentEntityID,
				value:    value,
				ts:       float64(ts.UnixNano()) / 1e9,
			})
		}
	}

	return records, nil
}

func mergeRecords(existing, new []record) []record {
	type key struct {
		sensorID string
		ts       float64
	}

	seen := make(map[key]record, len(existing)+len(new))
	for _, r := range existing {
		seen[key{r.sensorID, r.ts}] = r
	}
	for _, r := range new {
		seen[key{r.sensorID, r.ts}] = r // new overwrites existing on conflict
	}

	merged := make([]record, 0, len(seen))
	for _, r := range seen {
		merged = append(merged, r)
	}

	sort.Slice(merged, func(i, j int) bool {
		if merged[i].sensorID != merged[j].sensorID {
			return merged[i].sensorID < merged[j].sensorID
		}
		return merged[i].ts < merged[j].ts
	})

	return merged
}

func writeCSV(path string, records []record) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	if err := w.Write([]string{"sensor_id", "value", "updated_ts"}); err != nil {
		return err
	}

	for _, r := range records {
		if err := w.Write([]string{
			r.sensorID,
			strconv.FormatFloat(r.value, 'f', -1, 64),
			strconv.FormatFloat(r.ts, 'f', 7, 64),
		}); err != nil {
			return err
		}
	}

	w.Flush()
	return w.Error()
}
