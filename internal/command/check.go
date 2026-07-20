package command

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"

	"github.com/8noki8/devflow/internal/state"
)

const checkSchemaVersion = 1

type CheckRequestResult struct {
	SchemaVersion int    `json:"schema_version"`
	FlowRunID     string `json:"flow_run_id"`
	FlowID        string `json:"flow_id"`
	StepID        string `json:"step_id"`
	EntrySequence uint64 `json:"entry_sequence"`
	CheckID       string `json:"check_id"`
}

type checkRecordFile struct {
	SchemaVersion *int    `json:"schema_version"`
	FlowRunID     string  `json:"flow_run_id"`
	FlowID        string  `json:"flow_id"`
	StepID        string  `json:"step_id"`
	EntrySequence *uint64 `json:"entry_sequence"`
	CheckID       string  `json:"check_id"`
	ExitCode      *int    `json:"exit_code"`
	LogPath       string  `json:"log_path"`
}

func CheckRequest(ctx Context, checkID string) CommandResult {
	active, diagnostics := LoadActiveFlow(ctx)
	if len(diagnostics) > 0 {
		return CommandResult{ExitCode: 1, Diagnostics: diagnostics}
	}
	if !requiredCheck(active.CurrentStep.RequiredChecks, checkID) {
		return commandFailure(CodeCheckNotRequired)
	}

	current := active.State
	attempt, _, ok := current.CurrentAttempt()
	if !ok || attempt.StepID != active.CurrentStep.ID {
		return commandFailure(CodeInvalidState)
	}

	return CommandResult{ExitCode: 0, CheckRequest: &CheckRequestResult{
		SchemaVersion: checkSchemaVersion,
		FlowRunID:     current.FlowRunID,
		FlowID:        active.Flow.ID,
		StepID:        attempt.StepID,
		EntrySequence: attempt.EntrySequence,
		CheckID:       checkID,
	}}
}

func CheckRecord(ctx Context, path string) CommandResult {
	record, err := readCheckRecord(path)
	if err != nil {
		return commandFailure(CodeInvalidCheckRecord)
	}
	if record.SchemaVersion == nil || *record.SchemaVersion != checkSchemaVersion {
		return commandFailure(CodeUnsupportedCheckSchema)
	}
	if record.EntrySequence == nil || *record.EntrySequence == 0 || record.ExitCode == nil || record.FlowRunID == "" || record.FlowID == "" || record.StepID == "" || record.CheckID == "" {
		return commandFailure(CodeInvalidCheckRecord)
	}
	if !state.IsValidFlowRunID(record.FlowRunID) {
		return commandFailure(CodeInvalidCheckRecord)
	}
	if invalidLogPath(record.LogPath) {
		return commandFailure(CodeInvalidCheckRecord)
	}

	active, diagnostics := LoadActiveFlow(ctx)
	if len(diagnostics) > 0 {
		return CommandResult{ExitCode: 1, Diagnostics: diagnostics}
	}
	current := active.State
	attempt, attemptIndex, ok := current.CurrentAttempt()
	if !ok || attempt.StepID != active.CurrentStep.ID {
		return commandFailure(CodeInvalidState)
	}
	if record.FlowRunID != current.FlowRunID || record.FlowID != active.Flow.ID || record.StepID != attempt.StepID || *record.EntrySequence != attempt.EntrySequence {
		return commandFailure(CodeCheckContextMismatch)
	}
	if !requiredCheck(active.CurrentStep.RequiredChecks, record.CheckID) {
		return commandFailure(CodeCheckNotRequired)
	}

	next := current.Clone()
	next.Attempts[attemptIndex].CheckResults[record.CheckID] = state.CheckResult{
		ExitCode: *record.ExitCode,
		LogPath:  record.LogPath,
	}
	if err := NewStore(ctx).SaveCurrent(next); err != nil {
		return commandFailure(CodeStateSaveFailed)
	}
	return CommandResult{ExitCode: 0}
}

func requiredCheck(requiredChecks []string, checkID string) bool {
	for _, required := range requiredChecks {
		if required == checkID {
			return true
		}
	}
	return false
}

func readCheckRecord(path string) (checkRecordFile, error) {
	file, err := os.Open(path)
	if err != nil {
		return checkRecordFile{}, err
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var record checkRecordFile
	if err := decoder.Decode(&record); err != nil {
		return checkRecordFile{}, err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return checkRecordFile{}, errors.New("trailing JSON")
	}
	return record, nil
}

func invalidLogPath(value string) bool {
	return strings.ContainsAny(value, "\n\r\x00")
}
