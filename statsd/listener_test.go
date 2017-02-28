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
			Type:  "c",
		},
		&am.Measurement{
			Name:  "gorets",
			Value: 1.0,
			Type:  "c",
		},
		&am.Measurement{
			Name:  "gaugor",
			Value: 333.0,
			Type:  "g",
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
