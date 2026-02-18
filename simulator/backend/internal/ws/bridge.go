package ws

import (
	"log"

	"energy_simulator/internal/simulator"
)

// Bridge implements simulator.Callback and broadcasts events to the WebSocket hub.
type Bridge struct {
	hub *Hub
}

func NewBridge(hub *Hub) *Bridge {
	return &Bridge{hub: hub}
}

func (b *Bridge) OnState(s simulator.State) {
	msg, err := NewEnvelope(TypeSimState, SimStateFromEngine(s))
	if err != nil {
		log.Printf("Error marshaling sim state: %v", err)
		return
	}
	b.hub.Broadcast(msg)
}

func (b *Bridge) OnReading(r simulator.SensorReading) {
	msg, err := NewEnvelope(TypeSensorReading, SensorReadingPayload{
		SensorID:  r.SensorID,
		Value:     r.Value,
		Unit:      r.Unit,
		Timestamp: r.Timestamp,
	})
	if err != nil {
		log.Printf("Error marshaling sensor reading: %v", err)
		return
	}
	b.hub.Broadcast(msg)
}

func (b *Bridge) OnSummary(s simulator.Summary) {
	msg, err := NewEnvelope(TypeSummaryUpdate, SummaryFromEngine(s))
	if err != nil {
		log.Printf("Error marshaling summary: %v", err)
		return
	}
	b.hub.Broadcast(msg)
}

func (b *Bridge) OnBatteryUpdate(u simulator.BatteryUpdate) {
	msg, err := NewEnvelope(TypeBatteryUpdate, BatteryUpdatePayload{
		BatteryPowerW: u.BatteryPowerW,
		AdjustedGridW: u.AdjustedGridW,
		SoCPercent:    u.SoCPercent,
		Timestamp:     u.Timestamp,
	})
	if err != nil {
		log.Printf("Error marshaling battery update: %v", err)
		return
	}
	b.hub.Broadcast(msg)
}

func (b *Bridge) OnBatterySummary(s simulator.BatterySummary) {
	msg, err := NewEnvelope(TypeBatterySummary, BatterySummaryPayload{
		SoCPercent:           s.SoCPercent,
		Cycles:               s.Cycles,
		EffectiveCapacityKWh: s.EffectiveCapacityKWh,
		DegradationPct:       s.DegradationPct,
		TimeAtPowerSec:       s.TimeAtPowerSec,
		TimeAtSoCPctSec:      s.TimeAtSoCPctSec,
		MonthSoCSeconds:      s.MonthSoCSeconds,
	})
	if err != nil {
		log.Printf("Error marshaling battery summary: %v", err)
		return
	}
	b.hub.Broadcast(msg)
}

func (b *Bridge) OnArbitrageDayLog(records []simulator.ArbitrageDayRecord) {
	msg, err := NewEnvelope(TypeArbitrageDayLog, ArbitrageDayLogFromEngine(records))
	if err != nil {
		log.Printf("Error marshaling arbitrage day log: %v", err)
		return
	}
	b.hub.Broadcast(msg)
}

func (b *Bridge) OnPredictionComparison(comp simulator.PredictionComparison) {
	msg, err := NewEnvelope(TypePredictionComparison, PredictionComparisonPayload{
		ActualPowerW:    comp.ActualPowerW,
		PredictedPowerW: comp.PredictedPowerW,
		ActualTempC:     comp.ActualTempC,
		PredictedTempC:  comp.PredictedTempC,
		HasActualTemp:   comp.HasActualTemp,
	})
	if err != nil {
		log.Printf("Error marshaling prediction comparison: %v", err)
		return
	}
	b.hub.Broadcast(msg)
}

func (b *Bridge) OnHeatingStats(stats []simulator.HeatingMonthStat) {
	msg, err := NewEnvelope(TypeHeatingStats, HeatingStatsFromEngine(stats))
	if err != nil {
		log.Printf("Error marshaling heating stats: %v", err)
		return
	}
	b.hub.Broadcast(msg)
}

func (b *Bridge) OnAnomalyDays(records []simulator.AnomalyDayRecord) {
	msg, err := NewEnvelope(TypeAnomalyDays, AnomalyDaysFromEngine(records))
	if err != nil {
		log.Printf("Error marshaling anomaly days: %v", err)
		return
	}
	b.hub.Broadcast(msg)
}

func (b *Bridge) OnLoadShiftStats(stats simulator.LoadShiftStats) {
	msg, err := NewEnvelope(TypeLoadShiftStats, LoadShiftStatsFromEngine(stats))
	if err != nil {
		log.Printf("Error marshaling load shift stats: %v", err)
		return
	}
	b.hub.Broadcast(msg)
}

func (b *Bridge) OnHPDiagnostics(diag simulator.HPDiagnostics) {
	msg, err := NewEnvelope(TypeHPDiagnostics, HPDiagnosticsFromEngine(diag))
	if err != nil {
		log.Printf("Error marshaling HP diagnostics: %v", err)
		return
	}
	b.hub.Broadcast(msg)
}

func (b *Bridge) OnPowerQuality(pq simulator.PowerQuality) {
	msg, err := NewEnvelope(TypePowerQuality, PowerQualityFromEngine(pq))
	if err != nil {
		log.Printf("Error marshaling power quality: %v", err)
		return
	}
	b.hub.Broadcast(msg)
}
