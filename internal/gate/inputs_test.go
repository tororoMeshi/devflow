package gate

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/8noki8/devflow/internal/flow"
	"github.com/8noki8/devflow/internal/state"
)

func TestCheckDoneGateInputs(t *testing.T) {
	root := t.TempDir()
	inputPath := filepath.Join(root, "docs", "request.md")
	if err := os.MkdirAll(filepath.Dir(inputPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(inputPath, []byte("request"), 0o644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name        string
		inputs      []flow.Artifact
		wantOK      bool
		wantMissing []string
	}{
		{name: "required input exists", inputs: []flow.Artifact{{Path: "docs/request.md", Required: true}}, wantOK: true, wantMissing: []string{}},
		{name: "required input missing", inputs: []flow.Artifact{{Path: "docs/missing.md", Required: true}}, wantMissing: []string{"docs/missing.md"}},
		{name: "optional input missing", inputs: []flow.Artifact{{Path: "docs/missing.md", Required: false}}, wantOK: true, wantMissing: []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CheckDoneGate(flow.Step{ID: "design", Inputs: tt.inputs}, state.State{}, root)
			if got.OK != tt.wantOK || !reflect.DeepEqual(got.MissingInputs, tt.wantMissing) {
				t.Fatalf("GateResult = %#v, want OK=%t MissingInputs=%#v", got, tt.wantOK, tt.wantMissing)
			}
		})
	}
}

func TestFileExistsRequiresRegularFile(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, "docs", "directory")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	if FileExists(root, "docs/directory") {
		t.Fatal("FileExists() = true for directory, want false")
	}
}
