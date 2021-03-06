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
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"strconv"
	"time"

	"github.com/heroku/agentmon"
)

// Debug is likely something that can be eliminated
// TODO(apg)
var Debug = true

// Parser contains the state necessary to parse statsd protocol messages from
// an arbitrary reader.
type Parser struct {
	reader       io.Reader
	buffer       []byte
	partialReads bool
	maxReadSize  int
	done         bool
}

// NewParser constructs a statsd parser.
//
// When partialReads is true, it is expected that a read (of
// maxReadSize) on the Reader may not produce a complete statsd
// message. On the next successful read, the partially read message
// will attempt to be completed.
func NewParser(r io.Reader, partialReads bool, maxReadSize int) *Parser {
	return &Parser{
		reader:       r,
		buffer:       []byte{},
		partialReads: partialReads,
		maxReadSize:  maxReadSize,
	}
}

// Next returns the next measurement parsed from the parser's Reader.
func (p *Parser) Next() (*agentmon.Measurement, bool) {
	buf := p.buffer

	for {
		line, rest := p.lineFrom(buf)

		if line != nil {
			p.buffer = rest
			m, _ := p.parseLine(line)
			return m, true
		}

		if p.done {
			m, _ := p.parseLine(rest)
			return m, false
		}

		idx := len(buf)
		end := idx
		end += p.maxReadSize

		if cap(buf) >= end {
			buf = buf[:end]
		} else {
			tmp := buf
			buf = make([]byte, end)
			copy(buf, tmp)
		}

		n, err := p.reader.Read(buf[idx:])
		buf = buf[:idx+n]
		if err != nil {
			if err != io.EOF {
				log.Printf("next: read: %s", err)
			}

			p.done = true

			line, rest = p.lineFrom(buf)
			if line != nil {
				p.buffer = rest
				m, _ := p.parseLine(line)
				return m, len(rest) > 0
			}

			if len(rest) > 0 {
				m, _ := p.parseLine(line)
				return m, false
			}

			return nil, false
		}
	}
}

func (p *Parser) lineFrom(input []byte) ([]byte, []byte) {
	split := bytes.SplitAfterN(input, []byte("\n"), 2)
	if len(split) == 2 {
		return split[0][:len(split[0])-1], split[1]
	}

	if !p.partialReads {
		if len(input) == 0 {
			input = nil
		}
		return input, []byte{}
	}

	if bytes.HasSuffix(input, []byte("\n")) {
		return input[:len(input)-1], []byte{}
	}

	return nil, input
}

func (p *Parser) parseLine(line []byte) (*agentmon.Measurement, error) {
	// metric name is [a-zA-Z0-9._-]+
	name, rest, err := readMetricName(line)
	if err != nil {
		return nil, fmt.Errorf("failed to read a name from %q: %s", string(line), err)
	}

	rest, ok := expect(rest, []byte(":"))
	if !ok {
		return nil, fmt.Errorf("expected ':' in %q", string(rest))
	}

	rawValue, rest, err := readValue(rest)
	if err != nil {
		return nil, fmt.Errorf("failed to read a value from %q: %s", string(rest), err)
	}

	rest, ok = expect(rest, []byte("|"))
	if !ok {
		return nil, fmt.Errorf("expected '|' in %q", string(rest))
	}

	measureType, rest, err := readType(rest)
	if err != nil {
		return nil, fmt.Errorf("failed to read type from %q: %s", string(rest), err)
	}

	rawSample, rest, err := maybeReadSample(rest)
	if err != nil {
		return nil, fmt.Errorf("failed to read sample from %q: %s", string(rest), err)
	}

	if len(rest) != 0 || (len(rest) > 0 && rest[0] != '\n') {
		return nil, fmt.Errorf("unexpected leftover (%d) %q", len(rest), rest)
	}

	var (
		sign   byte
		value  float64
		sample = float32(1.0)
	)

	// TODO: Now we've gotta do some fun stuff in regards to value checking.
	// We might get a `g` which would make +/- OK. Not OK, in other types.
	switch string(measureType) {
	case "c", "ms":
		value, err = strconv.ParseFloat(string(rawValue), 64)
		if err != nil {
			return nil, fmt.Errorf("failed to ParseFloat %q: %s", string(rawValue), err)
		}

	case "g":
		if rawValue[0] == '-' || rawValue[0] == '+' {
			sign = rawValue[0]
			rawValue = rawValue[1:]
		}
		value, err = strconv.ParseFloat(string(rawValue), 64)
		if err != nil {
			return nil, fmt.Errorf("failed to ParseFloat %q: %s", string(rawValue), err)
		}

	default:
		return nil, fmt.Errorf("unrecognized type: %q", string(measureType))
	}

	if len(rawSample) > 0 {
		samp, err := strconv.ParseFloat(string(rawSample), 64)
		if err != nil {
			return nil, fmt.Errorf("failed to ParseFloat %q: %s", string(rawSample), err)
		}
		sample = float32(samp)
	}

	out := &agentmon.Measurement{
		Name:       string(name),
		Timestamp:  time.Now(),
		Type:       stringToMetricType(string(measureType)),
		Value:      value,
		SampleRate: sample,
	}

	if sign > 0 {
		out.Modifier = string([]byte{sign})
	}

	return out, nil
}

