package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/8noki8/devflow/internal/flow"
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
	next := value.Clone()
	if err := validateState(next); err != nil {
		return err
	}
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
	next := value.Clone()
	if err := validateState(next); err != nil {
		return err
	}
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
	for _, field := range []string{"schema_version", "flow_snapshot", "status", "current_step_id"} {
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
	if state.FlowSnapshot.Flow.ID == "" {
		return errors.New("missing required flow_snapshot.flow.id")
	}
	if state.CurrentStepID == "" {
		return errors.New("missing required field \"current_step_id\"")
	}
	if !IsValidFlowRunID(state.FlowRunID) {
		return errors.New("invalid flow_run_id")
	}
	if state.CurrentEntrySequence == 0 {
		return errors.New("invalid current_entry_sequence")
	}
	if state.Status == StatusRunning && !flowHasStep(state.FlowSnapshot.Flow, state.CurrentStepID) {
		return errors.New("current_step_id is not in flow_snapshot")
	}
	for _, result := range state.CheckResults {
		if result.EntrySequence != state.CurrentEntrySequence {
			return errors.New("check result entry sequence mismatch")
		}
	}
	switch state.Status {
	case StatusRunning, StatusCompleted, StatusFinished:
		return nil
	default:
		return fmt.Errorf("unknown status %q", state.Status)
	}
}

func flowHasStep(snapshotFlow flow.Flow, stepID string) bool {
	for _, step := range snapshotFlow.Steps {
		if step.ID == stepID {
			return true
		}
	}
	return false
}
