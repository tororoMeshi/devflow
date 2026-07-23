package state

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/8noki8/devflow/internal/flow"
	"github.com/8noki8/devflow/internal/task"
)

const testRunID = "run_00000000000000000000000000000000"

func TestStoreCreateLoadAndSaveCurrent(t *testing.T) {
	store := testStore(t)
	want := testState(t, StatusRunning, "first")
	if err := store.CreateRun(want); err != nil {
		t.Fatal(err)
	}
	assertRegularFile(t, filepath.Join(store.RunsDir(), testRunID, "state.json"))
	assertRegularFile(t, store.CurrentPath())
	if _, err := os.Stat(store.LegacyPath()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("legacy state: %v", err)
	}
	loaded := store.LoadCurrent()
	if loaded.Status != LoadOK || !reflect.DeepEqual(*loaded.State, want) {
		t.Fatalf("LoadCurrent = %#v, want %#v", loaded, want)
	}

	closed, err := CloseStepAttempt(want.Attempts[0], StepAttemptExitDone, "")
	if err != nil {
		t.Fatal(err)
	}
	nextAttempt, err := NewStepAttempt("second", 2)
	if err != nil {
		t.Fatal(err)
	}
	want.Attempts[0] = closed
	want.Attempts = append(want.Attempts, nextAttempt)
	want.CurrentAttemptID = nextAttempt.ID
	want.CurrentStepID = "second"
	if err := store.SaveCurrent(want); err != nil {
		t.Fatal(err)
	}
	loaded = store.LoadCurrent()
	if loaded.Status != LoadOK || !reflect.DeepEqual(*loaded.State, want) {
		t.Fatalf("updated = %#v, want %#v", loaded, want)
	}
	assertNoTemps(t, store.Root)
}

func TestStoreCreateRunRejectsExistingRunAndKeepsPointer(t *testing.T) {
	store := testStore(t)
	value := testState(t, StatusRunning, "first")
	if err := store.CreateRun(value); err != nil {
		t.Fatal(err)
	}
	before := mustRead(t, store.CurrentPath())
	if err := store.CreateRun(value); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("CreateRun error = %v", err)
	}
	if got := mustRead(t, store.CurrentPath()); !reflect.DeepEqual(got, before) {
		t.Fatal("pointer changed")
	}
}

func TestStoreSaveCurrentRejectsDifferentRun(t *testing.T) {
	store := testStore(t)
	value := testState(t, StatusRunning, "first")
	if err := store.CreateRun(value); err != nil {
		t.Fatal(err)
	}
	value.FlowRunID = "run_11111111111111111111111111111111"
	if err := store.SaveCurrent(value); err == nil {
		t.Fatal("expected mismatch error")
	}
}

