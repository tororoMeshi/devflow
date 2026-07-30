package command

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/tororoMeshi/devflow/internal/flow"
	"github.com/tororoMeshi/devflow/internal/gate"
	"github.com/tororoMeshi/devflow/internal/state"
	"github.com/tororoMeshi/devflow/internal/task"
	"github.com/tororoMeshi/devflow/internal/transition"
)

func Start(ctx Context, flowID, taskPath string) CommandResult {
	if diagnostics := validateStartFlowID(flowID); len(diagnostics) > 0 {
		return CommandResult{ExitCode: 1, Diagnostics: diagnostics}
	}
	normalizedTaskPath, ok := normalizeTaskPath(taskPath)
	if !ok {
		return commandFailure(CodeInvalidTaskPath)
	}

	store := NewStore(ctx)
	loaded := store.LoadCurrent()
	current, diagnostics := startCurrentState(loaded)
	if len(diagnostics) > 0 {
		return CommandResult{ExitCode: 1, Diagnostics: diagnostics}
	}
	if current != nil && current.Status == state.StatusRunning {
		return CommandResult{ExitCode: 1, Diagnostics: []transition.Diagnostic{{Level: transition.LevelError, Code: transition.CodeFlowAlreadyRunning, StepID: current.CurrentStepID}}}
	}

	fl, err := flow.LoadFile(filepath.Join(FlowDir(ctx.ProjectRoot), flowID+".cue"))
	if err != nil {
		return commandFailure(CodeStateFlowMismatch)
	}
	if fl.ID != flowID {
		return commandFailure(CodeStateFlowMismatch)
	}
	flowSnapshot, err := flow.BuildSnapshot(fl, flow.FlowSource{Path: filepath.Join(FlowDir(ctx.ProjectRoot), flowID+".cue")})
	if err != nil {
		return commandFailure(CodeStateFlowMismatch)
	}
	taskFilePath := filepath.Join(ctx.ProjectRoot, filepath.FromSlash(normalizedTaskPath))
	info, err := os.Stat(taskFilePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return commandFailureWithError(CodeTaskFileNotFound, err)
		}
		return commandFailureWithError(CodeTaskFileReadFailed, err)
	}
	if info.IsDir() {
		return commandFailure(CodeTaskPathIsDirectory)
	}
	content, err := os.ReadFile(taskFilePath)
	if err != nil {
		return commandFailureWithError(CodeTaskFileReadFailed, err)
	}
	taskSnapshot, err := task.BuildSnapshot(string(content), task.TaskSource{Path: normalizedTaskPath})
	if err != nil {
		if errors.Is(err, task.ErrEmptyTask) {
			return commandFailure(CodeTaskEmpty)
		}
		if errors.Is(err, task.ErrInvalidUTF8) {
			return commandFailure(CodeTaskInvalidUTF8)
		}
		return commandFailure(CodeTaskSnapshotBuildFailed)
	}

	firstStep := flowSnapshot.Flow.Steps[0]
	entryResult := gate.InspectEntryGate(ctx.ProjectRoot, firstStep, gate.NewInspectionSet())
	if !entryResult.Ready {
		return CommandResult{ExitCode: 1, Diagnostics: entryGateDiagnostics(firstStep.ID, entryResult)}
	}

	flowRunID, err := newFlowRunID()
	if err != nil {
		return commandFailure(CodeFlowRunIDGenerationFailed)
	}
	result := transition.ApplyStart(flowSnapshot, taskSnapshot, current, flowRunID)
	if result.State != nil && result.ExitCode == 0 {
		if err := store.CreateRun(*result.State); err != nil {
			result.Diagnostics = append(result.Diagnostics, commandErrorDiagnostic(CodeStateSaveFailed))
			return CommandResult{ExitCode: 1, Diagnostics: result.Diagnostics}
		}
	}
	if result.State == nil && result.ExitCode == 0 {
		result.Diagnostics = append(result.Diagnostics, commandErrorDiagnostic(CodeStateSaveFailed))
		return CommandResult{ExitCode: 1, Diagnostics: result.Diagnostics}
	}

	return CommandResult{
		ExitCode:    result.ExitCode,
		Success:     startSuccess(result),
		Diagnostics: result.Diagnostics,
	}
}

func normalizeTaskPath(path string) (string, bool) {
	if strings.TrimSpace(path) == "" || filepath.IsAbs(path) {
		return "", false
	}
	for _, part := range strings.FieldsFunc(path, func(r rune) bool { return r == '/' || r == '\\' }) {
		if part == ".." {
			return "", false
		}
	}
	cleaned := filepath.Clean(path)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return "", false
	}
	return filepath.ToSlash(cleaned), true
}

func startSuccess(result transition.TransitionResult) *SuccessResult {
	if result.ExitCode != 0 || result.State == nil {
		return nil
	}
	return &SuccessResult{
		StartedFlowID: result.State.FlowSnapshot.Flow.ID,
		CurrentStepID: result.State.CurrentStepID,
	}
}

func validateStartFlowID(flowID string) []transition.Diagnostic {
	if flowID == "" || !flow.IsValidID(flowID) {
		code := string(flow.ErrorInvalidFlowID)
		if strings.TrimSpace(flowID) == "" {
			code = string(flow.ErrorMissingFlowID)
		}
		return []transition.Diagnostic{commandErrorDiagnostic(code)}
	}
	return nil
}

func startCurrentState(loaded state.LoadResult) (*state.State, []transition.Diagnostic) {
	switch loaded.Status {
	case state.LoadNoState:
		return nil, nil
	case state.LoadInvalid:
		if isUnsupportedStateVersion(loaded.Err) {
			return nil, []transition.Diagnostic{unsupportedStateVersionDiagnostic()}
		}
		return nil, []transition.Diagnostic{commandErrorDiagnostic(CodeInvalidState)}
	case state.LoadOK:
		return loaded.State, nil
	default:
		return nil, []transition.Diagnostic{commandErrorDiagnostic(CodeInvalidState)}
	}
}

func commandFailure(code string) CommandResult {
	return CommandResult{
		ExitCode:    1,
		Diagnostics: []transition.Diagnostic{commandErrorDiagnostic(code)},
	}
}

func commandFailureWithError(code string, err error) CommandResult {
	diagnostic := commandErrorDiagnostic(code)
	diagnostic.Message = err.Error()
	return CommandResult{ExitCode: 1, Diagnostics: []transition.Diagnostic{diagnostic}}
}
