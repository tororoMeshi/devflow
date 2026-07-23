package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/8noki8/devflow/internal/flow"
	"github.com/8noki8/devflow/internal/task"
)

const CurrentPointerSchemaVersion = 1

type Store struct {
	Root     string
	saveJSON func(string, any) error
}

type CurrentPointer struct {
	SchemaVersion int    `json:"schema_version"`
	FlowRunID     string `json:"flow_run_id"`
}

type LoadStatus string

const (
	LoadNoState LoadStatus = "no_state"
	LoadOK      LoadStatus = "ok"
	LoadInvalid LoadStatus = "invalid_state"
)

type LoadResult struct {
	Status LoadStatus
	State  *State
	Err    error
}

type UnsupportedSchemaVersionError struct{ Actual int }

func (e *UnsupportedSchemaVersionError) Error() string {
	return fmt.Sprintf("unsupported state schema version %d (supported: %d)", e.Actual, CurrentSchemaVersion)
}

type UnsupportedCurrentPointerSchemaVersionError struct{ Actual int }

func (e *UnsupportedCurrentPointerSchemaVersionError) Error() string {
	return fmt.Sprintf("unsupported current pointer schema version %d (supported: %d)", e.Actual, CurrentPointerSchemaVersion)
}

type LegacyStateError struct{ Path string }

func (e *LegacyStateError) Error() string {
	return fmt.Sprintf("legacy state file exists at %s; this version does not migrate it automatically", e.Path)
}

func (s Store) CurrentPath() string { return filepath.Join(s.Root, "current.json") }
func (s Store) RunsDir() string     { return filepath.Join(s.Root, "runs") }
func (s Store) LegacyPath() string  { return filepath.Join(s.Root, "state.json") }

func (s Store) RunStatePath(flowRunID string) (string, error) {
	if !IsValidFlowRunID(flowRunID) {
		return "", errors.New("invalid flow_run_id")
	}
	return filepath.Join(s.RunsDir(), flowRunID, "state.json"), nil
}

func (s Store) LoadCurrent() LoadResult {
	pointer, err := s.loadCurrentPointer()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if _, legacyErr := os.Stat(s.LegacyPath()); legacyErr == nil {
				return LoadResult{Status: LoadInvalid, Err: &LegacyStateError{Path: s.LegacyPath()}}
			} else if !errors.Is(legacyErr, os.ErrNotExist) {
				return LoadResult{Status: LoadInvalid, Err: legacyErr}
			}
			return LoadResult{Status: LoadNoState}
		}
		return LoadResult{Status: LoadInvalid, Err: err}
	}
	loaded := s.LoadRun(pointer.FlowRunID)
	if loaded.Status == LoadNoState {
		return LoadResult{Status: LoadInvalid, Err: fmt.Errorf("current Run state does not exist for %s", pointer.FlowRunID)}
	}
	if loaded.Status == LoadOK && loaded.State.FlowRunID != pointer.FlowRunID {
		return LoadResult{Status: LoadInvalid, Err: fmt.Errorf("current pointer flow_run_id %q does not match State flow_run_id %q", pointer.FlowRunID, loaded.State.FlowRunID)}
	}
	return loaded
}

func (s Store) LoadRun(flowRunID string) LoadResult {
	path, err := s.RunStatePath(flowRunID)
	if err != nil {
		return LoadResult{Status: LoadInvalid, Err: err}
	}
	return loadState(path)
}

func (s Store) CreateRun(value State) error {
	if err := validateState(value); err != nil {
		return err
	}
	next := value.Clone()
	statePath, err := s.RunStatePath(next.FlowRunID)
	if err != nil {
		return err
	}
	runDir := filepath.Dir(statePath)
	if err := os.MkdirAll(s.RunsDir(), 0o755); err != nil {
		return err
	}
	if err := os.Mkdir(runDir, 0o755); err != nil {
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("Run %s already exists", next.FlowRunID)
		}
		return err
	}
	if err := s.saveJSONFile(statePath, next); err != nil {
		return err
	}
	pointer := CurrentPointer{SchemaVersion: CurrentPointerSchemaVersion, FlowRunID: next.FlowRunID}
	return s.saveJSONFile(s.CurrentPath(), pointer)
}

