package main

import (
	"fmt"
	"sort"
	"strings"

	"energy_simulator/internal/model"
)

func main() {
	// Collect and sort for stable output.
	type entry struct {
		slug string
		haID string
	}
	entries := make([]entry, 0, len(model.SensorHomeAssistantID))
	for slug, haID := range model.SensorHomeAssistantID {
		entries = append(entries, entry{string(slug), haID})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].slug < entries[j].slug })

	var inLines []string
	for _, e := range entries {
		inLines = append(inLines, fmt.Sprintf("  '%s'  -- %s", e.haID, e.slug))
	}

	inList := strings.Join(inLines, "\n  ,")

	fmt.Printf(`-- Long-term statistics (hourly aggregates, kept indefinitely)
SELECT
  statistics_meta.statistic_id AS sensor_id,
  statistics.start_ts AS start_time,
  statistics.mean AS avg,
  statistics.min AS min_val,
  statistics.max AS max_val
FROM statistics
JOIN statistics_meta ON statistics.metadata_id = statistics_meta.id
WHERE statistics_meta.statistic_id IN (
%s
)
ORDER BY statistics_meta.statistic_id, statistics.start_ts;
`, inList)

	fmt.Printf(`
-- Recent states (raw measurements, kept ~2 weeks)
SELECT
  states_meta.entity_id AS sensor_id,
  states.state AS value,
  states.last_updated_ts AS updated_ts
FROM states
INNER JOIN states_meta ON states.metadata_id = states_meta.metadata_id
WHERE states_meta.entity_id IN (
%s
)
ORDER BY states_meta.entity_id, states.last_updated_ts DESC;
`, inList)
}
