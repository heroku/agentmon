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
	reg.MustRegister(gaugeVec)

	expectations := map[string]float64{
		"some_counter.code_200.type_http":              1,
		"some_counter.code_500.type_http":              1,
		"some_gauge.location_office.type_temperature":  75,
		"some_gauge.location_kitchen.type_temperature": 76,
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
	config := PrometheusConfig{
		URL:          u,
		Interval:     50 * time.Millisecond,
		AcceptHeader: acceptHeader,
	}

	found := make(map[string]int)

	in := make(chan *am.Measurement, 4)
	poller := PrometheusPoller{Config: config, Inbox: in}

	ctx, cancel := context.WithCancel(context.Background())
	poller.Poll(ctx)
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

func fakeMetricFamily() (*dto.MetricFamily, []*am.Measurement) {
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
				Type:  "c",
			},
			{
				Name:  "some_counter.code_500.type_http",
				Value: 1,
				Type:  "c",
			},
		}

}

func TestPollerSync(t *testing.T) {
	mf, expected := fakeMetricFamily()

	inbox := make(chan *am.Measurement, 2)
	poller := PrometheusPoller{Inbox: inbox}

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
	poller := PrometheusPoller{Inbox: inbox}
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
