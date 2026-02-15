package solar

import (
	"math"
	"time"

	"energy_simulator/internal/model"
)

// PVProfile holds an hourly generation shape derived from actual PV data.
type PVProfile struct {
	// HourlyFactor holds the normalized capacity factor for each hour [0-23].
	// Peak hour = 1.0, other hours scaled relative to peak.
	HourlyFactor [24]float64
	// PeakHour is the hour with the highest average generation.
	PeakHour int
	// PeakWp is the reference peak power (W) from the source installation.
	PeakWp float64
}

// BuildProfileFromReadings analyzes PV readings to derive an hourly generation shape.
// Focuses on June/July (clearest generation pattern) for a clean profile.
func BuildProfileFromReadings(readings []model.Reading, peakWp float64) PVProfile {
	if len(readings) == 0 {
		return defaultProfile(peakWp)
	}

	// Accumulate average power per hour, focusing on summer months
	var hourSum [24]float64
	var hourCount [24]int

	for _, r := range readings {
		month := r.Timestamp.Month()
		// Use May-August for best data (June/July preferred but wider range for robustness)
		if month < time.May || month > time.August {
			continue
		}
		if r.Value <= 0 {
			continue
		}
		h := r.Timestamp.Hour()
		hourSum[h] += r.Value
		hourCount[h]++
	}

	// If no summer data, use all data
	hasData := false
	for _, c := range hourCount {
		if c > 0 {
			hasData = true
			break
		}
	}
	if !hasData {
		for _, r := range readings {
			if r.Value <= 0 {
				continue
			}
			h := r.Timestamp.Hour()
			hourSum[h] += r.Value
			hourCount[h]++
		}
	}

	var profile PVProfile
	profile.PeakWp = peakWp

	// Compute average per hour
	var maxAvg float64
	for h := 0; h < 24; h++ {
		if hourCount[h] > 0 {
			avg := hourSum[h] / float64(hourCount[h])
			profile.HourlyFactor[h] = avg
			if avg > maxAvg {
				maxAvg = avg
				profile.PeakHour = h
			}
		}
	}

	// Normalize to peak = 1.0
	if maxAvg > 0 {
		for h := 0; h < 24; h++ {
			profile.HourlyFactor[h] /= maxAvg
		}
	}

	return profile
}

// GenerateOrientedProfile creates a shifted profile for a different panel orientation.
// The base profile's peak corresponds to its original orientation.
//
// azimuthDeg: panel azimuth (0=N, 90=E, 180=S, 270=W)
// tiltDeg: panel tilt from horizontal (0=flat, 90=vertical wall)
// baseAzimuth: the azimuth of the original installation (e.g., 90 for east)
func GenerateOrientedProfile(base PVProfile, azimuthDeg, tiltDeg, baseAzimuth float64) PVProfile {
	// East peaks at ~10:00, South at ~12:00, West at ~14:00
	// Shift = (newAzimuth - baseAzimuth) / 45 hours
	shiftHours := (azimuthDeg - baseAzimuth) / 45.0

	// Tilt factor: affects the width of the generation curve.
	// Steeper tilt → narrower peak (more directional),
	// Flatter tilt → broader curve (more omnidirectional).
	// Reference tilt assumed 40°.
	tiltWidthFactor := 1.0
	if tiltDeg > 40 {
		// Steeper: narrower peak (scale factor < 1 means more concentrated)
		tiltWidthFactor = 1.0 - (tiltDeg-40)/200.0
	} else if tiltDeg < 40 {
		// Flatter: broader peak
		tiltWidthFactor = 1.0 + (40-tiltDeg)/200.0
	}
	if tiltWidthFactor < 0.5 {
		tiltWidthFactor = 0.5
	}
	if tiltWidthFactor > 1.5 {
		tiltWidthFactor = 1.5
	}

	// Also reduce total output for extreme tilts (vertical wall or flat)
	tiltEfficiency := math.Cos((tiltDeg - 35) * math.Pi / 180)
	if tiltEfficiency < 0.5 {
		tiltEfficiency = 0.5
	}

	result := PVProfile{
		PeakWp: base.PeakWp,
	}

	// Shift and interpolate the profile
	peakHour := float64(base.PeakHour)
	var maxFactor float64
	for h := 0; h < 24; h++ {
		// Map this hour back to the original profile's time
		srcHour := float64(h) - shiftHours
		// Apply tilt width: stretch/compress around the shifted peak
		newPeak := peakHour + shiftHours
		distFromPeak := srcHour - peakHour
		// Compress/expand distance from peak based on tilt
		adjustedSrcHour := peakHour + distFromPeak/tiltWidthFactor
		_ = newPeak // just for clarity

		// Interpolate from base profile
		factor := interpolateProfile(base.HourlyFactor, adjustedSrcHour) * tiltEfficiency
		if factor < 0 {
			factor = 0
		}
		result.HourlyFactor[h] = factor
		if factor > maxFactor {
			maxFactor = factor
			result.PeakHour = h
		}
	}

	// Re-normalize so peak = 1.0
	if maxFactor > 0 {
		for h := 0; h < 24; h++ {
			result.HourlyFactor[h] /= maxFactor
		}
	}

	return result
}

// PowerAt returns estimated PV power in watts for the given fractional hour.
func (p *PVProfile) PowerAt(hour float64, peakWp float64) float64 {
	factor := interpolateProfile(p.HourlyFactor, hour)
	if factor < 0 {
		return 0
	}
	return factor * peakWp
}

// interpolateProfile returns linearly interpolated factor for a fractional hour.
func interpolateProfile(factors [24]float64, hour float64) float64 {
	// Wrap to [0, 24)
	for hour < 0 {
		hour += 24
	}
	for hour >= 24 {
		hour -= 24
	}

	lo := int(math.Floor(hour)) % 24
	hi := (lo + 1) % 24
	frac := hour - math.Floor(hour)

	return factors[lo]*(1-frac) + factors[hi]*frac
}

// defaultProfile returns a reasonable default east-facing PV profile.
func defaultProfile(peakWp float64) PVProfile {
	p := PVProfile{
		PeakHour: 10,
		PeakWp:   peakWp,
	}
	// Simple bell curve centered at hour 10
	for h := 0; h < 24; h++ {
		dist := float64(h) - 10.0
		p.HourlyFactor[h] = math.Exp(-dist * dist / 18.0)
		if p.HourlyFactor[h] < 0.01 {
			p.HourlyFactor[h] = 0
		}
	}
	return p
}
