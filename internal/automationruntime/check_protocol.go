package automationruntime

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"strconv"
	"strings"
)

var (
	errCheckIdentityMismatch = errors.New("check identity mismatch")
	errInvalidLogPath        = errors.New("invalid log path")
)

type checkIdentity struct {
	SchemaVersion int    `json:"schema_version"`
	FlowRunID     string `json:"flow_run_id"`
	StepID        string `json:"step_id"`
	AttemptID     string `json:"attempt_id"`
	CheckID       string `json:"check_id"`
}

type checkRecordProjection struct {
	checkIdentity
	ExitCode int
	LogPath  *string
}

func validateCheckRequest(data []byte, pkg workPackageHeader, checkID string) error {
	var value struct {
		SchemaVersion *int    `json:"schema_version"`
		FlowRunID     *string `json:"flow_run_id"`
		StepID        *string `json:"step_id"`
		AttemptID     *string `json:"attempt_id"`
		CheckID       *string `json:"check_id"`
	}
	if err := strictDecode(data, &value); err != nil {
		return err
	}
	if value.SchemaVersion == nil || *value.SchemaVersion != 2 ||
		value.FlowRunID == nil || value.StepID == nil || value.AttemptID == nil || value.CheckID == nil {
		return errors.New("invalid check request")
	}
	if *value.FlowRunID != pkg.FlowRunID || *value.StepID != pkg.StepID ||
		*value.AttemptID != pkg.AttemptID || *value.CheckID != checkID {
		return errCheckIdentityMismatch
	}
	return nil
}

func parseCheckRecord(data []byte, pkg workPackageHeader, checkID, projectRoot string) (checkRecordProjection, error) {
	type resultProjection struct {
		ExitCode *int            `json:"exit_code"`
		LogPath  json.RawMessage `json:"log_path"`
	}
	var value struct {
		SchemaVersion *int              `json:"schema_version"`
		FlowRunID     *string           `json:"flow_run_id"`
		StepID        *string           `json:"step_id"`
		AttemptID     *string           `json:"attempt_id"`
		CheckID       *string           `json:"check_id"`
		Result        *resultProjection `json:"result"`
	}
	if err := strictDecode(data, &value); err != nil {
		return checkRecordProjection{}, err
	}
	if value.SchemaVersion == nil || *value.SchemaVersion != 2 || value.FlowRunID == nil ||
		value.StepID == nil || value.AttemptID == nil || value.CheckID == nil ||
		value.Result == nil || value.Result.ExitCode == nil || *value.Result.ExitCode < 0 {
		return checkRecordProjection{}, errors.New("invalid check record")
	}
	if *value.FlowRunID != pkg.FlowRunID || *value.StepID != pkg.StepID ||
		*value.AttemptID != pkg.AttemptID || *value.CheckID != checkID {
		return checkRecordProjection{}, errCheckIdentityMismatch
	}
	record := checkRecordProjection{
		checkIdentity: checkIdentity{SchemaVersion: 2, FlowRunID: *value.FlowRunID, StepID: *value.StepID,
			AttemptID: *value.AttemptID, CheckID: *value.CheckID},
		ExitCode: *value.Result.ExitCode,
	}
	if len(value.Result.LogPath) > 0 {
		if bytes.Equal(bytes.TrimSpace(value.Result.LogPath), []byte("null")) {
			return checkRecordProjection{}, errors.New("invalid check record")
		}
		var path string
		if err := json.Unmarshal(value.Result.LogPath, &path); err != nil {
			return checkRecordProjection{}, errors.New("invalid check record")
		}
		if !validRuntimeLogPath(path, projectRoot) {
			return checkRecordProjection{}, errInvalidLogPath
		}
		record.LogPath = &path
	}
	return record, nil
}

func strictDecode(data []byte, target any) error {
	if len(data) == 0 {
		return errors.New("empty JSON")
	}
	if err := rejectDuplicateKeys(data); err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("trailing JSON")
	}
	return nil
}

func rejectDuplicateKeys(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	var walk func() error
	walk = func() error {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		delim, ok := token.(json.Delim)
		if !ok {
			return nil
		}
		switch delim {
		case '{':
			seen := map[string]struct{}{}
			for decoder.More() {
				keyToken, err := decoder.Token()
				if err != nil {
					return err
				}
				key, ok := keyToken.(string)
				if !ok {
					return errors.New("invalid object key")
				}
				if _, exists := seen[key]; exists {
					return errors.New("duplicate JSON key")
				}
				seen[key] = struct{}{}
				if err := walk(); err != nil {
					return err
				}
			}
			_, err = decoder.Token()
			return err
		case '[':
			for decoder.More() {
				if err := walk(); err != nil {
					return err
				}
			}
			_, err = decoder.Token()
			return err
		default:
			return errors.New("unexpected JSON delimiter")
		}
	}
	if err := walk(); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("trailing JSON")
	}
	return nil
}

func validRuntimeLogPath(path, projectRoot string) bool {
	if path == "" || strings.ContainsAny(path, "\r\n\x00") || filepath.IsAbs(path) ||
		filepath.VolumeName(path) != "" || hasPortableVolumeName(path) {
		return false
	}
	portable := strings.ReplaceAll(path, `\`, "/")
	cleanPortable := filepath.ToSlash(filepath.Clean(portable))
	if cleanPortable == "." || cleanPortable == ".." || strings.HasPrefix(cleanPortable, "../") {
		return false
	}
	if cleanPortable == ".devflow" || strings.HasPrefix(cleanPortable, ".devflow/") {
		return false
	}
	clean := filepath.FromSlash(cleanPortable)
	joined := filepath.Join(projectRoot, clean)
	relative, err := filepath.Rel(projectRoot, joined)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func hasPortableVolumeName(path string) bool {
	if strings.HasPrefix(path, `\\`) || strings.HasPrefix(path, "//") {
		return true
	}
	return len(path) >= 2 && ((path[0] >= 'A' && path[0] <= 'Z') || (path[0] >= 'a' && path[0] <= 'z')) &&
		path[1] == ':'
}

func parseCheckRecordSuccess(data []byte, record checkRecordProjection) error {
	lines := strings.Split(string(data), "\n")
	if len(lines) != 6 || lines[5] != "" ||
		lines[0] != "Recorded check: "+record.CheckID ||
		lines[1] != "Run: "+record.FlowRunID ||
		lines[2] != "Step: "+record.StepID ||
		lines[3] != "Attempt: "+record.AttemptID {
		return errors.New("invalid check record success output")
	}
	const prefix = "Exit code: "
	if !strings.HasPrefix(lines[4], prefix) {
		return errors.New("invalid check exit code")
	}
	raw := strings.TrimPrefix(lines[4], prefix)
	if raw == "" || (len(raw) > 1 && raw[0] == '0') {
		return errors.New("invalid check exit code")
	}
	exitCode, err := strconv.ParseUint(raw, 10, 0)
	if err != nil || int(exitCode) != record.ExitCode {
		return errors.New("invalid check exit code")
	}
	return nil
}
