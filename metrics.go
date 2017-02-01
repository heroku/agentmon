package agentmon

type Measurement struct {
	Name     string
	Type     string
	Value    float32
	Sample   float32
	Modifier string
}
