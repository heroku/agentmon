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
// ~~~
// Portions of this file were inspired by, or directly taken from
// https://github.com/bitly/statsdaemon, which is public domain software.

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
