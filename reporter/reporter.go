package reporter

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	am "github.com/heroku/agentmon"
)

const (
	defaultHerokuReporterInterval = 20 * time.Second
)

type HerokuConfig struct {
	URL      string
	Interval time.Duration
}

type HerokuReporter struct {
	Config HerokuConfig
	Inbox  chan *am.Measurement
}

func (r HerokuReporter) Report(ctx context.Context) {
	if r.Config.URL == "" {
		r.Config.URL = os.Getenv("HEROKU_METRICS_URL")
	}
	if r.Config.Interval <= 0 {
		r.Config.Interval = defaultHerokuReporterInterval
	}

	go r.reportLoop(ctx)
}

func (r HerokuReporter) reportLoop(ctx context.Context) {
	measurements := am.NewMeasurementSet()
	ticks := time.Tick(r.Config.Interval)
	for {
		select {
		case <-ctx.Done():
			return
		case m := <-r.Inbox:
			measurements.Update(m)
		case <-ticks:
			out := measurements
			measurements = am.NewMeasurementSet()

			go r.flush(ctx, out)
		}
	}
}

func (r HerokuReporter) flush(ctx context.Context, set *am.MeasurementSet) {
	l := set.Len()
	if l == 0 {
		log.Printf("Nothing to flush in this interval.")
	}

	log.Printf("Flushing %d metrics in this interval.", set.Len())

	var buffer bytes.Buffer
	enc := json.NewEncoder(&buffer)
	err := enc.Encode(set)
	if err != nil {
		// TODO: handle the error in some way here.
		log.Printf("ERROR: HerokuReporter.flush: %s", err)
		return
	}

	// Send to the reporter.
	// TODO: ROBUSTIFY THIS!!!!!

	log.Printf("SENDING TO: %s", r.Config.URL)
	http.Post(r.Config.URL, "application/json", &buffer)
}
