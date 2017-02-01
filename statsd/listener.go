package statsd

import (
	"context"
	"io"
	"log"
	"net"
	"os"

	"github.com/heroku/agentmon"
)

const (
	defaultMaxPacketSizeUDP = 1472
)

type StatsdConfig struct {
	MaxPacketSize int64
	Addr          string
	PartialReads  bool
}

type StatsdListener struct {
	Config StatsdConfig
	Inbox  chan *agentmon.Measurement
}

func (s StatsdListener) ListenUDP(ctx context.Context) {
	addr := s.Config.Addr
	if addr == "" {
		addr = ":" + os.Getenv("PORT")
	}

	resAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		log.Fatalf("ERROR: Could not resolve UDP address: %q", err)
	}

	log.Printf("Listening on %s...", resAddr)
	listener, err := net.ListenUDP("udp", resAddr)
	if err != nil {
		log.Fatalf("ERROR: ListenUDP: %q", err)
	}

	go s.parseLoop(ctx, listener)
}

func (s StatsdListener) parseLoop(ctx context.Context, conn io.ReadCloser) {
	defer conn.Close()

	if s.Config.MaxPacketSize == 0 {
		s.Config.MaxPacketSize = defaultMaxPacketSizeUDP
	}

	parser := NewStatsdParser(conn, s.Config.PartialReads, int(s.Config.MaxPacketSize))

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
