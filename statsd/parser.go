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

var Debug = true

type Parser struct {
	reader       io.Reader
	buffer       []byte
	partialReads bool
	maxReadSize  int
	done         bool
}

func NewParser(r io.Reader, partialReads bool, maxReadSize int) *Parser {
	return &Parser{
		reader:       r,
		buffer:       []byte{},
		partialReads: partialReads,
		maxReadSize:  maxReadSize,
	}
}

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
		if samp, err := strconv.ParseFloat(string(rawSample), 64); err != nil {
			return nil, fmt.Errorf("failed to ParseFloat %q: %s", string(rawSample), err)
		} else {
			sample = float32(samp)
		}

	}

	out := &agentmon.Measurement{
		Name:      string(name),
		Timestamp: time.Now(),
		Type:      stringToMetricType(string(measureType)),
		Value:     value,
		Sample:    sample,
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
