package agentmon

type Measurement interface {
	Name     string
	Type     string
	Value    float32
	Sample   float32
	Modifier string
}
