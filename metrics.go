package agentmon

import "time"

// MetricType represents the type of metric, Measurements will result in.
type MetricType int

const (
	// Counter represents a positive change in value for a flush interval.
	Counter MetricType = iota

	// DerivedCounter represents a monotonically increasing counter value that
	// is not supposed to be reset by the source.
	DerivedCounter

	// Gauge represents the value right now.
	Gauge

	// Timer is currently unused, but exists for statsd parsing purposes.
	Timer
)

// Measurement is a point in time value that is used to amend a metric.
type Measurement struct {
	// Name is the metric to contribute to.
	Name string

	// Timestamp is the time at which the measurement was taken.
	Timestamp time.Time

	// Type is the type of the metric we wish to amend.
	Type MetricType

	// Value is the amount by which we will amend the metric.
	// For Gauges, the amendment might be "replace".
	Value float64

	// Sample stores the sample rate that this measurement was taken at.
	// For most applications, this should be set to 1.0
	Sample float32

	// Modifier allows gauges to be treated differently
	//
	// A value of "-" subtracts Value from the metric's previous value.
	// A value of "+" adds Value to metric's previous value.  An empty
	// value replaces the metric's value with Value.
	Modifier string
}

// MetricSet provides a container for a set of metrics, and encodes
// the rules for how metrics are updated given a Measurement.
type MetricSet struct {
	Counters     map[string]float64 `json:"counters,omitempty"`
	Gauges       map[string]float64 `json:"gauges,omitempty"`
	monoCounters map[string]float64
	parent       *MetricSet
}

// NewMetricSet constructs a MetricSet which can be used to turn
// Measurements into metrics, that can be reported via a reporter.
//
// If a parent is given, it is expected to be the previously reported
// MetricSet, in order to capture the change in metrics, for both
// DerivedCounters, and modified Guages.
func NewMetricSet(parent *MetricSet) *MetricSet {
	return &MetricSet{
		Counters:     make(map[string]float64),
		Gauges:       make(map[string]float64),
		monoCounters: make(map[string]float64),
		parent:       parent,
	}
}

// Update applies a Measurement to the MetricSet depending on its
// Type.
//
// In cases where a Measurement for a Metric has a different Type than
// was previously updated, a new Metric with that type will be created.
func (ms *MetricSet) Update(m *Measurement) {
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
			ms.Counters[m.Name] += val
		} else {
			ms.Counters[m.Name] += val - prev
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

// Snapshot returns a copy of the MetricSet, removing the reference to
// the MetricSet's parent.
func (ms *MetricSet) Snapshot() *MetricSet {
	out := &MetricSet{
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

// Len returns the cardinality of this set.
func (ms *MetricSet) Len() int {
	return len(ms.Counters) + len(ms.Gauges)
}
