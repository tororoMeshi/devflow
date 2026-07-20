package flow

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"regexp"
)

const FlowSnapshotSchemaVersion = 1

var (
	ErrUnsupportedSnapshotSchema = errors.New("unsupported flow snapshot schema version")
	ErrSnapshotNotNormalized     = errors.New("flow snapshot is not normalized")
	ErrInvalidSnapshotDigest     = errors.New("invalid flow snapshot digest")
	ErrSnapshotDigestMismatch    = errors.New("flow snapshot digest mismatch")
	flowSnapshotDigestPattern    = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
)

// FlowSource identifies the origin of a Flow for traceability. It is not part
// of the Flow contract and is excluded from the snapshot digest.
type FlowSource struct {
	Path string `json:"path,omitempty"`
}

// FlowSnapshot is the normalized, validated Flow contract that can be fixed
// when a Run starts.
type FlowSnapshot struct {
	SchemaVersion int        `json:"schema_version"`
	Digest        string     `json:"digest"`
	Source        FlowSource `json:"source"`
	Flow          Flow       `json:"flow"`
}

// CloneSnapshot returns a deep copy that does not share Flow collections or
// pointers with snapshot.
func CloneSnapshot(snapshot FlowSnapshot) FlowSnapshot {
	clone := snapshot
	clone.Flow = copyFlow(snapshot.Flow)
	return clone
}

// BuildSnapshot creates a normalized, validated FlowSnapshot without changing
// or sharing mutable memory with the input Flow.
func BuildSnapshot(flow Flow, source FlowSource) (FlowSnapshot, error) {
	normalized := Normalize(copyFlow(flow))
	if err := Validate(normalized); err != nil {
		return FlowSnapshot{}, fmt.Errorf("validate flow snapshot: %w", err)
	}

	digest, err := snapshotDigest(normalized)
	if err != nil {
		return FlowSnapshot{}, err
	}
	return FlowSnapshot{
		SchemaVersion: FlowSnapshotSchemaVersion,
		Digest:        digest,
		Source:        source,
		Flow:          normalized,
	}, nil
}

// ValidateSnapshot verifies a persisted FlowSnapshot without changing it.
func ValidateSnapshot(snapshot FlowSnapshot) error {
	if snapshot.SchemaVersion != FlowSnapshotSchemaVersion {
		return fmt.Errorf("%w: %d", ErrUnsupportedSnapshotSchema, snapshot.SchemaVersion)
	}
	if !flowSnapshotDigestPattern.MatchString(snapshot.Digest) {
		return fmt.Errorf("%w: want sha256:<64 lowercase hex characters>", ErrInvalidSnapshotDigest)
	}
	normalized := Normalize(copyFlow(snapshot.Flow))
	if !reflect.DeepEqual(snapshot.Flow, normalized) {
		return ErrSnapshotNotNormalized
	}
	if err := Validate(normalized); err != nil {
		return fmt.Errorf("validate flow snapshot: %w", err)
	}
	digest, err := snapshotDigest(normalized)
	if err != nil {
		return err
	}
	if snapshot.Digest != digest {
		return fmt.Errorf("%w: got %q, want %q", ErrSnapshotDigestMismatch, snapshot.Digest, digest)
	}
	return nil
}

func snapshotDigest(flow Flow) (string, error) {
	payload, err := canonicalJSON(flow)
	if err != nil {
		return "", fmt.Errorf("marshal canonical flow snapshot: %w", err)
	}
	digest := sha256.Sum256(payload)
	return "sha256:" + hex.EncodeToString(digest[:]), nil
}

type canonicalFlowSnapshot struct {
	SchemaVersion int           `json:"schema_version"`
	Flow          canonicalFlow `json:"flow"`
}

type canonicalFlow struct {
	ID          string          `json:"id"`
	Title       string          `json:"title"`
	Description string          `json:"description"`
	Steps       []canonicalStep `json:"steps"`
}

type canonicalStep struct {
	ID             string              `json:"id"`
	Title          string              `json:"title"`
	Instruction    string              `json:"instruction"`
	Inputs         []canonicalArtifact `json:"inputs"`
	Artifacts      []canonicalArtifact `json:"artifacts"`
	Approval       *canonicalApproval  `json:"approval"`
	RequiredChecks []string            `json:"required_checks"`
}

type canonicalArtifact struct {
	Path     string `json:"path"`
	Required bool   `json:"required"`
}

type canonicalApproval struct {
	Required bool `json:"required"`
}

func canonicalJSON(flow Flow) ([]byte, error) {
	steps := make([]canonicalStep, len(flow.Steps))
	for i, step := range flow.Steps {
		inputs := make([]canonicalArtifact, len(step.Inputs))
		for j, input := range step.Inputs {
			inputs[j] = canonicalArtifact{Path: input.Path, Required: input.Required}
		}

		artifacts := make([]canonicalArtifact, len(step.Artifacts))
		for j, artifact := range step.Artifacts {
			artifacts[j] = canonicalArtifact{Path: artifact.Path, Required: artifact.Required}
		}

		var approval *canonicalApproval
		if step.Approval != nil {
			approval = &canonicalApproval{Required: step.Approval.Required}
		}

		steps[i] = canonicalStep{
			ID:             step.ID,
			Title:          step.Title,
			Instruction:    step.Instruction,
			Inputs:         inputs,
			Artifacts:      artifacts,
			Approval:       approval,
			RequiredChecks: append([]string{}, step.RequiredChecks...),
		}
	}

	return json.Marshal(canonicalFlowSnapshot{
		SchemaVersion: FlowSnapshotSchemaVersion,
		Flow: canonicalFlow{
			ID:          flow.ID,
			Title:       flow.Title,
			Description: flow.Description,
			Steps:       steps,
		},
	})
}

func copyFlow(flow Flow) Flow {
	copy := Flow{
		ID:          flow.ID,
		Title:       flow.Title,
		Description: flow.Description,
	}
	if flow.Steps != nil {
		copy.Steps = make([]Step, len(flow.Steps))
	}

	for i, step := range flow.Steps {
		copy.Steps[i] = Step{
			ID:          step.ID,
			Title:       step.Title,
			Instruction: step.Instruction,
		}
		if step.Inputs != nil {
			copy.Steps[i].Inputs = append([]Artifact{}, step.Inputs...)
		}
		if step.Artifacts != nil {
			copy.Steps[i].Artifacts = append([]Artifact{}, step.Artifacts...)
		}
		if step.RequiredChecks != nil {
			copy.Steps[i].RequiredChecks = append([]string{}, step.RequiredChecks...)
		}
		if step.Approval != nil {
			copy.Steps[i].Approval = &Approval{Required: step.Approval.Required}
		}
	}

	return copy
}
