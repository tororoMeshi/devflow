package state

import (
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"
)

func TestStepAttemptID(t *testing.T) {
	for _, tt := range []struct {
		name     string
		sequence uint64
		want     string
	}{
		{"one", 1, "attempt_00000000000000000001"},
		{"forty two", 42, "attempt_00000000000000000042"},
		{"maximum uint64", math.MaxUint64, "attempt_18446744073709551615"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := StepAttemptID(tt.sequence)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("StepAttemptID(%d) = %q, want %q", tt.sequence, got, tt.want)
			}
			if strings.ContainsAny(got, "/\\") {
				t.Fatalf("StepAttemptID(%d) contains a path separator: %q", tt.sequence, got)
			}
		})
	}

	first, err := StepAttemptID(42)
	if err != nil {
		t.Fatal(err)
	}
	again, err := StepAttemptID(42)
	if err != nil {
		t.Fatal(err)
	}
	other, err := StepAttemptID(43)
	if err != nil {
		t.Fatal(err)
	}
	if first != again {
		t.Fatalf("same sequence produced %q and %q", first, again)
	}
	if first == other {
		t.Fatalf("different sequences produced %q", first)
	}

	if _, err := StepAttemptID(0); !errors.Is(err, ErrInvalidStepAttemptEntrySequence) {
		t.Fatalf("StepAttemptID(0) error = %v", err)
	}
}

func TestNewStepAttempt(t *testing.T) {
	attempt, err := NewStepAttempt(" implement ", 1)
	if err != nil {
		t.Fatal(err)
	}
	want := StepAttempt{
		ID:               "attempt_00000000000000000001",
		StepID:           " implement ",
		EntrySequence:    1,
		Status:           StepAttemptActive,
		ArtifactEvidence: map[string]ArtifactEvidence{},
		CheckResults:     map[string]CheckResult{},
	}
	if !reflect.DeepEqual(attempt, want) {
		t.Fatalf("NewStepAttempt() = %#v, want %#v", attempt, want)
	}
	if attempt.CheckResults == nil {
		t.Fatal("CheckResults is nil")
	}
	if attempt.ArtifactEvidence == nil {
		t.Fatal("ArtifactEvidence is nil")
	}
	if attempt.Approval != nil {
		t.Fatalf("Approval = %#v, want nil", attempt.Approval)
	}

	for _, stepID := range []string{"", " \t\n ", "\u3000\u2003"} {
		if _, err := NewStepAttempt(stepID, 1); !errors.Is(err, ErrInvalidStepAttemptStepID) {
			t.Fatalf("NewStepAttempt(%q, 1) error = %v", stepID, err)
		}
	}
	if _, err := NewStepAttempt("step", 0); !errors.Is(err, ErrInvalidStepAttemptEntrySequence) {
		t.Fatalf("NewStepAttempt sequence error = %v", err)
	}
}

