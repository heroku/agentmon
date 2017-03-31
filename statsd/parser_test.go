package statsd

import (
	"bytes"
	"fmt"
	"testing"

	am "github.com/heroku/agentmon"
)

func testParser(t *testing.T, parser *Parser, expected []*am.Measurement) {
	for _, exp := range expected {
		act, more := parser.Next()
		if act == nil && more {
			continue
		} else if act == nil {
			t.Fatalf("Expected another measurement. Found nil instead")
		}

		if exp.Name != act.Name {
			t.Errorf("Expected name=%q, got=%q", exp.Name, act.Name)
		}
		if exp.Value != act.Value {
			t.Errorf("Expected value=%v, got=%v", exp.Value, act.Value)
		}
		if exp.Type != act.Type {
			t.Errorf("Expected type=%q, got=%q", exp.Type, act.Type)
		}
	}

	if m, _ := parser.Next(); m != nil {
		t.Errorf("Found an additional measurement: %+v", m)
	}
}

func TestParserWithoutPartialReads(t *testing.T) {
	input := bytes.NewBuffer([]byte(`gorets:1|c
gorets:1|c|@0.1
gaugor:333|g
`))

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

	parser := NewParser(input, false, 100)

	testParser(t, parser, expected)
}

func TestParserWithPartialReads(t *testing.T) {
	input := bytes.NewBuffer([]byte(`gorets:1|c
gorets:1|c|@0.1
gaugor:333|g
`))

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

	parser := NewParser(input, true, 20)

	testParser(t, parser, expected)
}

func TestParse(t *testing.T) {
	testCases := map[string]map[string]*am.Measurement{
		"Counters": map[string]*am.Measurement{
			"gorets:1|c": &am.Measurement{
				Name:       "gorets",
				Value:      1.0,
				SampleRate: 1.0,
				Type:       am.Counter,
				Modifier:   "",
			},
			"gorets:1|c|@0.1": &am.Measurement{
				Name:       "gorets",
				Value:      1.0,
				SampleRate: 0.1,
				Type:       am.Counter,
				Modifier:   "",
			},
		},
		"Gauges": map[string]*am.Measurement{
			"gaugor:333|g": &am.Measurement{
				Name:       "gaugor",
				Value:      333,
				SampleRate: 1.0,
				Type:       am.Gauge,
				Modifier:   "",
			},
			"gaugor:+4.4|g": &am.Measurement{
				Name:       "gaugor",
				Value:      4.4,
				SampleRate: 1.0,
				Type:       am.Gauge,
				Modifier:   "+",
			},
			"gaugor:-14.2|g": &am.Measurement{
				Name:       "gaugor",
				Value:      14.2,
				SampleRate: 1.0,
				Type:       am.Gauge,
				Modifier:   "-",
			},
		},
		"Timers": map[string]*am.Measurement{
			"glork:320|ms|@0.1": &am.Measurement{
				Name:       "glork",
				Value:      320.0,
				SampleRate: 0.1,
				Type:       am.Timer,
				Modifier:   "",
			},
		},
	}

	for name, tests := range testCases {
		t.Run(fmt.Sprintf("Parse for %s", name), func(t *testing.T) {
			for input, exp := range tests {
				parser := &Parser{}
				out, err := parser.parseLine([]byte(input))
				if err != nil {
					t.Errorf("Got unexpected error for %q: %s", input, err)
					continue
				}

				if out.Name != exp.Name {
					t.Errorf("Expected name=%q, got %q", exp.Name, out.Name)
				}
				if out.Value != exp.Value {
					t.Errorf("Expected value=%f, got %f", exp.Value, out.Value)
				}
				if out.SampleRate != exp.SampleRate {
					t.Errorf("Expected sample=%f, got %f", exp.SampleRate, out.SampleRate)
				}
				if out.Type != exp.Type {
					t.Errorf("Expected type=%q, got %q", exp.Type, out.Type)
				}
				if out.Modifier != exp.Modifier {
					t.Errorf("Expected modifier=%q, got %q", exp.Modifier, out.Modifier)
				}
			}
		})
	}
}

