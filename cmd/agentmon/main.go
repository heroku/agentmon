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
	flushInterval = flag.Int("interval", 20, "Sink flush interval in seconds")
	promURL       = flag.String("prom-url", "", "Prometheus URL")
	promInterval  = flag.Int("prom-interval", 5, "Prometheus poll interval in seconds")
	statsdAddr    = flag.String("statsd-addr", "", "UDP port for statsd listener")
	bufferSize    = flag.Int("backlog", 1000, "Size of pending measurement buffer")
)

const measurementBufferSize = 1000

func main() {
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
		startPromPoller(ctx, *promURL, inbox)
	}
	if *statsdAddr != "" {
		startStatsdListener(ctx, *statsdAddr, inbox)
	}

	startReporter(ctx, time.Duration(*flushInterval)*time.Second, rURL, inbox)
	handleSignals(sigs, cancel)
}

func handleSignals(sigs chan os.Signal, cancel func()) {
	select {
	case s := <-sigs:
		log.Printf("Got signal %s. Shutting down.\n", s)
		cancel()
	}
}

func startReporter(ctx context.Context, i time.Duration, rURL string, inbox chan *agentmon.Measurement) {
	reporter := reporter.Heroku{
		URL:      rURL,
		Interval: i,
		Inbox:    inbox,
	}
	go reporter.Report(ctx)
}

func startPromPoller(ctx context.Context, u string, inbox chan *agentmon.Measurement) {
	pu, err := url.Parse(u)
	if err != nil {
		log.Fatalf("ERROR: Invalid Prometheus URL: %s", err)
	}

	poller := prom.PrometheusPoller{
		Config: prom.PrometheusConfig{
			URL:      pu,
			Interval: time.Duration(*promInterval) * time.Second,
		},
		Inbox: inbox,
	}
	poller.Poll(ctx)
}

func startStatsdListener(ctx context.Context, a string, inbox chan *agentmon.Measurement) {
	listener := statsd.Listener{
		Addr:  a,
		Inbox: inbox,
	}
	listener.ListenUDP(ctx)
}