func TestStoreRejectsPointerFailures(t *testing.T) {
	tests := []struct{ name, content string }{
		{"broken JSON", "{"},
		{"trailing JSON", `{"schema_version":1,"flow_run_id":"` + testRunID + `"} {}`},
		{"unsupported schema", `{"schema_version":2,"flow_run_id":"` + testRunID + `"}`},
		{"invalid run id", `{"schema_version":1,"flow_run_id":"../escape"}`},
		{"missing run id", `{"schema_version":1}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := testStore(t)
			writeFile(t, store.CurrentPath(), tt.content)
			if got := store.LoadCurrent(); got.Status != LoadInvalid || got.Err == nil {
				t.Fatalf("LoadCurrent = %#v", got)
			}
		})
	}
}

func TestStoreCurrentPointerAllowsUnknownFieldsLikeStateJSON(t *testing.T) {
	store := testStore(t)
	value := testState(t, StatusRunning, "first")
	path, err := store.RunStatePath(value.FlowRunID)
	if err != nil {
		t.Fatal(err)
	}
	writeJSON(t, path, value)
	writeFile(t, store.CurrentPath(), `{"schema_version":1,"flow_run_id":"`+testRunID+`","future_field":true}`)
	if got := store.LoadCurrent(); got.Status != LoadOK {
		t.Fatalf("LoadCurrent = %#v", got)
	}
}

func TestStoreRejectsMissingAndMismatchedRunState(t *testing.T) {
	t.Run("missing", func(t *testing.T) {
		store := testStore(t)
		writePointer(t, store, testRunID)
		if got := store.LoadCurrent(); got.Status != LoadInvalid || got.Err == nil {
			t.Fatalf("LoadCurrent = %#v", got)
		}
	})
	t.Run("mismatch", func(t *testing.T) {
		store := testStore(t)
		other := testState(t, StatusRunning, "first")
		other.FlowRunID = "run_11111111111111111111111111111111"
		path, _ := store.RunStatePath(testRunID)
		writeJSON(t, path, other)
		writePointer(t, store, testRunID)
		if got := store.LoadCurrent(); got.Status != LoadInvalid || got.Err == nil {
			t.Fatalf("LoadCurrent = %#v", got)
		}
		if err := store.SaveCurrent(testState(t, StatusRunning, "first")); err == nil {
			t.Fatal("SaveCurrent accepted mismatched stored State")
		}
	})
}

func TestStoreRejectsInvalidState(t *testing.T) {
	store := testStore(t)
	value := testState(t, StatusRunning, "first")
	value.FlowSnapshot.Digest = "bad"
	if err := store.CreateRun(value); err == nil {
		t.Fatalf("CreateRun error = %v", err)
	}
	if got := store.LoadCurrent(); got.Status != LoadNoState {
		t.Fatalf("LoadCurrent = %#v", got)
	}
}

func TestStoreRejectsMissingNullEvidenceAndSchemaV6(t *testing.T) {
	for _, tt := range []struct {
		name   string
		value  func(*testing.T) State
		mutate func(map[string]any)
	}{
		{"missing evidence", nil, func(raw map[string]any) {
			delete(raw["attempts"].([]any)[0].(map[string]any), "artifact_evidence")
		}},
		{"null evidence", nil, func(raw map[string]any) {
			raw["attempts"].([]any)[0].(map[string]any)["artifact_evidence"] = nil
		}},
		{"invalid digest", nil, func(raw map[string]any) {
			raw["attempts"].([]any)[0].(map[string]any)["artifact_evidence"] = map[string]any{
				"out/report.md": map[string]any{"digest": "SHA256:bad", "size": float64(1)},
			}
		}},
		{"negative size", nil, func(raw map[string]any) {
			raw["attempts"].([]any)[0].(map[string]any)["artifact_evidence"] = map[string]any{
				"out/report.md": map[string]any{"digest": "sha256:" + strings.Repeat("a", 64), "size": float64(-1)},
			}
		}},
		{"unknown evidence path", nil, func(raw map[string]any) {
			raw["attempts"].([]any)[0].(map[string]any)["artifact_evidence"] = map[string]any{
				"out/unknown.md": map[string]any{"digest": "sha256:" + strings.Repeat("a", 64), "size": float64(1)},
			}
		}},
		{"optional evidence path", func(t *testing.T) State {
			value := testState(t, StatusRunning, "first")
			fl := value.FlowSnapshot.Flow
			fl.Steps[0].Artifacts = []flow.Artifact{{Path: "out/optional.md", Required: false}}
			var err error
			value.FlowSnapshot, err = flow.BuildSnapshot(fl, value.FlowSnapshot.Source)
			if err != nil {
				t.Fatal(err)
			}
			return value
		}, func(raw map[string]any) {
			raw["attempts"].([]any)[0].(map[string]any)["artifact_evidence"] = map[string]any{
				"out/optional.md": map[string]any{"digest": "sha256:" + strings.Repeat("a", 64), "size": float64(1)},
			}
		}},
		{"schema v6", nil, func(raw map[string]any) { raw["schema_version"] = float64(6) }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			store := testStore(t)
			value := testState(t, StatusRunning, "first")
			if tt.value != nil {
				value = tt.value(t)
			}
			data, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			var raw map[string]any
			if err := json.Unmarshal(data, &raw); err != nil {
				t.Fatal(err)
			}
			tt.mutate(raw)
			path, _ := store.RunStatePath(value.FlowRunID)
			writeJSON(t, path, raw)
			writePointer(t, store, value.FlowRunID)
			if got := store.LoadCurrent(); got.Status != LoadInvalid {
				t.Fatalf("LoadCurrent = %#v", got)
			}
		})
	}
}

func TestStoreRejectsInvalidTaskSnapshots(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*State)
	}{
		{"missing", func(s *State) { s.TaskSnapshot = task.TaskSnapshot{} }},
		{"schema", func(s *State) { s.TaskSnapshot.SchemaVersion++ }},
		{"content tampering", func(s *State) { s.TaskSnapshot.Content = "tampered\n" }},
		{"digest tampering", func(s *State) { s.TaskSnapshot.Digest = "sha256:" + strings.Repeat("0", 64) }},
		{"unnormalized CRLF", func(s *State) { s.TaskSnapshot.Content = "Test task\r\n" }},
		{"empty", func(s *State) { s.TaskSnapshot.Content = "" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := testStore(t)
			value := testState(t, StatusRunning, "first")
			tt.mutate(&value)
			if err := store.CreateRun(value); err == nil || !strings.Contains(err.Error(), "task_snapshot") {
				t.Fatalf("CreateRun error = %v", err)
			}
			if got := store.LoadCurrent(); got.Status != LoadNoState {
				t.Fatalf("LoadCurrent = %#v", got)
			}
		})
	}
}

func TestStoreLoadRejectsOldSchemaAndMissingTaskSnapshot(t *testing.T) {
	for _, tt := range []struct {
		name   string
		mutate func(map[string]any)
	}{
		{"schema 4", func(raw map[string]any) { raw["schema_version"] = float64(4) }},
		{"schema 5", func(raw map[string]any) {
			raw["schema_version"] = float64(5)
			raw["approvals"] = map[string]any{"first": map[string]any{"approved": true, "note": "old"}}
		}},
		{"schema 3", func(raw map[string]any) { raw["schema_version"] = float64(3) }},
		{"missing task snapshot", func(raw map[string]any) { delete(raw, "task_snapshot") }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			store := testStore(t)
			value := testState(t, StatusRunning, "first")
			data, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			var raw map[string]any
			if err := json.Unmarshal(data, &raw); err != nil {
				t.Fatal(err)
			}
			tt.mutate(raw)
			path, _ := store.RunStatePath(value.FlowRunID)
			writeJSON(t, path, raw)
			writePointer(t, store, value.FlowRunID)
			got := store.LoadCurrent()
			if got.Status != LoadInvalid || got.Err == nil {
				t.Fatalf("LoadCurrent = %#v", got)
			}
		})
	}
}

func TestStoreLoadRejectsLegacyApprovalFieldsInSchemaV6(t *testing.T) {
	for _, tt := range []struct {
		name   string
		mutate func(map[string]any)
	}{
		{"top-level approvals", func(raw map[string]any) {
			raw["approvals"] = map[string]any{"first": map[string]any{"note": "old"}}
		}},
		{"attempt approval approved", func(raw map[string]any) {
			attempt := raw["attempts"].([]any)[0].(map[string]any)
			attempt["approval"] = map[string]any{"approved": true, "note": "old"}
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			store := testStore(t)
			value := testState(t, StatusRunning, "first")
			data, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			var raw map[string]any
			if err := json.Unmarshal(data, &raw); err != nil {
				t.Fatal(err)
			}
			tt.mutate(raw)
			path, _ := store.RunStatePath(value.FlowRunID)
			writeJSON(t, path, raw)
			writePointer(t, store, value.FlowRunID)
			got := store.LoadCurrent()
			if got.Status != LoadInvalid || got.Err == nil {
				t.Fatalf("LoadCurrent = %#v", got)
			}
		})
	}
}

func TestStoreDetectsLegacyStateOnlyAndLeavesItUntouched(t *testing.T) {
	store := testStore(t)
	legacy := []byte("legacy")
	writeFile(t, store.LegacyPath(), string(legacy))
	got := store.LoadCurrent()
	var legacyErr *LegacyStateError
	if got.Status != LoadInvalid || !errors.As(got.Err, &legacyErr) {
		t.Fatalf("LoadCurrent = %#v", got)
	}
	if !reflect.DeepEqual(mustRead(t, store.LegacyPath()), legacy) {
		t.Fatal("legacy file changed")
	}

	value := testState(t, StatusRunning, "first")
	if err := store.CreateRun(value); err != nil {
		t.Fatal(err)
	}
	if got := store.LoadCurrent(); got.Status != LoadOK {
		t.Fatalf("valid pointer did not win: %#v", got)
	}
	if !reflect.DeepEqual(mustRead(t, store.LegacyPath()), legacy) {
		t.Fatal("legacy file changed")
	}
}

func TestStoreNoPointerAndNoLegacyReturnsNoState(t *testing.T) {
	store := testStore(t)
	if got := store.LoadCurrent(); got.Status != LoadNoState || got.Err != nil {
		t.Fatalf("LoadCurrent = %#v", got)
	}
}

func TestStoreBrokenPointerDoesNotFallBackToLegacy(t *testing.T) {
	store := testStore(t)
	legacy := []byte("legacy")
	writeFile(t, store.LegacyPath(), string(legacy))
	writeFile(t, store.CurrentPath(), "{")
	got := store.LoadCurrent()
	var legacyErr *LegacyStateError
	if got.Status != LoadInvalid || got.Err == nil || errors.As(got.Err, &legacyErr) {
		t.Fatalf("LoadCurrent = %#v", got)
	}
	if !reflect.DeepEqual(mustRead(t, store.LegacyPath()), legacy) {
		t.Fatal("legacy file changed")
	}
}

func TestStoreCreateRunPointerFailureLeavesUnreferencedRun(t *testing.T) {
	store := testStore(t)
	if err := os.MkdirAll(store.CurrentPath(), 0o755); err != nil {
		t.Fatal(err)
	}
	value := testState(t, StatusRunning, "first")
	if err := store.CreateRun(value); err == nil {
		t.Fatal("expected pointer save failure")
	}
	assertRegularFile(t, filepath.Join(store.RunsDir(), testRunID, "state.json"))
	if info, err := os.Stat(store.CurrentPath()); err != nil || !info.IsDir() {
		t.Fatalf("current target changed: %v", err)
	}
	assertNoTemps(t, store.Root)
}

func TestStoreCreateRunStateSaveFailureKeepsPreviousPointer(t *testing.T) {
	store := testStore(t)
	current := testState(t, StatusCompleted, "first")
	if err := store.CreateRun(current); err != nil {
		t.Fatal(err)
	}
	pointerBefore := mustRead(t, store.CurrentPath())

	next := testState(t, StatusRunning, "first")
	next.FlowRunID = "run_11111111111111111111111111111111"
	failing := Store{Root: store.Root, saveJSON: func(path string, value any) error {
		if path != store.CurrentPath() {
			return errors.New("injected state save failure")
		}
		return atomicSaveJSON(path, value)
	}}
	if err := failing.CreateRun(next); err == nil {
		t.Fatal("expected state save failure")
	}
	if got := mustRead(t, store.CurrentPath()); !reflect.DeepEqual(got, pointerBefore) {
		t.Fatal("pointer changed after state save failure")
	}
}

func TestStoreCreateRunPointerSaveFailureKeepsPreviousPointer(t *testing.T) {
	store := testStore(t)
	current := testState(t, StatusCompleted, "first")
	if err := store.CreateRun(current); err != nil {
		t.Fatal(err)
	}
	pointerBefore := mustRead(t, store.CurrentPath())

	next := testState(t, StatusRunning, "first")
	next.FlowRunID = "run_11111111111111111111111111111111"
	failing := Store{Root: store.Root, saveJSON: func(path string, value any) error {
		if path == store.CurrentPath() {
			return errors.New("injected pointer save failure")
		}
		return atomicSaveJSON(path, value)
	}}
	if err := failing.CreateRun(next); err == nil {
		t.Fatal("expected pointer save failure")
	}
	if got := mustRead(t, store.CurrentPath()); !reflect.DeepEqual(got, pointerBefore) {
		t.Fatal("previous pointer changed")
	}
	path, err := store.RunStatePath(next.FlowRunID)
	if err != nil {
		t.Fatal(err)
	}
	assertRegularFile(t, path)
}

func TestStoreInvalidStateDoesNotChangePointer(t *testing.T) {
	store := testStore(t)
	value := testState(t, StatusRunning, "first")
	if err := store.CreateRun(value); err != nil {
		t.Fatal(err)
	}
	before := mustRead(t, store.CurrentPath())
	bad := testState(t, StatusRunning, "missing")
	bad.FlowRunID = "run_11111111111111111111111111111111"
	if err := store.CreateRun(bad); err == nil {
		t.Fatal("expected state validation failure")
	}
	if got := mustRead(t, store.CurrentPath()); !reflect.DeepEqual(got, before) {
		t.Fatal("pointer changed")
	}
}

func TestStoreRunIDPathSafety(t *testing.T) {
	store := testStore(t)
	for _, id := range []string{"../escape", "run_../../escape", "/tmp/escape", "run_BAD"} {
		if _, err := store.RunStatePath(id); err == nil {
			t.Fatalf("RunStatePath(%q) accepted", id)
		}
		value := testState(t, StatusRunning, "first")
		value.FlowRunID = id
		if err := store.CreateRun(value); err == nil {
			t.Fatalf("CreateRun(%q) accepted", id)
		}
	}
}

func TestStorePreservesStateSchemaAndSnapshotValidation(t *testing.T) {
	for _, status := range []Status{StatusRunning, StatusCompleted, StatusFinished} {
		t.Run(string(status), func(t *testing.T) {
			store := testStore(t)
			value := testState(t, status, "first")
			if err := store.CreateRun(value); err != nil {
				t.Fatal(err)
			}
			if got := store.LoadCurrent(); got.Status != LoadOK || got.State.SchemaVersion != CurrentSchemaVersion {
				t.Fatalf("LoadCurrent = %#v", got)
			}
		})
	}
}

func TestStoreRoundTripsAttemptApproval(t *testing.T) {
	store := testStore(t)
	value := testState(t, StatusRunning, "first")
	fl := value.FlowSnapshot.Flow
	fl.Steps[0].Approval = &flow.Approval{Required: true}
	var err error
	value.FlowSnapshot, err = flow.BuildSnapshot(fl, value.FlowSnapshot.Source)
	if err != nil {
		t.Fatal(err)
	}
	value.Attempts[0].Approval = &ApprovalRecord{Note: "stored", EvidenceSetDigest: "sha256:d65728983c6fe0d4f09c0c18ad90370ea86c8b7e63e3367413abc99d88bda60f"}
	if err := store.CreateRun(value); err != nil {
		t.Fatal(err)
	}
	loaded := store.LoadCurrent()
	if loaded.Status != LoadOK || loaded.State.Attempts[0].Approval == nil || loaded.State.Attempts[0].Approval.Note != "stored" {
		t.Fatalf("LoadCurrent = %#v", loaded)
	}
	loaded.State.Attempts[0].Approval.Note = "saved"
	if err := store.SaveCurrent(*loaded.State); err != nil {
		t.Fatal(err)
	}
	again := store.LoadCurrent()
	if again.Status != LoadOK || again.State.Attempts[0].Approval.Note != "saved" {
		t.Fatalf("LoadCurrent after Save = %#v", again)
	}
}

func TestStoreLoadStrictApprovalEvidenceSetDigestV8(t *testing.T) {
	validState := func(t *testing.T) State {
		t.Helper()
		value := testState(t, StatusRunning, "first")
		fl := value.FlowSnapshot.Flow
		fl.Steps[0].Approval = &flow.Approval{Required: true}
		var err error
		value.FlowSnapshot, err = flow.BuildSnapshot(fl, value.FlowSnapshot.Source)
		if err != nil {
			t.Fatal(err)
		}
		value.Attempts[0].Approval = &ApprovalRecord{
			Note:              "reviewed",
			EvidenceSetDigest: emptyEvidenceSetDigest,
		}
		return value
	}

	tests := []struct {
		name   string
		mutate func(map[string]any)
	}{
		{"schema v7", func(raw map[string]any) { raw["schema_version"] = float64(7) }},
		{"missing digest", func(raw map[string]any) {
			delete(raw["attempts"].([]any)[0].(map[string]any)["approval"].(map[string]any), "evidence_set_digest")
		}},
		{"null digest", func(raw map[string]any) {
			raw["attempts"].([]any)[0].(map[string]any)["approval"].(map[string]any)["evidence_set_digest"] = nil
		}},
		{"number digest", func(raw map[string]any) {
			raw["attempts"].([]any)[0].(map[string]any)["approval"].(map[string]any)["evidence_set_digest"] = float64(1)
		}},
		{"bool digest", func(raw map[string]any) {
			raw["attempts"].([]any)[0].(map[string]any)["approval"].(map[string]any)["evidence_set_digest"] = true
		}},
		{"object digest", func(raw map[string]any) {
			raw["attempts"].([]any)[0].(map[string]any)["approval"].(map[string]any)["evidence_set_digest"] = map[string]any{}
		}},
		{"array digest", func(raw map[string]any) {
			raw["attempts"].([]any)[0].(map[string]any)["approval"].(map[string]any)["evidence_set_digest"] = []any{}
		}},
		{"approval null", func(raw map[string]any) {
			raw["attempts"].([]any)[0].(map[string]any)["approval"] = nil
		}},
		{"empty digest", func(raw map[string]any) {
			raw["attempts"].([]any)[0].(map[string]any)["approval"].(map[string]any)["evidence_set_digest"] = ""
		}},
		{"wrong prefix", func(raw map[string]any) {
			raw["attempts"].([]any)[0].(map[string]any)["approval"].(map[string]any)["evidence_set_digest"] = "sha512:" + strings.Repeat("a", 64)
		}},
		{"uppercase", func(raw map[string]any) {
			raw["attempts"].([]any)[0].(map[string]any)["approval"].(map[string]any)["evidence_set_digest"] = "sha256:" + strings.Repeat("A", 64)
		}},
		{"short", func(raw map[string]any) {
			raw["attempts"].([]any)[0].(map[string]any)["approval"].(map[string]any)["evidence_set_digest"] = "sha256:" + strings.Repeat("a", 63)
		}},
		{"long", func(raw map[string]any) {
			raw["attempts"].([]any)[0].(map[string]any)["approval"].(map[string]any)["evidence_set_digest"] = "sha256:" + strings.Repeat("a", 65)
		}},
		{"non-hex", func(raw map[string]any) {
			raw["attempts"].([]any)[0].(map[string]any)["approval"].(map[string]any)["evidence_set_digest"] = "sha256:" + strings.Repeat("g", 64)
		}},
		{"leading whitespace", func(raw map[string]any) {
			raw["attempts"].([]any)[0].(map[string]any)["approval"].(map[string]any)["evidence_set_digest"] = " " + emptyEvidenceSetDigest
		}},
		{"trailing whitespace", func(raw map[string]any) {
			raw["attempts"].([]any)[0].(map[string]any)["approval"].(map[string]any)["evidence_set_digest"] = emptyEvidenceSetDigest + " "
		}},
		{"unicode whitespace", func(raw map[string]any) {
			raw["attempts"].([]any)[0].(map[string]any)["approval"].(map[string]any)["evidence_set_digest"] = "\u3000" + emptyEvidenceSetDigest
		}},
		{"canonical mismatch", func(raw map[string]any) {
			raw["attempts"].([]any)[0].(map[string]any)["approval"].(map[string]any)["evidence_set_digest"] = "sha256:" + strings.Repeat("a", 64)
		}},
		{"legacy approved", func(raw map[string]any) {
			raw["attempts"].([]any)[0].(map[string]any)["approval"].(map[string]any)["approved"] = true
		}},
		{"top-level approvals", func(raw map[string]any) {
			raw["approvals"] = map[string]any{"first": map[string]any{"note": "old"}}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := testStore(t)
			value := validState(t)
			data, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			var raw map[string]any
			if err := json.Unmarshal(data, &raw); err != nil {
				t.Fatal(err)
			}
			tt.mutate(raw)
			path, _ := store.RunStatePath(value.FlowRunID)
			writeJSON(t, path, raw)
			writePointer(t, store, value.FlowRunID)
			got := store.LoadCurrent()
			if got.Status != LoadInvalid || got.Err == nil {
				t.Fatalf("LoadCurrent = %#v", got)
			}
		})
	}

	t.Run("valid approval", func(t *testing.T) {
		store := testStore(t)
		value := validState(t)
		if err := store.CreateRun(value); err != nil {
			t.Fatal(err)
		}
		got := store.LoadCurrent()
		if got.Status != LoadOK || got.State.Attempts[0].Approval.EvidenceSetDigest != emptyEvidenceSetDigest {
			t.Fatalf("LoadCurrent = %#v", got)
		}
	})
}

func testState(t testing.TB, status Status, currentStepID string) State {
	t.Helper()
	fl := flow.Flow{ID: "post-task-review", Title: "Post Task Review", Steps: []flow.Step{{ID: "first", Title: "First", Instruction: "Do first."}, {ID: "second", Title: "Second", Instruction: "Do second."}}}
	snapshot, err := flow.BuildSnapshot(fl, flow.FlowSource{Path: ".devflow/flows/post-task-review.cue"})
	if err != nil {
		t.Fatal(err)
	}
	taskSnapshot, err := task.BuildSnapshot("Test task\n", task.TaskSource{Path: "tasks/task.md"})
	if err != nil {
		t.Fatal(err)
	}
	attempt, err := NewStepAttempt(currentStepID, 1)
	if err != nil {
		t.Fatal(err)
	}
	value := State{SchemaVersion: CurrentSchemaVersion, FlowSnapshot: snapshot, TaskSnapshot: taskSnapshot, Status: status, CurrentStepID: currentStepID, FlowRunID: testRunID, Attempts: []StepAttempt{attempt}, CurrentAttemptID: attempt.ID}
	if status != StatusRunning {
		reason := StepAttemptExitDone
		closeReason := ""
		if status == StatusFinished {
			reason = StepAttemptExitFinish
			closeReason = "finished"
			value.Finish = &Finish{Reason: closeReason}
		}
		value.Attempts[0], err = CloseStepAttempt(attempt, reason, closeReason)
		if err != nil {
			t.Fatal(err)
		}
		value.CurrentAttemptID = ""
	}
	value.Normalize()
	return value
}

func testStore(t testing.TB) Store {
	t.Helper()
	return Store{Root: filepath.Join(t.TempDir(), ".devflow")}
}
func writePointer(t testing.TB, store Store, id string) {
	t.Helper()
	writeJSON(t, store.CurrentPath(), CurrentPointer{SchemaVersion: 1, FlowRunID: id})
}
func writeJSON(t testing.TB, path string, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, path, string(data))
}
func writeFile(t testing.TB, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
func mustRead(t testing.TB, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
func assertRegularFile(t testing.TB, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		t.Fatalf("%s not regular: %v", path, err)
	}
}
func assertNoTemps(t testing.TB, root string) {
	t.Helper()
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if strings.Contains(d.Name(), ".tmp-") {
			t.Fatalf("temporary file remains: %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
