package state

import (
	"regexp"

	"github.com/8noki8/devflow/internal/flow"
	"github.com/8noki8/devflow/internal/task"
)

const CurrentSchemaVersion = 5

var flowRunIDPattern = regexp.MustCompile(`^run_[0-9a-f]{32}$`)

type Status string

const (
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
	StatusFinished  Status = "finished"
)

type State struct {
	SchemaVersion    int                       `json:"schema_version"`
	FlowSnapshot     flow.FlowSnapshot         `json:"flow_snapshot"`
	TaskSnapshot     task.TaskSnapshot         `json:"task_snapshot"`
	Status           Status                    `json:"status"`
	CurrentStepID    string                    `json:"current_step_id"`
	CompletedSteps   []string                  `json:"completed_steps"`
	SkippedSteps     map[string]SkippedStep    `json:"skipped_steps"`
	Approvals        map[string]ApprovalRecord `json:"approvals"`
	BackHistory      []BackHistory             `json:"back_history"`
	Finish           *Finish                   `json:"finish"`
	FlowRunID        string                    `json:"flow_run_id,omitempty"`
	Attempts         []StepAttempt             `json:"attempts"`
	CurrentAttemptID string                    `json:"current_attempt_id,omitempty"`
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
	ExitCode int    `json:"exit_code"`
	LogPath  string `json:"log_path,omitempty"`
}

func (s State) CurrentAttempt() (StepAttempt, int, bool) {
	if s.CurrentAttemptID == "" {
		return StepAttempt{}, -1, false
	}
	for i := range s.Attempts {
		if s.Attempts[i].ID == s.CurrentAttemptID {
			attempt := s.Attempts[i]
			attempt.CheckResults = cloneStepAttemptCheckResults(attempt.CheckResults)
			return attempt, i, true
		}
	}
	return StepAttempt{}, -1, false
}

func (s State) LastAttempt() (StepAttempt, bool) {
	if len(s.Attempts) == 0 {
		return StepAttempt{}, false
	}
	attempt := s.Attempts[len(s.Attempts)-1]
	attempt.CheckResults = cloneStepAttemptCheckResults(attempt.CheckResults)
	return attempt, true
}

func (s State) EntrySequence() uint64 {
	if attempt, _, ok := s.CurrentAttempt(); ok {
		return attempt.EntrySequence
	}
	if attempt, ok := s.LastAttempt(); ok {
		return attempt.EntrySequence
	}
	return 0
}
