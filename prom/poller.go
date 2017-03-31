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

// Poller defined parameters for scraping a Prometheus endpoint, over HTTP.
type Poller struct {
	// URL is the URL that the poller should scrape.
	URL *url.URL

	// Interval is the amount of time that should pass between scrapes.
	Interval time.Duration

	// AcceptHeader is used to negotiate the exposition format from the
	// Prometheus endpoint.
	AcceptHeader string

	// Inbox is the channel to use to observe scraped measurements.
	Inbox chan *ag.Measurement

	// Debug is used to turn on extended logging, useful for debugging
	// purposes.
	Debug bool
}

// Poll performs a scrape of the Prometheus endpoint every Poller.Interval.
// The measurements found while scraping will be sent to Poller.Inbox.
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
		log.Fatalf("fetchFamilies: http.newrequest: failed: %s", err)
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
		log.Fatalf("fetchFamilies: bad status: %s", resp.Status)
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
			p.debugMF("protobuff mf", mf)
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
			p.debugMF("non protobuff mf", mf)
			ch <- mf
			familyCount++
		}
	}

	if p.Debug {
		log.Printf("debug: fetched %d families via %s response from Prometheus\n", familyCount, mtype)
	}
	close(ch)
}

func (p Poller) debugMF(msg string, mf *dto.MetricFamily) {
	if !p.Debug {
		return
	}
	log.Printf("%s mf.GetName(): %s\n", msg, mf.GetName())
	switch t := mf.GetType(); t {
	case dto.MetricType_COUNTER:
		log.Printf("%d = dto.MetricType_COUNTER\n", t)
	case dto.MetricType_GAUGE:
		log.Printf("%d = dto.MetricType_GAUGE\n", t)
	case dto.MetricType_HISTOGRAM:
		log.Printf("%d = dto.MetricType_HISTOGRAM\n", t)
	case dto.MetricType_SUMMARY:
		log.Printf("%d = dto.MetricType_SUMMARY\n", t)
	case dto.MetricType_UNTYPED:
		log.Printf("%d = dto.MetricType_UNTYPED\n", t)
	default:
		log.Printf("%d = UNKNOWN!\n", t)
	}
	for i, m := range mf.Metric {
		for j, lp := range m.GetLabel() {
			log.Printf("metric %d: lp %d: GetName(): %q GetValue: %q\n", i, j, lp.GetName(), lp.GetValue())
		}
		log.Printf("metric %d: m.GetTimestampsMs(): %d\n", i, m.GetTimestampMs())
		if c := m.GetCounter(); c != nil {
			log.Printf("metric %d: m.GetCounter().String(): %q\n", i, c.String())
			log.Printf("metric %d: m.GetCounter().GetValue(): %f\n", i, c.GetValue())
		}
		if g := m.GetGauge(); g != nil {
			log.Printf("metric %d: m.GetGauge().String(): %q\n", i, g.String())
			log.Printf("metric %d: m.GetGauge().GetVaue(): %f\n", i, g.GetValue())
		}
		if h := m.GetHistogram(); h != nil {
			log.Printf("metric %d: m.GetHistogram().String(): %q\n", i, h.String())
			log.Printf("metric %d: h.GetSampleCount(): %d\n", i, h.GetSampleCount())
			log.Printf("metric %d: h.GetSampleSum(): %f\n", i, h.GetSampleSum())
			for j, b := range h.GetBucket() {
				log.Printf("metric %d: bucket %d: b.String(): %q\n", i, j, b.String())
				log.Printf("metric %d: bucket %d: b.GetCumulativeCount(): %d\n", i, j, b.GetCumulativeCount())
				log.Printf("metric %d: bucket %d: b.GetUpperBound(): %f\n", i, j, b.GetUpperBound())
			}
		}
		if s := m.GetSummary(); s != nil {
			log.Printf("metric %d: m.GetSummary().String(): %q\n", i, s.String())
			log.Printf("metric %d: m.GetSummary().GetSampleCount(): %d\n", i, s.GetSampleCount())
			log.Printf("metric %d: m.GetSummary().GetSampleSum(): %f\n", i, s.GetSampleSum())
			for j, q := range s.GetQuantile() {
				log.Printf("metric %d: quantile %d: q.String(): %q\n", i, j, q.String())
				log.Printf("metric %d: quantile %d: q.GetQuantile(): %f\n", i, j, q.GetQuantile())
				log.Printf("metric %d: quantile %d: q.GetValue(): %f\n", i, j, q.GetValue())
			}
		}
		if u := m.GetUntyped(); u != nil {
			log.Printf("metric %d: m.GetUntyped().String(): %q\n", i, u.String())
			log.Printf("metric %d: m.GetUntyped().GetValue(): %f\n", i, u.GetValue())
		}
	}
	log.Println("--------------------------------------------")
}

func familyToMeasurements(mf *dto.MetricFamily) (out []*ag.Measurement, ok bool) {
	name := mf.GetName()
	switch mf.GetType() {
	case dto.MetricType_GAUGE:
		for _, m := range mf.Metric {
			out = append(out, &ag.Measurement{
				Name:      name + suffixFor(m),
				Timestamp: msToTime(m.GetTimestampMs()),
				Type:      ag.Gauge,
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
				Type:      ag.DerivedCounter,
				Value:     getValue(m),
				Sample:    1.0,
			})
			ok = true
		}
	case dto.MetricType_SUMMARY:
		for _, m := range mf.Metric {
			summary := m.GetSummary()
			out = append(out, &ag.Measurement{
				Name:      name + "_sum" + suffixFor(m),
				Timestamp: msToTime(m.GetTimestampMs()),
				Type:      ag.DerivedCounter,
				Value:     summary.GetSampleSum(),
				Sample:    1.0,
			})
			out = append(out, &ag.Measurement{
				Name:      name + "_count" + suffixFor(m),
				Timestamp: msToTime(m.GetTimestampMs()),
				Type:      ag.DerivedCounter,
				Value:     float64(summary.GetSampleCount()),
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
