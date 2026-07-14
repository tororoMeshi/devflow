package state

import "regexp"

const CurrentSchemaVersion = 2

var flowRunIDPattern = regexp.MustCompile(`^run_[0-9a-f]{32}$`)

type Status string

const (
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
	StatusFinished  Status = "finished"
)

type State struct {
	SchemaVersion        int                       `json:"schema_version"`
	FlowID               string                    `json:"flow_id"`
	Status               Status                    `json:"status"`
	CurrentStepID        string                    `json:"current_step_id"`
	CompletedSteps       []string                  `json:"completed_steps"`
	SkippedSteps         map[string]SkippedStep    `json:"skipped_steps"`
	Approvals            map[string]ApprovalRecord `json:"approvals"`
	BackHistory          []BackHistory             `json:"back_history"`
	Finish               *Finish                   `json:"finish"`
	FlowRunID            string                    `json:"flow_run_id,omitempty"`
	CurrentEntrySequence uint64                    `json:"current_entry_sequence,omitempty"`
	CheckResults         map[string]CheckResult    `json:"check_results,omitempty"`
}

func IsValidFlowRunID(value string) bool { return flowRunIDPattern.MatchString(value) }

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

type CheckResult struct {
	EntrySequence uint64 `json:"entry_sequence"`
	ExitCode      int    `json:"exit_code"`
	LogPath       string `json:"log_path,omitempty"`
}
