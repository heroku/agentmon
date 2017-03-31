package agentmon

import "time"

type MetricType int

const (
	Counter MetricType = iota
	DerivedCounter
	Gauge

	// Currently unused, except in statsd parsing.
	Timer
)

type Measurement struct {
	Name      string
	Timestamp time.Time
	Type      MetricType
	Value     float64
	Sample    float32
	Modifier  string
}

type MeasurementSet struct {
	Counters map[string]float64 `json:"counters,omitempty"`
	Gauges   map[string]float64 `json:"gauges,omitempty"`
}

func NewMeasurementSet() *MeasurementSet {
	return &MeasurementSet{
		Counters: make(map[string]float64),
		Gauges:   make(map[string]float64),
	}
}

func (ms *MeasurementSet) Update(m *Measurement) {
	switch m.Type {
	case Counter:
		ms.Counters[m.Name] += m.Value * (1 / float64(m.Sample))

	case DerivedCounter:
	case Gauge:
	}
}

func (ms *MeasurementSet) Len() int {
	return len(ms.Counters) + len(ms.Gauges)
}
