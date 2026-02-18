package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseHistoryResponse(t *testing.T) {
	// Two entities, 2 states each (first has entity_id, second is minimal)
	response := [][]map[string]string{
		{
			{"entity_id": "sensor.0x943469fffed2bf71_power", "state": "150.5", "last_changed": "2025-01-01T10:00:00.000+00:00"},
			{"state": "200.3", "last_changed": "2025-01-01T10:05:00.000+00:00"},
		},
		{
			{"entity_id": "sensor.hoymiles_gateway_solarh_3054300_real_power", "state": "1200", "last_changed": "2025-01-01T10:00:00.000+00:00"},
			{"state": "1350.7", "last_changed": "2025-01-01T10:05:00.000+00:00"},
		},
	}

	data, err := json.Marshal(response)
	require.NoError(t, err)

	records, err := parseHistoryResponse(data)
	require.NoError(t, err)
	assert.Len(t, records, 4)

	assert.Equal(t, "sensor.0x943469fffed2bf71_power", records[0].sensorID)
	assert.Equal(t, 150.5, records[0].value)

	assert.Equal(t, "sensor.0x943469fffed2bf71_power", records[1].sensorID)
	assert.Equal(t, 200.3, records[1].value)

	assert.Equal(t, "sensor.hoymiles_gateway_solarh_3054300_real_power", records[2].sensorID)
	assert.Equal(t, 1200.0, records[2].value)

	assert.Equal(t, "sensor.hoymiles_gateway_solarh_3054300_real_power", records[3].sensorID)
	assert.Equal(t, 1350.7, records[3].value)
}

func TestSkipUnavailable(t *testing.T) {
	response := [][]map[string]string{
		{
			{"entity_id": "sensor.0x943469fffed2bf71_power", "state": "150.5", "last_changed": "2025-01-01T10:00:00.000+00:00"},
			{"state": "unavailable", "last_changed": "2025-01-01T10:05:00.000+00:00"},
			{"state": "unknown", "last_changed": "2025-01-01T10:10:00.000+00:00"},
			{"state": "", "last_changed": "2025-01-01T10:15:00.000+00:00"},
			{"state": "300", "last_changed": "2025-01-01T10:20:00.000+00:00"},
		},
	}

	data, err := json.Marshal(response)
	require.NoError(t, err)

	records, err := parseHistoryResponse(data)
	require.NoError(t, err)
	assert.Len(t, records, 2)
	assert.Equal(t, 150.5, records[0].value)
	assert.Equal(t, 300.0, records[1].value)
}

func TestMinimalResponse(t *testing.T) {
	// entity_id only on first entry — rest inherit it
	response := [][]map[string]string{
		{
			{"entity_id": "sensor.0x943469fffed2bf71_power", "state": "100", "last_changed": "2025-01-01T10:00:00.000+00:00"},
			{"state": "200", "last_changed": "2025-01-01T10:01:00.000+00:00"},
			{"state": "300", "last_changed": "2025-01-01T10:02:00.000+00:00"},
		},
	}

	data, err := json.Marshal(response)
	require.NoError(t, err)

	records, err := parseHistoryResponse(data)
	require.NoError(t, err)
	assert.Len(t, records, 3)

	for _, r := range records {
		assert.Equal(t, "sensor.0x943469fffed2bf71_power", r.sensorID)
	}
	assert.Equal(t, 100.0, records[0].value)
	assert.Equal(t, 200.0, records[1].value)
	assert.Equal(t, 300.0, records[2].value)
}

func TestIncrementalMerge(t *testing.T) {
	existing := []record{
		{sensorID: "sensor.a", value: 100, ts: 1000},
		{sensorID: "sensor.a", value: 200, ts: 2000},
		{sensorID: "sensor.b", value: 50, ts: 1500},
	}

	newRecords := []record{
		{sensorID: "sensor.a", value: 200, ts: 2000}, // duplicate — same key
		{sensorID: "sensor.a", value: 300, ts: 3000}, // new
		{sensorID: "sensor.b", value: 75, ts: 2500},  // new
	}

	merged := mergeRecords(existing, newRecords)

	assert.Len(t, merged, 5)

	// Verify sorted by (sensorID, ts)
	assert.Equal(t, "sensor.a", merged[0].sensorID)
	assert.Equal(t, 1000.0, merged[0].ts)
	assert.Equal(t, "sensor.a", merged[1].sensorID)
	assert.Equal(t, 2000.0, merged[1].ts)
	assert.Equal(t, "sensor.a", merged[2].sensorID)
	assert.Equal(t, 3000.0, merged[2].ts)
	assert.Equal(t, "sensor.b", merged[3].sensorID)
	assert.Equal(t, 1500.0, merged[3].ts)
	assert.Equal(t, "sensor.b", merged[4].sensorID)
	assert.Equal(t, 2500.0, merged[4].ts)
}

func TestLoadDotEnv(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")

	content := "# comment\nTEST_HA_FOO=bar\nTEST_HA_BAZ=qux\n\n# another comment\nTEST_HA_EMPTY=\n"
	require.NoError(t, os.WriteFile(envPath, []byte(content), 0o644))

	// Clear to ensure they're not set
	os.Unsetenv("TEST_HA_FOO")
	os.Unsetenv("TEST_HA_BAZ")
	os.Unsetenv("TEST_HA_EMPTY")

	loadDotEnv(envPath)

	assert.Equal(t, "bar", os.Getenv("TEST_HA_FOO"))
	assert.Equal(t, "qux", os.Getenv("TEST_HA_BAZ"))
	assert.Equal(t, "", os.Getenv("TEST_HA_EMPTY"))

	// Verify existing env vars are not overwritten
	os.Setenv("TEST_HA_FOO", "original")
	loadDotEnv(envPath)
	assert.Equal(t, "original", os.Getenv("TEST_HA_FOO"))

	// Cleanup
	os.Unsetenv("TEST_HA_FOO")
	os.Unsetenv("TEST_HA_BAZ")
	os.Unsetenv("TEST_HA_EMPTY")
}

