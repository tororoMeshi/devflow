package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"
)

func TestStoreLoad(t *testing.T) {
	tests := []struct {
		name       string
		json       string
		wantStatus LoadStatus
		wantState  *State
		wantErr    bool
	}{
		{
			name:       "returns no_state when state file does not exist",
			wantStatus: LoadNoState,
		},
		{
			name: "loads valid running state",
			json: `{
				"schema_version": 2,
				"flow_id": "post-task-review",
				"status": "running",
				"current_step_id": "check_changes",
				"completed_steps": [],
				"skipped_steps": {},
				"approvals": {},
				"back_history": [],
				"finish": null
				,"flow_run_id":"run_00000000000000000000000000000000","current_entry_sequence":1
			}`,
			wantStatus: LoadOK,
			wantState: &State{
				FlowID:         "post-task-review",
				SchemaVersion:  CurrentSchemaVersion,
				Status:         StatusRunning,
				CurrentStepID:  "check_changes",
				CompletedSteps: []string{},
				SkippedSteps:   map[string]SkippedStep{},
				Approvals:      map[string]ApprovalRecord{},
				BackHistory:    []BackHistory{},
				FlowRunID:      "run_00000000000000000000000000000000", CurrentEntrySequence: 1, CheckResults: map[string]CheckResult{},
			},
		},
		{
			name: "loads valid completed state",
			json: `{
				"schema_version": 2,
				"flow_id": "post-task-review",
				"status": "completed",
				"current_step_id": "human_approval",
				"completed_steps": ["check_changes", "human_approval"],
				"skipped_steps": {},
				"approvals": {},
				"back_history": [],
				"finish": null
				,"flow_run_id":"run_00000000000000000000000000000000","current_entry_sequence":1
			}`,
			wantStatus: LoadOK,
			wantState: &State{
				FlowID:         "post-task-review",
				SchemaVersion:  CurrentSchemaVersion,
				Status:         StatusCompleted,
				CurrentStepID:  "human_approval",
				CompletedSteps: []string{"check_changes", "human_approval"},
				SkippedSteps:   map[string]SkippedStep{},
				Approvals:      map[string]ApprovalRecord{},
				BackHistory:    []BackHistory{},
				FlowRunID:      "run_00000000000000000000000000000000", CurrentEntrySequence: 1, CheckResults: map[string]CheckResult{},
			},
		},
		{
			name: "rejects v0.1.0 history without schema version",
			json: `{
				"flow_id": "post-task-review",
				"status": "running",
				"current_step_id": "check_changes",
				"back_history": [{"from_step_id":"summarize_changes","to_step_id":"check_changes","reason":"revise"}]
			}`,
			wantStatus: LoadInvalid, wantErr: true,
		},
		{
			name: "loads valid finished state",
			json: `{
				"schema_version": 2,
				"flow_id": "post-task-review",
				"status": "finished",
				"current_step_id": "check_changes",
				"completed_steps": [],
				"skipped_steps": {},
				"approvals": {},
				"back_history": [],
				"finish": {"reason": "out of scope"}
				,"flow_run_id":"run_00000000000000000000000000000000","current_entry_sequence":1
			}`,
			wantStatus: LoadOK,
			wantState: &State{
				FlowID:         "post-task-review",
				SchemaVersion:  CurrentSchemaVersion,
				Status:         StatusFinished,
				CurrentStepID:  "check_changes",
				CompletedSteps: []string{},
				SkippedSteps:   map[string]SkippedStep{},
				Approvals:      map[string]ApprovalRecord{},
				BackHistory:    []BackHistory{},
				Finish:         &Finish{Reason: "out of scope"},
				FlowRunID:      "run_00000000000000000000000000000000", CurrentEntrySequence: 1, CheckResults: map[string]CheckResult{},
			},
		},
		{
			name:       "returns invalid_state for broken json",
			json:       `{"flow_id":`,
			wantStatus: LoadInvalid,
			wantErr:    true,
		},
		{
			name: "returns invalid_state for unknown status",
			json: `{
				"schema_version": 2,
				"flow_id": "post-task-review",
				"status": "paused",
				"current_step_id": "check_changes"
			}`,
			wantStatus: LoadInvalid,
			wantErr:    true,
		},
		{
			name: "returns invalid_state for missing flow_id",
			json: `{
				"status": "running",
				"current_step_id": "check_changes"
			}`,
			wantStatus: LoadInvalid,
			wantErr:    true,
		},
		{
			name: "returns invalid_state for missing status",
			json: `{
				"flow_id": "post-task-review",
				"current_step_id": "check_changes"
			}`,
			wantStatus: LoadInvalid,
			wantErr:    true,
		},
		{
			name: "returns invalid_state for missing current_step_id",
			json: `{
				"flow_id": "post-task-review",
				"status": "running"
			}`,
			wantStatus: LoadInvalid,
			wantErr:    true,
		},
		{
			name: "returns invalid_state for type mismatch",
			json: `{
				"flow_id": "post-task-review",
				"status": "running",
				"current_step_id": "check_changes",
				"completed_steps": {}
			}`,
			wantStatus: LoadInvalid,
			wantErr:    true,
		},
		{
			name: "normalizes null collections",
			json: `{
				"schema_version": 2,
				"flow_id": "post-task-review",
				"status": "running",
				"current_step_id": "check_changes",
				"completed_steps": null,
				"skipped_steps": null,
				"approvals": null,
				"back_history": null,
				"finish": null
				,"flow_run_id":"run_00000000000000000000000000000000","current_entry_sequence":1
			}`,
			wantStatus: LoadOK,
			wantState: &State{
				FlowID:         "post-task-review",
				SchemaVersion:  CurrentSchemaVersion,
				Status:         StatusRunning,
				CurrentStepID:  "check_changes",
				CompletedSteps: []string{},
				SkippedSteps:   map[string]SkippedStep{},
				Approvals:      map[string]ApprovalRecord{},
				BackHistory:    []BackHistory{},
				FlowRunID:      "run_00000000000000000000000000000000", CurrentEntrySequence: 1, CheckResults: map[string]CheckResult{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			store := Store{Path: filepath.Join(root, ".devflow", "state.json")}
			if tt.json != "" {
				writeFile(t, store.Path, tt.json)
			}

			got := store.Load()

			assertLoadResult(t, got, tt.wantStatus, tt.wantState, tt.wantErr)
		})
	}
}

func TestStoreSave(t *testing.T) {
	tests := []struct {
		name  string
		state State
	}{
		{
			name: "saves running state",
			state: State{
				SchemaVersion: CurrentSchemaVersion,
				FlowID:        "post-task-review",
				Status:        StatusRunning,
				CurrentStepID: "check_changes",
				FlowRunID:     "run_00000000000000000000000000000000", CurrentEntrySequence: 1,
			},
		},
		{
			name: "saves completed state",
			state: State{
				SchemaVersion:  CurrentSchemaVersion,
				FlowID:         "post-task-review",
				Status:         StatusCompleted,
				CurrentStepID:  "human_approval",
				CompletedSteps: []string{"check_changes", "human_approval"},
				FlowRunID:      "run_00000000000000000000000000000000", CurrentEntrySequence: 1,
			},
		},
		{
			name: "saves finished state with reason",
			state: State{
				SchemaVersion: CurrentSchemaVersion,
				FlowID:        "post-task-review",
				Status:        StatusFinished,
				CurrentStepID: "check_changes",
				Finish:        &Finish{Reason: "out of scope"},
				FlowRunID:     "run_00000000000000000000000000000000", CurrentEntrySequence: 1,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			store := Store{Path: filepath.Join(root, ".devflow", "state.json")}

			if err := store.Save(tt.state); err != nil {
				t.Fatal(err)
			}

			got := store.Load()
			want := tt.state.Clone()
			assertLoadResult(t, got, LoadOK, &want, false)
			assertJSONHasArray(t, readFile(t, store.Path), "completed_steps")
			assertJSONHasObject(t, readFile(t, store.Path), "skipped_steps")
			assertJSONHasObject(t, readFile(t, store.Path), "approvals")
			assertJSONHasArray(t, readFile(t, store.Path), "back_history")
			assertNoTmpFile(t, filepath.Dir(store.Path))
		})
	}
}

func TestStoreSaveCreatesParentDirectory(t *testing.T) {
	root := t.TempDir()
	store := Store{Path: filepath.Join(root, ".devflow", "state.json")}

	if err := store.Save(minimalRunningState()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(store.Path); err != nil {
		t.Fatalf("state file was not created: %v", err)
	}
}

func TestStoreSaveReplacesExistingState(t *testing.T) {
	root := t.TempDir()
	store := Store{Path: filepath.Join(root, ".devflow", "state.json")}

	if err := store.Save(minimalRunningState()); err != nil {
		t.Fatal(err)
	}

	next := State{
		SchemaVersion:  CurrentSchemaVersion,
		FlowID:         "next-flow",
		Status:         StatusCompleted,
		CurrentStepID:  "last_step",
		CompletedSteps: []string{"last_step"},
		FlowRunID:      "run_00000000000000000000000000000000", CurrentEntrySequence: 1,
	}
	if err := store.Save(next); err != nil {
		t.Fatal(err)
	}

	want := next.Clone()
	assertLoadResult(t, store.Load(), LoadOK, &want, false)
	assertNoTmpFile(t, filepath.Dir(store.Path))
}

func TestStoreRejectsUnsupportedSchemaVersions(t *testing.T) {
	for _, version := range []int{0, 1, 3, -1} {
		t.Run(strconv.Itoa(version), func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, ".devflow", "state.json")
			writeFile(t, path, fmt.Sprintf(`{"schema_version":%d,"flow_id":"f","status":"running","current_step_id":"s","flow_run_id":"run_00000000000000000000000000000000","current_entry_sequence":1}`, version))
			if got := (Store{Path: path}).Load(); got.Status != LoadInvalid {
				t.Fatalf("status=%q, want invalid", got.Status)
			} else {
				var unsupported *UnsupportedSchemaVersionError
				if !errors.As(got.Err, &unsupported) || unsupported.Actual != version {
					t.Fatalf("err=%v", got.Err)
				}
			}
		})
	}
}

