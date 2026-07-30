package command

import (
	"errors"

	"github.com/tororoMeshi/devflow/internal/artifact"
	"github.com/tororoMeshi/devflow/internal/state"
	"github.com/tororoMeshi/devflow/internal/transition"
)

func RecordArtifact(ctx Context, stepID, attemptID, path string) CommandResult {
	active, diagnostics := LoadActiveFlow(ctx)
	if len(diagnostics) > 0 {
		return CommandResult{ExitCode: 1, Diagnostics: diagnostics}
	}
	// Run the pure contract checks before touching the filesystem.
	probeEvidence := state.ArtifactEvidence{
		Digest: "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		Size:   0,
	}
	if current, _, ok := active.State.CurrentAttempt(); ok {
		if existing, recorded := current.ArtifactEvidence[path]; recorded {
			probeEvidence = existing
		}
	}
	probe := transition.ApplyRecordArtifactEvidence(active.State, stepID, attemptID, path, probeEvidence)
	if probe.ExitCode != 0 {
		return CommandResult{ExitCode: 1, Diagnostics: probe.Diagnostics}
	}
	fileEvidence, err := artifact.ReadFile(ctx.ProjectRoot, path)
	if err != nil {
		return CommandResult{ExitCode: 1, Diagnostics: []transition.Diagnostic{artifactDiagnostic(err, stepID)}}
	}
	evidence := state.ArtifactEvidence{Digest: fileEvidence.Digest, Size: fileEvidence.Size}
	result := transition.ApplyRecordArtifactEvidence(active.State, stepID, attemptID, path, evidence)
	success := &SuccessResult{
		RecordedArtifactPath: path, RecordedAttemptID: attemptID,
		RecordedArtifactDigest: evidence.Digest, RecordedArtifactSize: evidence.Size,
	}
	return transitionCommandResult(ctx, result, success)
}

func artifactDiagnostic(err error, stepID string) transition.Diagnostic {
	code := CodeArtifactUnreadable
	switch {
	case errors.Is(err, artifact.ErrInvalidPath):
		code = CodeInvalidArtifactPath
	case errors.Is(err, artifact.ErrMissing):
		code = CodeArtifactFileMissing
	case errors.Is(err, artifact.ErrNotRegular):
		code = CodeArtifactNotRegular
	case errors.Is(err, artifact.ErrSymlink):
		code = CodeArtifactSymlink
	case errors.Is(err, artifact.ErrOutsideProject):
		code = CodeArtifactOutsideProject
	case errors.Is(err, artifact.ErrInsideDevflow):
		code = CodeArtifactInsideDevflow
	case errors.Is(err, artifact.ErrChanged):
		code = CodeArtifactChanged
	}
	return transition.Diagnostic{Level: transition.LevelError, Code: code, StepID: stepID}
}
