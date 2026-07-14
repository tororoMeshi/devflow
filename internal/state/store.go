package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type Store struct {
	Path string
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

type UnsupportedSchemaVersionError struct {
	Actual int
}

func (e *UnsupportedSchemaVersionError) Error() string {
	return fmt.Sprintf("unsupported state schema version %d", e.Actual)
}

func (s Store) Load() LoadResult {
	data, err := os.ReadFile(s.Path)
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

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return LoadResult{Status: LoadInvalid, Err: err}
	}
	if err := validateStateFile(raw, state); err != nil {
		return LoadResult{Status: LoadInvalid, Err: err}
	}

	state.Normalize()
	return LoadResult{Status: LoadOK, State: &state}
}

func (s Store) Save(state State) error {
	next := state.Clone()
	if err := validateState(next); err != nil {
		return err
	}

	dir := filepath.Dir(s.Path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, filepath.Base(s.Path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	keepTmp := false
	defer func() {
		if !keepTmp {
			_ = os.Remove(tmpPath)
		}
	}()

	encoder := json.NewEncoder(tmp)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(next); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, s.Path); err != nil {
		return err
	}

	keepTmp = true
	return nil
}

func validateStateFile(raw map[string]json.RawMessage, state State) error {
	if _, ok := raw["schema_version"]; !ok {
		return &UnsupportedSchemaVersionError{}
	}
	for _, field := range []string{"schema_version", "flow_id", "status", "current_step_id"} {
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
	if state.FlowID == "" {
		return errors.New("missing required field \"flow_id\"")
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
