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

type Poller struct {
	URL          *url.URL
	Interval     time.Duration
	AcceptHeader string
	Inbox        chan *ag.Measurement
	Debug        bool
}

func (p Poller) Poll(ctx context.Context) {
	if p.Interval == 0 {
		p.Interval = defaultPollInterval
	}

	t := time.NewTicker(p.Interval)

	for {
		select {
		case <-ctx.Done():
			if p.Debug {
				log.Println("debug: stopping Prometheus Pooler loop")
			}
			return
		case <-t.C:
			ch := make(chan *dto.MetricFamily, 1024)
			tctx, cancel := context.WithTimeout(ctx, p.Interval)
			go p.fetchFamilies(tctx, ch)
			p.sync(tctx, ch)
			cancel()
		}
	}
}

func (p Poller) sync(ctx context.Context, ch <-chan *dto.MetricFamily) {
	for {
		select {
		case <-ctx.Done():
			return
		case fam, ok := <-ch:
			if !ok {
				return
			}

			if ms, ok := familyToMeasurements(fam); ok {
				for _, m := range ms {
					select {
					case p.Inbox <- m:
					default:
						log.Printf("sync: metric set send would block: dropping")
					}
				}
			}
		}
	}
}

func (p Poller) fetchFamilies(ctx context.Context, ch chan<- *dto.MetricFamily) {
	u := p.URL.String()
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		log.Fatalf("fetchFamilies: http.newrequest: failed: %s", u, err)
	}

	req = req.WithContext(ctx)
	if p.AcceptHeader == "" {
		req.Header.Add("Accept", defaultAcceptHeader)
	} else {
		req.Header.Add("Accept", p.AcceptHeader)
	}

	if p.Debug {
		log.Printf("debug: fetching families via Prometheus from %s\n", u)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatalf("fetchFamilies: http.do: failed: %s", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Fatalf("fetchFamilies: bad status: %s", u, resp.Status)
	}

	familyCount := 0

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
				log.Fatalln("fetchFamilies: read-pb: failed: %s", err)
			}
			ch <- mf
			familyCount++
		}
	} else {
		// We could do further content-type checks here, but the
		// fallback for now will anyway be the text format
		// version 0.0.4, so just go for it and see if it works.
		var parser expfmt.TextParser

		metricFamilies, err := parser.TextToMetricFamilies(resp.Body)

		if err != nil {
			log.Fatalln("fetchFamilies: read-text: failed: %s", err)
		}
		for _, mf := range metricFamilies {
			ch <- mf
			familyCount++
		}
	}

	if p.Debug {
		log.Printf("debug: fetched %d families via %s response from Prometheus\n", familyCount, mtype)
	}
	close(ch)
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
		labelName := strings.Map(charMapper, lp.GetName())
		labelVal := strings.Map(charMapper, lp.GetValue())
		result = append(result, labelName+"_"+labelVal)
	}

	if len(result) == 0 {
		return ""
	}
	return "." + strings.Join(result, ".")
}

func charMapper(r rune) rune {
	switch {
	case r >= 'A' && r <= 'Z':
		return r
	case r >= 'a' && r <= 'z':
		return r
	case r >= '0' && r <= '9':
		return r
	case r == '-' || r == '_' || r == '.':
		return r
	default:
		return '_'
	}
}
