package executionreport

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/tororoMeshi/devflow/internal/flow"
	"github.com/tororoMeshi/devflow/internal/jsonprotocol"
	"github.com/tororoMeshi/devflow/internal/pathcheck"
	"github.com/tororoMeshi/devflow/internal/state"
	"github.com/tororoMeshi/devflow/internal/workpackage"
)

var digestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

type wireReport struct {
	SchemaVersion     wireSchemaVersion `json:"schema_version"`
	FlowRunID         *string           `json:"flow_run_id"`
	StepID            *string           `json:"step_id"`
	AttemptID         *string           `json:"attempt_id"`
	WorkPackageDigest *string           `json:"work_package_digest"`
	Outcome           *Outcome          `json:"outcome"`
	Summary           *string           `json:"summary"`
	Decisions         *[]wireDecision   `json:"decisions"`
	ArtifactRefs      *[]string         `json:"artifact_refs"`
	UnresolvedIssues  *[]string         `json:"unresolved_issues"`
	NextAction        *string           `json:"next_action"`
}

type wireSchemaVersion struct {
	Present bool
	Null    bool
	Value   int
}

func (value *wireSchemaVersion) UnmarshalJSON(data []byte) error {
	value.Present = true
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		value.Null = true
		return nil
	}
	return json.Unmarshal(data, &value.Value)
}

type wireDecision struct {
	Decision     *string              `json:"decision"`
	Rationale    *string              `json:"rationale"`
	EvidenceRefs *[]EvidenceReference `json:"evidence_refs"`
}

