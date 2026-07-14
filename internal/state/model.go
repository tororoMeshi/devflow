package state

type Status string

const (
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
	StatusFinished  Status = "finished"
)

type State struct {
	FlowID         string                    `json:"flow_id"`
	Status         Status                    `json:"status"`
	CurrentStepID  string                    `json:"current_step_id"`
	CompletedSteps []string                  `json:"completed_steps"`
	SkippedSteps   map[string]SkippedStep    `json:"skipped_steps"`
	Approvals      map[string]ApprovalRecord `json:"approvals"`
	BackHistory    []BackHistory             `json:"back_history"`
	Finish         *Finish                   `json:"finish"`
}

type SkippedStep struct {
	Reason string `json:"reason"`
}

type ApprovalRecord struct {
	Approved bool   `json:"approved"`
	Note     string `json:"note"`
}

type BackHistory struct {
	FromStepID         string   `json:"from_step_id"`
	ToStepID           string   `json:"to_step_id"`
	Reason             string   `json:"reason"`
	InvalidatedStepIDs []string `json:"invalidated_step_ids,omitempty"`
}

type Finish struct {
	Reason string `json:"reason"`
}
