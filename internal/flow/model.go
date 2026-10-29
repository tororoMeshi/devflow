package flow

type Flow struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Purpose string `json:"purpose"` // Why the Flow exists. Required.
	Steps   []Step `json:"steps"`
}

type Step struct {
	ID             string     `json:"id"`
	Title          string     `json:"title"`
	Purpose        string     `json:"purpose"` // Why this Step exists independently. Optional.
	Outcome        string     `json:"outcome"` // The state that must hold when this Step is complete. Required.
	Inputs         []Artifact `json:"inputs"`
	Artifacts      []Artifact `json:"artifacts"`
	Approval       *Approval  `json:"approval"`
	RequiredChecks []string   `json:"required_checks"`
}

type Artifact struct {
	Path     string `json:"path"`
	Required bool   `json:"required"`
}

type Approval struct {
	Required bool `json:"required"`
}