func Decode(data []byte) (Report, error) {
	if len(data) > MaxDocumentBytes {
		return Report{}, ErrTooLarge
	}
	if err := jsonprotocol.ValidateKeysAndTrailing(data); err != nil {
		switch {
		case errors.Is(err, jsonprotocol.ErrDuplicateKey):
			return Report{}, ErrDuplicateJSONKey
		case errors.Is(err, jsonprotocol.ErrTrailingJSON):
			return Report{}, ErrTrailingJSON
		default:
			return Report{}, fmt.Errorf("%w: JSON syntax", ErrInvalidReport)
		}
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var wire wireReport
	if err := decoder.Decode(&wire); err != nil {
		if strings.Contains(err.Error(), "json: unknown field ") {
			return Report{}, ErrUnknownField
		}
		return Report{}, fmt.Errorf("%w: field decode", ErrInvalidReport)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return Report{}, ErrTrailingJSON
	}
	if !wire.SchemaVersion.Present || wire.SchemaVersion.Null {
		return Report{}, ErrInvalidReport
	}
	if wire.SchemaVersion.Value != SchemaVersion {
		return Report{}, ErrUnsupportedSchema
	}
	if wire.FlowRunID == nil || wire.StepID == nil || wire.AttemptID == nil ||
		wire.WorkPackageDigest == nil || wire.Outcome == nil || wire.Summary == nil ||
		wire.Decisions == nil || wire.ArtifactRefs == nil || wire.UnresolvedIssues == nil ||
		wire.NextAction == nil {
		return Report{}, ErrInvalidReport
	}
	decisions := make([]DecisionRecord, len(*wire.Decisions))
	for i, decision := range *wire.Decisions {
		if decision.Decision == nil || decision.Rationale == nil || decision.EvidenceRefs == nil {
			return Report{}, ErrInvalidReport
		}
		decisions[i] = DecisionRecord{
			Decision: *decision.Decision, Rationale: *decision.Rationale,
			EvidenceRefs: append([]EvidenceReference{}, (*decision.EvidenceRefs)...),
		}
	}
	report := Report{
		SchemaVersion: wire.SchemaVersion.Value, FlowRunID: *wire.FlowRunID,
		StepID: *wire.StepID, AttemptID: *wire.AttemptID,
		WorkPackageDigest: *wire.WorkPackageDigest, Outcome: *wire.Outcome,
		Summary: *wire.Summary, Decisions: decisions,
		ArtifactRefs:     append([]string{}, (*wire.ArtifactRefs)...),
		UnresolvedIssues: append([]string{}, (*wire.UnresolvedIssues)...),
		NextAction:       *wire.NextAction,
	}
	if err := Validate(report); err != nil {
		return Report{}, err
	}
	return report, nil
}

func Validate(report Report) error {
	if report.SchemaVersion != SchemaVersion {
		return ErrUnsupportedSchema
	}
	if !validIdentifier(report.FlowRunID, state.IsValidFlowRunID) ||
		!validIdentifier(report.StepID, flow.IsValidID) ||
		!validIdentifier(report.AttemptID, state.IsValidStepAttemptID) ||
		!validIdentifier(report.WorkPackageDigest, digestPattern.MatchString) {
		return ErrInvalidReport
	}
	if report.Outcome != OutcomeCompleted && report.Outcome != OutcomeBlocked && report.Outcome != OutcomeFailed {
		return ErrInvalidReport
	}
	if !requiredText(report.Summary) || !optionalText(report.NextAction) ||
		report.Decisions == nil || report.ArtifactRefs == nil || report.UnresolvedIssues == nil ||
		len(report.Decisions) > MaxCollection || len(report.ArtifactRefs) > MaxCollection ||
		len(report.UnresolvedIssues) > MaxCollection {
		return ErrInvalidReport
	}
	for _, decision := range report.Decisions {
		if !requiredText(decision.Decision) || !requiredText(decision.Rationale) ||
			decision.EvidenceRefs == nil || len(decision.EvidenceRefs) > MaxCollection {
			return ErrInvalidReport
		}
		seen := map[string]struct{}{}
		for _, ref := range decision.EvidenceRefs {
			if ref.Kind != EvidenceInput && ref.Kind != EvidenceArtifact && ref.Kind != EvidenceCheck ||
				!requiredText(ref.ID) {
				return ErrInvalidReport
			}
			if (ref.Kind == EvidenceInput || ref.Kind == EvidenceArtifact) && pathcheck.ValidateArtifactPath(ref.ID) != nil {
				return ErrInvalidReport
			}
			if ref.Kind == EvidenceCheck && !flow.IsValidID(ref.ID) {
				return ErrInvalidReport
			}
			key := string(ref.Kind) + "\x00" + ref.ID
			if _, ok := seen[key]; ok {
				return ErrInvalidReport
			}
			seen[key] = struct{}{}
		}
	}
	seenArtifacts := map[string]struct{}{}
	for _, ref := range report.ArtifactRefs {
		if !requiredText(ref) || pathcheck.ValidateArtifactPath(ref) != nil {
			return ErrInvalidReport
		}
		if _, ok := seenArtifacts[ref]; ok {
			return ErrInvalidReport
		}
		seenArtifacts[ref] = struct{}{}
	}
	for _, issue := range report.UnresolvedIssues {
		if !requiredText(issue) {
			return ErrInvalidReport
		}
	}
	return nil
}

func ValidateBinding(report Report, pkg workpackage.WorkPackage) error {
	if report.FlowRunID != pkg.FlowRunID || report.StepID != pkg.StepID ||
		report.AttemptID != pkg.AttemptID || report.WorkPackageDigest != pkg.WorkPackageDigest {
		return ErrBindingMismatch
	}
	inputs, artifacts, checks := map[string]struct{}{}, map[string]struct{}{}, map[string]struct{}{}
	for _, item := range pkg.Step.Inputs {
		inputs[item.Path] = struct{}{}
	}
	for _, item := range pkg.Step.Artifacts {
		artifacts[item.Path] = struct{}{}
	}
	for _, id := range pkg.Step.RequiredChecks {
		checks[id] = struct{}{}
	}
	for _, decision := range report.Decisions {
		for _, ref := range decision.EvidenceRefs {
			var ok bool
			switch ref.Kind {
			case EvidenceInput:
				_, ok = inputs[ref.ID]
			case EvidenceArtifact:
				_, ok = artifacts[ref.ID]
			case EvidenceCheck:
				_, ok = checks[ref.ID]
			}
			if !ok {
				return ErrUnknownEvidence
			}
		}
	}
	for _, ref := range report.ArtifactRefs {
		if _, ok := artifacts[ref]; !ok {
			return ErrUnknownArtifact
		}
	}
	return nil
}

func validIdentifier(value string, valid func(string) bool) bool {
	return requiredText(value) && valid(value)
}

func requiredText(value string) bool {
	return value != "" && strings.TrimSpace(value) == value && !strings.ContainsRune(value, '\x00')
}

func optionalText(value string) bool {
	return value == "" || requiredText(value)
}
