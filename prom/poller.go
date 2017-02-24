package prom

import (
	"context"
	"io"
	"log"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"time"

	ag "github.com/heroku/agentmon"
	"github.com/heroku/agentmon/context/online"
	"github.com/matttproud/golang_protobuf_extensions/pbutil"
	"github.com/prometheus/common/expfmt"

	dto "github.com/prometheus/client_model/go"
)

const (
	defaultPollInterval = 5 * time.Second

	defaultAcceptHeader = `application/vnd.google.protobuf;proto=io.prometheus.client.MetricFamily;encoding=delimited;q=0.7,text/plain;version=0.0.4;q=0.3`
	promMediaType       = "application/vnd.google.protobuf"
	promEncoding        = "delimited"
	promProto           = "io.prometheus.client.MetricFamily"
)

type PrometheusConfig struct {
	URL          *url.URL
	Interval     time.Duration
	AcceptHeader string
}

type PrometheusPoller struct {
	Config PrometheusConfig
	Inbox  chan *ag.Measurement
}

func (p PrometheusPoller) Poll(ctx context.Context) {
	if p.Config.Interval == 0 {
		p.Config.Interval = defaultPollInterval
	}
	go p.pollLoop(ctx)
}

func (p PrometheusPoller) pollLoop(ctx context.Context) {
	online.Online(ctx)

	t := time.NewTicker(p.Config.Interval)

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			ch := make(chan *dto.MetricFamily, 1024)
			tctx, cancel := context.WithTimeout(ctx, p.Config.Interval)
			go p.fetchFamilies(tctx, ch)
			p.sync(tctx, ch)
			cancel()
		}
	}
}

func (p PrometheusPoller) sync(ctx context.Context, ch <-chan *dto.MetricFamily) {
	for {
		select {
		case <-ctx.Done():
			return
		case fam := <-ch:
			if ms, ok := familyToMeasurements(fam); ok {
				for _, m := range ms {
					// TODO: Probably don't want to block on this, drop instead.
					p.Inbox <- m
				}
			}
		}
	}
}

func (p PrometheusPoller) fetchFamilies(ctx context.Context, ch chan<- *dto.MetricFamily) {
	u := p.Config.URL.String()
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		log.Fatalf("creating GET request for URL %q failed: %s", u, err)
	}

	req = req.WithContext(ctx)
	if p.Config.AcceptHeader == "" {
		req.Header.Add("Accept", defaultAcceptHeader)
	} else {
		req.Header.Add("Accept", p.Config.AcceptHeader)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatalf("executing GET request for URL %q failed: %s", u, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Fatalf("GET request for URL %q returned HTTP status %s", u, resp.Status)
	}

	mtype, params, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err == nil && mtype == promMediaType &&
		params["encoding"] == promEncoding &&
		params["proto"] == promProto {

		for {
			mf := &dto.MetricFamily{}
			if _, err := pbutil.ReadDelimited(resp.Body, mf); err != nil {
				if err == io.EOF {
					break
				}
				log.Fatalln("reading metric family protocol buffer failed:", err)
			}
			ch <- mf
		}
	} else {
		// We could do further content-type checks here, but the
		// fallback for now will anyway be the text format
		// version 0.0.4, so just go for it and see if it works.
		var parser expfmt.TextParser

		metricFamilies, err := parser.TextToMetricFamilies(resp.Body)

		if err != nil {
			log.Fatalln("reading text format failed:", err)
		}
		for _, mf := range metricFamilies {
			ch <- mf
		}
	}
}

func familyToMeasurements(mf *dto.MetricFamily) (out []*ag.Measurement, ok bool) {
	name := mf.GetName()
	switch mf.GetType() {
	case dto.MetricType_GAUGE:
		for _, m := range mf.Metric {
			out = append(out, &ag.Measurement{
				Name:      name + suffixFor(m),
				Timestamp: msToTime(m.GetTimestampMs()),
				Type:      "g",
				Value:     getValue(m),
				Sample:    1.0,
			})
			ok = true
		}
	case dto.MetricType_COUNTER:
		for _, m := range mf.Metric {
			out = append(out, &ag.Measurement{
				Name:      name + suffixFor(m),
				Timestamp: msToTime(m.GetTimestampMs()),
				Type:      "c",
				Value:     getValue(m),
				Sample:    1.0,
			})
			ok = true
		}
	}
	return
}

func msToTime(ms int64) time.Time {
	secs := ms / 1000
	ns := time.Duration(ms%1000) * time.Millisecond
	return time.Unix(secs, int64(ns)).UTC()
}

func getValue(m *dto.Metric) float64 {
	if m.Gauge != nil {
		return m.GetGauge().GetValue()
	}
	if m.Counter != nil {
		return m.GetCounter().GetValue()
	}
	if m.Untyped != nil {
		return m.GetUntyped().GetValue()
	}
	return 0
}

// suffixFor returns a dot separated string of `label_values`.
func suffixFor(m *dto.Metric) string {
	result := make([]string, 0, len(m.Label))

	for _, lp := range m.Label {
		result = append(result, lp.GetName()+"_"+lp.GetValue())
	}

	if len(result) == 0 {
		return ""
	}
	return "." + strings.Join(result, ".")
}