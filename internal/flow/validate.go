package flow

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/8noki8/devflow/internal/pathcheck"
)

type ErrorCode string

const (
	ErrorMissingFlowID            ErrorCode = "error_missing_flow_id"
	ErrorMissingFlowTitle         ErrorCode = "error_missing_flow_title"
	ErrorFlowHasNoSteps           ErrorCode = "error_flow_has_no_steps"
	ErrorMissingStepID            ErrorCode = "error_missing_step_id"
	ErrorMissingStepTitle         ErrorCode = "error_missing_step_title"
	ErrorMissingStepInstruction   ErrorCode = "error_missing_step_instruction"
	ErrorDuplicateStepID          ErrorCode = "error_duplicate_step_id"
	ErrorMissingArtifactPath      ErrorCode = "error_missing_artifact_path"
	ErrorInvalidArtifactPath      ErrorCode = "error_invalid_artifact_path"
	ErrorFlowIDFilenameMismatch   ErrorCode = "error_flow_id_filename_mismatch"
	ErrorInvalidFlowID            ErrorCode = "error_invalid_flow_id"
	ErrorInvalidStepID            ErrorCode = "error_invalid_step_id"
	ErrorMissingRequiredCheckID   ErrorCode = "error_missing_required_check_id"
	ErrorInvalidRequiredCheckID   ErrorCode = "error_invalid_required_check_id"
	ErrorDuplicateRequiredCheckID ErrorCode = "error_duplicate_required_check_id"
)

type ValidationError struct {
	Code ErrorCode
	Err  error
}

func (e *ValidationError) Error() string {
	if e.Err == nil {
		return string(e.Code)
	}
	return fmt.Sprintf("%s: %v", e.Code, e.Err)
}

func (e *ValidationError) Unwrap() error {
	return e.Err
}

var idPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

func IsValidID(id string) bool {
	return !blank(id) && idPattern.MatchString(id)
}

func Validate(flow Flow) error {
	if blank(flow.ID) {
		return validationError(ErrorMissingFlowID, nil)
	}
	if !IsValidID(flow.ID) {
		return validationError(ErrorInvalidFlowID, nil)
	}
	if blank(flow.Title) {
		return validationError(ErrorMissingFlowTitle, nil)
	}
	if len(flow.Steps) == 0 {
		return validationError(ErrorFlowHasNoSteps, nil)
	}

	seen := map[string]struct{}{}
	for _, step := range flow.Steps {
		if blank(step.ID) {
			return validationError(ErrorMissingStepID, nil)
		}
		if !IsValidID(step.ID) {
			return validationError(ErrorInvalidStepID, nil)
		}
		if _, ok := seen[step.ID]; ok {
			return validationError(ErrorDuplicateStepID, nil)
		}
		seen[step.ID] = struct{}{}

		seenChecks := map[string]struct{}{}
		for _, checkID := range step.RequiredChecks {
			if blank(checkID) {
				return validationError(ErrorMissingRequiredCheckID, nil)
			}
			if !IsValidID(checkID) {
				return validationError(ErrorInvalidRequiredCheckID, nil)
			}
			if _, ok := seenChecks[checkID]; ok {
				return validationError(ErrorDuplicateRequiredCheckID, nil)
			}
			seenChecks[checkID] = struct{}{}
		}

		if blank(step.Title) {
			return validationError(ErrorMissingStepTitle, nil)
		}
		if blank(step.Instruction) {
			return validationError(ErrorMissingStepInstruction, nil)
		}

		for _, artifact := range step.Artifacts {
			if blank(artifact.Path) {
				return validationError(ErrorMissingArtifactPath, nil)
			}
			if err := pathcheck.ValidateArtifactPath(artifact.Path); err != nil {
				return validationError(ErrorInvalidArtifactPath, err)
			}
		}
	}

	return nil
}

func ValidateFilename(flow Flow, path string) error {
	id := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	if flow.ID != id {
		return validationError(ErrorFlowIDFilenameMismatch, nil)
	}
	return nil
}

func ErrorCodeOf(err error) (ErrorCode, bool) {
	var validationErr *ValidationError
	if errors.As(err, &validationErr) {
		return validationErr.Code, true
	}
	return "", false
}

func validationError(code ErrorCode, err error) error {
	return &ValidationError{Code: code, Err: err}
}

func blank(value string) bool {
	return strings.TrimSpace(value) == ""
}
