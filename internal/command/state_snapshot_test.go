package command

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/tororoMeshi/devflow/internal/flow"
	"github.com/tororoMeshi/devflow/internal/state"
)

func saveCommandState(t testing.TB, root string, value state.State) error {
	t.Helper()
	path := filepath.Join(FlowDir(root), value.FlowSnapshot.Flow.ID+".cue")
	if _, err := os.Stat(path); err == nil {
		loaded, err := flow.LoadFile(path)
		if err != nil {
			return err
		}
		snapshot, err := flow.BuildSnapshot(loaded, flow.FlowSource{Path: path})
		if err != nil {
			return err
		}
		value.FlowSnapshot = snapshot
	}
	store := NewStore(Context{ProjectRoot: root})
	if loaded := store.LoadCurrent(); loaded.Status == state.LoadNoState {
		return store.CreateRun(value)
	}
	return store.SaveCurrent(value)
}

func writeCommandStateUnchecked(t *testing.T, root string, value state.State) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	statePath, err := NewStore(Context{ProjectRoot: root}).RunStatePath(value.FlowRunID)
	if err != nil {
		t.Fatal(err)
	}
	writeCommandTestFile(t, statePath, string(data))
	pointer, err := json.Marshal(state.CurrentPointer{SchemaVersion: state.CurrentPointerSchemaVersion, FlowRunID: value.FlowRunID})
	if err != nil {
		t.Fatal(err)
	}
	writeCommandTestFile(t, CurrentPath(root), string(pointer))
}

func currentStatePath(t testing.TB, root string) string {
	t.Helper()
	store := NewStore(Context{ProjectRoot: root})
	loaded := store.LoadCurrent()
	if loaded.Status != state.LoadOK || loaded.State == nil {
		t.Fatalf("LoadCurrent() = %#v", loaded)
	}
	path, err := store.RunStatePath(loaded.State.FlowRunID)
	if err != nil {
		t.Fatal(err)
	}
	return path
}
