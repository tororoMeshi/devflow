package workpackage

import (
	"fmt"

	"github.com/tororoMeshi/devflow/internal/flow"
	"github.com/tororoMeshi/devflow/internal/state"
)

// Generate creates an immutable projection from the snapshots and current
// active Attempt held by st. It performs no I/O and does not mutate st.
func Generate(st state.State, stepID, attemptID string) (WorkPackage, error) {
	if err := state.Validate(st); err != nil {
		return WorkPackage{}, fmt.Errorf("%w: %v", ErrInvalidState, err)
	}
	if st.Status != state.StatusRunning {
		return WorkPackage{}, ErrNoActiveFlow
	}
	if !state.IsValidStepAttemptID(attemptID) {
		return WorkPackage{}, ErrInvalidAttemptID
	}

	found := false
	for _, attempt := range st.Attempts {
		if attempt.ID == attemptID {
			found = true
			break
		}
	}
	if !found {
		return WorkPackage{}, ErrAttemptNotFound
	}

	attempt, _, ok := st.CurrentAttempt()
	if !ok {
		return WorkPackage{}, ErrInactiveAttempt
	}
	if attempt.ID != attemptID {
		return WorkPackage{}, ErrStaleAttempt
	}
	if attempt.Status != state.StepAttemptActive {
		return WorkPackage{}, ErrInactiveAttempt
	}
	if attempt.StepID != stepID || st.CurrentStepID != stepID {
		return WorkPackage{}, ErrStepAttemptMismatch
	}

	step, ok := findStep(st.FlowSnapshot.Flow, stepID)
	if !ok {
		return WorkPackage{}, fmt.Errorf("%w: Step absent from FlowSnapshot", ErrInvalidState)
	}
	contract := projectStep(step)
	pkg := WorkPackage{
		SchemaVersion:      SchemaVersion,
		FlowRunID:          st.FlowRunID,
		StepID:             stepID,
		AttemptID:          attemptID,
		EntrySequence:      attempt.EntrySequence,
		FlowSnapshotDigest: st.FlowSnapshot.Digest,
		TaskSnapshotDigest: st.TaskSnapshot.Digest,
		TaskContent:        st.TaskSnapshot.Content,
		WorkingRoot:        WorkingRoot,
		Step:               contract,
	}
	digest, err := Digest(pkg)
	if err != nil {
		return WorkPackage{}, err
	}
	pkg.WorkPackageDigest = digest
	if err := Validate(pkg); err != nil {
		return WorkPackage{}, fmt.Errorf("%w: %v", ErrInvalidWorkPackage, err)
	}
	return pkg, nil
}

func findStep(snapshot flow.Flow, stepID string) (flow.Step, bool) {
	for _, step := range snapshot.Steps {
		if step.ID == stepID {
			return step, true
		}
	}
	return flow.Step{}, false
}

func projectStep(step flow.Step) StepContract {
	inputs := make([]ArtifactContract, len(step.Inputs))
	for i, input := range step.Inputs {
		inputs[i] = ArtifactContract{Path: input.Path, Required: input.Required}
	}
	artifacts := make([]ArtifactContract, len(step.Artifacts))
	for i, artifact := range step.Artifacts {
		artifacts[i] = ArtifactContract{Path: artifact.Path, Required: artifact.Required}
	}
	approvalRequired := step.Approval != nil && step.Approval.Required
	return StepContract{
		Title:            step.Title,
		Objective:        step.Objective,
		Inputs:           inputs,
		Artifacts:        artifacts,
		RequiredChecks:   append([]string{}, step.RequiredChecks...),
		ApprovalRequired: approvalRequired,
	}
}
