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
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/heroku/agentmon"
	"github.com/heroku/agentmon/prom"
	"github.com/heroku/agentmon/reporter"
	"github.com/heroku/agentmon/statsd"
)

var (
	showVersion   = flag.Bool("version", false, "print version string")
	debug         = flag.Bool("debug", false, "debug mode is more verbose")
	flushInterval = flag.Int("interval", 20, "Sink flush interval in seconds")
	promURL       = flag.String("prom-url", "", "Prometheus URL")
	promInterval  = flag.Int("prom-interval", 5, "Prometheus poll interval in seconds")
	statsdAddr    = flag.String("statsd-addr", "", "UDP port for statsd listener")
	bufferSize    = flag.Int("backlog", 1000, "Size of pending measurement buffer")
)

const measurementBufferSize = 1000

func main() {
	log.SetPrefix("agentmon: ")
	log.SetFlags(0)

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: %s [flags] sink-URL", os.Args[0])
	}

	flag.Parse()

	if *showVersion {
		fmt.Printf("agentmon/v%s (built w/%s)\n",
			agentmon.VERSION, runtime.Version())
		return
	}

	if flag.Arg(0) == "" {
		fmt.Fprintf(os.Stderr, "ERROR: sink URL required")
		flag.PrintDefaults()
		os.Exit(1)
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM, syscall.SIGINT)

	ctx, cancel := context.WithCancel(context.Background())

	// For statsd, default to :$PORT, if not specified.
	if port := os.Getenv("PORT"); *statsdAddr == "" && port != "" {
		*statsdAddr = ":" + port
	}

	if *promURL == "" && *statsdAddr == "" {
		log.Fatal("Nothing to start. Exiting.")
	}

	rURL := flag.Arg(0)
	if rURL == "" {
		rURL = os.Getenv("HEROKU_METRICS_URL")
		if rURL == "" {
			log.Fatal("Don't know where to report metrics. Exiting.")
		}
	}

	inbox := make(chan *agentmon.Measurement, *bufferSize)

	if *promURL != "" {
		startPromPoller(ctx, *promURL, inbox, *debug)
	}
	if *statsdAddr != "" {
		startStatsdListener(ctx, *statsdAddr, inbox, *debug)
	}

	startReporter(ctx, time.Duration(*flushInterval)*time.Second, rURL, inbox, *debug)
	handleSignals(sigs, cancel)
}

func handleSignals(sigs chan os.Signal, cancel func()) {
	select {
	case s := <-sigs:
		log.Printf("Got signal %s. Shutting down.\n", s)
		cancel()
	}
}

func startReporter(ctx context.Context, i time.Duration, rURL string, inbox chan *agentmon.Measurement, debug bool) {
	reporter := reporter.Heroku{
		URL:      rURL,
		Interval: i,
		Inbox:    inbox,
		Debug:    debug,
	}
	go reporter.Report(ctx)
}

func startPromPoller(ctx context.Context, u string, inbox chan *agentmon.Measurement, debug bool) {
	pu, err := url.Parse(u)
	if err != nil {
		log.Fatal("Invalid Prometheus URL: %s", err)
	}

	poller := prom.Poller{
		URL:      pu,
		Interval: time.Duration(*promInterval) * time.Second,
		Inbox:    inbox,
		Debug:    debug,
	}
	go poller.Poll(ctx)
}

func startStatsdListener(ctx context.Context, a string, inbox chan *agentmon.Measurement, debug bool) {
	listener := statsd.Listener{
		Addr:  a,
		Inbox: inbox,
		Debug: debug,
	}
	go listener.ListenUDP(ctx)
}