func (s Store) SaveCurrent(value State) error {
	pointer, err := s.loadCurrentPointer()
	if err != nil {
		return err
	}
	if value.FlowRunID != pointer.FlowRunID {
		return fmt.Errorf("cannot save flow_run_id %q to current Run %q", value.FlowRunID, pointer.FlowRunID)
	}
	loaded := s.LoadRun(pointer.FlowRunID)
	if loaded.Status != LoadOK {
		if loaded.Status == LoadNoState {
			return fmt.Errorf("current Run state does not exist for %s", pointer.FlowRunID)
		}
		return loaded.Err
	}
	if loaded.State.FlowRunID != pointer.FlowRunID {
		return fmt.Errorf("current pointer flow_run_id %q does not match State flow_run_id %q", pointer.FlowRunID, loaded.State.FlowRunID)
	}
	if err := validateState(value); err != nil {
		return err
	}
	next := value.Clone()
	path, _ := s.RunStatePath(pointer.FlowRunID)
	return s.saveJSONFile(path, next)
}

func (s Store) saveJSONFile(path string, value any) error {
	if s.saveJSON != nil {
		return s.saveJSON(path, value)
	}
	return atomicSaveJSON(path, value)
}

func (s Store) loadCurrentPointer() (CurrentPointer, error) {
	data, err := os.ReadFile(s.CurrentPath())
	if err != nil {
		return CurrentPointer{}, err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return CurrentPointer{}, fmt.Errorf("invalid current pointer JSON: %w", err)
	}
	var pointer CurrentPointer
	if err := json.Unmarshal(data, &pointer); err != nil {
		return CurrentPointer{}, fmt.Errorf("invalid current pointer: %w", err)
	}
	if _, ok := raw["schema_version"]; !ok || pointer.SchemaVersion != CurrentPointerSchemaVersion {
		return CurrentPointer{}, &UnsupportedCurrentPointerSchemaVersionError{Actual: pointer.SchemaVersion}
	}
	if _, ok := raw["flow_run_id"]; !ok || !IsValidFlowRunID(pointer.FlowRunID) {
		return CurrentPointer{}, errors.New("invalid current pointer flow_run_id")
	}
	return pointer, nil
}

func (s Store) LoadCurrentPointer() (CurrentPointer, error) {
	return s.loadCurrentPointer()
}

func loadState(path string) LoadResult {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return LoadResult{Status: LoadNoState}
		}
		return LoadResult{Status: LoadInvalid, Err: err}
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return LoadResult{Status: LoadInvalid, Err: err}
	}
	var value State
	if err := json.Unmarshal(data, &value); err != nil {
		return LoadResult{Status: LoadInvalid, Err: err}
	}
	if err := validateStateFile(raw, value); err != nil {
		return LoadResult{Status: LoadInvalid, Err: err}
	}
	value.Normalize()
	return LoadResult{Status: LoadOK, State: &value}
}

func atomicSaveJSON(path string, value any) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	renamed := false
	defer func() {
		if !renamed {
			_ = os.Remove(tmpPath)
		}
	}()
	encoder := json.NewEncoder(tmp)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	renamed = true
	return nil
}

