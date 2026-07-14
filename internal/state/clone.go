package state

func (s State) Clone() State {
	next := State{
		SchemaVersion:        s.SchemaVersion,
		FlowID:               s.FlowID,
		Status:               s.Status,
		CurrentStepID:        s.CurrentStepID,
		Finish:               cloneFinish(s.Finish),
		FlowRunID:            s.FlowRunID,
		CurrentEntrySequence: s.CurrentEntrySequence,
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
	if s.CheckResults != nil {
		next.CheckResults = make(map[string]CheckResult, len(s.CheckResults))
		for checkID, result := range s.CheckResults {
			next.CheckResults[checkID] = result
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
	if s.CheckResults == nil {
		s.CheckResults = map[string]CheckResult{}
	}
}

func cloneFinish(finish *Finish) *Finish {
	if finish == nil {
		return nil
	}
	next := *finish
	return &next
}