func TestApproveStepAttempt(t *testing.T) {
	original := mustNewStepAttempt(t)
	original.CheckResults["check"] = CheckResult{ExitCode: 0}
	before := withAttempt(original, func(*StepAttempt) {})
	approved, err := ApproveStepAttempt(original, " approved ", "sha256:d65728983c6fe0d4f09c0c18ad90370ea86c8b7e63e3367413abc99d88bda60f")
	if err != nil {
		t.Fatal(err)
	}
	if approved.Approval == nil || approved.Approval.Note != " approved " {
		t.Fatalf("Approval = %#v", approved.Approval)
	}
	if approved.Approval.EvidenceSetDigest != emptyEvidenceSetDigest {
		t.Fatalf("EvidenceSetDigest = %q", approved.Approval.EvidenceSetDigest)
	}
	if !reflect.DeepEqual(original, before) || original.Approval != nil {
		t.Fatal("input attempt changed")
	}
	approved.CheckResults["check"] = CheckResult{ExitCode: 1}
	if original.CheckResults["check"].ExitCode != 0 {
		t.Fatal("CheckResults map is shared")
	}
	approved.ArtifactEvidence["out/report.md"] = ArtifactEvidence{
		Digest: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Size:   1,
	}
	if len(original.ArtifactEvidence) != 0 {
		t.Fatal("ArtifactEvidence map is shared")
	}

	for _, note := range []string{"", " \t\n", "\u3000\u2003"} {
		if _, err := ApproveStepAttempt(original, note, "sha256:d65728983c6fe0d4f09c0c18ad90370ea86c8b7e63e3367413abc99d88bda60f"); !errors.Is(err, ErrInvalidApprovalNote) {
			t.Fatalf("note %q: %v", note, err)
		}
	}
	for _, digest := range []string{"", " " + emptyEvidenceSetDigest, "sha256:" + strings.Repeat("A", 64)} {
		if _, err := ApproveStepAttempt(original, "ok", digest); !errors.Is(err, ErrInvalidEvidenceSetDigest) {
			t.Fatalf("digest %q: %v", digest, err)
		}
	}
	closed := mustCloseStepAttempt(t, original, StepAttemptExitDone, "")
	if _, err := ApproveStepAttempt(closed, "ok", "sha256:d65728983c6fe0d4f09c0c18ad90370ea86c8b7e63e3367413abc99d88bda60f"); !errors.Is(err, ErrStepAttemptNotActive) {
		t.Fatalf("closed: %v", err)
	}
	if _, err := ApproveStepAttempt(approved, "replacement", "sha256:d65728983c6fe0d4f09c0c18ad90370ea86c8b7e63e3367413abc99d88bda60f"); !errors.Is(err, ErrStepAttemptAlreadyApproved) {
		t.Fatalf("duplicate: %v", err)
	}
	if approved.Approval.Note != " approved " {
		t.Fatal("duplicate approval overwrote note")
	}

	closedApproved, err := CloseStepAttempt(approved, StepAttemptExitDone, "")
	if err != nil {
		t.Fatal(err)
	}
	if closedApproved.Approval == nil || closedApproved.Approval.Note != " approved " {
		t.Fatal("close lost approval")
	}
	closedApproved.Approval.Note = "changed"
	closedApproved.Approval.EvidenceSetDigest = "sha256:" + strings.Repeat("b", 64)
	if approved.Approval.Note != " approved " {
		t.Fatal("CloseStepAttempt shares Approval pointer")
	}
	if approved.Approval.EvidenceSetDigest != emptyEvidenceSetDigest {
		t.Fatal("CloseStepAttempt shares Approval digest")
	}
}

func TestIsValidStepAttemptID(t *testing.T) {
	if !IsValidStepAttemptID("attempt_00000000000000000001") {
		t.Fatal("valid ID rejected")
	}
	for _, value := range []string{"", "attempt_1", "attempt_0000000000000000000", "attempt_000000000000000000001", "attempt_0000000000000000000x", "other_00000000000000000001", "attempt_00000000000000000000", "attempt_0000000000000000000/"} {
		if IsValidStepAttemptID(value) {
			t.Fatalf("invalid ID accepted: %q", value)
		}
	}
}

