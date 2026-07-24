package command

import (
	"errors"
	"reflect"
	"testing"

	"github.com/8noki8/devflow/internal/state"
	"github.com/8noki8/devflow/internal/transition"
	"github.com/8noki8/devflow/internal/workpackage"
)

func TestWorkPackageReturnsProjectionWithoutChangingState(t *testing.T) {
	root := t.TempDir()
	if result := Init(Context{ProjectRoot: root}); result.ExitCode != 0 {
		t.Fatalf("Init() = %#v", result)
	}
	if result := startWithTestTask(t, root, "post-task-review"); result.ExitCode != 0 {
		t.Fatalf("Start() = %#v", result)
	}
	store := NewStore(Context{ProjectRoot: root})
	loaded := store.LoadCurrent()
	if loaded.Status != state.LoadOK || loaded.State == nil {
		t.Fatalf("LoadCurrent() = %#v", loaded)
	}
	before := loaded.State.Clone()
	result := WorkPackage(Context{ProjectRoot: root}, before.CurrentStepID, before.CurrentAttemptID)
	if result.ExitCode != 0 || result.WorkPackage == nil || len(result.Diagnostics) != 0 {
		t.Fatalf("WorkPackage() = %#v", result)
	}
	if err := workpackage.Validate(*result.WorkPackage); err != nil {
		t.Fatal(err)
	}
	after := store.LoadCurrent()
	if after.Status != state.LoadOK || !reflect.DeepEqual(before, *after.State) {
		t.Fatal("WorkPackage command changed persisted State")
	}
}

func TestWorkPackageDiagnostics(t *testing.T) {
	root := t.TempDir()
	if got := WorkPackage(Context{ProjectRoot: root}, "step", "attempt_00000000000000000001"); got.ExitCode != 1 ||
		len(got.Diagnostics) != 1 || got.Diagnostics[0].Code != CodeNoActiveFlow || got.WorkPackage != nil {
		t.Fatalf("no active flow result = %#v", got)
	}

	if result := Init(Context{ProjectRoot: root}); result.ExitCode != 0 {
		t.Fatalf("Init() = %#v", result)
	}
	if result := startWithTestTask(t, root, "post-task-review"); result.ExitCode != 0 {
		t.Fatalf("Start() = %#v", result)
	}
	st := NewStore(Context{ProjectRoot: root}).LoadCurrent().State
	tests := []struct {
		name, step, attempt, code string
	}{
		{"malformed", st.CurrentStepID, "bad", transition.CodeInvalidAttemptID},
		{"nonexistent", st.CurrentStepID, "attempt_00000000000000000002", transition.CodeInvalidAttemptID},
		{"step mismatch", "other", st.CurrentAttemptID, transition.CodeStepAttemptMismatch},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := WorkPackage(Context{ProjectRoot: root}, tt.step, tt.attempt)
			if got.ExitCode != 1 || got.WorkPackage != nil || len(got.Diagnostics) != 1 || got.Diagnostics[0].Code != tt.code {
				t.Fatalf("WorkPackage() = %#v", got)
			}
		})
	}
}

func TestWorkPackageFailureMapping(t *testing.T) {
	tests := []struct {
		err  error
		code string
	}{
		{workpackage.ErrNoActiveFlow, CodeNoActiveFlow},
		{workpackage.ErrInvalidState, CodeInvalidState},
		{workpackage.ErrInactiveAttempt, CodeInvalidState},
		{workpackage.ErrInvalidAttemptID, transition.CodeInvalidAttemptID},
		{workpackage.ErrAttemptNotFound, transition.CodeInvalidAttemptID},
		{workpackage.ErrStaleAttempt, transition.CodeStaleAttempt},
		{workpackage.ErrStepAttemptMismatch, transition.CodeStepAttemptMismatch},
		{workpackage.ErrDigestGeneration, CodeWorkPackageDigestFailed},
		{errors.New("other"), CodeInvalidWorkPackage},
	}
	for _, tt := range tests {
		got := workPackageFailure(tt.err)
		if got.ExitCode != 1 || len(got.Diagnostics) != 1 || got.Diagnostics[0].Code != tt.code {
			t.Fatalf("workPackageFailure(%v) = %#v", tt.err, got)
		}
	}
}
