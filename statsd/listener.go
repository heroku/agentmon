package statsd

import (
	"context"
	"io"
	"log"
	"net"

	"github.com/heroku/agentmon"
)

const (
	defaultMaxPacketSizeUDP = 1472
)

// Listener defines the parameters needed to accept statsd protocol
// UDP packets.
type Listener struct {
	// MaxPacketSize is the maximum amount of bytes that will be read per incoming statsd datagram.
	MaxPacketSize int64

	// Addr is the address to be used for listening for UDP datagrams.
	Addr string

	PartialReads bool

	// Inbox is the channel to use to observe incoming measurements
	Inbox chan *agentmon.Measurement

	// Debug is used to turn on extended logging, useful for debugging
	// purposes.
	Debug bool
}

// ListenUDP sets
func (s Listener) ListenUDP(ctx context.Context) {
	resAddr, err := net.ResolveUDPAddr("udp", s.Addr)
	if err != nil {
		log.Fatalf("listenUDP: resolve addr: %s", err)
	}

	log.Printf("Listening on %s...", resAddr)
	listener, err := net.ListenUDP("udp", resAddr)
	if err != nil {
		log.Fatalf("listenUDP: %s", err)
	}

	s.parseLoop(ctx, listener)
}

func (s Listener) parseLoop(ctx context.Context, conn io.ReadCloser) {
	defer conn.Close()

	if s.MaxPacketSize == 0 {
		s.MaxPacketSize = defaultMaxPacketSizeUDP
	}

	if s.Debug {
		log.Println("debug: handling incoming statsd packet")
	}

	parser := NewParser(conn, s.PartialReads, int(s.MaxPacketSize))

	for {
		select {
		case <-ctx.Done():
			break
		default:
			m, more := parser.Next()
			if m != nil {
				s.Inbox <- m
			}

			if !more {
				break
			}
		}
	}
}
