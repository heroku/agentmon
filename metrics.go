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
	Counters     map[string]float64 `json:"counters,omitempty"`
	Gauges       map[string]float64 `json:"gauges,omitempty"`
	monoCounters map[string]float64
	parent       *MeasurementSet
}

func NewMeasurementSet(parent *MeasurementSet) *MeasurementSet {
	return &MeasurementSet{
		Counters:     make(map[string]float64),
		Gauges:       make(map[string]float64),
		monoCounters: make(map[string]float64),
		parent:       parent,
	}
}

func (ms *MeasurementSet) Update(m *Measurement) {
	switch m.Type {
	case Counter:
		ms.Counters[m.Name] += m.Value * (1 / float64(m.Sample))

	case DerivedCounter:
		ms.monoCounters[m.Name] = m.Value
		val := m.Value
		if ms.parent != nil {
			if v, ok := ms.parent.monoCounters[m.Name]; ok {
				val = val - v
			}
		}
		ms.Counters[m.Name] += val * (1 / float64(m.Sample))

	case Gauge:
		val := m.Value
		prev := 0.0
		if ms.parent != nil {
			if v, ok := ms.parent.Gauges[m.Name]; ok {
				prev = v
			}
		}

		ms.Gauges[m.Name] += val * (1 / float64(m.Sample))
	}
}

func (ms *MeasurementSet) Snapshot() *MeasurementSet {
	out := &MeasurementSet{
		Counters: make(map[string]float64),
		Gauges:   make(map[string]float64),
	}
	for k, v := range ms.Counters {
		out.Counters[k] = v
	}
	for k, v := range ms.Gauges {
		out.Gauges[k] = v
	}

	return out
}

func (ms *MeasurementSet) Len() int {
	return len(ms.Counters) + len(ms.Gauges)
}
