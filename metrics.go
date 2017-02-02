package agentmon

import "time"

type Measurement struct {
	Name      string
	Timestamp time.Time
	Type      string
	Value     float64
	Sample    float32
	Modifier  string
}
