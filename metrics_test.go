// Copyright (c) 2017, Heroku Inc <metrics-feedback@heroku.com>.
// All rights reserved.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions are
// met:
//
// * Redistributions of source code must retain the above copyright
//   notice, this list of conditions and the following disclaimer.
//
// * Redistributions in binary form must reproduce the above copyright
//   notice, this list of conditions and the following disclaimer in the
//   documentation and/or other materials provided with the distribution.
//
// * The names of its contributors may not be used to endorse or promote
//   products derived from this software without specific prior written
//   permission.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
// "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
// LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
// A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
// OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
// SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
// LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
// DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
// THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
// (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
// OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

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
	underTest := NewMetricSet(nil)
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

		underTest = NewMetricSet(underTest.Snapshot())
	}
}

func TestCounters(t *testing.T) {
	events := []event{
		{
			m: Measurement{
				Name:       "foo.bar",
				Timestamp:  time.Now(),
				Type:       Counter,
				Value:      3.0,
				SampleRate: 1.0,
			},
			want: 3.0,
		},
		{
			m: Measurement{
				Name:       "foo.bar",
				Timestamp:  time.Now(),
				Type:       Counter,
				Value:      9.0,
				SampleRate: 1.0,
			},
			want: 9.0,
		},
		{
			m: Measurement{
				Name:       "foo.bar",
				Timestamp:  time.Now(),
				Type:       Counter,
				Value:      3.0,
				SampleRate: 0.5,
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
				Name:       "foo.bar",
				Timestamp:  time.Now(),
				Type:       DerivedCounter,
				Value:      1.0,
				SampleRate: 1.0,
			},
			want: 1.0,
		},
		{
			m: Measurement{
				Name:       "foo.bar",
				Timestamp:  time.Now(),
				Type:       DerivedCounter,
				Value:      3.0,
				SampleRate: 1.0,
			},
			want: 2.0,
		},
		{
			m: Measurement{
				Name:       "foo.bar",
				Timestamp:  time.Now(),
				Type:       DerivedCounter,
				Value:      8.0,
				SampleRate: 1.0,
			},
			want: 5.0,
		},
	}

	driveTest(t, events)
}

func TestDerivedCountersWithRest(t *testing.T) {
	events := []event{
		{
			m: Measurement{
				Name:       "foo.bar",
				Timestamp:  time.Now(),
				Type:       DerivedCounter,
				Value:      1.0,
				SampleRate: 1.0,
			},
			want: 1.0,
		},
		{
			m: Measurement{
				Name:       "foo.bar",
				Timestamp:  time.Now(),
				Type:       DerivedCounter,
				Value:      8.0,
				SampleRate: 1.0,
			},
			want: 7.0,
		},
		{
			m: Measurement{
				Name:       "foo.bar",
				Timestamp:  time.Now(),
				Type:       DerivedCounter,
				Value:      3.0,
				SampleRate: 1.0,
			},
			want: 3.0,
		},
	}

	driveTest(t, events)
}

func TestGauges(t *testing.T) {
	events := []event{
		{
			m: Measurement{
				Name:       "foo.bar",
				Timestamp:  time.Now(),
				Type:       Gauge,
				Value:      1.0,
				SampleRate: 0.5, // SampleRate should be ignored for gauges.
			},
			want: 1.0,
		},
		{
			m: Measurement{
				Name:       "foo.bar",
				Timestamp:  time.Now(),
				Type:       Gauge,
				Value:      3.0,
				SampleRate: 1.0,
			},
			want: 3.0,
		},
		{
			m: Measurement{
				Name:       "foo.bar",
				Timestamp:  time.Now(),
				Type:       Gauge,
				Value:      8.0,
				SampleRate: 1.0,
			},
			want: 8.0,
		},
		{
			m: Measurement{
				Name:       "foo.bar",
				Timestamp:  time.Now(),
				Type:       Gauge,
				Value:      8.0,
				SampleRate: 1.0,
				Modifier:   "+",
			},
			want: 16.0,
		},
		{
			m: Measurement{
				Name:       "foo.bar",
				Timestamp:  time.Now(),
				Type:       Gauge,
				Value:      8.0,
				SampleRate: 1.0,
				Modifier:   "-",
			},
			want: 8.0,
		},
	}

	driveTest(t, events)
}

func TestMetricSetLen(t *testing.T) {
	underTest := NewMetricSet(nil)
	underTest.Update(&Measurement{
		Name:       "foo.bar",
		Timestamp:  time.Now(),
		Type:       Counter,
		Value:      3.0,
		SampleRate: 1.0,
	})

	underTest.Update(&Measurement{
		Name:       "foo.baz",
		Timestamp:  time.Now(),
		Type:       Counter,
		Value:      5.0,
		SampleRate: 1.0,
	})

	if underTest.Len() != 2 {
		t.Errorf("got %d, want 2", underTest.Len())
	}
}

func TestMetricSetSnapshot(t *testing.T) {
	underTest := NewMetricSet(nil)
	underTest.Update(&Measurement{
		Name:       "foo.bar",
		Timestamp:  time.Now(),
		Type:       Counter,
		Value:      3.0,
		SampleRate: 1.0,
	})

	underTest.Update(&Measurement{
		Name:       "foo.baz",
		Timestamp:  time.Now(),
		Type:       Counter,
		Value:      5.0,
		SampleRate: 1.0,
	})

	snap := underTest.Snapshot()

	underTest.Update(&Measurement{
		Name:       "foo.baz",
		Timestamp:  time.Now(),
		Type:       Counter,
		Value:      20.0,
		SampleRate: 1.0,
	})

	snapBaz := snap.Counters["foo.baz"]
	origBaz := underTest.Counters["foo.baz"]

	if snapBaz == origBaz {
		t.Errorf("snapshot should not reflect updates to it's parent: snap = %f, orig = %f", snapBaz, origBaz)
	}

}
