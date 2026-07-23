package gate

import (
	"github.com/8noki8/devflow/internal/artifact"
	"github.com/8noki8/devflow/internal/state"
)

type EntryBlockerKind string

const (
	EntryBlockerMissingInput     EntryBlockerKind = "missing_input"
	EntryBlockerInputUnavailable EntryBlockerKind = "input_unavailable"
)

type EntryBlocker struct {
	Kind EntryBlockerKind
	Path string
}

type EntryGateResult struct {
	Ready    bool
	Blockers []EntryBlocker
}

type CompletionBlockerKind string

const (
	CompletionBlockerMissingInput             CompletionBlockerKind = "missing_input"
	CompletionBlockerInputUnavailable         CompletionBlockerKind = "input_unavailable"
	CompletionBlockerMissingArtifactEvidence  CompletionBlockerKind = "missing_artifact_evidence"
	CompletionBlockerMissingArtifact          CompletionBlockerKind = "missing_artifact"
	CompletionBlockerArtifactEvidenceMismatch CompletionBlockerKind = "artifact_evidence_mismatch"
	CompletionBlockerArtifactUnavailable      CompletionBlockerKind = "artifact_unavailable"
	CompletionBlockerMissingCheck             CompletionBlockerKind = "missing_check"
	CompletionBlockerFailedCheck              CompletionBlockerKind = "failed_check"
	CompletionBlockerMissingApproval          CompletionBlockerKind = "missing_approval"
)

type CompletionBlocker struct {
	Kind    CompletionBlockerKind
	Path    string
	CheckID string
}

type CompletionGateResult struct {
	Ready    bool
	Blockers []CompletionBlocker
}

type fileInspection struct {
	Evidence artifact.FileEvidence
	Err      error
}

// InspectionSet is a command-local cache of safe filesystem inspections.
// It is never persisted or shared between commands.
type InspectionSet struct {
	files    map[string]fileInspection
	readFile func(string, string) (artifact.FileEvidence, error)
}

func NewInspectionSet() *InspectionSet {
	return &InspectionSet{
		files:    make(map[string]fileInspection),
		readFile: artifact.ReadFile,
	}
}

func (set *InspectionSet) inspect(projectRoot, path string) fileInspection {
	if set == nil {
		evidence, err := artifact.ReadFile(projectRoot, path)
		return fileInspection{Evidence: evidence, Err: err}
	}
	if set.files == nil {
		set.files = make(map[string]fileInspection)
	}
	if set.readFile == nil {
		set.readFile = artifact.ReadFile
	}
	key := projectRoot + "\x00" + path
	if inspection, ok := set.files[key]; ok {
		return inspection
	}
	evidence, err := set.readFile(projectRoot, path)
	inspection := fileInspection{Evidence: evidence, Err: err}
	set.files[key] = inspection
	return inspection
}

// ArtifactInspection is the transient projection used by prompt and execution
// context. It is derived from the current attempt and filesystem.
type ArtifactInspection struct {
	Path            string
	Required        bool
	Exists          bool
	Evidence        *state.ArtifactEvidence
	MatchesEvidence *bool
	Problem         CompletionBlockerKind
}
