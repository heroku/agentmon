package reporter

import (
	"bytes"
	"context"
	"log"
	"os"
	"testing"
	"time"

	am "github.com/heroku/agentmon"
)

func TestReporterLoopCancel(t *testing.T) {
	inbox := make(chan *am.Measurement, 1)
	config := HerokuConfig{Interval: time.Duration(1)}
	reporter := HerokuReporter{config, inbox}
	ctx, cancel := context.WithCancel(context.Background())

	// Redirect logging temporarily
	var buf bytes.Buffer

	log.SetOutput(&buf)
	defer log.SetOutput(os.Stdout)

	// Cancel before loop starts
	cancel()
	go reporter.reportLoop(ctx)

	// Populate lines from log into a channel.
	lines := make(chan string, 10)
	done := make(chan struct{}, 1)
	go func(buf *bytes.Buffer) {
		var pre string
		for {
			select {
			case <-done:
				return
			default:
				if in, err := buf.ReadString('\n'); len(in) > 0 && err == nil {
					lines <- in
				} else if len(in) > 0 {
					pre = pre + in
				}
			}
		}
	}(&buf)

	select {
	case line := <-lines:
		t.Errorf("Got a line %q, expected nothing", line)
	case <-time.After(1 * time.Millisecond):
		done <- struct{}{}
		close(lines)
	}
}
