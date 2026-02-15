package solar

import (
	"math"
	"testing"
	"time"

	"energy_simulator/internal/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeReadings(hours int, peakHour int, peakPower float64) []model.Reading {
	start := time.Date(2024, time.June, 15, 0, 0, 0, 0, time.UTC)
	var readings []model.Reading

	for day := 0; day < 30; day++ {
		for h := 0; h < 24; h++ {
			ts := start.AddDate(0, 0, day).Add(time.Duration(h) * time.Hour)
			// Simple bell curve around peakHour
			dist := float64(h) - float64(peakHour)
			power := peakPower * gaussLike(dist, 4.0)
			if power < 10 {
				power = 0
			}
			readings = append(readings, model.Reading{
				Timestamp: ts,
				SensorID:  "sensor.pv",
				Type:      model.SensorPVPower,
				Value:     power,
				Unit:      "W",
			})
		}
	}
	return readings
}

func gaussLike(x, sigma float64) float64 {
	return math.Exp(-x * x / (2 * sigma * sigma))
}

func TestBuildProfileFromReadings_PeakDetection(t *testing.T) {
	readings := makeReadings(24, 10, 6500)
	profile := BuildProfileFromReadings(readings, 6500)

	// Peak should be around hour 10 (east-facing)
	assert.InDelta(t, 10, profile.PeakHour, 1, "peak hour should be near 10")
	assert.Equal(t, 6500.0, profile.PeakWp)

	// Peak hour should have factor 1.0
	assert.InDelta(t, 1.0, profile.HourlyFactor[profile.PeakHour], 0.01)

	// Night hours should have very low or zero factor
	assert.Less(t, profile.HourlyFactor[0], 0.05, "midnight should have near-zero generation")
	assert.Less(t, profile.HourlyFactor[23], 0.1, "11pm should have near-zero generation")
}

func TestBuildProfileFromReadings_Empty(t *testing.T) {
	profile := BuildProfileFromReadings(nil, 6500)
	assert.Equal(t, 10, profile.PeakHour, "default profile peaks at 10")
	assert.InDelta(t, 1.0, profile.HourlyFactor[10], 0.01)
}

func TestGenerateOrientedProfile_SouthShift(t *testing.T) {
	// Base: east-facing (azimuth 90°), peak at hour 10
	base := PVProfile{PeakHour: 10, PeakWp: 6500}
	for h := 0; h < 24; h++ {
		dist := float64(h) - 10.0
		if dist*dist < 100 {
			base.HourlyFactor[h] = 1.0 - dist*dist/100.0
		}
	}
	base.HourlyFactor[10] = 1.0 // ensure normalized

	south := GenerateOrientedProfile(base, 180, 40, 90)

	// South-facing should peak around hour 12 (shifted +2h from east)
	assert.InDelta(t, 12, south.PeakHour, 1, "south-facing peak should be near noon")

	// South peak should have factor 1.0 (re-normalized)
	assert.InDelta(t, 1.0, south.HourlyFactor[south.PeakHour], 0.01)
}

func TestGenerateOrientedProfile_WestShift(t *testing.T) {
	base := PVProfile{PeakHour: 10, PeakWp: 6500}
	for h := 0; h < 24; h++ {
		dist := float64(h) - 10.0
		if dist*dist < 100 {
			base.HourlyFactor[h] = 1.0 - dist*dist/100.0
		}
	}
	base.HourlyFactor[10] = 1.0

	west := GenerateOrientedProfile(base, 270, 40, 90)

	// West-facing should peak around hour 14 (shifted +4h from east)
	assert.InDelta(t, 14, west.PeakHour, 1, "west-facing peak should be near 14:00")
}

func TestPowerAt(t *testing.T) {
	profile := PVProfile{PeakHour: 12, PeakWp: 6500}
	profile.HourlyFactor[12] = 1.0
	profile.HourlyFactor[11] = 0.8
	profile.HourlyFactor[13] = 0.8

	// At peak hour, should return peakWp
	power := profile.PowerAt(12.0, 6500)
	assert.InDelta(t, 6500.0, power, 1.0)

	// At half hour between 11 and 12
	power = profile.PowerAt(11.5, 6500)
	expected := (0.8*0.5 + 1.0*0.5) * 6500
	assert.InDelta(t, expected, power, 1.0)

	// At midnight
	power = profile.PowerAt(0.0, 6500)
	assert.InDelta(t, 0.0, power, 1.0)
}

func TestInterpolateProfile_Wrapping(t *testing.T) {
	var factors [24]float64
	factors[23] = 0.1
	factors[0] = 0.2

	// At 23.5, should interpolate between 23 and 0
	val := interpolateProfile(factors, 23.5)
	assert.InDelta(t, 0.15, val, 0.01)
}

func TestDefaultProfile(t *testing.T) {
	p := defaultProfile(6500)
	require.Equal(t, 10, p.PeakHour)
	assert.Greater(t, p.HourlyFactor[10], 0.9)
	assert.Less(t, p.HourlyFactor[0], 0.05)
}