func TestFetchDay(t *testing.T) {
	response := [][]map[string]string{
		{
			{"entity_id": "sensor.0x943469fffed2bf71_power", "state": "500", "last_changed": "2025-06-01T12:00:00.000+00:00"},
			{"state": "600", "last_changed": "2025-06-01T12:05:00.000+00:00"},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		assert.Contains(t, r.URL.Path, "/api/history/period/")
		assert.Contains(t, r.URL.RawQuery, "minimal_response")
		assert.Contains(t, r.URL.RawQuery, "no_attributes")

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer srv.Close()

	client := srv.Client()
	entityIDs := "sensor.0x943469fffed2bf71_power"

	records, err := fetchDay(client, srv.URL, "test-token",
		mustParseTime("2025-06-01T00:00:00Z"),
		mustParseTime("2025-06-02T00:00:00Z"),
		entityIDs,
	)

	require.NoError(t, err)
	assert.Len(t, records, 2)
	assert.Equal(t, 500.0, records[0].value)
	assert.Equal(t, 600.0, records[1].value)
}

func TestFetchDayAuth401(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		w.Write([]byte("Unauthorized"))
	}))
	defer srv.Close()

	client := srv.Client()
	_, err := fetchDay(client, srv.URL, "bad-token",
		mustParseTime("2025-06-01T00:00:00Z"),
		mustParseTime("2025-06-02T00:00:00Z"),
		"sensor.test",
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "401")
}

func TestLoadExistingDir(t *testing.T) {
	dir := t.TempDir()

	// Write two week files
	writeTestCSV(t, filepath.Join(dir, "2026-W06.csv"), []record{
		{sensorID: "sensor.a", value: 100, ts: 1000},
		{sensorID: "sensor.a", value: 200, ts: 2000},
	})
	writeTestCSV(t, filepath.Join(dir, "2026-W07.csv"), []record{
		{sensorID: "sensor.b", value: 50, ts: 3000},
	})

	records, minTS, maxTS := loadExistingDir(dir)

	assert.Len(t, records, 3)
	assert.Equal(t, 1000.0, minTS)
	assert.Equal(t, 3000.0, maxTS)
}

func TestLoadExistingDirEmpty(t *testing.T) {
	dir := t.TempDir()

	records, minTS, maxTS := loadExistingDir(dir)
	assert.Nil(t, records)
	assert.Equal(t, 0.0, minTS)
	assert.Equal(t, 0.0, maxTS)
}

func TestWeekKey(t *testing.T) {
	// 2025-01-01 is Wednesday of ISO week 1
	ts := float64(time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC).Unix())
	assert.Equal(t, "2025-W01", weekKey(ts))

	// 2025-06-15 is Sunday of ISO week 24
	ts = float64(time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC).Unix())
	assert.Equal(t, "2025-W24", weekKey(ts))
}

func TestGroupByWeek(t *testing.T) {
	// Two records in same week, one in different week
	week1ts := float64(time.Date(2025, 1, 6, 10, 0, 0, 0, time.UTC).Unix()) // Monday W02
	week2ts := float64(time.Date(2025, 1, 13, 10, 0, 0, 0, time.UTC).Unix()) // Monday W03

	records := []record{
		{sensorID: "sensor.a", value: 100, ts: week1ts},
		{sensorID: "sensor.a", value: 200, ts: week1ts + 3600},
		{sensorID: "sensor.a", value: 300, ts: week2ts},
	}

	byWeek := groupByWeek(records)
	assert.Len(t, byWeek, 2)
	assert.Len(t, byWeek["2025-W02"], 2)
	assert.Len(t, byWeek["2025-W03"], 1)
}

func TestWriteAndLoadCSV(t *testing.T) {
	dir := t.TempDir()
	csvPath := filepath.Join(dir, "test.csv")

	records := []record{
		{sensorID: "sensor.a", value: 123.456, ts: 1000.1234567},
		{sensorID: "sensor.b", value: -50, ts: 2000.0},
	}

	require.NoError(t, writeCSV(csvPath, records))

	loaded := loadCSVFile(csvPath)
	assert.Len(t, loaded, 2)
	assert.Equal(t, "sensor.a", loaded[0].sensorID)
	assert.Equal(t, 123.456, loaded[0].value)
	assert.Equal(t, "sensor.b", loaded[1].sensorID)
	assert.Equal(t, -50.0, loaded[1].value)
}

func TestLoadCSVFileMissing(t *testing.T) {
	records := loadCSVFile("/nonexistent/path.csv")
	assert.Nil(t, records)
}

func writeTestCSV(t *testing.T, path string, records []record) {
	t.Helper()
	f, err := os.Create(path)
	require.NoError(t, err)
	w := csv.NewWriter(f)
	w.Write([]string{"sensor_id", "value", "updated_ts"})
	for _, r := range records {
		w.Write([]string{
			r.sensorID,
			fmt.Sprintf("%g", r.value),
			fmt.Sprintf("%.7f", r.ts),
		})
	}
	w.Flush()
	f.Close()
}

func mustParseTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}
