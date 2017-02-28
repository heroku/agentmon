package reporter

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	am "github.com/heroku/agentmon"
)

const (
	defaultHerokuReporterInterval = 20 * time.Second

	headerMeasurementsCount = "Measurements-Count"
	headerMeasurementsTime  = "Measurements-Time"
)

// Heroku reporter
type Heroku struct {
	URL      string
	Interval time.Duration
	Inbox    chan *am.Measurement
}

// Report measurments to Heroku
func (r Heroku) Report(ctx context.Context) {
	if r.Interval <= 0 {
		r.Interval = defaultHerokuReporterInterval
	}

	measurements := am.NewMeasurementSet()
	ticks := time.Tick(r.Interval)

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

func (r Heroku) flush(ctx context.Context, set *am.MeasurementSet) {
	l := set.Len()
	if l == 0 {
		return
	}

	var buffer bytes.Buffer
	enc := json.NewEncoder(&buffer)
	err := enc.Encode(set)
	if err != nil {
		log.Printf("flush: encode: %s", err)
		return
	}

	req, err := http.NewRequest(http.MethodPost, r.URL, &buffer)
	if err != nil {
		log.Printf("flush: %s", err)
		return
	}

	now := time.Now().UTC()
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add(headerMeasurementsCount, strconv.Itoa(l))
	req.Header.Add(headerMeasurementsTime, now.Format(time.RFC3339))

	// send() will retry, but we should probably give up at some point...
	ctx, cancel := context.WithTimeout(ctx, r.Interval*2)
	defer cancel()

	req = req.WithContext(ctx)

	err = send(req)
	if err != nil {
		log.Printf("flush: send: %s", err)
	}
}