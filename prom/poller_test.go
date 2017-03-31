package prom

import (
	"bytes"
	"context"
	"log"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	am "github.com/heroku/agentmon"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	dto "github.com/prometheus/client_model/go"
)

func setup() (*url.URL, map[string]float64, func()) {
	reg := prometheus.NewRegistry()

	counterVec := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name:        "some_counter",
			Help:        "counter help",
			ConstLabels: prometheus.Labels{"type": "http"},
		},
		[]string{"code"},
	)
	counterVec.WithLabelValues("200").Inc()
	counterVec.WithLabelValues("500").Inc()
	reg.MustRegister(counterVec)

	gaugeVec := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name:        "some_gauge",
			Help:        "gauge help",
			ConstLabels: prometheus.Labels{"type": "temperature"},
		},
		[]string{"location"},
	)
	gaugeVec.WithLabelValues("office").Add(75)
	gaugeVec.WithLabelValues("kitchen").Add(76)
	gaugeVec.WithLabelValues("pantry #1").Add(71)
	reg.MustRegister(gaugeVec)

	expectations := map[string]float64{
		"some_counter.code_200.type_http":                1,
		"some_counter.code_500.type_http":                1,
		"some_gauge.location_office.type_temperature":    75,
		"some_gauge.location_kitchen.type_temperature":   76,
		"some_gauge.location_pantry__1.type_temperature": 71,
	}

	buf := &bytes.Buffer{}
	logger := log.New(buf, "", 0)

	handler := promhttp.HandlerFor(reg, promhttp.HandlerOpts{
		ErrorLog:      logger,
		ErrorHandling: promhttp.HTTPErrorOnError,
	})

	server := httptest.NewServer(handler)
	u, _ := url.Parse(server.URL)
	return u, expectations, func() {
		server.Close()
	}
}

func TestPromPoller(t *testing.T) {
	acceptHeaders := []string{
		`text/plain; version=0.0.4`,
		`application/vnd.google.protobuf; proto=io.prometheus.client.MetricFamily; encoding=delimited`,
		`application/vnd.google.protobuf; proto=io.prometheus.client.MetricFamily; encoding=delimited; q=0.7,text/plain; version=0.0.4; q=0.3`,
	}

	u, exp, teardown := setup()
	defer teardown()

	for _, ah := range acceptHeaders {
		t.Run(ah, func(t *testing.T) {
			expCopy := make(map[string]float64)
			for k, v := range exp {
				expCopy[k] = v
			}
			testPollerForType(t, u, expCopy, ah)
		})
	}
}

func testPollerForType(t *testing.T, u *url.URL, exp map[string]float64, acceptHeader string) {
	found := make(map[string]int)

	in := make(chan *am.Measurement, 4)
	poller := Poller{
		URL:          u,
		Interval:     50 * time.Millisecond,
		AcceptHeader: acceptHeader,
		Inbox:        in,
	}

	ctx, cancel := context.WithCancel(context.Background())
	go poller.Poll(ctx)
	defer cancel()

	timeout := time.After(90 * time.Millisecond)

	for {
		select {
		case m := <-in:
			if val, ok := exp[m.Name]; !ok {
				t.Fatalf("Received measurement for unexpected metric %s", m.Name)
			} else if val != m.Value {
				t.Fatalf("Expected want=%f, got=%f", val, m.Value)
			}
			found[m.Name]++
		case <-timeout:
			if len(found) != len(exp) {
				t.Fatalf("Expectations left unsatisfied: %+v", len(exp)-len(found))
			}
			return
		}
	}
}

func fakeSummaryFamily() (*dto.MetricFamily, []*am.Measurement) {
	name := "some_summary"
	path := "path"
	index := "index"
	typ := dto.MetricType_SUMMARY
	cnt := uint64(2)
	sum := float64(20.0)

	return &dto.MetricFamily{
			Name: &name,
			Type: &typ,
			Metric: []*dto.Metric{
				{
					Label: []*dto.LabelPair{
						&dto.LabelPair{Name: &path, Value: &index},
					},
					Summary: &dto.Summary{SampleCount: &cnt, SampleSum: &sum},
				},
			},
		}, []*am.Measurement{
			{
				Name:  "some_summary_sum.path_index",
				Value: 20,
				Type:  am.DerivedCounter,
			},
			{
				Name:  "some_summary_count.path_index",
				Value: 2,
				Type:  am.DerivedCounter,
			},
		}
}

