//go:build !unix

package automationruntime

import (
	"errors"
	"os"
	"os/exec"
)

func configureProcessGroup(*exec.Cmd) {}

func terminateProcess(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	err := cmd.Process.Kill()
	if errors.Is(err, os.ErrProcessDone) {
		return nil
	}
	return err
}

func killProcess(cmd *exec.Cmd) error { return terminateProcess(cmd) }
