package state

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/8noki8/devflow/internal/flow"
)

const testRunID = "run_00000000000000000000000000000000"

func TestStoreLoad(t *testing.T) {
	for _, status := range []Status{StatusRunning, StatusCompleted, StatusFinished} {
		t.Run(string(status), func(t *testing.T) {
			store := testStore(t)
			want := testState(t, status, "first")
			writeStateJSON(t, store.Path, want)

			got := store.Load()
			if got.Status != LoadOK || got.Err != nil || got.State == nil {
				t.Fatalf("Load() = %#v", got)
			}
			want.Normalize()
			if !reflect.DeepEqual(*got.State, want) {
				t.Fatalf("State = %#v, want %#v", *got.State, want)
			}
		})
	}
}

func TestStoreLoadNoState(t *testing.T) {
	got := testStore(t).Load()
	if got.Status != LoadNoState || got.State != nil || got.Err != nil {
		t.Fatalf("Load() = %#v", got)
	}
}

func TestStoreLoadRejectsBrokenJSONAndTypeMismatch(t *testing.T) {
	for _, tt := range []struct {
		name string
		json string
	}{
		{"broken json", `{"schema_version":3`},
		{"type mismatch", `{"schema_version":3,"flow_snapshot":{},"status":42,"current_step_id":"first"}`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			store := testStore(t)
			writeFile(t, store.Path, tt.json)
			got := store.Load()
			if got.Status != LoadInvalid || got.Err == nil || got.State != nil {
				t.Fatalf("Load() = %#v", got)
			}
		})
	}
}

func TestStoreLoadNormalizesStateCollections(t *testing.T) {
	store := testStore(t)
	value := testState(t, StatusRunning, "first")
	value.CompletedSteps = nil
	value.SkippedSteps = nil
	value.Approvals = nil
	value.BackHistory = nil
	value.CheckResults = nil
	writeStateJSON(t, store.Path, value)

	got := store.Load()
	if got.Status != LoadOK || got.State == nil {
		t.Fatalf("Load() = %#v", got)
	}
	if got.State.CompletedSteps == nil || got.State.SkippedSteps == nil || got.State.Approvals == nil || got.State.BackHistory == nil || got.State.CheckResults == nil {
		t.Fatalf("collections were not normalized: %#v", got.State)
	}
}

