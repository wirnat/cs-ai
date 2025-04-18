package cs_ai

type Modeler interface {
	ModelName() string
	ApiURL() string
	Train() []string
}
