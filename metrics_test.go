package agentmon

import (
	"testing"
	"time"
)

type event struct {
	m    Measurement
	want float64
}

func driveTest(t *testing.T, events []event) {
	underTest := NewMeasurementSet(nil)
	for i, e := range events {
		underTest.Update(&(e.m))
		var got float64
		switch e.m.Type {
		case Counter, DerivedCounter:
			got = underTest.Counters[e.m.Name]
		default:
			got = underTest.Gauges[e.m.Name]
		}

		if got != e.want {
			t.Errorf("after event %d: got %f, want %f", i+1, got, e.want)
		}

		underTest = NewMeasurementSet(underTest.Snapshot())
	}
}

func TestCounters(t *testing.T) {
	events := []event{
		{
			m: Measurement{
				Name:      "foo.bar",
				Timestamp: time.Now(),
				Type:      Counter,
				Value:     3.0,
				Sample:    1.0,
			},
			want: 3.0,
		},
		{
			m: Measurement{
				Name:      "foo.bar",
				Timestamp: time.Now(),
				Type:      Counter,
				Value:     9.0,
				Sample:    1.0,
			},
			want: 9.0,
		},
		{
			m: Measurement{
				Name:      "foo.bar",
				Timestamp: time.Now(),
				Type:      Counter,
				Value:     3.0,
				Sample:    0.5,
			},
			want: 6.0,
		},
	}

	driveTest(t, events)
}

func TestDerivedCounters(t *testing.T) {
	events := []event{
		{
			m: Measurement{
				Name:      "foo.bar",
				Timestamp: time.Now(),
				Type:      DerivedCounter,
				Value:     1.0,
				Sample:    1.0,
			},
			want: 1.0,
		},
		{
			m: Measurement{
				Name:      "foo.bar",
				Timestamp: time.Now(),
				Type:      DerivedCounter,
				Value:     3.0,
				Sample:    1.0,
			},
			want: 2.0,
		},
		{
			m: Measurement{
				Name:      "foo.bar",
				Timestamp: time.Now(),
				Type:      DerivedCounter,
				Value:     8.0,
				Sample:    1.0,
			},
			want: 5.0,
		},
	}

	driveTest(t, events)

}

func TestGauges(t *testing.T) {
	events := []event{
		{
			m: Measurement{
				Name:      "foo.bar",
				Timestamp: time.Now(),
				Type:      Gauge,
				Value:     1.0,
				Sample:    0.5,
			},
			want: 2.0,
		},
		{
			m: Measurement{
				Name:      "foo.bar",
				Timestamp: time.Now(),
				Type:      Gauge,
				Value:     3.0,
				Sample:    1.0,
			},
			want: 3.0,
		},
		{
			m: Measurement{
				Name:      "foo.bar",
				Timestamp: time.Now(),
				Type:      Gauge,
				Value:     8.0,
				Sample:    1.0,
			},
			want: 8.0,
		},
		{
			m: Measurement{
				Name:      "foo.bar",
				Timestamp: time.Now(),
				Type:      Gauge,
				Value:     8.0,
				Sample:    1.0,
				Modifier:  "+",
			},
			want: 16.0,
		},
		{
			m: Measurement{
				Name:      "foo.bar",
				Timestamp: time.Now(),
				Type:      Gauge,
				Value:     8.0,
				Sample:    1.0,
				Modifier:  "-",
			},
			want: 8.0,
		},
	}

	driveTest(t, events)
}

func TestMeasurementSetLen(t *testing.T) {
	underTest := NewMeasurementSet(nil)
	underTest.Update(&Measurement{
		Name:      "foo.bar",
		Timestamp: time.Now(),
		Type:      Counter,
		Value:     3.0,
		Sample:    1.0,
	})

	underTest.Update(&Measurement{
		Name:      "foo.baz",
		Timestamp: time.Now(),
		Type:      Counter,
		Value:     5.0,
		Sample:    1.0,
	})

	if underTest.Len() != 2 {
		t.Errorf("got %d, want 2", underTest.Len())
	}
}

func TestMeasurementSetSnapshot(t *testing.T) {
	underTest := NewMeasurementSet(nil)
	underTest.Update(&Measurement{
		Name:      "foo.bar",
		Timestamp: time.Now(),
		Type:      Counter,
		Value:     3.0,
		Sample:    1.0,
	})

	underTest.Update(&Measurement{
		Name:      "foo.baz",
		Timestamp: time.Now(),
		Type:      Counter,
		Value:     5.0,
		Sample:    1.0,
	})

	snap := underTest.Snapshot()

	underTest.Update(&Measurement{
		Name:      "foo.baz",
		Timestamp: time.Now(),
		Type:      Counter,
		Value:     20.0,
		Sample:    1.0,
	})

	snapBaz := snap.Counters["foo.baz"]
	origBaz := underTest.Counters["foo.baz"]

	if snapBaz == origBaz {
		t.Errorf("snapshot should not reflect updates to it's parent: snap = %f, orig = %f", snapBaz, origBaz)
	}

}
