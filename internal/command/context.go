package command

import (
	"io"
	"path/filepath"

	"github.com/8noki8/devflow/internal/state"
)

type Context struct {
	ProjectRoot string
	Stdout      io.Writer
	Stderr      io.Writer
}

func LegacyStatePath(projectRoot string) string {
	return filepath.Join(projectRoot, ".devflow", "state.json")
}

func CurrentPath(projectRoot string) string {
	return filepath.Join(projectRoot, ".devflow", "current.json")
}
func RunsDir(projectRoot string) string { return filepath.Join(projectRoot, ".devflow", "runs") }

func FlowDir(projectRoot string) string {
	return filepath.Join(projectRoot, ".devflow", "flows")
}

func NewStore(ctx Context) state.Store {
	return state.Store{Root: filepath.Join(ctx.ProjectRoot, ".devflow")}
}