func validateStateFile(raw map[string]json.RawMessage, state State) error {
	if _, ok := raw["schema_version"]; !ok {
		return &UnsupportedSchemaVersionError{}
	}
	if state.SchemaVersion != CurrentSchemaVersion {
		return &UnsupportedSchemaVersionError{Actual: state.SchemaVersion}
	}
	if _, ok := raw["approvals"]; ok {
		return errors.New("unsupported top-level field \"approvals\"")
	}
	if attemptsJSON, ok := raw["attempts"]; ok {
		var attempts []map[string]json.RawMessage
		if err := json.Unmarshal(attemptsJSON, &attempts); err != nil {
			return fmt.Errorf("invalid attempts: %w", err)
		}
		for index, attempt := range attempts {
			evidenceJSON, ok := attempt["artifact_evidence"]
			if !ok {
				return fmt.Errorf("missing required field \"artifact_evidence\" in attempt %d", index)
			}
			if string(evidenceJSON) == "null" {
				return fmt.Errorf("artifact_evidence must not be null in attempt %d", index)
			}
			approvalJSON, ok := attempt["approval"]
			if !ok {
				continue
			}
			if string(approvalJSON) == "null" {
				return fmt.Errorf("approval must be omitted rather than null in attempt %d", index)
			}
			var approval map[string]json.RawMessage
			if err := json.Unmarshal(approvalJSON, &approval); err != nil {
				return fmt.Errorf("invalid attempt %d approval: %w", index, err)
			}
			if _, ok := approval["approved"]; ok {
				return fmt.Errorf("unsupported field \"approved\" in attempt %d approval", index)
			}
			digestJSON, ok := approval["evidence_set_digest"]
			if !ok {
				return fmt.Errorf("missing required field \"evidence_set_digest\" in attempt %d approval", index)
			}
			if string(digestJSON) == "null" {
				return fmt.Errorf("evidence_set_digest must not be null in attempt %d approval", index)
			}
			var digest string
			if err := json.Unmarshal(digestJSON, &digest); err != nil {
				return fmt.Errorf("evidence_set_digest must be a string in attempt %d approval: %w", index, err)
			}
			if !isValidEvidenceSetDigest(digest) {
				return fmt.Errorf("invalid attempt %d approval evidence_set_digest: %w", index, ErrInvalidEvidenceSetDigest)
			}
		}
	}
	for _, field := range []string{"schema_version", "flow_snapshot", "task_snapshot", "status", "current_step_id", "attempts"} {
		if _, ok := raw[field]; !ok {
			return fmt.Errorf("missing required field %q", field)
		}
	}
	return validateState(state)
}