func TestStoreSaveRoundTripPreservesSnapshot(t *testing.T) {
	store := testStore(t)
	want := testState(t, StatusRunning, "first")

	if err := store.Save(want); err != nil {
		t.Fatal(err)
	}
	got := store.Load()
	if got.Status != LoadOK || got.State == nil {
		t.Fatalf("Load() = %#v", got)
	}
	if !reflect.DeepEqual(got.State.FlowSnapshot, want.FlowSnapshot) {
		t.Fatalf("FlowSnapshot changed:\ngot  %#v\nwant %#v", got.State.FlowSnapshot, want.FlowSnapshot)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(readFile(t, store.Path), &raw); err != nil {
		t.Fatal(err)
	}
	if _, ok := raw["flow_snapshot"]; !ok {
		t.Fatal("top-level flow_snapshot is missing")
	}
	if _, ok := raw["flow_id"]; ok {
		t.Fatal("legacy top-level flow_id is present")
	}
	for _, field := range []string{"completed_steps", "back_history"} {
		if got := string(raw[field]); got != "[]" {
			t.Fatalf("%s = %s, want []", field, got)
		}
	}
	for _, field := range []string{"skipped_steps", "approvals"} {
		if got := string(raw[field]); got != "{}" {
			t.Fatalf("%s = %s, want {}", field, got)
		}
	}
	if _, err := os.Stat(filepath.Dir(store.Path)); err != nil {
		t.Fatalf("parent directory was not created: %v", err)
	}
	matches, err := filepath.Glob(filepath.Join(filepath.Dir(store.Path), "*.tmp-*"))
	if err != nil || len(matches) != 0 {
		t.Fatalf("temporary files = %v, err = %v", matches, err)
	}
}

func TestStoreSaveAllStatuses(t *testing.T) {
	for _, status := range []Status{StatusRunning, StatusCompleted, StatusFinished} {
		t.Run(string(status), func(t *testing.T) {
			store := testStore(t)
			want := testState(t, status, "first")
			if status == StatusFinished {
				want.Finish = &Finish{Reason: "out of scope"}
			}
			if err := store.Save(want); err != nil {
				t.Fatal(err)
			}
			got := store.Load()
			if got.Status != LoadOK || got.State == nil || !reflect.DeepEqual(*got.State, want) {
				t.Fatalf("Load() = %#v, want %#v", got, want)
			}
		})
	}
}

func TestStoreSaveReplacesExistingState(t *testing.T) {
	store := testStore(t)
	if err := store.Save(testState(t, StatusRunning, "first")); err != nil {
		t.Fatal(err)
	}
	want := testState(t, StatusCompleted, "second")
	want.CompletedSteps = []string{"first", "second"}
	if err := store.Save(want); err != nil {
		t.Fatal(err)
	}
	got := store.Load()
	if got.Status != LoadOK || got.State == nil || !reflect.DeepEqual(*got.State, want) {
		t.Fatalf("Load() = %#v, want %#v", got, want)
	}
}

func TestStoreRejectsUnsupportedStateSchema(t *testing.T) {
	for _, version := range []int{0, 1, 2, 4, -1} {
		t.Run(strconv.Itoa(version), func(t *testing.T) {
			store := testStore(t)
			value := testState(t, StatusRunning, "first")
			value.SchemaVersion = version
			writeStateJSON(t, store.Path, value)
			got := store.Load()
			var unsupported *UnsupportedSchemaVersionError
			if got.Status != LoadInvalid || !errors.As(got.Err, &unsupported) || unsupported.Actual != version {
				t.Fatalf("Load() = %#v", got)
			}
		})
	}
}

func TestStoreRejectsMissingStateSchema(t *testing.T) {
	store := testStore(t)
	writeFile(t, store.Path, `{"status":"running"}`)
	got := store.Load()
	var unsupported *UnsupportedSchemaVersionError
	if got.Status != LoadInvalid || !errors.As(got.Err, &unsupported) {
		t.Fatalf("Load() = %#v", got)
	}
}

func TestStoreRejectsInvalidSnapshot(t *testing.T) {
	valid := testState(t, StatusRunning, "first")
	tests := []struct {
		name string
		edit func(*State)
		want error
	}{
		{"digest changed", func(s *State) { s.FlowSnapshot.Digest = "sha256:" + strings.Repeat("0", 64) }, flow.ErrSnapshotDigestMismatch},
		{"flow changed", func(s *State) { s.FlowSnapshot.Flow.Title = "tampered" }, flow.ErrSnapshotDigestMismatch},
		{"unsupported snapshot schema", func(s *State) { s.FlowSnapshot.SchemaVersion++ }, flow.ErrUnsupportedSnapshotSchema},
		{"not normalized", func(s *State) { s.FlowSnapshot.Flow.Steps[0].Inputs = nil }, flow.ErrSnapshotNotNormalized},
		{"invalid flow", func(s *State) { s.FlowSnapshot.Flow.ID = "" }, nil},
		{"invalid digest format", func(s *State) { s.FlowSnapshot.Digest = "SHA256:nope" }, flow.ErrInvalidSnapshotDigest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := testStore(t)
			value := valid.Clone()
			tt.edit(&value)
			writeStateJSON(t, store.Path, value)
			got := store.Load()
			if got.Status != LoadInvalid || got.Err == nil {
				t.Fatalf("Load() = %#v", got)
			}
			if tt.want != nil && !errors.Is(got.Err, tt.want) {
				t.Fatalf("error = %v, want errors.Is(%v)", got.Err, tt.want)
			}
		})
	}
}

func TestStoreSaveRejectsUnnormalizedSnapshot(t *testing.T) {
	store := testStore(t)
	value := testState(t, StatusRunning, "first")
	value.FlowSnapshot.Flow.Steps[0].Inputs = nil
	if err := store.Save(value); !errors.Is(err, flow.ErrSnapshotNotNormalized) {
		t.Fatalf("Save() error = %v, want %v", err, flow.ErrSnapshotNotNormalized)
	}
	if got := store.Load(); got.Status != LoadNoState {
		t.Fatalf("Load() after rejected Save = %#v", got)
	}
}