func TestValidateStepAttempt(t *testing.T) {
	validActive := mustNewStepAttempt(t)
	validDone := mustCloseStepAttempt(t, validActive, StepAttemptExitDone, "")
	validSkip := mustCloseStepAttempt(t, validActive, StepAttemptExitSkip, "not applicable")
	validBack := mustCloseStepAttempt(t, validActive, StepAttemptExitBack, "needs revision")
	validFinish := mustCloseStepAttempt(t, validActive, StepAttemptExitFinish, "scope complete")

	for _, tt := range []struct {
		name    string
		attempt StepAttempt
		wantErr error
	}{
		{"valid active", validActive, nil},
		{"valid closed done", validDone, nil},
		{"valid closed skip", validSkip, nil},
		{"valid closed back", validBack, nil},
		{"valid closed finish", validFinish, nil},
		{"invalid sequence", StepAttempt{ID: "attempt_00000000000000000000", StepID: "step", Status: StepAttemptActive, CheckResults: map[string]CheckResult{}}, ErrInvalidStepAttemptEntrySequence},
		{"empty id", withAttempt(validActive, func(a *StepAttempt) { a.ID = "" }), ErrInvalidStepAttemptID},
		{"mismatched id", withAttempt(validActive, func(a *StepAttempt) { a.ID = "attempt_00000000000000000002" }), ErrInvalidStepAttemptID},
		{"id with too few digits", withAttempt(validActive, func(a *StepAttempt) { a.ID = "attempt_0000000000000000001" }), ErrInvalidStepAttemptID},
		{"id with too many digits", withAttempt(validActive, func(a *StepAttempt) { a.ID = "attempt_000000000000000000001" }), ErrInvalidStepAttemptID},
		{"id with non-digit", withAttempt(validActive, func(a *StepAttempt) { a.ID = "attempt_0000000000000000000x" }), ErrInvalidStepAttemptID},
		{"invalid step id", withAttempt(validActive, func(a *StepAttempt) { a.StepID = " \t" }), ErrInvalidStepAttemptStepID},
		{"unknown status", withAttempt(validActive, func(a *StepAttempt) { a.Status = "unknown" }), ErrInvalidStepAttemptStatus},
		{"active exit reason", withAttempt(validActive, func(a *StepAttempt) { a.ExitReason = StepAttemptExitDone }), ErrInvalidStepAttemptExitReason},
		{"active unknown exit reason", withAttempt(validActive, func(a *StepAttempt) { a.ExitReason = "unknown" }), ErrInvalidStepAttemptExitReason},
		{"active reason", withAttempt(validActive, func(a *StepAttempt) { a.Reason = "why" }), ErrInvalidStepAttemptReason},
		{"active exit reason and reason", withAttempt(validActive, func(a *StepAttempt) { a.ExitReason, a.Reason = StepAttemptExitSkip, "why" }), ErrInvalidStepAttemptExitReason},
		{"closed missing exit reason", withAttempt(validActive, func(a *StepAttempt) { a.Status = StepAttemptClosed }), ErrInvalidStepAttemptExitReason},
		{"unknown exit reason", withAttempt(validDone, func(a *StepAttempt) { a.ExitReason = "unknown" }), ErrInvalidStepAttemptExitReason},
		{"done reason", withAttempt(validDone, func(a *StepAttempt) { a.Reason = "why" }), ErrInvalidStepAttemptReason},
		{"done blank reason", withAttempt(validDone, func(a *StepAttempt) { a.Reason = " \t" }), ErrInvalidStepAttemptReason},
		{"skip missing reason", withAttempt(validSkip, func(a *StepAttempt) { a.Reason = "" }), ErrInvalidStepAttemptReason},
		{"back blank reason", withAttempt(validBack, func(a *StepAttempt) { a.Reason = " \t" }), ErrInvalidStepAttemptReason},
		{"finish blank reason", withAttempt(validFinish, func(a *StepAttempt) { a.Reason = "\n" }), ErrInvalidStepAttemptReason},
		{"nil check results", withAttempt(validActive, func(a *StepAttempt) { a.CheckResults = nil }), ErrNilStepAttemptCheckResults},
		{"nil artifact evidence", withAttempt(validActive, func(a *StepAttempt) { a.ArtifactEvidence = nil }), ErrNilArtifactEvidence},
		{"blank approval note", withAttempt(validActive, func(a *StepAttempt) { a.Approval = &ApprovalRecord{Note: " \t"} }), ErrInvalidApprovalNote},
		{"missing approval digest", withAttempt(validActive, func(a *StepAttempt) {
			a.Approval = &ApprovalRecord{Note: "ok"}
		}), ErrInvalidEvidenceSetDigest},
		{"invalid approval digest", withAttempt(validActive, func(a *StepAttempt) {
			a.Approval = &ApprovalRecord{Note: "ok", EvidenceSetDigest: "SHA256:bad"}
		}), ErrInvalidEvidenceSetDigest},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateStepAttempt(tt.attempt)
			if tt.wantErr == nil {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("ValidateStepAttempt() error = %v, want errors.Is(_, %v)", err, tt.wantErr)
			}
		})
	}
}