// Summaries are time based, and so very hard to actually test as
// integration tests with a web handler as we can with counters and
// gauges. This test provides much of the same functionality as
// TestPromPoller, but assumes summaries will simply be included in
// the exposition.
func TestSummaryNaming(t *testing.T) {
	family, exps := fakeSummaryFamily()

	out, ok := familyToMeasurements(family)
	if !ok {
		t.Fatalf("got %b, want true", ok)
	}
	if len(out) != 2 {
		t.Fatalf("got len(%d), want len(2)", len(out))
	}

	for i, got := range out {
		want := exps[i]
		if want.Name != got.Name {
			t.Errorf("want(name) = %v, got(name) = %v", want.Name, got.Name)
		}
		if want.Value != got.Value {
			t.Errorf("want(value) = %f, got(value) = %f", want.Value, got.Value)
		}
		if want.Type != got.Type {
			t.Errorf("want(type) = %v, got(type) = %v", want.Type, got.Type)
		}
	}
}

func fakeCounterFamily() (*dto.MetricFamily, []*am.Measurement) {
	sc := "some_counter"
	mt := dto.MetricType_COUNTER
	t00 := "200"
	f00 := "500"
	code := "code"
	typ := "type"
	htp := "http"
	one := float64(1)

	return &dto.MetricFamily{
			Name: &sc,
			Type: &mt,
			Metric: []*dto.Metric{
				{
					Label: []*dto.LabelPair{
						&dto.LabelPair{Name: &code, Value: &t00},
						&dto.LabelPair{Name: &typ, Value: &htp},
					},
					Counter: &dto.Counter{Value: &one},
				},
				{
					Label: []*dto.LabelPair{
						&dto.LabelPair{Name: &code, Value: &f00},
						&dto.LabelPair{Name: &typ, Value: &htp},
					},
					Counter: &dto.Counter{Value: &one},
				},
			},
		}, []*am.Measurement{
			{
				Name:  "some_counter.code_200.type_http",
				Value: 1,
				Type:  am.DerivedCounter,
			},
			{
				Name:  "some_counter.code_500.type_http",
				Value: 1,
				Type:  am.DerivedCounter,
			},
		}

}

func TestPollerSync(t *testing.T) {
	mf, expected := fakeCounterFamily()

	inbox := make(chan *am.Measurement, 2)
	poller := Poller{Inbox: inbox}

	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan *dto.MetricFamily, 1)
	go poller.sync(ctx, ch)
	ch <- mf

	ei := 0

loop:
	for {
		select {
		case m := <-inbox:
			if expected[ei].Name != m.Name {
				t.Errorf("Expected name=%q got=%q", expected[ei].Name, m.Name)
			}
			if expected[ei].Value != m.Value {
				t.Errorf("Expected name=%f got=%f", expected[ei].Value, m.Value)
			}
			if expected[ei].Type != m.Type {
				t.Errorf("Expected type=%q got=%q", expected[ei].Type, m.Type)
			}
			ei++

		case <-time.After(1 * time.Millisecond):
			if len(expected) != ei {
				t.Fatalf("Expected to receive %d measurements, found %d", len(expected), ei)
			}
			break loop
		}
	}

	cancel()
}

func TestPollerSyncCancel(t *testing.T) {
	inbox := make(chan *am.Measurement, 2)
	poller := Poller{Inbox: inbox}
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan *dto.MetricFamily, 1)
	cancel()
	go poller.sync(ctx, ch)

	select {
	case m := <-inbox:
		t.Fatalf("Got %+v, want nothing", m)
	case <-time.After(10 * time.Millisecond):
	}
}