func TestStoreRejectsCurrentStepOutsideSnapshotForRunningState(t *testing.T) {
	store := testStore(t)
	value := testState(t, StatusRunning, "missing")
	writeStateJSON(t, store.Path, value)
	got := store.Load()
	if got.Status != LoadInvalid || got.Err == nil || !strings.Contains(got.Err.Error(), "current_step_id") {
		t.Fatalf("Load() = %#v", got)
	}
}

func TestStoreRequiresValidSnapshotForTerminalState(t *testing.T) {
	for _, status := range []Status{StatusCompleted, StatusFinished} {
		t.Run(string(status), func(t *testing.T) {
			store := testStore(t)
			value := testState(t, status, "first")
			value.FlowSnapshot = flow.FlowSnapshot{}
			writeStateJSON(t, store.Path, value)
			got := store.Load()
			if got.Status != LoadInvalid || got.Err == nil {
				t.Fatalf("Load() = %#v", got)
			}
		})
	}
}

func TestStoreRejectsInvalidRunContext(t *testing.T) {
	tests := []struct {
		name string
		edit func(*State)
	}{
		{"missing run id", func(s *State) { s.FlowRunID = "" }},
		{"bad run id", func(s *State) { s.FlowRunID = "run_BAD" }},
		{"zero entry sequence", func(s *State) { s.CurrentEntrySequence = 0 }},
		{"check entry mismatch", func(s *State) { s.CheckResults["test"] = CheckResult{EntrySequence: 2} }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := testStore(t)
			value := testState(t, StatusRunning, "first")
			tt.edit(&value)
			writeStateJSON(t, store.Path, value)
			got := store.Load()
			if got.Status != LoadInvalid || got.Err == nil {
				t.Fatalf("Load() = %#v", got)
			}
		})
	}
}

func TestStoreRejectsMissingRequiredFieldsAndUnknownStatus(t *testing.T) {
	valid := testState(t, StatusRunning, "first")
	data, err := json.Marshal(valid)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"flow_snapshot", "status", "current_step_id"} {
		t.Run("missing "+field, func(t *testing.T) {
			copy := make(map[string]json.RawMessage, len(raw))
			for key, value := range raw {
				copy[key] = value
			}
			delete(copy, field)
			store := testStore(t)
			writeRawJSON(t, store.Path, copy)
			got := store.Load()
			if got.Status != LoadInvalid || got.Err == nil {
				t.Fatalf("Load() = %#v", got)
			}
		})
	}

	valid.Status = "unknown"
	store := testStore(t)
	writeStateJSON(t, store.Path, valid)
	if got := store.Load(); got.Status != LoadInvalid || got.Err == nil {
		t.Fatalf("Load() = %#v", got)
	}
}

func testState(t testing.TB, status Status, currentStepID string) State {
	t.Helper()
	fl := flow.Flow{
		ID: "post-task-review", Title: "Post Task Review",
		Steps: []flow.Step{
			{ID: "first", Title: "First", Instruction: "Do first."},
			{ID: "second", Title: "Second", Instruction: "Do second."},
		},
	}
	snapshot, err := flow.BuildSnapshot(fl, flow.FlowSource{Path: ".devflow/flows/post-task-review.cue"})
	if err != nil {
		t.Fatal(err)
	}
	value := State{
		SchemaVersion: CurrentSchemaVersion, FlowSnapshot: snapshot, Status: status,
		CurrentStepID: currentStepID, FlowRunID: testRunID, CurrentEntrySequence: 1,
	}
	value.Normalize()
	return value
}

func testStore(t testing.TB) Store {
	t.Helper()
	return Store{Path: filepath.Join(t.TempDir(), ".devflow", "state.json")}
}

func writeStateJSON(t testing.TB, path string, value State) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, path, string(data))
}

func writeRawJSON(t testing.TB, path string, value any) {
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

func readFile(t testing.TB, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