func TestStoreIdentifiesMissingSchemaVersionByType(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".devflow", "state.json")
	writeFile(t, path, `{"flow_id":"f","status":"running","current_step_id":"s"}`)
	got := (Store{Path: path}).Load()
	var unsupported *UnsupportedSchemaVersionError
	if got.Status != LoadInvalid || !errors.As(got.Err, &unsupported) {
		t.Fatalf("result=%#v", got)
	}
}

func TestStoreRejectsInvalidCheckContext(t *testing.T) {
	base := `{"schema_version":2,"flow_id":"f","status":"running","current_step_id":"s","flow_run_id":"run_00000000000000000000000000000000","current_entry_sequence":1,"check_results":%s}`
	for _, tt := range []struct {
		name string
		json string
	}{
		{"missing run id", `{"schema_version":2,"flow_id":"f","status":"running","current_step_id":"s","current_entry_sequence":1}`},
		{"zero entry", `{"schema_version":2,"flow_id":"f","status":"running","current_step_id":"s","flow_run_id":"run_00000000000000000000000000000000","current_entry_sequence":0}`},
		{"bad run id", `{"schema_version":2,"flow_id":"f","status":"running","current_step_id":"s","flow_run_id":"run_BAD","current_entry_sequence":1}`},
		{"mismatched result", fmt.Sprintf(base, `{"go-test":{"entry_sequence":2,"exit_code":0}}`)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, ".devflow", "state.json")
			writeFile(t, path, tt.json)
			if got := (Store{Path: path}).Load(); got.Status != LoadInvalid {
				t.Fatalf("status=%q, want invalid", got.Status)
			} else {
				var unsupported *UnsupportedSchemaVersionError
				if errors.As(got.Err, &unsupported) {
					t.Fatalf("unexpected unsupported schema error: %v", got.Err)
				}
			}
		})
	}
}

