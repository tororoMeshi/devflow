package state

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrInvalidStepAttemptEntrySequence = errors.New("invalid step attempt entry sequence")
	ErrInvalidStepAttemptID            = errors.New("invalid step attempt id")
	ErrInvalidStepAttemptStepID        = errors.New("invalid step attempt step id")
	ErrInvalidStepAttemptStatus        = errors.New("invalid step attempt status")
	ErrInvalidStepAttemptExitReason    = errors.New("invalid step attempt exit reason")
	ErrInvalidStepAttemptReason        = errors.New("invalid step attempt reason")
	ErrNilStepAttemptCheckResults      = errors.New("nil step attempt check results")
	ErrStepAttemptNotActive            = errors.New("step attempt is not active")
	ErrInvalidApprovalNote             = errors.New("invalid approval note")
	ErrStepAttemptAlreadyApproved      = errors.New("step attempt already approved")
)

type StepAttemptStatus string

const (
	StepAttemptActive StepAttemptStatus = "active"
	StepAttemptClosed StepAttemptStatus = "closed"
)

type StepAttemptExitReason string

const (
	StepAttemptExitDone   StepAttemptExitReason = "done"
	StepAttemptExitSkip   StepAttemptExitReason = "skip"
	StepAttemptExitBack   StepAttemptExitReason = "back"
	StepAttemptExitFinish StepAttemptExitReason = "finish"
)

type StepAttempt struct {
	ID            string                 `json:"id"`
	StepID        string                 `json:"step_id"`
	EntrySequence uint64                 `json:"entry_sequence"`
	Status        StepAttemptStatus      `json:"status"`
	ExitReason    StepAttemptExitReason  `json:"exit_reason,omitempty"`
	Reason        string                 `json:"reason,omitempty"`
	CheckResults  map[string]CheckResult `json:"check_results"`
	Approval      *ApprovalRecord        `json:"approval,omitempty"`
}

type ApprovalRecord struct {
	Note string `json:"note"`
}

func IsValidStepAttemptID(value string) bool {
	if len(value) != len("attempt_")+20 || !strings.HasPrefix(value, "attempt_") {
		return false
	}
	for _, character := range value[len("attempt_"):] {
		if character < '0' || character > '9' {
			return false
		}
	}
	return value != "attempt_00000000000000000000"
}

func StepAttemptID(entrySequence uint64) (string, error) {
	if entrySequence == 0 {
		return "", ErrInvalidStepAttemptEntrySequence
	}

	return fmt.Sprintf("attempt_%020d", entrySequence), nil
}

func NewStepAttempt(stepID string, entrySequence uint64) (StepAttempt, error) {
	if strings.TrimSpace(stepID) == "" {
		return StepAttempt{}, ErrInvalidStepAttemptStepID
	}

	id, err := StepAttemptID(entrySequence)
	if err != nil {
		return StepAttempt{}, err
	}

	return StepAttempt{
		ID:            id,
		StepID:        stepID,
		EntrySequence: entrySequence,
		Status:        StepAttemptActive,
		CheckResults:  map[string]CheckResult{},
	}, nil
}

func ValidateStepAttempt(attempt StepAttempt) error {
	if attempt.ID == "" {
		return ErrInvalidStepAttemptID
	}
	if strings.TrimSpace(attempt.StepID) == "" {
		return ErrInvalidStepAttemptStepID
	}

	expectedID, err := StepAttemptID(attempt.EntrySequence)
	if err != nil {
		return err
	}
	if attempt.ID != expectedID {
		return ErrInvalidStepAttemptID
	}
	if attempt.CheckResults == nil {
		return ErrNilStepAttemptCheckResults
	}
	if attempt.Approval != nil && strings.TrimSpace(attempt.Approval.Note) == "" {
		return ErrInvalidApprovalNote
	}

	switch attempt.Status {
	case StepAttemptActive:
		if attempt.ExitReason != "" {
			return ErrInvalidStepAttemptExitReason
		}
		if attempt.Reason != "" {
			return ErrInvalidStepAttemptReason
		}
		return nil
	case StepAttemptClosed:
		return validateStepAttemptClosure(attempt.ExitReason, attempt.Reason)
	default:
		return ErrInvalidStepAttemptStatus
	}
}

func CloseStepAttempt(attempt StepAttempt, exitReason StepAttemptExitReason, reason string) (StepAttempt, error) {
	if err := ValidateStepAttempt(attempt); err != nil {
		return StepAttempt{}, err
	}
	if attempt.Status != StepAttemptActive {
		return StepAttempt{}, ErrStepAttemptNotActive
	}
	if err := validateStepAttemptClosure(exitReason, reason); err != nil {
		return StepAttempt{}, err
	}

	closed := attempt
	closed.CheckResults = cloneStepAttemptCheckResults(attempt.CheckResults)
	closed.Approval = cloneApprovalRecord(attempt.Approval)
	closed.Status = StepAttemptClosed
	closed.ExitReason = exitReason
	closed.Reason = reason
	if err := ValidateStepAttempt(closed); err != nil {
		return StepAttempt{}, err
	}

	return closed, nil
}

func ApproveStepAttempt(attempt StepAttempt, note string) (StepAttempt, error) {
	if err := ValidateStepAttempt(attempt); err != nil {
		return StepAttempt{}, err
	}
	if attempt.Status != StepAttemptActive {
		return StepAttempt{}, ErrStepAttemptNotActive
	}
	if attempt.Approval != nil {
		return StepAttempt{}, ErrStepAttemptAlreadyApproved
	}
	if strings.TrimSpace(note) == "" {
		return StepAttempt{}, ErrInvalidApprovalNote
	}
	approved := attempt
	approved.CheckResults = cloneStepAttemptCheckResults(attempt.CheckResults)
	approved.Approval = &ApprovalRecord{Note: note}
	if err := ValidateStepAttempt(approved); err != nil {
		return StepAttempt{}, err
	}
	return approved, nil
}

func validateStepAttemptClosure(exitReason StepAttemptExitReason, reason string) error {
	if !isValidStepAttemptExitReason(exitReason) {
		return ErrInvalidStepAttemptExitReason
	}
	if exitReason == StepAttemptExitDone {
		if reason != "" {
			return ErrInvalidStepAttemptReason
		}
		return nil
	}
	if strings.TrimSpace(reason) == "" {
		return ErrInvalidStepAttemptReason
	}
	return nil
}

func isValidStepAttemptExitReason(reason StepAttemptExitReason) bool {
	switch reason {
	case StepAttemptExitDone, StepAttemptExitSkip, StepAttemptExitBack, StepAttemptExitFinish:
		return true
	default:
		return false
	}
}

func cloneStepAttemptCheckResults(source map[string]CheckResult) map[string]CheckResult {
	if source == nil {
		return nil
	}
	cloned := make(map[string]CheckResult, len(source))
	for name, result := range source {
		cloned[name] = result
	}
	return cloned
}

func cloneApprovalRecord(source *ApprovalRecord) *ApprovalRecord {
	if source == nil {
		return nil
	}
	cloned := *source
	return &cloned
}
