package state

func (s State) Clone() State {
	next := State{
		FlowID:        s.FlowID,
		Status:        s.Status,
		CurrentStepID: s.CurrentStepID,
		Finish:        cloneFinish(s.Finish),
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
		next.BackHistory = append([]BackHistory(nil), s.BackHistory...)
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
