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

package statsd

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	am "github.com/heroku/agentmon"
)

type ClosableBuffer struct {
	*bytes.Buffer
	closed bool
}

func (c *ClosableBuffer) Close() error {
	if c.closed {
		return errors.New("already closed")
	}
	c.closed = true
	return nil
}

func TestParseLoop(t *testing.T) {
	buf := bytes.NewBuffer([]byte(`gorets:1|c
gorets:1|c|@0.1
gaugor:333|g
`))
	input := &ClosableBuffer{buf, false}

	var ei int
	expected := []*am.Measurement{
		&am.Measurement{
			Name:  "gorets",
			Value: 1.0,
			Type:  am.Counter,
		},
		&am.Measurement{
			Name:  "gorets",
			Value: 1.0,
			Type:  am.Counter,
		},
		&am.Measurement{
			Name:  "gaugor",
			Value: 333.0,
			Type:  am.Gauge,
		},
	}

	inbox := make(chan *am.Measurement, 3)
	listener := Listener{MaxPacketSize: 100, PartialReads: true, Inbox: inbox}

	go listener.parseLoop(context.Background(), input)

loop:
	for {
		select {
		case m := <-inbox:
			if m.Name != expected[ei].Name {
				t.Errorf("Expected name=%q got=%q", m.Name, expected[ei].Name)
			}
			if m.Value != expected[ei].Value {
				t.Errorf("Expected value=%f got=%f", m.Value, expected[ei].Value)
			}
			if m.Type != expected[ei].Type {
				t.Errorf("Expected type=%q got=%q", m.Type, expected[ei].Type)
			}
			ei++
		case <-time.After(2 * time.Millisecond):
			if len(expected) != ei {
				t.Fatalf("Expected to receive %d measurements, found %d", len(expected), ei)
			}
			break loop
		}
	}
}

func TestParseLoopCancel(t *testing.T) {
	buf := bytes.NewBuffer([]byte(`gorets:1|c
gorets:1|c|@0.1
gaugor:333|g
`))
	input := &ClosableBuffer{buf, false}
	inbox := make(chan *am.Measurement, 1)
	listener := Listener{MaxPacketSize: 100, PartialReads: true, Inbox: inbox}
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel before loop starts
	cancel()
	go listener.parseLoop(ctx, input)

	select {
	case m := <-inbox:
		t.Errorf("Got something %+v, expected nothing", m)
	case <-time.After(1 * time.Millisecond):
	}
}
