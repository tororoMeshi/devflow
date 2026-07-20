package state

import "github.com/8noki8/devflow/internal/flow"

func (s State) Clone() State {
	next := State{
		SchemaVersion:    s.SchemaVersion,
		FlowSnapshot:     flow.CloneSnapshot(s.FlowSnapshot),
		TaskSnapshot:     s.TaskSnapshot,
		Status:           s.Status,
		CurrentStepID:    s.CurrentStepID,
		Finish:           cloneFinish(s.Finish),
		FlowRunID:        s.FlowRunID,
		CurrentAttemptID: s.CurrentAttemptID,
	}

	if s.CompletedSteps != nil {
		next.CompletedSteps = append([]string(nil), s.CompletedSteps...)
	}
	if s.SkippedSteps != nil {
		next.SkippedSteps = make(map[string]SkippedStep, len(s.SkippedSteps))
		for stepID, skipped := range s.SkippedSteps {
			next.SkippedSteps[stepID] = skipped
		}
	}
	if s.Approvals != nil {
		next.Approvals = make(map[string]ApprovalRecord, len(s.Approvals))
		for stepID, approval := range s.Approvals {
			next.Approvals[stepID] = approval
		}
	}
	if s.BackHistory != nil {
		next.BackHistory = make([]BackHistory, len(s.BackHistory))
		for i, history := range s.BackHistory {
			next.BackHistory[i] = history
			if history.InvalidatedStepIDs != nil {
				next.BackHistory[i].InvalidatedStepIDs = append([]string(nil), history.InvalidatedStepIDs...)
			}
		}
	}
	if s.Attempts != nil {
		next.Attempts = make([]StepAttempt, len(s.Attempts))
		for i, attempt := range s.Attempts {
			next.Attempts[i] = attempt
			next.Attempts[i].CheckResults = cloneStepAttemptCheckResults(attempt.CheckResults)
		}
	}

	next.Normalize()
	return next
}

func (s *State) Normalize() {
	if s == nil {
		return
	}
	if s.CompletedSteps == nil {
		s.CompletedSteps = []string{}
	}
	if s.SkippedSteps == nil {
		s.SkippedSteps = map[string]SkippedStep{}
	}
	if s.Approvals == nil {
		s.Approvals = map[string]ApprovalRecord{}
	}
	if s.BackHistory == nil {
		s.BackHistory = []BackHistory{}
	}
}

func cloneFinish(finish *Finish) *Finish {
	if finish == nil {
		return nil
	}
	next := *finish
	return &next
}