func TestParseValue(t *testing.T) {
	for _, tc := range []struct {
		input    string
		expected string
		rest     string
		ok       bool
	}{
		{"", "", "", false},
		{".", "", ".", false},
		{"1", "1", "", true},
		// modifiers
		{"+1", "+1", "", true},
		{"-1", "-1", "", true},
		// modifiers with extra
		{"+1+", "+1", "+", true},
		{"-1-", "-1", "-", true},
		// only 1 dot
		{"1.1.1", "", "1.1.1", false},
		// leading dot
		{".1", ".1", "", true},
		// larger number
		{"101.1001929", "101.1001929", "", true},
		// extras
		{"1|c", "1", "|c", true},
		{"+1|c", "+1", "|c", true},
	} {
		out, rest, err := readValue([]byte(tc.input))
		if err == nil && !tc.ok {
			t.Errorf("Expected an error for %q, got none", tc.input)
		} else if err != nil && tc.ok {
			t.Errorf("Got an unexpected error for %q: %q", tc.input, err)
		}

		if string(out) != string(tc.expected) {
			t.Errorf("Expected value=%q, got=%q, in %q", string(tc.expected), string(out), string(tc.input))
		}

		if string(rest) != string(tc.rest) {
			t.Errorf("Expected rest=%q, got=%q, in %q", string(tc.rest), string(rest), tc.input)
		}
	}
}

func TestParseName(t *testing.T) {
	for _, tc := range []struct {
		input    string
		expected string
		rest     string
		ok       bool
	}{
		{"", "", "", false},
		{".", "", ".", false},
		{"foo", "foo", "", true},
		{"foo.", "foo.", "", true},
		{"foo.bar", "foo.bar", "", true},
		{"foo_", "foo_", "", true},
		{"foo_bar", "foo_bar", "", true},
		{"foo-", "foo-", "", true},
		{"foo-bar", "foo-bar", "", true},
		{"-foo", "-foo", "", true},
		{"_foo", "_foo", "", true},
		{".foo", ".foo", "", true},
	} {
		out, rest, err := readMetricName([]byte(tc.input))
		if err == nil && !tc.ok {
			t.Errorf("Expected an error for %q, got none", tc.input)
		} else if err != nil && tc.ok {
			t.Errorf("Got an unexpected error for %q: %q", tc.input, err)
		}

		if string(out) != string(tc.expected) {
			t.Errorf("Expected name=%q, got=%q, in %q", string(tc.expected), string(out), string(tc.input))
		}

		if string(rest) != string(tc.rest) {
			t.Errorf("Expected rest=%q, got=%q, in %q", string(tc.rest), string(rest), tc.input)
		}
	}
}

func TestMetricType(t *testing.T) {
	for _, tc := range []struct {
		input    string
		expected string
		rest     string
		ok       bool
	}{
		{"c", "c", "", true},
		{"g", "g", "", true},
		{"ms", "ms", "", true},
		{"c|", "c", "|", true},
		{"g|", "g", "|", true},
		{"ms|", "ms", "|", true},
		// bad
		{"m", "", "m", false},
		{"z", "", "z", false},
	} {
		out, rest, err := readType([]byte(tc.input))
		if err == nil && !tc.ok {
			t.Errorf("Expected an error for %q, got none", tc.input)
		} else if err != nil && tc.ok {
			t.Errorf("Got an unexpected error for %q: %q", tc.input, err)
		}

		if string(out) != string(tc.expected) {
			t.Errorf("Expected type=%q, got=%q, in %q", string(tc.expected), string(out), string(tc.input))
		}

		if string(rest) != string(tc.rest) {
			t.Errorf("Expected rest=%q, got=%q, in %q", string(tc.rest), string(rest), tc.input)
		}
	}
}

func TestSample(t *testing.T) {
	for _, tc := range []struct {
		input    string
		expected string
		rest     string
		ok       bool
	}{
		{"", "", "", true},
		{"|@1.0", "1.0", "", true},
		{"|@", "", "", false},
		// bad
		{"m", "", "m", false},
		{"z", "", "z", false},
	} {
		out, rest, err := maybeReadSample([]byte(tc.input))
		if err == nil && !tc.ok {
			t.Errorf("Expected an error for %q, got none", tc.input)
		} else if err != nil && tc.ok {
			t.Errorf("Got an unexpected error for %q: %q", tc.input, err)
		}

		if string(out) != string(tc.expected) {
			t.Errorf("Expected sample=%q, got=%q, in %q", string(tc.expected), string(out), string(tc.input))
		}

		if string(rest) != string(tc.rest) {
			t.Errorf("Expected rest=%q, got=%q, in %q", string(tc.rest), string(rest), tc.input)
		}
	}
}

func BenchmarkParse(b *testing.B) {
	parser := &Parser{}

	lines := []string{
		"gorets:1|c",
		"gorets:1|c|@0.1",
		"glork:320|ms|@0.1",
		"gaugor:333|g",
		"gaugor:+4.19910|g",
		"gaugor:-14.019910|g",
	}

	for i := 0; i < b.N; i++ {
		for _, l := range lines {
			parser.parseLine([]byte(l))
		}
	}
}
