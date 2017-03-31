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
		ms.Counters[m.Name] += m.Value / float64(m.Sample)

	case DerivedCounter:
		current := m.Value
		prev := 0.0

		ms.monoCounters[m.Name] = current

		if ms.parent != nil {
			prev = ms.parent.monoCounters[m.Name]
		}

		val := current / float64(m.Sample)
		if current < prev { // A reset has occurred
			ms.Counters[m.Name] = val
		} else {
			ms.Counters[m.Name] = val - prev
		}

	case Gauge:
		prev := 0.0
		if ms.parent != nil {
			prev = ms.parent.Gauges[m.Name]
		}

		val := (m.Value / float64(m.Sample))

		switch m.Modifier {
		case "+":
			ms.Gauges[m.Name] = prev + val
		case "-":
			ms.Gauges[m.Name] = prev - val
		default:
			ms.Gauges[m.Name] = val
		}
	}
}

func (ms *MeasurementSet) Snapshot() *MeasurementSet {
	out := &MeasurementSet{
		Counters:     make(map[string]float64),
		Gauges:       make(map[string]float64),
		monoCounters: make(map[string]float64),
	}
	for k, v := range ms.Counters {
		out.Counters[k] = v
	}
	for k, v := range ms.Gauges {
		out.Gauges[k] = v
	}
	for k, v := range ms.monoCounters {
		out.monoCounters[k] = v
	}

	return out
}

func (ms *MeasurementSet) Len() int {
	return len(ms.Counters) + len(ms.Gauges)
}
