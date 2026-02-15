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

func main() {
	urlFlag := flag.String("url", "", "Home Assistant base URL (overrides HA_URL)")
	tokenFlag := flag.String("token", "", "Long-lived access token (overrides HA_TOKEN)")
	days := flag.Int("days", 7, "Days to fetch on first run (ignored if output file has data)")
	output := flag.String("output", "input/recent/ha-fetch.csv", "Output CSV path")
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

	existing, latestTS := loadExistingRecords(*output)

	var startTime time.Time
	if latestTS > 0 {
		startTime = time.Unix(int64(latestTS), 0).Add(-1 * time.Minute)
		log.Printf("resuming from %s (latest timestamp minus 1min overlap)", startTime.Format(time.RFC3339))
	} else {
		startTime = time.Now().AddDate(0, 0, -*days)
		log.Printf("first run — fetching last %d days from %s", *days, startTime.Format(time.RFC3339))
	}

	endTime := time.Now()
	client := &http.Client{Timeout: 30 * time.Second}
	entityIDStr := strings.Join(entityIDs, ",")

	var newRecords []record
	for start := startTime; start.Before(endTime); start = start.Add(24 * time.Hour) {
		end := start.Add(24 * time.Hour)
		if end.After(endTime) {
			end = endTime
		}

		dayRecords, err := fetchDay(client, haURL, haToken, start, end, entityIDStr)
		if err != nil {
			log.Fatalf("fetching %s: %v", start.Format("2006-01-02"), err)
		}
		newRecords = append(newRecords, dayRecords...)
		log.Printf("  %s: %d records", start.Format("2006-01-02"), len(dayRecords))

		if end.Before(endTime) {
			time.Sleep(500 * time.Millisecond)
		}
	}

	merged := mergeRecords(existing, newRecords)

	if err := os.MkdirAll(filepath.Dir(*output), 0o755); err != nil {
		log.Fatalf("creating output directory: %v", err)
	}
	if err := writeCSV(*output, merged); err != nil {
		log.Fatalf("writing CSV: %v", err)
	}

	log.Printf("wrote %d records to %s (was %d, fetched %d new)", len(merged), *output, len(existing), len(newRecords))
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

func loadExistingRecords(path string) ([]record, float64) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0
	}
	defer f.Close()

	cr := csv.NewReader(f)
	// skip header
	if _, err := cr.Read(); err != nil {
		return nil, 0
	}

	var records []record
	var maxTS float64

	for {
		row, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}
		if len(row) < 3 {
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
		if ts > maxTS {
			maxTS = ts
		}
	}

	return records, maxTS
}

func fetchDay(client *http.Client, baseURL, token string, start, end time.Time, entityIDs string) ([]record, error) {
	url := fmt.Sprintf("%s/api/history/period/%s?end_time=%s&filter_entity_id=%s&minimal_response&no_attributes",
		baseURL,
		start.Format(time.RFC3339),
		end.Format(time.RFC3339),
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
