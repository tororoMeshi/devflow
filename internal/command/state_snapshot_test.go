package command

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/8noki8/devflow/internal/flow"
	"github.com/8noki8/devflow/internal/state"
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
	return NewStore(Context{ProjectRoot: root}).Save(value)
}

func writeCommandStateUnchecked(t *testing.T, root string, value state.State) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	writeCommandTestFile(t, StatePath(root), string(data))
}
