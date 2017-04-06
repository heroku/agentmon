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

// Heroku reporter stores the parameters necessary to report Metrics to
// Heroku's metrics ingestion endpoint.
type Heroku struct {
	// URL is the URL of the Heroku service.
	URL string

	// Interval is the duration in which to wait before sending the next
	// MetricSet.
	Interval time.Duration

	// Inbox is the channel Measurements from pollers and listeners are
	// received on.
	Inbox chan *am.Measurement

	// Debug turns on more verbose logging.
	Debug bool
}

// Report reads measurements from Inbox, and produces MetricSets that get
// sent to the Heroku metrics service.
func (r Heroku) Report(ctx context.Context) {
	if r.Interval <= 0 {
		r.Interval = defaultHerokuReporterInterval
	}

	currentSet := am.NewMetricSet(nil)
	ticks := time.Tick(r.Interval)

	for {
		select {
		case <-ctx.Done():
			if r.Debug {
				log.Println("debug: stopping HerokuReporter loop")
			}
			return
		case m := <-r.Inbox:
			currentSet.Update(m)
		case <-ticks:
			flushSet := currentSet.Snapshot()
			currentSet = am.NewMetricSet(flushSet)
			go r.flush(ctx, flushSet)
		}
	}
}

func (r Heroku) flush(ctx context.Context, set *am.MetricSet) {
	l := set.Len()
	if l == 0 {
		return
	}

	var buffer bytes.Buffer
	enc := json.NewEncoder(&buffer)
	err := enc.Encode(set)
	if err != nil {
		log.Printf("flush: encode: %s: %#v", err, set)
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

	if r.Debug {
		log.Printf("debug: flushed %d measurements to Heroku", l)
	}
}
