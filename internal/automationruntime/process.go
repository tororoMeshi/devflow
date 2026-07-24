package automationruntime

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"time"
)

type processResult struct {
	stdout          []byte
	stderr          []byte
	stderrTruncated bool
	exitCode        *int
	startErr        error
	waitErr         error
	stdinErr        error
	stdoutErr       error
	stderrErr       error
	overflow        bool
	timedOut        bool
	cancelled       bool
	signaled        bool
}

func runCaptured(ctx context.Context, cwd, executable string, args []string, limit int) processResult {
	cmd := exec.CommandContext(ctx, executable, args...)
	cmd.Dir = cwd
	cmd.Stdin = nil
	out := newLimitedCapture(limit, func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	})
	errOut := newTailBuffer(MaxExecutorStderrTailBytes)
	cmd.Stdout, cmd.Stderr = out, errOut
	err := cmd.Run()
	data, overflow := out.snapshot()
	stderr, truncated := errOut.snapshot()
	result := processResult{stdout: data, stderr: stderr, overflow: overflow, stderrTruncated: truncated, waitErr: err}
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			code := exitErr.ExitCode()
			result.exitCode = &code
		} else {
			result.startErr = err
		}
	}
	return result
}

func runExecutor(ctx context.Context, cfg Config, input []byte) processResult {
	cmd := exec.Command(cfg.Executor, cfg.ExecutorArgs...)
	cmd.Dir = cfg.ProjectRoot
	cmd.Env = os.Environ()
	configureProcessGroup(cmd)
	stdinReader, stdinWriter, err := os.Pipe()
	if err != nil {
		return processResult{startErr: err}
	}
	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		_ = stdinReader.Close()
		_ = stdinWriter.Close()
		return processResult{startErr: err}
	}
	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		_ = stdinReader.Close()
		_ = stdinWriter.Close()
		_ = stdoutReader.Close()
		_ = stdoutWriter.Close()
		return processResult{startErr: err}
	}
	cmd.Stdin = stdinReader
	cmd.Stdout = stdoutWriter
	cmd.Stderr = stderrWriter
	if err := cmd.Start(); err != nil {
		_ = stdinReader.Close()
		_ = stdinWriter.Close()
		_ = stdoutReader.Close()
		_ = stdoutWriter.Close()
		_ = stderrReader.Close()
		_ = stderrWriter.Close()
		return processResult{startErr: err}
	}
	_ = stdinReader.Close()
	_ = stdoutWriter.Close()
	_ = stderrWriter.Close()

	overflowSignal := make(chan struct{}, 1)
	out := newLimitedCapture(MaxExecutorStdoutBytes, func() {
		select {
		case overflowSignal <- struct{}{}:
		default:
		}
	})
	errTail := newTailBuffer(MaxExecutorStderrTailBytes)
	stdinDone := make(chan error, 1)
	stdoutDone := make(chan error, 1)
	stderrDone := make(chan error, 1)
	waitDone := make(chan error, 1)
	go func() {
		_, writeErr := stdinWriter.Write(input)
		closeErr := stdinWriter.Close()
		if writeErr != nil {
			stdinDone <- writeErr
		} else {
			stdinDone <- closeErr
		}
	}()
	go func() {
		err := copyAll(out, stdoutReader)
		_ = stdoutReader.Close()
		stdoutDone <- err
	}()
	go func() {
		err := copyAll(errTail, stderrReader)
		_ = stderrReader.Close()
		stderrDone <- err
	}()
	go func() { waitDone <- cmd.Wait() }()

	var timeout <-chan time.Time
	var timer *time.Timer
	if cfg.Timeout > 0 {
		timer = time.NewTimer(cfg.Timeout)
		timeout = timer.C
		defer timer.Stop()
	}
	result := processResult{}
	select {
	case result.waitErr = <-waitDone:
		_ = killProcess(cmd)
	case <-ctx.Done():
		result.cancelled = true
		terminateAndWait(cmd, cfg.TerminateGrace, waitDone, &result)
	case <-timeout:
		result.timedOut = true
		terminateAndWait(cmd, cfg.TerminateGrace, waitDone, &result)
	case <-overflowSignal:
		select {
		case <-ctx.Done():
			result.cancelled = true
		default:
			select {
			case <-timeout:
				result.timedOut = true
			default:
				result.overflow = true
			}
		}
		terminateAndWait(cmd, cfg.TerminateGrace, waitDone, &result)
	}
	result.stdinErr = <-stdinDone
	result.stdoutErr = <-stdoutDone
	result.stderrErr = <-stderrDone
	result.stdout, result.overflow = out.snapshot()
	result.stderr, result.stderrTruncated = errTail.snapshot()
	if result.waitErr != nil {
		var exitErr *exec.ExitError
		if errors.As(result.waitErr, &exitErr) {
			code := exitErr.ExitCode()
			if code < 0 {
				result.signaled = true
			} else {
				result.exitCode = &code
			}
		}
	} else {
		code := 0
		result.exitCode = &code
	}
	if result.cancelled || result.timedOut || result.overflow {
		result.exitCode = nil
	}
	return result
}

func terminateAndWait(cmd *exec.Cmd, grace time.Duration, waitDone <-chan error, result *processResult) {
	_ = terminateProcess(cmd)
	if grace <= 0 {
		_ = killProcess(cmd)
		result.waitErr = <-waitDone
		return
	}
	timer := time.NewTimer(grace)
	defer timer.Stop()
	select {
	case result.waitErr = <-waitDone:
	case <-timer.C:
		_ = killProcess(cmd)
		result.waitErr = <-waitDone
	}
}

var _ io.Writer = (*limitedCapture)(nil)
