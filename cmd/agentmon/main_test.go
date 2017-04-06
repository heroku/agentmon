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

package main

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/heroku/agentmon"
)

func makeTestHandler(t *testing.T, found chan string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var set agentmon.MetricSet
		decoder := json.NewDecoder(r.Body)
		err := decoder.Decode(&set)
		if err != nil {
			t.Fatalf("decode: %s", err)
		}
		defer r.Body.Close()

		if set.Len() > 0 {
			for k := range set.Counters {
				found <- k
			}

			for k := range set.Gauges {
				found <- k
			}
		}
	})
}

func TestMainStatsdEndToEnd(t *testing.T) {
	const addr = ":9999"

	ctx, cancel := context.WithCancel(context.Background())

	inbox := make(chan *agentmon.Measurement, 100)
	found := make(chan string, 1)

	handler := makeTestHandler(t, found)
	testServer := httptest.NewServer(handler)
	defer testServer.Close()

	startReporter(ctx, 100*time.Millisecond, testServer.URL, inbox, false)
	startStatsdListener(ctx, addr, inbox, false)

	// Wait for Listener to come online.
	time.Sleep(100 * time.Millisecond)

	conn, err := net.Dial("udp", addr)
	if err != nil {
		t.Fatalf("dial: %s", err)
	}

	// Write a statsd metric to here.
	_, err = conn.Write([]byte("gorets:1|c\n"))
	if err != nil {
		t.Fatalf("write: %s", err)
	}

	select {
	case <-time.After(1 * time.Second):
		t.Errorf("no metric found in 1 second.")
	case m := <-found:
		if m != "gorets" {
			t.Errorf("got %s, want %s", m, "gorets")
		}
	}
	cancel()
}
