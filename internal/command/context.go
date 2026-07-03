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

func StatePath(projectRoot string) string {
	return filepath.Join(projectRoot, ".devflow", "state.json")
}

func FlowDir(projectRoot string) string {
	return filepath.Join(projectRoot, ".devflow", "flows")
}

func NewStore(ctx Context) state.Store {
	return state.Store{Path: StatePath(ctx.ProjectRoot)}
}
