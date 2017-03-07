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
		var set agentmon.MeasurementSet
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