func stringToMetricType(s string) agentmon.MetricType {
	switch s {
	case "c":
		return agentmon.Counter
	case "ms":
		return agentmon.Timer
	default:
		return agentmon.Gauge
	}
}

func readMetricName(buf []byte) ([]byte, []byte, error) {
	i := 0

loop:
	for ; i < len(buf); i++ {
		c := buf[i]
		switch {
		case c >= 'a' && c <= 'z':
		case c >= 'A' && c <= 'Z':
		case c >= '0' && c <= '9':
		case c == '.' || c == '_' || c == '-':
		default:
			break loop
		}
	}
	if i > 1 {
		return buf[0:i], buf[i:], nil
	}
	return []byte{}, buf, errors.New("empty metric name")
}

func readType(buf []byte) ([]byte, []byte, error) {
	if len(buf) == 0 {
		return []byte{}, []byte{}, errors.New("unexpected end of line")
	}

	i := 0
	switch buf[i] {
	case 'c', 'g':
		return buf[0:1], buf[1:], nil
	case 'm':
		if len(buf) > 1 && buf[1] == 's' {
			return buf[0:2], buf[2:], nil
		}
	}

	return []byte{}, buf, errors.New("unexpected type")
}

func readValue(buf []byte) ([]byte, []byte, error) {
	var (
		sawdot bool
		i      int
	)

	if len(buf) > 1 && (buf[0] == '+' || buf[0] == '-') {
		i++
	}

loop:
	for ; i < len(buf); i++ {
		c := buf[i]
		switch {
		case c >= '0' && c <= '9':
		case c == '.':
			if sawdot {
				return []byte{}, buf, errors.New("unexpected '.'")
			}
			sawdot = true
		default:
			break loop
		}
	}

	// handle special '.' only case.
	if i == 1 && buf[0] == '.' {
		return []byte{}, buf, errors.New("invalid value")
	}

	if i > 0 {
		return buf[0:i], buf[i:], nil
	}

	return []byte{}, []byte{}, errors.New("empty value")
}

func maybeReadSample(buf []byte) ([]byte, []byte, error) {
	if len(buf) == 0 {
		return []byte{}, buf, nil
	}

	rest, ok := expect(buf, []byte{'|', '@'})
	if !ok {
		return []byte{}, buf, errors.New("unexpected characters")
	}

	return readValue(rest)
}

func expect(buf []byte, xs []byte) ([]byte, bool) {
	if len(buf) < len(xs) {
		return []byte{}, false
	}

	i := 0
	for ; i < len(xs); i++ {
		if buf[i] != xs[i] {
			return []byte{}, false
		}
	}
	return buf[i:], true
}

func logParseFail(line []byte) {
	if Debug {
		log.Printf("ERROR: failed to parse line: %q\n", string(line))
	}
}
