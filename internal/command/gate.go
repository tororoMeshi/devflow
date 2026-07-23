package command

import (
	"github.com/8noki8/devflow/internal/gate"
	"github.com/8noki8/devflow/internal/transition"
)

func entryGateDiagnostics(stepID string, result gate.EntryGateResult) []transition.Diagnostic {
	diagnostics := make([]transition.Diagnostic, 0, len(result.Blockers))
	for _, blocker := range result.Blockers {
		code := CodeEntryMissingRequiredInput
		if blocker.Kind == gate.EntryBlockerInputUnavailable {
			code = CodeEntryInputUnavailable
		}
		diagnostics = append(diagnostics, transition.Diagnostic{
			Level:     transition.LevelError,
			Code:      code,
			StepID:    stepID,
			Artifacts: []string{blocker.Path},
			Message:   "Path: " + blocker.Path,
		})
	}
	return diagnostics
}

func completionGateDiagnostics(stepID string, result gate.CompletionGateResult) []transition.Diagnostic {
	diagnostics := make([]transition.Diagnostic, 0, len(result.Blockers))
	for _, blocker := range result.Blockers {
		code := transition.CodeMissingRequiredInput
		diagnosticStepID := stepID
		switch blocker.Kind {
		case gate.CompletionBlockerInputUnavailable:
			code = CodeCompletionInputUnavailable
		case gate.CompletionBlockerMissingArtifactEvidence:
			code = transition.CodeMissingArtifactEvidence
		case gate.CompletionBlockerMissingArtifact:
			code = transition.CodeMissingRequiredArtifact
		case gate.CompletionBlockerArtifactEvidenceMismatch:
			code = transition.CodeArtifactEvidenceMismatch
		case gate.CompletionBlockerArtifactUnavailable:
			code = transition.CodeArtifactUnsafe
		case gate.CompletionBlockerMissingCheck:
			code = transition.CodeMissingRequiredCheck
			diagnosticStepID = blocker.CheckID
		case gate.CompletionBlockerFailedCheck:
			code = transition.CodeFailedRequiredCheck
			diagnosticStepID = blocker.CheckID
		case gate.CompletionBlockerMissingApproval:
			code = transition.CodeMissingRequiredApproval
		}
		diagnostic := transition.Diagnostic{Level: transition.LevelError, Code: code, StepID: diagnosticStepID}
		if blocker.Path != "" {
			diagnostic.Artifacts = []string{blocker.Path}
			diagnostic.Message = "Path: " + blocker.Path
		}
		diagnostics = append(diagnostics, diagnostic)
	}
	return diagnostics
}