func TestCloseStepAttempt(t *testing.T) {
	for _, tt := range []struct {
		name       string
		exitReason StepAttemptExitReason
		reason     string
	}{
		{"done", StepAttemptExitDone, ""},
		{"skip", StepAttemptExitSkip, " not applicable "},
		{"back", StepAttemptExitBack, " needs revision "},
		{"finish", StepAttemptExitFinish, " scope complete "},
	} {
		t.Run(tt.name, func(t *testing.T) {
			original := mustNewStepAttempt(t)
			original.CheckResults = map[string]CheckResult{
				"build": {ExitCode: 0, LogPath: "logs/build.log"},
				"lint":  {ExitCode: 1, LogPath: "logs/lint.log"},
			}
			before := withAttempt(original, func(*StepAttempt) {})

			closed, err := CloseStepAttempt(original, tt.exitReason, tt.reason)
			if err != nil {
				t.Fatal(err)
			}
			if closed.Status != StepAttemptClosed || closed.ExitReason != tt.exitReason || closed.Reason != tt.reason {
				t.Fatalf("CloseStepAttempt() = %#v", closed)
			}
			if !reflect.DeepEqual(original, before) {
				t.Fatalf("CloseStepAttempt changed input: got %#v, want %#v", original, before)
			}
			if closed.ID != original.ID || closed.StepID != original.StepID || closed.EntrySequence != original.EntrySequence || !reflect.DeepEqual(closed.CheckResults, original.CheckResults) {
				t.Fatalf("CloseStepAttempt did not preserve attempt identity or check results: %#v", closed)
			}
			closed.CheckResults["build"] = CheckResult{ExitCode: 2}
			closed.CheckResults["new"] = CheckResult{ExitCode: 0}
			if original.CheckResults["build"].ExitCode != 0 || len(original.CheckResults) != 2 {
				t.Fatalf("CloseStepAttempt shares CheckResults: %#v", original.CheckResults)
			}
			original.CheckResults["input-only"] = CheckResult{ExitCode: 3}
			if _, ok := closed.CheckResults["input-only"]; ok {
				t.Fatalf("closed CheckResults changed through input map: %#v", closed.CheckResults)
			}
			if _, err := CloseStepAttempt(closed, tt.exitReason, tt.reason); !errors.Is(err, ErrStepAttemptNotActive) {
				t.Fatalf("second CloseStepAttempt error = %v", err)
			}
		})
	}

	active := mustNewStepAttempt(t)
	for _, tt := range []struct {
		name       string
		exitReason StepAttemptExitReason
		reason     string
		wantErr    error
	}{
		{"unknown exit", "unknown", "reason", ErrInvalidStepAttemptExitReason},
		{"done reason", StepAttemptExitDone, "reason", ErrInvalidStepAttemptReason},
		{"done blank reason", StepAttemptExitDone, " \t", ErrInvalidStepAttemptReason},
		{"skip empty reason", StepAttemptExitSkip, "", ErrInvalidStepAttemptReason},
		{"back blank reason", StepAttemptExitBack, " \t", ErrInvalidStepAttemptReason},
		{"finish blank reason", StepAttemptExitFinish, "\n", ErrInvalidStepAttemptReason},
	} {
		t.Run(tt.name, func(t *testing.T) {
			input := withAttempt(active, func(a *StepAttempt) {
				a.CheckResults["check"] = CheckResult{ExitCode: 0}
			})
			before := withAttempt(input, func(*StepAttempt) {})
			if _, err := CloseStepAttempt(input, tt.exitReason, tt.reason); !errors.Is(err, tt.wantErr) {
				t.Fatalf("CloseStepAttempt() error = %v, want errors.Is(_, %v)", err, tt.wantErr)
			}
			if !reflect.DeepEqual(input, before) {
				t.Fatalf("failed CloseStepAttempt changed input: got %#v, want %#v", input, before)
			}
		})
	}

	invalid := mustNewStepAttempt(t)
	invalid.CheckResults = nil
	if _, err := CloseStepAttempt(invalid, StepAttemptExitDone, ""); !errors.Is(err, ErrNilStepAttemptCheckResults) {
		t.Fatalf("CloseStepAttempt invalid input error = %v", err)
	}

	empty := mustNewStepAttempt(t)
	closed, err := CloseStepAttempt(empty, StepAttemptExitDone, "")
	if err != nil {
		t.Fatal(err)
	}
	if closed.CheckResults == nil || len(closed.CheckResults) != 0 {
		t.Fatalf("empty CheckResults = %#v", closed.CheckResults)
	}
}