func minimalRunningState() State {
	return State{
		SchemaVersion: CurrentSchemaVersion,
		FlowID:        "post-task-review",
		Status:        StatusRunning,
		CurrentStepID: "check_changes",
		FlowRunID:     "run_00000000000000000000000000000000", CurrentEntrySequence: 1,
	}
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func assertLoadResult(t *testing.T, got LoadResult, wantStatus LoadStatus, wantState *State, wantErr bool) {
	t.Helper()

	if got.Status != wantStatus {
		t.Fatalf("Status = %q, want %q", got.Status, wantStatus)
	}
	if (got.Err != nil) != wantErr {
		t.Fatalf("Err = %v, wantErr %v", got.Err, wantErr)
	}
	if wantState != nil {
		want := wantState.Clone()
		wantState = &want
	}
	if !reflect.DeepEqual(got.State, wantState) {
		t.Fatalf("State = %#v, want %#v", got.State, wantState)
	}
}

func assertJSONHasArray(t *testing.T, data []byte, key string) {
	t.Helper()

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if _, ok := decoded[key].([]any); !ok {
		t.Fatalf("%s is %T, want array", key, decoded[key])
	}
}

func assertJSONHasObject(t *testing.T, data []byte, key string) {
	t.Helper()

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if _, ok := decoded[key].(map[string]any); !ok {
		t.Fatalf("%s is %T, want object", key, decoded[key])
	}
}

func assertNoTmpFile(t *testing.T, dir string) {
	t.Helper()

	matches, err := filepath.Glob(filepath.Join(dir, "state.json.tmp-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary files remain: %v", matches)
	}
}
