package command

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"

	"github.com/8noki8/devflow/internal/flow"
	"github.com/8noki8/devflow/internal/jsonprotocol"
	"github.com/8noki8/devflow/internal/state"
	"github.com/8noki8/devflow/internal/transition"
)

const checkSchemaVersion = 2

var (
	errDuplicateJSONKey = jsonprotocol.ErrDuplicateKey
	errTrailingJSON     = jsonprotocol.ErrTrailingJSON
)

type CheckRequestResult struct {
	SchemaVersion int    `json:"schema_version"`
	FlowRunID     string `json:"flow_run_id"`
	StepID        string `json:"step_id"`
	AttemptID     string `json:"attempt_id"`
	CheckID       string `json:"check_id"`
}

type checkRecordFile struct {
	SchemaVersion *int               `json:"schema_version"`
	FlowRunID     *string            `json:"flow_run_id"`
	StepID        *string            `json:"step_id"`
	AttemptID     *string            `json:"attempt_id"`
	CheckID       *string            `json:"check_id"`
	Result        *checkRecordResult `json:"result"`
}

type checkRecordResult struct {
	ExitCode *int               `json:"exit_code"`
	LogPath  checkRecordLogPath `json:"log_path,omitempty"`
}

type checkRecordLogPath struct {
	Value string
}

func (value *checkRecordLogPath) UnmarshalJSON(data []byte) error {
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return errors.New("log_path must be a string")
	}
	return json.Unmarshal(data, &value.Value)
}

func CheckRequest(ctx Context, stepID, attemptID, checkID string) CommandResult {
	active, diagnostics := LoadActiveFlow(ctx)
	if len(diagnostics) > 0 {
		return CommandResult{ExitCode: 1, Diagnostics: diagnostics}
	}
	current := active.State
	if !state.IsValidStepAttemptID(attemptID) {
		return commandFailure(transition.CodeInvalidAttemptID)
	}
	found := false
	for _, attempt := range current.Attempts {
		if attempt.ID == attemptID {
			found = true
			break
		}
	}
	if !found {
		return commandFailure(transition.CodeInvalidAttemptID)
	}
	attempt, _, ok := current.CurrentAttempt()
	if !ok || attempt.Status != state.StepAttemptActive {
		return commandFailure(CodeInvalidState)
	}
	if attempt.ID != attemptID {
		return commandFailure(transition.CodeStaleAttempt)
	}
	if attempt.StepID != stepID || current.CurrentStepID != stepID {
		return commandFailure(transition.CodeStepAttemptMismatch)
	}
	if active.CurrentStep.ID != stepID {
		return commandFailure(CodeInvalidState)
	}
	if !requiredCheck(active.CurrentStep.RequiredChecks, checkID) {
		return commandFailure(CodeCheckNotRequired)
	}
	if _, recorded := attempt.CheckResults[checkID]; recorded {
		return commandFailure(CodeCheckResultAlreadyRecorded)
	}

	return CommandResult{ExitCode: 0, CheckRequest: &CheckRequestResult{
		SchemaVersion: checkSchemaVersion,
		FlowRunID:     current.FlowRunID,
		StepID:        stepID,
		AttemptID:     attemptID,
		CheckID:       checkID,
	}}
}

func CheckRecord(ctx Context, path string) CommandResult {
	record, err := readCheckRecord(path)
	if err != nil {
		if errors.Is(err, errDuplicateJSONKey) {
			return commandFailure(CodeDuplicateJSONKey)
		}
		if errors.Is(err, errTrailingJSON) {
			return commandFailure(CodeTrailingCheckRecordJSON)
		}
		if strings.Contains(err.Error(), "json: unknown field ") {
			return commandFailure(CodeUnknownCheckRecordField)
		}
		return commandFailure(CodeInvalidCheckRecord)
	}
	if record.SchemaVersion == nil || *record.SchemaVersion != checkSchemaVersion {
		return commandFailure(CodeUnsupportedCheckSchema)
	}
	if !validRecordIdentifier(record.FlowRunID, state.IsValidFlowRunID) ||
		!validRecordIdentifier(record.StepID, flow.IsValidID) ||
		!validRecordIdentifier(record.AttemptID, state.IsValidStepAttemptID) ||
		!validRecordIdentifier(record.CheckID, flow.IsValidID) ||
		record.Result == nil || record.Result.ExitCode == nil ||
		*record.Result.ExitCode < 0 || invalidLogPath(record.Result.LogPath.Value) {
		return commandFailure(CodeInvalidCheckRecord)
	}
	checkResult := state.CheckResult{
		ExitCode: *record.Result.ExitCode,
		LogPath:  record.Result.LogPath.Value,
	}

	store := NewStore(ctx)
	pointer, err := store.LoadCurrentPointer()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return commandFailure(CodeNoActiveFlow)
		}
		return commandFailure(CodeInvalidState)
	}
	if *record.FlowRunID != pointer.FlowRunID {
		return commandFailure(CodeCheckRunMismatch)
	}
	loaded := store.LoadCurrent()
	if loaded.Status == state.LoadNoState {
		return commandFailure(CodeNoActiveFlow)
	}
	if loaded.Status != state.LoadOK || loaded.State == nil {
		return commandFailure(CodeInvalidState)
	}
	if *record.FlowRunID != loaded.State.FlowRunID {
		return commandFailure(CodeCheckRunMismatch)
	}

	applied := transition.ApplyRecordCheckResult(
		*loaded.State, *record.StepID, *record.AttemptID, *record.CheckID, checkResult,
	)
	if applied.ExitCode != 0 {
		return CommandResult{ExitCode: applied.ExitCode, Diagnostics: applied.Diagnostics}
	}
	if applied.StateChanged {
		if err := store.SaveCurrent(*applied.State); err != nil {
			return commandFailure(CodeStateSaveFailed)
		}
	}
	exitCode := checkResult.ExitCode
	return CommandResult{ExitCode: 0, Success: &SuccessResult{
		RecordedCheckRunID:     *record.FlowRunID,
		RecordedCheckStepID:    *record.StepID,
		RecordedCheckAttemptID: *record.AttemptID,
		RecordedCheckID:        *record.CheckID,
		RecordedCheckExitCode:  &exitCode,
	}}
}

func requiredCheck(requiredChecks []string, checkID string) bool {
	for _, required := range requiredChecks {
		if required == checkID {
			return true
		}
	}
	return false
}

func validRecordIdentifier(value *string, valid func(string) bool) bool {
	return value != nil && *value != "" && strings.TrimSpace(*value) == *value && valid(*value)
}

func readCheckRecord(path string) (checkRecordFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return checkRecordFile{}, err
	}
	if err := rejectDuplicateJSONKeys(data); err != nil {
		return checkRecordFile{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var record checkRecordFile
	if err := decoder.Decode(&record); err != nil {
		return checkRecordFile{}, err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return checkRecordFile{}, errTrailingJSON
	}
	return record, nil
}

func rejectDuplicateJSONKeys(data []byte) error {
	return jsonprotocol.ValidateKeysAndTrailing(data)
}

func invalidLogPath(value string) bool {
	return strings.ContainsAny(value, "\n\r\x00")
}
