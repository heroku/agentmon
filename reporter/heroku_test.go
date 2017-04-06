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
	"log"
	"os"
	"testing"
	"time"

	am "github.com/heroku/agentmon"
)

func TestReporterLoopCancel(t *testing.T) {
	inbox := make(chan *am.Measurement, 1)
	reporter := Heroku{Interval: time.Duration(1), Inbox: inbox}
	ctx, cancel := context.WithCancel(context.Background())

	// Redirect logging temporarily
	var buf bytes.Buffer

	log.SetOutput(&buf)
	defer log.SetOutput(os.Stdout)

	// Cancel before loop starts
	cancel()
	go reporter.Report(ctx)

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
