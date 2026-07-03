package flow

type Flow struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Steps       []Step `json:"steps"`
}

type Step struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Instruction string     `json:"instruction"`
	Artifacts   []Artifact `json:"artifacts"`
	Approval    *Approval  `json:"approval"`
}

type Artifact struct {
	Path     string `json:"path"`
	Required bool   `json:"required"`
}

type Approval struct {
	Required bool `json:"required"`
}
