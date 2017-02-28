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

type Config struct {
	MaxPacketSize int64
	Addr          string
	PartialReads  bool
}

type Listener struct {
	MaxPacketSize int64
	Addr          string
	PartialReads  bool
	Inbox         chan *agentmon.Measurement
}

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

	go s.parseLoop(ctx, listener)
}

func (s Listener) parseLoop(ctx context.Context, conn io.ReadCloser) {
	defer conn.Close()

	if s.MaxPacketSize == 0 {
		s.MaxPacketSize = defaultMaxPacketSizeUDP
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
