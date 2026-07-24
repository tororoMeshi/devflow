package executionreport

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/8noki8/devflow/internal/state"
)

func Record(projectRoot string, report Report) (RecordResult, error) {
	canonical, err := CanonicalBytes(report)
	if err != nil {
		return RecordResult{}, err
	}
	digest, err := Digest(report)
	if err != nil {
		return RecordResult{}, err
	}
	if !state.IsValidFlowRunID(report.FlowRunID) || !state.IsValidStepAttemptID(report.AttemptID) {
		return RecordResult{}, ErrInvalidReport
	}
	runDir := filepath.Join(projectRoot, ".devflow", "runs", report.FlowRunID)
	for _, dir := range []string{
		filepath.Join(projectRoot, ".devflow"),
		filepath.Join(projectRoot, ".devflow", "runs"),
	} {
		info, err := os.Lstat(dir)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return RecordResult{}, ErrUnsafeStore
		}
	}
	info, err := os.Lstat(runDir)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return RecordResult{}, ErrUnsafeStore
	}
	reportDir := filepath.Join(runDir, "execution-reports")
	if info, err := os.Lstat(reportDir); err == nil {
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return RecordResult{}, ErrUnsafeStore
		}
	} else if errors.Is(err, os.ErrNotExist) {
		if err := os.Mkdir(reportDir, 0o755); err != nil {
			if !errors.Is(err, os.ErrExist) {
				return RecordResult{}, ErrSave
			}
		} else {
			if err := os.Chmod(reportDir, 0o755); err != nil {
				return RecordResult{}, ErrSave
			}
		}
		info, err := os.Lstat(reportDir)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return RecordResult{}, ErrUnsafeStore
		}
	} else {
		return RecordResult{}, ErrUnsafeStore
	}
	target := filepath.Join(reportDir, report.AttemptID+".json")
	if _, err := os.Lstat(target); err == nil {
		return compareExisting(target, report, canonical, digest)
	} else if !errors.Is(err, os.ErrNotExist) {
		return RecordResult{}, ErrUnsafeStore
	}
	tmp, err := os.CreateTemp(reportDir, ".execution-report-*.tmp")
	if err != nil {
		return RecordResult{}, ErrSave
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return RecordResult{}, ErrSave
	}
	payload := append(append([]byte{}, canonical...), '\n')
	if _, err := tmp.Write(payload); err != nil {
		tmp.Close()
		return RecordResult{}, ErrSave
	}
	if err := tmp.Close(); err != nil {
		return RecordResult{}, ErrSave
	}
	if err := os.Link(tmpPath, target); err != nil {
		if _, statErr := os.Lstat(target); statErr == nil {
			return compareExisting(target, report, canonical, digest)
		}
		return RecordResult{}, ErrSave
	}
	return RecordResult{Digest: digest, Idempotent: false}, nil
}

func compareExisting(path string, incoming Report, canonical []byte, digest string) (RecordResult, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return RecordResult{}, ErrUnsafeStore
	}
	if info.Size() > MaxDocumentBytes+1 {
		return RecordResult{}, ErrInvalidExisting
	}
	file, err := os.Open(path)
	if err != nil {
		return RecordResult{}, ErrInvalidExisting
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, MaxDocumentBytes+2))
	if err != nil || len(data) > MaxDocumentBytes+1 || len(data) == 0 || data[len(data)-1] != '\n' {
		return RecordResult{}, ErrInvalidExisting
	}
	existing, err := Decode(data[:len(data)-1])
	if err != nil || existing.FlowRunID != incoming.FlowRunID || existing.AttemptID != incoming.AttemptID {
		return RecordResult{}, ErrInvalidExisting
	}
	existingCanonical, err := CanonicalBytes(existing)
	if err != nil || !bytes.Equal(data, append(append([]byte{}, existingCanonical...), '\n')) {
		return RecordResult{}, ErrInvalidExisting
	}
	if !bytes.Equal(existingCanonical, canonical) {
		return RecordResult{}, ErrConflict
	}
	return RecordResult{Digest: digest, Idempotent: true}, nil
}

func ReportPath(projectRoot, flowRunID, attemptID string) (string, error) {
	if !state.IsValidFlowRunID(flowRunID) || !state.IsValidStepAttemptID(attemptID) {
		return "", fmt.Errorf("%w: identifiers", ErrInvalidReport)
	}
	return filepath.Join(projectRoot, ".devflow", "runs", flowRunID, "execution-reports", attemptID+".json"), nil
}