func TestStepAttemptJSON(t *testing.T) {
	active := mustNewStepAttempt(t)
	active.StepID = "implement"
	closed := mustCloseStepAttempt(t, active, StepAttemptExitSkip, "not applicable")
	approved, err := ApproveStepAttempt(active, "ok", "sha256:d65728983c6fe0d4f09c0c18ad90370ea86c8b7e63e3367413abc99d88bda60f")
	if err != nil {
		t.Fatal(err)
	}

	for _, tt := range []struct {
		name    string
		attempt StepAttempt
		want    string
	}{
		{"active", active, `{"id":"attempt_00000000000000000001","step_id":"implement","entry_sequence":1,"status":"active","artifact_evidence":{},"check_results":{}}`},
		{"closed skip", closed, `{"id":"attempt_00000000000000000001","step_id":"implement","entry_sequence":1,"status":"closed","exit_reason":"skip","reason":"not applicable","artifact_evidence":{},"check_results":{}}`},
		{"approved", approved, `{"id":"attempt_00000000000000000001","step_id":"implement","entry_sequence":1,"status":"active","artifact_evidence":{},"check_results":{},"approval":{"note":"ok","evidence_set_digest":"sha256:d65728983c6fe0d4f09c0c18ad90370ea86c8b7e63e3367413abc99d88bda60f"}}`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(tt.attempt)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tt.want {
				t.Fatalf("json.Marshal() = %s, want %s", got, tt.want)
			}

			var roundTripped StepAttempt
			if err := json.Unmarshal(got, &roundTripped); err != nil {
				t.Fatal(err)
			}
			if err := ValidateStepAttempt(roundTripped); err != nil {
				t.Fatalf("ValidateStepAttempt after JSON round trip: %v", err)
			}
			if !reflect.DeepEqual(roundTripped, tt.attempt) {
				t.Fatalf("JSON round trip = %#v, want %#v", roundTripped, tt.attempt)
			}
		})
	}
}

func TestStepAttemptJSONRejectsMissingOrNullCheckResults(t *testing.T) {
	for _, tt := range []struct {
		name string
		data string
	}{
		{"missing", `{"id":"attempt_00000000000000000001","step_id":"implement","entry_sequence":1,"status":"active"}`},
		{"null", `{"id":"attempt_00000000000000000001","step_id":"implement","entry_sequence":1,"status":"active","check_results":null}`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var attempt StepAttempt
			if err := json.Unmarshal([]byte(tt.data), &attempt); err != nil {
				t.Fatal(err)
			}
			if !errors.Is(ValidateStepAttempt(attempt), ErrNilStepAttemptCheckResults) {
				t.Fatalf("ValidateStepAttempt(%s) did not reject nil CheckResults", tt.data)
			}
		})
	}
}

func mustNewStepAttempt(t *testing.T) StepAttempt {
	t.Helper()
	attempt, err := NewStepAttempt("step", 1)
	if err != nil {
		t.Fatal(err)
	}
	return attempt
}

func mustCloseStepAttempt(t *testing.T, attempt StepAttempt, exitReason StepAttemptExitReason, reason string) StepAttempt {
	t.Helper()
	closed, err := CloseStepAttempt(attempt, exitReason, reason)
	if err != nil {
		t.Fatal(err)
	}
	return closed
}

func withAttempt(attempt StepAttempt, mutate func(*StepAttempt)) StepAttempt {
	copy := attempt
	copy.ArtifactEvidence = cloneArtifactEvidence(attempt.ArtifactEvidence)
	copy.CheckResults = cloneStepAttemptCheckResults(attempt.CheckResults)
	mutate(&copy)
	return copy
}