func validateState(state State) error {
	if state.SchemaVersion != CurrentSchemaVersion {
		return &UnsupportedSchemaVersionError{Actual: state.SchemaVersion}
	}
	if err := flow.ValidateSnapshot(state.FlowSnapshot); err != nil {
		return fmt.Errorf("invalid flow_snapshot: %w", err)
	}
	if err := task.ValidateSnapshot(state.TaskSnapshot); err != nil {
		return fmt.Errorf("invalid task_snapshot: %w", err)
	}
	if state.FlowSnapshot.Flow.ID == "" {
		return errors.New("missing required flow_snapshot.flow.id")
	}
	if state.CurrentStepID == "" {
		return errors.New("missing required field \"current_step_id\"")
	}
	if !IsValidFlowRunID(state.FlowRunID) {
		return errors.New("invalid flow_run_id")
	}
	if state.CompletedSteps == nil || state.SkippedSteps == nil || state.BackHistory == nil {
		return errors.New("state collections must not be null")
	}
	if !flowHasStep(state.FlowSnapshot.Flow, state.CurrentStepID) {
		return errors.New("current_step_id is not in flow_snapshot")
	}
	if state.Attempts == nil {
		return errors.New("attempts must not be null")
	}
	if len(state.Attempts) == 0 {
		return errors.New("attempts must not be empty")
	}
	seenIDs := make(map[string]struct{}, len(state.Attempts))
	activeCount := 0
	for i, attempt := range state.Attempts {
		if err := ValidateStepAttempt(attempt); err != nil {
			return fmt.Errorf("invalid attempt %d: %w", i, err)
		}
		wantSequence := uint64(i + 1)
		if attempt.EntrySequence != wantSequence {
			return fmt.Errorf("attempt %d entry sequence must be %d", i, wantSequence)
		}
		if _, duplicate := seenIDs[attempt.ID]; duplicate {
			return errors.New("duplicate attempt id")
		}
		seenIDs[attempt.ID] = struct{}{}
		if !flowHasStep(state.FlowSnapshot.Flow, attempt.StepID) {
			return fmt.Errorf("attempt step_id %q is not in flow_snapshot", attempt.StepID)
		}
		step, _ := flowStep(state.FlowSnapshot.Flow, attempt.StepID)
		requiredArtifacts := map[string]struct{}{}
		requiredArtifactPaths := []string{}
		for _, artifact := range step.Artifacts {
			if artifact.Required {
				requiredArtifacts[artifact.Path] = struct{}{}
				requiredArtifactPaths = append(requiredArtifactPaths, artifact.Path)
			}
		}
		evidencePaths := make([]string, 0, len(attempt.ArtifactEvidence))
		for path := range attempt.ArtifactEvidence {
			evidencePaths = append(evidencePaths, path)
		}
		sort.Strings(evidencePaths)
		for _, path := range evidencePaths {
			if _, ok := requiredArtifacts[path]; !ok {
				return fmt.Errorf("%w: attempt artifact evidence %q is not required by step %q", ErrUnknownArtifactEvidence, path, attempt.StepID)
			}
		}
		if attempt.Status == StepAttemptClosed && attempt.ExitReason == StepAttemptExitDone {
			for _, artifact := range step.Artifacts {
				if !artifact.Required {
					continue
				}
				if _, ok := attempt.ArtifactEvidence[artifact.Path]; !ok {
					return fmt.Errorf("done attempt missing artifact evidence %q", artifact.Path)
				}
			}
		}
		if attempt.Approval != nil && (step.Approval == nil || !step.Approval.Required) {
			return fmt.Errorf("attempt step %q does not require approval", attempt.StepID)
		}
		if attempt.Approval != nil {
			digest, err := ArtifactEvidenceSetDigest(requiredArtifactPaths, attempt.ArtifactEvidence)
			if err != nil {
				return fmt.Errorf("attempt %d approval artifact evidence set: %w", i, err)
			}
			if digest != attempt.Approval.EvidenceSetDigest {
				return fmt.Errorf("attempt %d approval: %w", i, ErrEvidenceSetDigestMismatch)
			}
		}
		for checkID, result := range attempt.CheckResults {
			if !containsString(step.RequiredChecks, checkID) {
				return fmt.Errorf("attempt check %q is not required by step %q", checkID, attempt.StepID)
			}
			if result.ExitCode < 0 {
				return fmt.Errorf("attempt check %q has invalid exit_code", checkID)
			}
			if invalidCheckLogPath(result.LogPath) {
				return fmt.Errorf("attempt check %q has invalid log_path", checkID)
			}
		}
		if attempt.Status == StepAttemptActive {
			activeCount++
		}
	}
	last := state.Attempts[len(state.Attempts)-1]
	switch state.Status {
	case StatusRunning:
		if state.CurrentAttemptID == "" || activeCount != 1 || last.Status != StepAttemptActive || last.ID != state.CurrentAttemptID || last.StepID != state.CurrentStepID {
			return errors.New("running state current attempt invariant violated")
		}
		if state.Finish != nil {
			return errors.New("running state must not have finish")
		}
	case StatusCompleted:
		if state.CurrentAttemptID != "" || activeCount != 0 || last.Status != StepAttemptClosed || (last.ExitReason != StepAttemptExitDone && last.ExitReason != StepAttemptExitSkip) || last.StepID != state.CurrentStepID {
			return errors.New("completed state attempt invariant violated")
		}
		if state.Finish != nil {
			return errors.New("completed state must not have finish")
		}
	case StatusFinished:
		if state.CurrentAttemptID != "" || activeCount != 0 || last.Status != StepAttemptClosed || last.ExitReason != StepAttemptExitFinish || last.StepID != state.CurrentStepID {
			return errors.New("finished state attempt invariant violated")
		}
		if state.Finish == nil || state.Finish.Reason != last.Reason {
			return errors.New("finished state reason mismatch")
		}
	default:
		return fmt.Errorf("unknown status %q", state.Status)
	}
	return nil
}

func Validate(value State) error { return validateState(value) }

func flowHasStep(snapshotFlow flow.Flow, stepID string) bool {
	_, ok := flowStep(snapshotFlow, stepID)
	return ok
}

func flowStep(snapshotFlow flow.Flow, stepID string) (flow.Step, bool) {
	for _, step := range snapshotFlow.Steps {
		if step.ID == stepID {
			return step, true
		}
	}
	return flow.Step{}, false
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func invalidCheckLogPath(value string) bool {
	for _, forbidden := range []string{"\n", "\r", "\x00"} {
		if strings.Contains(value, forbidden) {
			return true
		}
	}
	return false
}
